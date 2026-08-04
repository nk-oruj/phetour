package main

import (
	"bufio"
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
	stateExists, err := stateFileExists()
	if err != nil {
		return err
	}
	if !stateExists {
		if _, err := initializeStateFromBuiltOutput(); err != nil {
			return err
		}
		fmt.Println("RSS: initialized state from the current build. Future changes are now tracked.")
		return nil
	}
	state, err := loadState()
	if err != nil {
		return err
	}
	changes := pendingLibraryChanges(state.Published, state.Deployed)
	posts := filterChangesByKind(changes, "post")
	if len(posts) == 0 {
		fmt.Println("RSS: no changed posts are waiting to publish.")
		return nil
	}
	selectedPosts, err := selectPostsToPublish(posts, force)
	if err != nil {
		return err
	}
	if len(selectedPosts) == 0 {
		fmt.Println("Cancelled.")
		return nil
	}
	selectedTags, deferredTags := selectAffectedTags(selectedPosts, changes)
	printPublicationSelection(selectedPosts, selectedTags, deferredTags)
	selected := append(append([]LibraryChange{}, selectedTags...), selectedPosts...)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	nextPublicationID := state.NextPublicationID
	publications := []*etree.Element{}
	for _, change := range selected {
		publications = append(publications, buildPublication(change, nextPublicationID, now))
		nextPublicationID++
	}
	candidateHistory := append(append([]*etree.Element{}, state.History...), publications...)
	plans := []RSSUploadPlan{}
	for _, publication := range config.RSS {
		items, err := renderRSSHistory(publication, config.Site, candidateHistory, config.RSSEntryLimit)
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
	state.Published = advancePublishedPages(state.Published, state.Deployed, selected)
	state.History = candidateHistory
	state.NextPublicationID = nextPublicationID
	return state.Save()
}

func filterChangesByKind(changes []LibraryChange, kind string) []LibraryChange {
	filtered := []LibraryChange{}
	for _, change := range changes {
		if change.Page.Kind == kind {
			filtered = append(filtered, change)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Page.ID < filtered[j].Page.ID })
	return filtered
}

func selectPostsToPublish(posts []LibraryChange, force bool) ([]LibraryChange, error) {
	if force {
		return posts, nil
	}
	fmt.Println("Changed posts:")
	for index, change := range posts {
		fmt.Printf("  %d. %s: [%s] %s\n", index+1, publicationStatus(change.Status), change.Page.ID, change.Page.Title)
		printPostTagEffects(change)
	}
	fmt.Print("Select posts to publish (numbers, 'all', or blank to cancel): ")
	response, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(response) == 0 {
		return nil, fmt.Errorf("failed to read post selection: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" {
		return nil, nil
	}
	if response == "all" {
		return posts, nil
	}
	selectedIndexes := map[int]bool{}
	for _, value := range strings.FieldsFunc(response, func(character rune) bool { return character == ',' || character == ' ' || character == '\t' }) {
		index, err := strconv.Atoi(value)
		if err != nil || index < 1 || index > len(posts) {
			return nil, fmt.Errorf("invalid post selection %q", value)
		}
		selectedIndexes[index-1] = true
	}
	selected := []LibraryChange{}
	for index, post := range posts {
		if selectedIndexes[index] {
			selected = append(selected, post)
		}
	}
	return selected, nil
}

func printPostTagEffects(change LibraryChange) {
	for _, member := range change.AddedMembers {
		fmt.Printf("     Tag catalog: [%s] %s gains this post\n", member.ID, member.Title)
	}
	for _, member := range change.RemovedMembers {
		fmt.Printf("     Tag catalog: [%s] %s loses this post\n", member.ID, member.Title)
	}
}

func selectAffectedTags(selectedPosts []LibraryChange, changes []LibraryChange) ([]LibraryChange, []LibraryChange) {
	affectedTagIDs := map[string]bool{}
	for _, change := range selectedPosts {
		for _, member := range change.AddedMembers {
			affectedTagIDs[member.ID] = true
		}
		for _, member := range change.RemovedMembers {
			affectedTagIDs[member.ID] = true
		}
	}
	selectedPostIDs := map[string]bool{}
	for _, change := range selectedPosts {
		selectedPostIDs[change.Page.ID] = true
	}
	tags := []LibraryChange{}
	deferred := []LibraryChange{}
	for _, tag := range filterChangesByKind(changes, "tag") {
		if !affectedTagIDs[tag.Page.ID] {
			continue
		}
		if tagHasUnselectedPostChange(tag.Page.ID, changes, selectedPostIDs) {
			deferred = append(deferred, tag)
		} else {
			tags = append(tags, tag)
		}
	}
	return tags, deferred
}

func tagHasUnselectedPostChange(tagID string, changes []LibraryChange, selectedPostIDs map[string]bool) bool {
	for _, change := range filterChangesByKind(changes, "post") {
		if selectedPostIDs[change.Page.ID] || !changeAffectsTag(change, tagID) {
			continue
		}
		return true
	}
	return false
}

func changeAffectsTag(change LibraryChange, tagID string) bool {
	for _, member := range change.AddedMembers {
		if member.ID == tagID {
			return true
		}
	}
	for _, member := range change.RemovedMembers {
		if member.ID == tagID {
			return true
		}
	}
	return false
}

func printPublicationSelection(posts []LibraryChange, tags []LibraryChange, deferredTags []LibraryChange) {
	fmt.Println("RSS entries to publish:")
	for _, change := range posts {
		fmt.Printf("  %s: post [%s] %s\n", publicationStatus(change.Status), change.Page.ID, change.Page.Title)
	}
	for _, change := range tags {
		fmt.Printf("  %s: tag [%s] %s (automatic)\n", publicationStatus(change.Status), change.Page.ID, change.Page.Title)
	}
	for _, change := range deferredTags {
		fmt.Printf("  Hold tag [%s] %s: it also has an unselected post change.\n", change.Page.ID, change.Page.Title)
	}
}

func renderRSSHistory(publication RSSPublication, site SiteConfig, history []*etree.Element, entryLimit int) ([]RSSItem, error) {
	items := []RSSItem{}
	first := len(history) - 1
	last := first - entryLimit
	for index := first; index > last && index >= 0; index-- {
		item, err := renderPublication(publication.Stylesheet, site, history[index])
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
