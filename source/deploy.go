package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runDeploy(forceAll bool, force bool) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	for _, output := range config.Deployment.Outputs {
		if _, err := os.Stat(filepath.Join("output", output.Name)); os.IsNotExist(err) {
			return fmt.Errorf("output/%s does not exist: run phetour build first", output.Name)
		} else if err != nil {
			return fmt.Errorf("failed to access output/%s: %w", output.Name, err)
		}
	}

	if err := prepareRSSPublications(config); err != nil {
		return err
	}

	plans := make([]RsyncPlan, 0, len(config.Deployment.Outputs))
	for _, output := range config.Deployment.Outputs {
		plan, err := previewRsync(config.Deployment.SSHAlias, output, forceAll, nil)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		printRsyncPlan(plan)
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
		if err := executeRsync(plan); err != nil {
			return err
		}
	}
	return nil
}

func prepareRSSPublications(config Config) error {
	if len(config.RSS) == 0 {
		return nil
	}

	keylock, err := LoadKeylock()
	if err != nil {
		return err
	}
	sourcePlans := map[string]RsyncPlan{}
	for _, publication := range config.RSS {
		if _, exists := sourcePlans[publication.Source]; exists {
			continue
		}
		output, _ := findDeploymentOutput(config.Deployment, publication.Source)
		excludes := rssPathsForSource(config.RSS, publication.Source)
		plan, err := previewRsync(config.Deployment.SSHAlias, output, false, excludes)
		if err != nil {
			return err
		}
		sourcePlans[publication.Source] = plan
	}

	for _, publication := range config.RSS {
		events, err := buildRSSEvents(sourcePlans[publication.Source].Changes, publication.Source, config.Site, keylock)
		if err != nil {
			return err
		}
		added, err := writeRSSPublication(publication, config.Site, config.Deployment, config.Deployment.SSHAlias, events)
		if err != nil {
			return err
		}
		fmt.Printf("RSS output/%s/%s: %d new event(s)\n", publication.Output, publication.Path, len(added))
		for _, event := range added {
			fmt.Println("  " + event.Title)
		}
	}
	return nil
}

func rssPathsForSource(publications []RSSPublication, source string) []string {
	paths := []string{}
	for _, publication := range publications {
		if publication.Output == source {
			paths = append(paths, publication.Path)
		}
	}
	return paths
}

type RsyncPlan struct {
	Alias    string
	Output   DeploymentOutput
	ForceAll bool
	Changes  string
	Excludes []string
}

func previewRsync(alias string, deploymentOutput DeploymentOutput, forceAll bool, excludes []string) (RsyncPlan, error) {
	plan := RsyncPlan{Alias: alias, Output: deploymentOutput, ForceAll: forceAll, Excludes: excludes}
	commandOutput, err := runRsync(rsyncArguments(plan, true)...)
	if err != nil {
		return RsyncPlan{}, fmt.Errorf("failed to preview deployment of output/%s: %w", plan.Output.Name, err)
	}
	plan.Changes = strings.TrimSpace(commandOutput)
	return plan, nil
}

func executeRsync(plan RsyncPlan) error {
	if _, err := runRsync(rsyncArguments(plan, false)...); err != nil {
		return fmt.Errorf("failed to deploy output/%s: %w", plan.Output.Name, err)
	}
	return nil
}

func rsyncArguments(plan RsyncPlan, dryRun bool) []string {
	arguments := []string{
		"--recursive",
		"--no-links",
		"--no-devices",
		"--no-specials",
		"--no-owner",
		"--no-group",
		"--no-times",
		"--no-perms",
		"--delete",
		"--delay-updates",
		"--delete-delay",
		"--itemize-changes",
		"--out-format=%i %n%L",
	}
	for _, excludedPath := range plan.Excludes {
		arguments = append(arguments, "--exclude=/"+excludedPath)
	}
	if plan.ForceAll {
		arguments = append(arguments, "--ignore-times")
	} else {
		arguments = append(arguments, "--checksum")
	}
	if dryRun {
		arguments = append(arguments, "--dry-run")
	}
	source := filepath.ToSlash(filepath.Join("output", plan.Output.Name)) + "/"
	destination := plan.Alias + ":" + plan.Output.RemotePath + "/"
	return append(arguments, source, destination)
}

func runRsync(arguments ...string) (string, error) {
	output, err := exec.Command("rsync", arguments...).CombinedOutput()
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("rsync is not available: install it locally before deploying")
	}
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func printRsyncPlan(plan RsyncPlan) {
	mode := "changes"
	if plan.ForceAll {
		mode = "all"
	}
	fmt.Printf("Deploy (%s) output/%s → %s:%s\n", mode, plan.Output.Name, plan.Alias, plan.Output.RemotePath)
	if plan.Changes == "" {
		fmt.Println("  No remote changes.")
	} else {
		for _, line := range strings.Split(plan.Changes, "\n") {
			fmt.Println("  " + line)
		}
	}
}
