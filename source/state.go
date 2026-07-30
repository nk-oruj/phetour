package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

const publishFilePath = "./publish.xml"

type PublishState struct {
	Entries []PublishEntry
}

type PublishEntry struct {
	Source              string
	ID                  string
	Kind                string
	PublishedRenderHash string
	CurrentRenderHash   string
	PendingType         string
	Snapshot            *etree.Element
}

type PublishUpdate struct {
	Source string
	ID     string
	Entry  *PublishEntry
}

func loadPublishState() (PublishState, error) {
	state := PublishState{Entries: []PublishEntry{}}
	if _, err := os.Stat(publishFilePath); os.IsNotExist(err) {
		return state, nil
	} else if err != nil {
		return PublishState{}, fmt.Errorf("failed to access publish.xml: %w", err)
	}

	document := etree.NewDocument()
	if err := document.ReadFromFile(publishFilePath); err != nil {
		return PublishState{}, fmt.Errorf("failed to read publish.xml: %w", err)
	}
	root := document.Root()
	if root == nil || root.Tag != "publish" {
		return PublishState{}, fmt.Errorf("publish.xml must have a publish root element")
	}
	for _, element := range root.SelectElements("entry") {
		entry := PublishEntry{
			Source:              element.SelectAttrValue("source", ""),
			ID:                  element.SelectAttrValue("id", ""),
			Kind:                element.SelectAttrValue("kind", ""),
			PublishedRenderHash: element.SelectAttrValue("published-render-hash", ""),
			CurrentRenderHash:   element.SelectAttrValue("current-render-hash", ""),
			PendingType:         element.SelectAttrValue("pending-type", ""),
		}
		if entry.Source == "" || entry.ID == "" || (entry.Kind != "post" && entry.Kind != "tag") {
			return PublishState{}, fmt.Errorf("publish.xml contains an invalid entry")
		}
		if snapshot := element.SelectElement("document"); snapshot != nil {
			entry.Snapshot = snapshot.Copy()
		}
		if entry.PendingType != "" && entry.Snapshot == nil {
			return PublishState{}, fmt.Errorf("pending entry %s has no document snapshot", entry.ID)
		}
		state.Entries = append(state.Entries, entry)
	}
	return state, nil
}

func (state *PublishState) Save() error {
	document := etree.NewDocument()
	root := document.CreateElement("publish")
	sort.Slice(state.Entries, func(i, j int) bool {
		if state.Entries[i].Source == state.Entries[j].Source {
			return state.Entries[i].ID < state.Entries[j].ID
		}
		return state.Entries[i].Source < state.Entries[j].Source
	})
	for _, entry := range state.Entries {
		element := root.CreateElement("entry")
		element.CreateAttr("source", entry.Source)
		element.CreateAttr("id", entry.ID)
		element.CreateAttr("kind", entry.Kind)
		element.CreateAttr("published-render-hash", entry.PublishedRenderHash)
		element.CreateAttr("current-render-hash", entry.CurrentRenderHash)
		element.CreateAttr("pending-type", entry.PendingType)
		if entry.Snapshot != nil {
			element.AddChild(entry.Snapshot.Copy())
		}
	}
	document.Indent(4)
	if err := document.WriteToFile(publishFilePath); err != nil {
		return fmt.Errorf("failed to write publish.xml: %w", err)
	}
	return nil
}

func (state PublishState) find(source string, id string) (PublishEntry, bool) {
	for _, entry := range state.Entries {
		if entry.Source == source && entry.ID == id {
			return entry, true
		}
	}
	return PublishEntry{}, false
}

func applyPublishUpdates(state *PublishState, updates []PublishUpdate) {
	for _, update := range updates {
		index := -1
		for i, entry := range state.Entries {
			if entry.Source == update.Source && entry.ID == update.ID {
				index = i
				break
			}
		}
		if update.Entry == nil {
			if index >= 0 {
				state.Entries = append(state.Entries[:index], state.Entries[index+1:]...)
			}
			continue
		}
		if index >= 0 {
			state.Entries[index] = *update.Entry
		} else {
			state.Entries = append(state.Entries, *update.Entry)
		}
	}
}

func planDeployPublicationUpdates(config Config, plans []RsyncPlan) (map[string][]PublishUpdate, error) {
	updatesBySource := map[string][]PublishUpdate{}
	if len(config.RSS) == 0 {
		return updatesBySource, nil
	}
	state, err := loadPublishState()
	if err != nil {
		return nil, err
	}
	keylock, err := LoadKeylock()
	if err != nil {
		return nil, err
	}
	keys := map[int]string{}
	for _, key := range keylock.Keys {
		keys[key.ID] = key.Value
	}
	for _, plan := range plans {
		if !hasRSSSource(config.RSS, plan.Output.Name) {
			continue
		}
		updates, err := planOutputPublicationUpdates(state, keys, plan)
		if err != nil {
			return nil, err
		}
		updatesBySource[plan.Output.Name] = updates
	}
	return updatesBySource, nil
}

func hasRSSSource(publications []RSSPublication, source string) bool {
	for _, publication := range publications {
		if publication.Source == source {
			return true
		}
	}
	return false
}

