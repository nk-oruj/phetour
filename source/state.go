package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
}

type LibraryMember struct {
	ID    string
	Title string
}

type LibrarySnapshot struct {
	Pages []LibraryPage
}

type LibraryState struct {
	Published LibrarySnapshot
	Deployed  LibrarySnapshot
	History   []*etree.Element
}

func loadState() (LibraryState, error) {
	state := LibraryState{Published: LibrarySnapshot{}, Deployed: LibrarySnapshot{}, History: []*etree.Element{}}
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
		for _, update := range history.SelectElements("library-update") {
			state.History = append(state.History, update.Copy())
		}
	}
	return state, nil
}

func (state LibraryState) Save() error {
	document := etree.NewDocument()
	root := document.CreateElement("state")
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
		snapshot.Pages = append(snapshot.Pages, page)
	}
	return snapshot
}

func contentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:])
}

func buildLibraryUpdate(published LibrarySnapshot, deployed LibrarySnapshot) *etree.Element {
	update := etree.NewElement("library-update")
	created := update.CreateElement("created")
	revised := update.CreateElement("revised")
	deleted := update.CreateElement("deleted")
	publishedPages := pagesByID(published)
	deployedPages := pagesByID(deployed)
	for _, page := range deployed.Pages {
		previous, exists := publishedPages[page.ID]
		if !exists {
			writePageChange(created, page, true)
		} else if previous.Fingerprint != page.Fingerprint {
			writePageChange(revised, page, true)
		}
	}
	for _, page := range published.Pages {
		if _, exists := deployedPages[page.ID]; !exists {
			writePageChange(deleted, page, false)
		}
	}
	removeEmptyChangeContainers(update)
	return update
}

func pagesByID(snapshot LibrarySnapshot) map[string]LibraryPage {
	pages := map[string]LibraryPage{}
	for _, page := range snapshot.Pages {
		pages[page.ID] = page
	}
	return pages
}

func writePageChange(parent *etree.Element, page LibraryPage, includeMembers bool) *etree.Element {
	element := parent.CreateElement(page.Kind)
	element.CreateAttr("id", page.ID)
	element.CreateAttr("title", page.Title)
	if includeMembers {
		for _, member := range page.Members {
			memberElement := element.CreateElement("member")
			memberElement.CreateAttr("id", member.ID)
			memberElement.CreateAttr("title", member.Title)
		}
	}
	return element
}

func removeEmptyChangeContainers(parent *etree.Element) {
	for _, child := range parent.ChildElements() {
		if len(child.ChildElements()) == 0 {
			parent.RemoveChild(child)
		}
	}
}

func hasLibraryChanges(update *etree.Element) bool {
	return len(update.FindElements("created/*")) > 0 || len(update.FindElements("revised/*")) > 0 || len(update.FindElements("deleted/*")) > 0
}
