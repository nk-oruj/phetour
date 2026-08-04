package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

const stateFilePath = "./state.xml"

type LibraryPage struct {
	ID          string
	Kind        string
	Title       string
	Fingerprint string
	Members     []LibraryMember
	Body        *etree.Element
}

type LibraryMember struct {
	ID    string
	Title string
}

type LibrarySnapshot struct {
	Pages []LibraryPage
}

type LibraryState struct {
	Published         LibrarySnapshot
	Deployed          LibrarySnapshot
	History           []*etree.Element
	NextPublicationID int
}

func stateFileExists() (bool, error) {
	if _, err := os.Stat(stateFilePath); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, fmt.Errorf("failed to access state.xml: %w", err)
	}
}

func initialLibraryState(snapshot LibrarySnapshot) LibraryState {
	return LibraryState{
		Published:         snapshot,
		Deployed:          snapshot,
		History:           []*etree.Element{},
		NextPublicationID: 1,
	}
}

func initializeStateFromBuiltOutput() (LibraryState, error) {
	if _, err := os.Stat(filepath.Join("output", "xml")); os.IsNotExist(err) {
		return LibraryState{}, fmt.Errorf("output/xml does not exist: run phetour build first")
	} else if err != nil {
		return LibraryState{}, fmt.Errorf("failed to access output/xml: %w", err)
	}
	keylock, err := LoadKeylock()
	if err != nil {
		return LibraryState{}, err
	}
	snapshot, err := buildLibrarySnapshot(keylock)
	if err != nil {
		return LibraryState{}, err
	}
	state := initialLibraryState(snapshot)
	if err := state.Save(); err != nil {
		return LibraryState{}, err
	}
	return state, nil
}

func loadState() (LibraryState, error) {
	state := LibraryState{Published: LibrarySnapshot{}, Deployed: LibrarySnapshot{}, History: []*etree.Element{}, NextPublicationID: 1}
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		return state, nil
	} else if err != nil {
		return LibraryState{}, fmt.Errorf("failed to access state.xml: %w", err)
	}
	document := etree.NewDocument()
	if err := document.ReadFromFile(stateFilePath); err != nil {
		return LibraryState{}, fmt.Errorf("failed to read state.xml: %w", err)
	}
	root := document.Root()
	if root == nil || root.Tag != "state" {
		return LibraryState{}, fmt.Errorf("state.xml must have a state root element")
	}
	if published := root.SelectElement("published"); published != nil {
		state.Published = readLibrarySnapshot(published)
	}
	if deployed := root.SelectElement("deployed"); deployed != nil {
		state.Deployed = readLibrarySnapshot(deployed)
	}
	if history := root.SelectElement("history"); history != nil {
		for _, publication := range history.SelectElements("publication") {
			state.History = append(state.History, publication.Copy())
		}
	}
	if nextPublicationID, err := strconv.Atoi(root.SelectAttrValue("next-publication-id", "1")); err == nil && nextPublicationID > 0 {
		state.NextPublicationID = nextPublicationID
	}
	return state, nil
}

func (state LibraryState) Save() error {
	document := etree.NewDocument()
	root := document.CreateElement("state")
	root.CreateAttr("next-publication-id", strconv.Itoa(state.NextPublicationID))
	writeLibrarySnapshot(root.CreateElement("published"), state.Published)
	writeLibrarySnapshot(root.CreateElement("deployed"), state.Deployed)
	history := root.CreateElement("history")
	for _, update := range state.History {
		history.AddChild(update.Copy())
	}
	document.Indent(4)
	if err := document.WriteToFile(stateFilePath); err != nil {
		return fmt.Errorf("failed to write state.xml: %w", err)
	}
	return nil
}