func planOutputPublicationUpdates(state PublishState, keys map[int]string, plan RsyncPlan) ([]PublishUpdate, error) {
	updates := []PublishUpdate{}
	for _, line := range strings.Split(plan.Changes, "\n") {
		marker, relativePath, valid := parseRsyncChange(line)
		if !valid {
			continue
		}
		key, page := keyForOutputPage(relativePath, plan.Output.Name)
		if !page {
			continue
		}
		value, known := keys[key]
		if !known {
			continue
		}
		id := strings.ToLower(KeyIDToHex(key))
		kind := ""
		if strings.HasPrefix(value, "POST:") {
			kind = "post"
		} else if strings.HasPrefix(value, "TAG:") {
			kind = "tag"
		} else {
			continue
		}

		if marker == "*deleting" {
			updates = append(updates, PublishUpdate{Source: plan.Output.Name, ID: id})
			continue
		}
		if kind == "post" {
			update, applicable, err := planPostPublicationUpdate(state, plan.Output.Name, id, marker)
			if err != nil {
				return nil, err
			}
			if applicable {
				updates = append(updates, update)
			}
			continue
		}
		update, err := planTagPublicationUpdate(state, plan, id, marker)
		if err != nil {
			return nil, err
		}
		updates = append(updates, update)
	}
	return updates, nil
}

func parseRsyncChange(line string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], strings.TrimSpace(parts[1]), true
}

func keyForOutputPage(relativePath string, outputName string) (int, bool) {
	if !strings.HasSuffix(relativePath, "/index."+outputName) {
		return 0, false
	}
	directory := strings.TrimSuffix(relativePath, "/index."+outputName)
	if strings.Contains(directory, "/") || !strings.HasPrefix(directory, "0x") {
		return 0, false
	}
	key, err := strconv.ParseInt(strings.TrimPrefix(directory, "0x"), 16, 0)
	if err != nil {
		return 0, false
	}
	return int(key), true
}

func planPostPublicationUpdate(state PublishState, source string, id string, marker string) (PublishUpdate, bool, error) {
	entry, known := state.find(source, id)
	if !strings.Contains(marker, "+++++++++") && (!known || entry.PendingType != "new-post") {
		return PublishUpdate{}, false, nil
	}
	snapshot, hash, err := currentOutputSnapshot(source, id)
	if err != nil {
		return PublishUpdate{}, false, err
	}
	return PublishUpdate{Source: source, ID: id, Entry: &PublishEntry{
		Source: source, ID: id, Kind: "post", CurrentRenderHash: hash, PendingType: "new-post", Snapshot: snapshot,
	}}, true, nil
}

func planTagPublicationUpdate(state PublishState, plan RsyncPlan, id string, marker string) (PublishUpdate, error) {
	snapshot, currentHash, err := currentOutputSnapshot(plan.Output.Name, id)
	if err != nil {
		return PublishUpdate{}, err
	}
	entry, known := state.find(plan.Output.Name, id)
	if !known {
		publishedHash := ""
		if !strings.Contains(marker, "+++++++++") {
			remotePath := path.Join(plan.Output.RemotePath, id, "index."+plan.Output.Name)
			remote, exists, err := readRemoteFile(plan.Alias, remotePath)
			if err != nil {
				return PublishUpdate{}, err
			}
			if !exists {
				return PublishUpdate{}, fmt.Errorf("changed tag %s is missing from the remote output", id)
			}
			publishedHash = contentHash(remote)
		}
		entry = PublishEntry{Source: plan.Output.Name, ID: id, Kind: "tag", PublishedRenderHash: publishedHash}
	}
	if entry.PublishedRenderHash != "" && currentHash == entry.PublishedRenderHash {
		entry.CurrentRenderHash = currentHash
		entry.PendingType = ""
		entry.Snapshot = nil
	} else {
		entry.CurrentRenderHash = currentHash
		entry.PendingType = "updated-tag"
		if entry.PublishedRenderHash == "" {
			entry.PendingType = "new-tag"
		}
		entry.Snapshot = snapshot
	}
	return PublishUpdate{Source: plan.Output.Name, ID: id, Entry: &entry}, nil
}

func currentOutputSnapshot(source string, id string) (*etree.Element, string, error) {
	xmlPath := filepath.Join("output", "xml", id, "index.xml")
	document := etree.NewDocument()
	if err := document.ReadFromFile(xmlPath); err != nil {
		return nil, "", fmt.Errorf("failed to read generated document %s: %w", id, err)
	}
	if document.Root() == nil || document.Root().Tag != "document" {
		return nil, "", fmt.Errorf("generated document %s has no document root", id)
	}
	renderedPath := filepath.Join("output", source, id, "index."+source)
	rendered, err := os.ReadFile(renderedPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read generated output %s: %w", renderedPath, err)
	}
	return document.Root().Copy(), contentHash(rendered), nil
}

func contentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:])
}

func printPendingPublicationUpdates(updatesBySource map[string][]PublishUpdate) {
	for source, updates := range updatesBySource {
		for _, update := range updates {
			if update.Entry == nil || update.Entry.PendingType == "" {
				continue
			}
			fmt.Printf("RSS pending (%s): %s %s\n", source, update.Entry.PendingType, update.ID)
		}
	}
}
