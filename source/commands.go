package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func runCommand(arguments []string) error {
	command := "build"
	force := false
	if len(arguments) > 0 {
		command = arguments[0]
		arguments = arguments[1:]
	}
	for _, argument := range arguments {
		if argument == "--force" {
			force = true
		} else {
			return fmt.Errorf("unknown option %q", argument)
		}
	}

	switch command {
	case "build", "build-all":
		return runBuild(force)
	case "deploy-changes":
		return runDeploy(false, force)
	case "deploy-all":
		return runDeploy(true, force)
	case "publish":
		return runPublish(force)
	case "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: phetour <command> [--force]")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Commands:")
	fmt.Fprintln(os.Stdout, "  build             Fully regenerate local output.")
	fmt.Fprintln(os.Stdout, "  build-all         Alias for build.")
	fmt.Fprintln(os.Stdout, "  deploy-changes    Synchronize changed output files with configured remotes.")
	fmt.Fprintln(os.Stdout, "  deploy-all        Force every output file to upload to configured remotes.")
	fmt.Fprintln(os.Stdout, "  publish           Select changed posts and publish RSS entries.")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Each command displays its plan and asks for confirmation. Use --force to skip the prompt.")
}

func confirmPlan(force bool) (bool, error) {
	if force {
		return true, nil
	}

	fmt.Print("Proceed? [y/N] ")
	response, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(response) == 0 {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}
