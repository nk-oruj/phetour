package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

const configFilePath = "./config.xml"

type Config struct {
	Deployment     DeploymentConfig
	Site           SiteConfig
	RSSEntryLimit  int
	RSSMemberLimit int
	RSS            []RSSPublication
}

type DeploymentOutput struct {
	Name       string
	RemotePath string
}

type DeploymentConfig struct {
	SSHAlias string
	Outputs  []DeploymentOutput
}

type SiteConfig struct {
	Title       string
	URL         string
	Description string
}

type RSSPublication struct {
	Output     string
	Path       string
	Stylesheet string
}

func loadConfig() (Config, error) {
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return Config{}, fmt.Errorf("deployment configuration not found: copy ~config.xml to config.xml and set its values")
	} else if err != nil {
		return Config{}, fmt.Errorf("failed to access config.xml: %w", err)
	}

	document := etree.NewDocument()
	if err := document.ReadFromFile(configFilePath); err != nil {
		return Config{}, fmt.Errorf("failed to read config.xml: %w", err)
	}

	root := document.Root()
	if root == nil || root.Tag != "config" {
		return Config{}, fmt.Errorf("config.xml must have a config root element")
	}
	deployment, err := readDeploymentConfig(root)
	if err != nil {
		return Config{}, err
	}

	config := Config{Deployment: deployment}
	rss := root.SelectElement("rss")
	if rss == nil {
		return config, nil
	}

	site := root.SelectElement("site")
	if site == nil {
		return Config{}, fmt.Errorf("RSS publication requires a site element")
	}
	config.Site = SiteConfig{
		Title:       strings.TrimSpace(site.SelectAttrValue("title", "")),
		URL:         strings.TrimSpace(site.SelectAttrValue("url", "")),
		Description: strings.TrimSpace(site.SelectAttrValue("description", "")),
	}
	if config.Site.Title == "" || config.Site.URL == "" || config.Site.Description == "" {
		return Config{}, fmt.Errorf("RSS publication requires site title, url, and description")
	}
	if !strings.HasPrefix(config.Site.URL, "https://") && !strings.HasPrefix(config.Site.URL, "http://") {
		return Config{}, fmt.Errorf("site url must start with https:// or http://")
	}
	entryLimit, err := strconv.Atoi(rss.SelectAttrValue("entry-limit", ""))
	if err != nil || entryLimit < 1 {
		return Config{}, fmt.Errorf("rss entry-limit must be a positive integer")
	}
	config.RSSEntryLimit = entryLimit
	memberLimit, err := strconv.Atoi(rss.SelectAttrValue("member-limit", ""))
	if err != nil || memberLimit < 1 {
		return Config{}, fmt.Errorf("rss member-limit must be a positive integer")
	}
	config.RSSMemberLimit = memberLimit

	outputs := outputNames(config.Deployment)
	publicationPaths := map[string]bool{}
	for _, publication := range rss.SelectElements("publish") {
		output := publication.SelectAttrValue("output", "")
		feedPath := publication.SelectAttrValue("path", "")
		stylesheet := publication.SelectAttrValue("stylesheet", "")
		if !outputs[output] {
			return Config{}, fmt.Errorf("RSS publication output must name a configured deployment output")
		}
		if !isSafeRelativePath(feedPath) {
			return Config{}, fmt.Errorf("RSS publication path %q must be a relative path without ..", feedPath)
		}
		if !isSafeRelativePath(stylesheet) || filepath.Ext(stylesheet) != ".xsl" {
			return Config{}, fmt.Errorf("RSS publication stylesheet %q must be a relative .xsl path without ..", stylesheet)
		}
		publicationKey := output + ":" + feedPath
		if publicationPaths[publicationKey] {
			return Config{}, fmt.Errorf("RSS publication output %q path %q is configured more than once", output, feedPath)
		}
		publicationPaths[publicationKey] = true
		config.RSS = append(config.RSS, RSSPublication{Output: output, Path: feedPath, Stylesheet: stylesheet})
	}
	if len(config.RSS) == 0 {
		return Config{}, fmt.Errorf("rss must contain at least one publish element")
	}
	return config, nil
}

func readDeploymentConfig(root *etree.Element) (DeploymentConfig, error) {
	deployment := root.SelectElement("deployment")
	if deployment == nil {
		return DeploymentConfig{}, fmt.Errorf("config.xml must contain a deployment element")
	}
	config := DeploymentConfig{SSHAlias: deployment.SelectAttrValue("ssh-alias", "")}
	if strings.TrimSpace(config.SSHAlias) == "" {
		return DeploymentConfig{}, fmt.Errorf("deployment ssh-alias is required")
	}

	outputs := map[string]bool{}
	for _, output := range deployment.SelectElements("output") {
		name := output.SelectAttrValue("name", "")
		remotePath := output.SelectAttrValue("remote", "")
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return DeploymentConfig{}, fmt.Errorf("invalid output name %q", name)
		}
		if outputs[name] {
			return DeploymentConfig{}, fmt.Errorf("output %q is configured more than once", name)
		}
		if !strings.HasPrefix(remotePath, "/") || strings.ContainsAny(remotePath, "\t\r\n ") {
			return DeploymentConfig{}, fmt.Errorf("remote path for output %q must be an absolute path without whitespace", name)
		}
		outputs[name] = true
		config.Outputs = append(config.Outputs, DeploymentOutput{Name: name, RemotePath: remotePath})
	}
	if len(config.Outputs) == 0 {
		return DeploymentConfig{}, fmt.Errorf("config.xml must configure at least one deployment output")
	}
	return config, nil
}

func outputNames(config DeploymentConfig) map[string]bool {
	names := map[string]bool{}
	for _, output := range config.Outputs {
		names[output.Name] = true
	}
	return names
}

func findDeploymentOutput(config DeploymentConfig, name string) (DeploymentOutput, bool) {
	for _, output := range config.Outputs {
		if output.Name == name {
			return output, true
		}
	}
	return DeploymentOutput{}, false
}

func isSafeRelativePath(value string) bool {
	return value != "" && !strings.HasPrefix(value, "/") && path.Clean(value) == value && !strings.HasPrefix(value, "../") && value != ".."
}
