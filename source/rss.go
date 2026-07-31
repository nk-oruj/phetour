package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

type RSSItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
}

func buildRSSPublications(config Config, keylock *Keylock) error {
	for _, publication := range config.RSS {
		items, err := buildRSSItems(publication, config.Site, keylock)
		if err != nil {
			return err
		}
		if err := writeRSSFeed(publication, config.Site, items); err != nil {
			return err
		}
	}
	return nil
}

func buildRSSItems(publication RSSPublication, site SiteConfig, keylock *Keylock) ([]RSSItem, error) {
	items := []RSSItem{}
	for index := len(keylock.Keys) - 1; index >= 0; index-- {
		key := keylock.Keys[index]
		kind := rssDocumentKind(key.Value)
		if kind == "" {
			continue
		}
		id := KeyIDToHex(key.ID)
		document, exists, err := generatedRSSDocument(id)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		item, err := renderRSSItem(publication.Stylesheet, site, kind, id, document)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func rssDocumentKind(keyValue string) string {
	if strings.HasPrefix(keyValue, "POST:") {
		return "post"
	}
	if strings.HasPrefix(keyValue, "TAG:") {
		return "tag"
	}
	return ""
}

func generatedRSSDocument(id string) (*etree.Element, bool, error) {
	document := etree.NewDocument()
	filePath := filepath.Join("output", "xml", id, "index.xml")
	if err := document.ReadFromFile(filePath); os.IsNotExist(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("failed to read generated document %s: %w", id, err)
	}
	if document.Root() == nil || document.Root().Tag != "document" {
		return nil, false, fmt.Errorf("generated document %s has no document root", id)
	}
	return document.Root(), true, nil
}

func renderRSSItem(stylesheet string, site SiteConfig, kind string, id string, source *etree.Element) (RSSItem, error) {
	input := etree.NewDocument()
	root := input.CreateElement("rss-item")
	root.CreateAttr("type", kind)
	root.CreateAttr("id", id)
	root.CreateAttr("site-url", strings.TrimRight(site.URL, "/"))
	root.CreateAttr("item-url", strings.TrimRight(site.URL, "/")+"/"+id+"/")
	root.AddChild(source.Copy())

	temporaryFile, err := os.CreateTemp("", "phetour-rss-*.xml")
	if err != nil {
		return RSSItem{}, fmt.Errorf("failed to create RSS stylesheet input: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if _, err := input.WriteTo(temporaryFile); err != nil {
		temporaryFile.Close()
		return RSSItem{}, fmt.Errorf("failed to write RSS stylesheet input: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return RSSItem{}, fmt.Errorf("failed to close RSS stylesheet input: %w", err)
	}

	content, err := transformRSSWithXsltproc(stylesheet, temporaryPath)
	if err != nil {
		return RSSItem{}, err
	}
	document := etree.NewDocument()
	if err := document.ReadFromBytes(content); err != nil {
		return RSSItem{}, fmt.Errorf("RSS stylesheet %s produced invalid XML: %w", stylesheet, err)
	}
	result := document.Root()
	if result == nil || result.Tag != "rss-content" {
		return RSSItem{}, fmt.Errorf("RSS stylesheet %s must produce an rss-content root element", stylesheet)
	}
	title := result.SelectElement("title")
	description := result.SelectElement("description")
	if title == nil || strings.TrimSpace(title.Text()) == "" || description == nil {
		return RSSItem{}, fmt.Errorf("RSS stylesheet %s must produce title and description elements", stylesheet)
	}
	descriptionMarkup := rssDescriptionMarkup(description)
	return RSSItem{
		Title:       strings.TrimSpace(title.Text()),
		Link:        strings.TrimRight(site.URL, "/") + "/" + id + "/",
		GUID:        rssGUID(descriptionMarkup),
		Description: descriptionMarkup,
	}, nil
}

func transformRSSWithXsltproc(stylesheet string, inputPath string) ([]byte, error) {
	if _, err := os.Stat(stylesheet); err != nil {
		return nil, fmt.Errorf("failed to access RSS stylesheet %s: %w", stylesheet, err)
	}
	output, err := exec.Command("xsltproc", stylesheet, inputPath).CombinedOutput()
	if err == nil {
		return output, nil
	}
	if !errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("RSS stylesheet %s failed: %s", stylesheet, strings.TrimSpace(string(output)))
	}
	return nil, fmt.Errorf("RSS stylesheet needs xsltproc: install it before building RSS")
}

func rssDescriptionMarkup(description *etree.Element) string {
	var builder strings.Builder
	for _, child := range description.Child {
		switch child := child.(type) {
		case *etree.Element:
			document := etree.NewDocumentWithRoot(child.Copy())
			content, err := document.WriteToString()
			if err == nil {
				builder.WriteString(content)
			}
		case *etree.CharData:
			builder.WriteString(string(child.Data))
		}
	}
	return strings.TrimSpace(builder.String())
}

func rssGUID(description string) string {
	hash := sha256.Sum256([]byte(description))
	return fmt.Sprintf("%x", hash[:])
}

func writeRSSFeed(publication RSSPublication, site SiteConfig, items []RSSItem) error {
	document := etree.NewDocument()
	rss := document.CreateElement("rss")
	rss.CreateAttr("version", "2.0")
	channel := rss.CreateElement("channel")
	channel.CreateElement("title").CreateText(site.Title)
	channel.CreateElement("link").CreateText(site.URL)
	channel.CreateElement("description").CreateText(site.Description)
	for _, item := range items {
		element := channel.CreateElement("item")
		element.CreateElement("title").CreateText(item.Title)
		element.CreateElement("link").CreateText(item.Link)
		guid := element.CreateElement("guid")
		guid.CreateAttr("isPermaLink", "false")
		guid.CreateText(item.GUID)
		element.CreateElement("description").CreateCData(item.Description)
	}

	localPath := filepath.Join("output", publication.Output, filepath.FromSlash(publication.Path))
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create RSS output directory: %w", err)
	}
	document.Indent(4)
	if err := document.WriteToFile(localPath); err != nil {
		return fmt.Errorf("failed to write RSS feed: %w", err)
	}
	return nil
}
