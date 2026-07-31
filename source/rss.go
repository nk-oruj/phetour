package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"
)

type RSSItem struct {
	Title       string
	Link        string
	GUID        string
	PublishedAt time.Time
	Description string
}

func renderLibraryUpdate(stylesheet string, site SiteConfig, update *etree.Element) (RSSItem, error) {
	input := etree.NewDocumentWithRoot(update.Copy())
	input.Root().CreateAttr("site-url", strings.TrimRight(site.URL, "/"))
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
	publishedAt, err := time.Parse(time.RFC3339, update.SelectAttrValue("published-at", ""))
	if err != nil {
		return RSSItem{}, fmt.Errorf("library update has an invalid publication date: %w", err)
	}
	return RSSItem{
		Title:       strings.TrimSpace(title.Text()),
		Link:        site.URL,
		GUID:        update.SelectAttrValue("guid", ""),
		PublishedAt: publishedAt,
		Description: rssDescriptionMarkup(description),
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
	return nil, fmt.Errorf("RSS stylesheet needs xsltproc: install it before publishing RSS")
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
		element.CreateElement("pubDate").CreateText(item.PublishedAt.UTC().Format(time.RFC1123Z))
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
