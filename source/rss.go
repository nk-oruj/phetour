package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"
)

type RSSItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	Categories  []string
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
	return nil, false, fmt.Errorf("failed to read remote file %s: %s", remotePath, strings.TrimSpace(string(output)))
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
		item.CreateElement("description").CreateCData(event.Description)
		for _, category := range event.Categories {
			item.CreateElement("category").CreateText(category)
		}
		knownGUIDs[event.GUID] = true
		added = append(added, event)
	}
	if len(added) > 0 {
		channel.CreateElement("lastBuildDate").CreateText(time.Now().UTC().Format(time.RFC1123Z))
	}
	return added
}
