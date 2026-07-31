package main

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

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
	state, err := loadState()
	if err != nil {
		return err
	}
	update := buildLibraryUpdate(state.Published, state.Deployed)
	if !hasLibraryChanges(update) {
		fmt.Println("RSS: no unpublished library changes.")
		return nil
	}
	now := time.Now().UTC()
	update.CreateAttr("guid", contentHash(libraryUpdateContent(update)))
	update.CreateAttr("published-at", now.Format(time.RFC3339))

	candidateHistory := append(append([]*etree.Element{}, state.History...), update)
	plans := []RSSUploadPlan{}
	for _, publication := range config.RSS {
		items, err := renderRSSHistory(publication, config.Site, candidateHistory, config.RSSEntryLimit, config.RSSMemberLimit)
		if err != nil {
			return err
		}
		if err := writeRSSFeed(publication, config.Site, items); err != nil {
			return err
		}
		output, _ := findDeploymentOutput(config.Deployment, publication.Output)
		remoteDirectory := path.Dir(path.Join(output.RemotePath, publication.Path))
		exists, err := remoteDirectoryExists(config.Deployment.SSHAlias, remoteDirectory)
		if err != nil {
			return err
		}
		changes := ""
		if exists {
			changes, err = previewRSSUpload(config.Deployment.SSHAlias, output, publication.Path)
			if err != nil {
				return err
			}
		} else {
			changes = "mkdir -p " + remoteDirectory + "\n+++++++++ " + publication.Path
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
	state.Published = state.Deployed
	state.History = candidateHistory
	return state.Save()
}

func libraryUpdateContent(update *etree.Element) []byte {
	copy := update.Copy()
	copy.RemoveAttr("guid")
	copy.RemoveAttr("published-at")
	document := etree.NewDocumentWithRoot(copy)
	content, _ := document.WriteToBytes()
	return content
}

func renderRSSHistory(publication RSSPublication, site SiteConfig, history []*etree.Element, entryLimit int, memberLimit int) ([]RSSItem, error) {
	items := []RSSItem{}
	first := len(history) - 1
	last := first - entryLimit
	for index := first; index > last && index >= 0; index-- {
		item, err := renderLibraryUpdate(publication.Stylesheet, site, history[index], memberLimit)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func previewRSSUpload(alias string, output DeploymentOutput, feedPath string) (string, error) {
	result, err := runRsync(rssUploadArguments(alias, output, feedPath, true)...)
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
	if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 3 {
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
	if _, err := runRsync(rssUploadArguments(alias, plan.Output, plan.Publication.Path, false)...); err != nil {
		return fmt.Errorf("failed to publish RSS output/%s/%s: %w", plan.Output.Name, plan.Publication.Path, err)
	}
	return nil
}

func rssUploadArguments(alias string, output DeploymentOutput, feedPath string, dryRun bool) []string {
	arguments := []string{
		"--no-links", "--no-devices", "--no-specials", "--no-owner", "--no-group", "--no-times", "--no-perms",
		"--checksum", "--itemize-changes", "--out-format=%i %n%L",
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
