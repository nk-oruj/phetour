package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

type RSSUploadPlan struct {
	Publication RSSPublication
	Output      DeploymentOutput
	Changes     string
}

func runPublish(force bool) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if len(config.RSS) == 0 {
		return fmt.Errorf("RSS publication is not configured")
	}
	state, err := loadPublishState()
	if err != nil {
		return err
	}
	pending := pendingPublishEntries(state)
	if len(pending) == 0 {
		fmt.Println("RSS: no pending publications.")
		return nil
	}

	plans := make([]RSSUploadPlan, 0, len(config.RSS))
	for _, publication := range config.RSS {
		events, err := renderRSSItems(publication, config.Site, pending)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			continue
		}
		added, err := writeRSSPublication(publication, config.Site, config.Deployment, config.Deployment.SSHAlias, events)
		if err != nil {
			return err
		}
		fmt.Printf("RSS output/%s/%s: %d item(s) ready\n", publication.Output, publication.Path, len(added))
		for _, item := range added {
			fmt.Println("  " + item.Title)
		}
		output, _ := findDeploymentOutput(config.Deployment, publication.Output)
		changes := ""
		directory := path.Dir(path.Join(output.RemotePath, publication.Path))
		exists, err := remoteDirectoryExists(config.Deployment.SSHAlias, directory)
		if err != nil {
			return err
		}
		if exists {
			changes, err = previewRSSUpload(config.Deployment.SSHAlias, output, publication.Path)
			if err != nil {
				return err
			}
		} else {
			changes = "mkdir -p " + directory + "\n+++++++++ " + publication.Path
		}
		plan := RSSUploadPlan{Publication: publication, Output: output, Changes: changes}
		plans = append(plans, plan)
		printRSSUploadPlan(config.Deployment.SSHAlias, plan)
	}

	proceed, err := confirmPlan(force)
	if err != nil {
		return err
	}
	if !proceed {
		fmt.Println("Cancelled.")
		return nil
	}
	for _, plan := range plans {
		if err := executeRSSUpload(config.Deployment.SSHAlias, plan); err != nil {
			return err
		}
	}
	publishedSources := map[string]bool{}
	for _, plan := range plans {
		publishedSources[plan.Publication.Source] = true
	}
	markPublished(&state, publishedSources)
	return state.Save()
}

func pendingPublishEntries(state PublishState) []PublishEntry {
	entries := []PublishEntry{}
	for _, entry := range state.Entries {
		if entry.PendingType != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func renderRSSItems(publication RSSPublication, site SiteConfig, entries []PublishEntry) ([]RSSItem, error) {
	items := []RSSItem{}
	for _, entry := range entries {
		if entry.Source != publication.Source {
			continue
		}
		item, err := renderRSSItem(publication.Stylesheet, site, entry)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func renderRSSItem(stylesheet string, site SiteConfig, entry PublishEntry) (RSSItem, error) {
	if entry.Snapshot == nil {
		return RSSItem{}, fmt.Errorf("pending entry %s has no document snapshot", entry.ID)
	}
	input := etree.NewDocument()
	root := input.CreateElement("rss-item")
	root.CreateAttr("type", entry.PendingType)
	root.CreateAttr("id", entry.ID)
	root.CreateAttr("site-url", strings.TrimRight(site.URL, "/"))
	root.CreateAttr("item-url", strings.TrimRight(site.URL, "/")+"/"+entry.ID+"/")
	root.AddChild(entry.Snapshot.Copy())

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
	return RSSItem{
		Title:       strings.TrimSpace(title.Text()),
		Link:        strings.TrimRight(site.URL, "/") + "/" + entry.ID + "/",
		GUID:        rssGUID(entry),
		Description: rssDescriptionMarkup(description),
		Categories:  rssCategories(result),
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
	return nil, fmt.Errorf("RSS stylesheet needs xsltproc: install it before running publish")
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

func rssCategories(result *etree.Element) []string {
	categories := []string{}
	for _, category := range result.SelectElements("category") {
		if value := strings.TrimSpace(category.Text()); value != "" {
			categories = append(categories, value)
		}
	}
	return categories
}

func rssGUID(entry PublishEntry) string {
	if entry.Kind == "post" {
		return "post:" + entry.ID
	}
	return "tag:" + entry.ID + ":" + entry.CurrentRenderHash
}

func previewRSSUpload(alias string, output DeploymentOutput, feedPath string) (string, error) {
	arguments := rssUploadArguments(alias, output, feedPath, true)
	result, err := runRsync(arguments...)
	if err != nil {
		return "", fmt.Errorf("failed to preview RSS upload output/%s/%s: %w", output.Name, feedPath, err)
	}
	return strings.TrimSpace(result), nil
}

func remoteDirectoryExists(alias string, remoteDirectory string) (bool, error) {
	command := "if test -d " + shellQuote(remoteDirectory) + "; then exit 0; else exit 3; fi"
	output, err := exec.Command("ssh", alias, command).CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 3 {
		return false, nil
	}
	return false, fmt.Errorf("failed to inspect RSS directory %s: %s", remoteDirectory, strings.TrimSpace(string(output)))
}

func executeRSSUpload(alias string, plan RSSUploadPlan) error {
	remoteDirectory := path.Dir(path.Join(plan.Output.RemotePath, plan.Publication.Path))
	command := "mkdir -p -- " + shellQuote(remoteDirectory)
	output, err := exec.Command("ssh", alias, command).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create RSS directory %s: %s", remoteDirectory, strings.TrimSpace(string(output)))
	}
	arguments := rssUploadArguments(alias, plan.Output, plan.Publication.Path, false)
	if _, err := runRsync(arguments...); err != nil {
		return fmt.Errorf("failed to publish RSS output/%s/%s: %w", plan.Output.Name, plan.Publication.Path, err)
	}
	return nil
}

func rssUploadArguments(alias string, output DeploymentOutput, feedPath string, dryRun bool) []string {
	arguments := []string{
		"--no-links",
		"--no-devices",
		"--no-specials",
		"--no-owner",
		"--no-group",
		"--no-times",
		"--no-perms",
		"--checksum",
		"--itemize-changes",
		"--out-format=%i %n%L",
	}
	if dryRun {
		arguments = append(arguments, "--dry-run")
	}
	localPath := filepath.Join("output", output.Name, filepath.FromSlash(feedPath))
	remotePath := path.Join(output.RemotePath, feedPath)
	return append(arguments, localPath, alias+":"+remotePath)
}

func printRSSUploadPlan(alias string, plan RSSUploadPlan) {
	fmt.Printf("Publish RSS output/%s/%s → %s:%s\n", plan.Output.Name, plan.Publication.Path, alias, path.Join(plan.Output.RemotePath, plan.Publication.Path))
	if plan.Changes == "" {
		fmt.Println("  No remote changes.")
		return
	}
	for _, line := range strings.Split(plan.Changes, "\n") {
		fmt.Println("  " + line)
	}
}

func markPublished(state *PublishState, publishedSources map[string]bool) {
	entries := state.Entries[:0]
	for _, entry := range state.Entries {
		if entry.PendingType == "" || !publishedSources[entry.Source] {
			entries = append(entries, entry)
			continue
		}
		if entry.Kind == "post" {
			continue
		}
		entry.PublishedRenderHash = entry.CurrentRenderHash
		entry.PendingType = ""
		entry.Snapshot = nil
		entries = append(entries, entry)
	}
	state.Entries = entries
}
