package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

type RSSItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
}

func buildRSSEvents(changes string, sourceOutput string, site SiteConfig, keylock *Keylock) ([]RSSItem, error) {
	keys := map[int]string{}
	for _, key := range keylock.Keys {
		keys[key.ID] = key.Value
	}

	events := []RSSItem{}
	for _, line := range strings.Split(changes, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) != 2 || strings.HasPrefix(parts[0], "*deleting") {
			continue
		}

		marker := parts[0]
		relativePath := strings.TrimSpace(parts[1])
		key, valid := keyForOutputPage(relativePath, sourceOutput)
		if !valid {
			continue
		}

		value, known := keys[key]
		if !known {
			continue
		}
		title, err := outputDocumentTitle(key)
		if err != nil {
			return nil, err
		}
		link := strings.TrimRight(site.URL, "/") + "/" + KeyIDToHex(key) + "/"
		if strings.HasPrefix(value, "POST:") && strings.Contains(marker, "+++++++++") {
			events = append(events, RSSItem{
				Title:       title,
				Link:        link,
				GUID:        "post:" + KeyIDToHex(key),
				Description: "A new post was published.",
			})
		}
		if strings.HasPrefix(value, "TAG:") {
			hash, err := outputDocumentHash(key)
			if err != nil {
				return nil, err
			}
			eventTitle := "Updated tag: " + title
			description := "The tag index was updated."
			if strings.Contains(marker, "+++++++++") {
				eventTitle = "New tag: " + title
				description = "A new tag was published."
			}
			events = append(events, RSSItem{
				Title:       eventTitle,
				Link:        link,
				GUID:        "tag:" + KeyIDToHex(key) + ":" + hash,
				Description: description,
			})
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].GUID < events[j].GUID })
	return events, nil
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

func outputDocumentTitle(key int) (string, error) {
	document := etree.NewDocument()
	filePath := filepath.Join("output", "xml", KeyIDToHex(key), "index.xml")
	if err := document.ReadFromFile(filePath); err != nil {
		return "", fmt.Errorf("failed to read generated document %s: %w", KeyIDToHex(key), err)
	}
	meta := document.Root().SelectElement("meta")
	if meta == nil {
		return "", fmt.Errorf("generated document %s has no meta element", KeyIDToHex(key))
	}
	title := meta.SelectElement("title")
	if title == nil {
		return "", fmt.Errorf("generated document %s has no title", KeyIDToHex(key))
	}
	return title.SelectAttrValue("value", ""), nil
}

func outputDocumentHash(key int) (string, error) {
	filePath := filepath.Join("output", "xml", KeyIDToHex(key), "index.xml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read generated document %s: %w", KeyIDToHex(key), err)
	}
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:]), nil
}

func writeRSSPublication(publication RSSPublication, site SiteConfig, deployment DeploymentConfig, alias string, events []RSSItem) ([]RSSItem, error) {
	output, found := findDeploymentOutput(deployment, publication.Output)
	if !found {
		return nil, fmt.Errorf("RSS publication output %q is not configured", publication.Output)
	}
	remotePath := path.Join(output.RemotePath, publication.Path)
	remoteContent, exists, err := readRemoteFile(alias, remotePath)
	if err != nil {
		return nil, err
	}

	feed, err := readRSSFeed(remoteContent, exists, site)
	if err != nil {
		return nil, err
	}
	added := appendRSSItems(feed, events)
	localPath := filepath.Join("output", publication.Output, filepath.FromSlash(publication.Path))
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create RSS output directory: %w", err)
	}
	if len(added) == 0 && exists {
		if err := os.WriteFile(localPath, remoteContent, 0644); err != nil {
			return nil, fmt.Errorf("failed to preserve RSS feed: %w", err)
		}
		return nil, nil
	}

	feed.Indent(4)
	if err := feed.WriteToFile(localPath); err != nil {
		return nil, fmt.Errorf("failed to write RSS feed: %w", err)
	}
	return added, nil
}

func readRemoteFile(alias string, remotePath string) ([]byte, bool, error) {
	command := "if test -f " + shellQuote(remotePath) + "; then cat -- " + shellQuote(remotePath) + "; else exit 3; fi"
	output, err := exec.Command("ssh", alias, command).CombinedOutput()
	if err == nil {
		return output, true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 3 {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("failed to read remote RSS feed: %s", strings.TrimSpace(string(output)))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func readRSSFeed(content []byte, exists bool, site SiteConfig) (*etree.Document, error) {
	if !exists {
		document := etree.NewDocument()
		rss := document.CreateElement("rss")
		rss.CreateAttr("version", "2.0")
		channel := rss.CreateElement("channel")
		channel.CreateElement("title").CreateText(site.Title)
		channel.CreateElement("link").CreateText(site.URL)
		channel.CreateElement("description").CreateText(site.Description)
		return document, nil
	}

	document := etree.NewDocument()
	if err := document.ReadFromBytes(content); err != nil {
		return nil, fmt.Errorf("remote RSS feed is invalid XML: %w", err)
	}
	if document.Root() == nil || document.Root().Tag != "rss" || document.Root().SelectElement("channel") == nil {
		return nil, fmt.Errorf("remote RSS feed is not an RSS 2.0 document")
	}
	return document, nil
}

func appendRSSItems(feed *etree.Document, events []RSSItem) []RSSItem {
	channel := feed.Root().SelectElement("channel")
	knownGUIDs := map[string]bool{}
	for _, item := range channel.SelectElements("item") {
		guid := item.SelectElement("guid")
		if guid != nil {
			knownGUIDs[guid.Text()] = true
		}
	}

	added := []RSSItem{}
	for _, event := range events {
		if knownGUIDs[event.GUID] {
			continue
		}
		item := channel.CreateElement("item")
		item.CreateElement("title").CreateText(event.Title)
		item.CreateElement("link").CreateText(event.Link)
		guid := item.CreateElement("guid")
		guid.CreateAttr("isPermaLink", "false")
		guid.CreateText(event.GUID)
		item.CreateElement("pubDate").CreateText(time.Now().UTC().Format(time.RFC1123Z))
		item.CreateElement("description").CreateText(event.Description)
		knownGUIDs[event.GUID] = true
		added = append(added, event)
	}
	if len(added) > 0 {
		channel.CreateElement("lastBuildDate").CreateText(time.Now().UTC().Format(time.RFC1123Z))
	}
	return added
}