func buildLibrarySnapshot(keylock *Keylock) (LibrarySnapshot, error) {
	snapshot := LibrarySnapshot{Pages: []LibraryPage{}}
	for _, key := range keylock.Keys {
		kind := libraryPageKind(key.Value)
		if kind == "" {
			continue
		}
		id := KeyIDToHex(key.ID)
		filePath := filepath.Join("output", "xml", id, "index.xml")
		content, err := os.ReadFile(filePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return LibrarySnapshot{}, fmt.Errorf("failed to read generated document %s: %w", id, err)
		}
		document := etree.NewDocument()
		if err := document.ReadFromBytes(content); err != nil {
			return LibrarySnapshot{}, fmt.Errorf("failed to parse generated document %s: %w", id, err)
		}
		meta := document.Root().SelectElement("meta")
		title := ""
		if meta != nil {
			if titleElement := meta.SelectElement("title"); titleElement != nil {
				title = titleElement.SelectAttrValue("value", "")
			}
		}
		page := LibraryPage{ID: id, Kind: kind, Title: title, Fingerprint: contentHash(content), Members: pageMembers(document.Root(), kind)}
		if body := document.Root().SelectElement("body"); body != nil {
			page.Body = body.Copy()
		}
		snapshot.Pages = append(snapshot.Pages, page)
	}
	sort.Slice(snapshot.Pages, func(i, j int) bool { return snapshot.Pages[i].ID < snapshot.Pages[j].ID })
	return snapshot, nil
}

func libraryPageKind(keyValue string) string {
	if strings.HasPrefix(keyValue, "POST:") {
		return "post"
	}
	if strings.HasPrefix(keyValue, "TAG:") {
		return "tag"
	}
	return ""
}

func pageMembers(document *etree.Element, kind string) []LibraryMember {
	members := []LibraryMember{}
	if kind == "post" {
		meta := document.SelectElement("meta")
		if meta == nil {
			return members
		}
		for _, tag := range meta.SelectElements("tag") {
			members = append(members, LibraryMember{ID: tag.SelectAttrValue("id", ""), Title: tag.SelectAttrValue("label", "")})
		}
		return members
	}
	body := document.SelectElement("body")
	if body == nil {
		return members
	}
	for _, link := range body.SelectElements("link") {
		href := strings.Trim(link.SelectAttrValue("href", ""), "/")
		if strings.HasPrefix(href, "0x") {
			title := strings.TrimPrefix(link.Text(), href+" - ")
			members = append(members, LibraryMember{ID: href, Title: title})
		}
	}
	return members
}

func writeLibrarySnapshot(parent *etree.Element, snapshot LibrarySnapshot) {
	for _, page := range snapshot.Pages {
		element := parent.CreateElement("page")
		element.CreateAttr("id", page.ID)
		element.CreateAttr("kind", page.Kind)
		element.CreateAttr("title", page.Title)
		element.CreateAttr("fingerprint", page.Fingerprint)
		for _, member := range page.Members {
			memberElement := element.CreateElement("member")
			memberElement.CreateAttr("id", member.ID)
			memberElement.CreateAttr("title", member.Title)
		}
		if page.Body != nil {
			element.AddChild(page.Body.Copy())
		}
	}
}

func readLibrarySnapshot(parent *etree.Element) LibrarySnapshot {
	snapshot := LibrarySnapshot{Pages: []LibraryPage{}}
	for _, element := range parent.SelectElements("page") {
		page := LibraryPage{
			ID:          element.SelectAttrValue("id", ""),
			Kind:        element.SelectAttrValue("kind", ""),
			Title:       element.SelectAttrValue("title", ""),
			Fingerprint: element.SelectAttrValue("fingerprint", ""),
		}
		for _, member := range element.SelectElements("member") {
			page.Members = append(page.Members, LibraryMember{ID: member.SelectAttrValue("id", ""), Title: member.SelectAttrValue("title", "")})
		}
		if body := element.SelectElement("body"); body != nil {
			page.Body = body.Copy()
		}
		snapshot.Pages = append(snapshot.Pages, page)
	}
	return snapshot
}

func contentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:])
}

type LibraryChange struct {
	Page           LibraryPage
	Previous       *LibraryPage
	Status         string
	AddedMembers   []LibraryMember
	RemovedMembers []LibraryMember
}

func pendingLibraryChanges(published LibrarySnapshot, deployed LibrarySnapshot) []LibraryChange {
	changes := []LibraryChange{}
	publishedPages := pagesByID(published)
	for _, page := range deployed.Pages {
		previous, exists := publishedPages[page.ID]
		if !exists {
			changes = append(changes, LibraryChange{Page: page, Status: "created", AddedMembers: page.Members})
		} else if previous.Fingerprint != page.Fingerprint {
			previousCopy := previous
			changes = append(changes, LibraryChange{
				Page:           page,
				Previous:       &previousCopy,
				Status:         "revised",
				AddedMembers:   addedMembers(previous.Members, page.Members),
				RemovedMembers: addedMembers(page.Members, previous.Members),
			})
		}
	}
	return changes
}

func pagesByID(snapshot LibrarySnapshot) map[string]LibraryPage {
	pages := map[string]LibraryPage{}
	for _, page := range snapshot.Pages {
		pages[page.ID] = page
	}
	return pages
}

func addedMembers(previous []LibraryMember, current []LibraryMember) []LibraryMember {
	previousByID := map[string]LibraryMember{}
	for _, member := range previous {
		previousByID[member.ID] = member
	}
	added := []LibraryMember{}
	for _, member := range current {
		if _, exists := previousByID[member.ID]; !exists {
			added = append(added, member)
		}
	}
	return added
}

func buildPublication(change LibraryChange, publicationID int, publishedAt string) *etree.Element {
	publication := etree.NewElement("publication")
	publication.CreateAttr("guid", publicationGUID(publicationID, change.Page, publishedAt))
	publication.CreateAttr("published-at", publishedAt)
	page := publication.CreateElement(change.Page.Kind)
	page.CreateAttr("id", change.Page.ID)
	page.CreateAttr("title", change.Page.Title)
	page.CreateAttr("status", publicationStatus(change.Status))
	for _, member := range change.Page.Members {
		memberElement := page.CreateElement("member")
		memberElement.CreateAttr("id", member.ID)
		memberElement.CreateAttr("title", member.Title)
	}
	for _, member := range change.AddedMembers {
		memberElement := page.CreateElement("added-member")
		memberElement.CreateAttr("id", member.ID)
		memberElement.CreateAttr("title", member.Title)
	}
	for _, member := range change.RemovedMembers {
		memberElement := page.CreateElement("removed-member")
		memberElement.CreateAttr("id", member.ID)
		memberElement.CreateAttr("title", member.Title)
	}
	if change.Page.Body != nil {
		page.AddChild(change.Page.Body.Copy())
	}
	return publication
}

func publicationGUID(publicationID int, page LibraryPage, publishedAt string) string {
	seed := fmt.Sprintf("%d:%s:%s:%s", publicationID, page.ID, page.Fingerprint, publishedAt)
	return contentHash([]byte(seed))
}

func publicationStatus(status string) string {
	if status == "created" {
		return "Created"
	}
	return "Revised"
}

func advancePublishedPages(published LibrarySnapshot, deployed LibrarySnapshot, selected []LibraryChange) LibrarySnapshot {
	pages := pagesByID(published)
	deployedPages := pagesByID(deployed)
	for _, change := range selected {
		if page, exists := deployedPages[change.Page.ID]; exists {
			pages[page.ID] = page
		}
	}
	result := LibrarySnapshot{Pages: []LibraryPage{}}
	for _, page := range pages {
		result.Pages = append(result.Pages, page)
	}
	sort.Slice(result.Pages, func(i, j int) bool { return result.Pages[i].ID < result.Pages[j].ID })
	return result
}
