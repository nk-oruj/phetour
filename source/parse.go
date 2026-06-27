package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/beevik/etree"
)

func parseDocument(content string, filePath string) (*etree.Document, error) {
	lines := strings.Split(content, "\n")

	var title string
	var tags []string
	var contentStart int

	for i, line := range lines {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "#") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			contentStart = i + 1
			break
		}
	}

	if title == "" {
		return nil, fmt.Errorf("no title found: expected a line starting with '#'")
	}

	i := contentStart
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			tags = append(tags, strings.TrimSpace(strings.TrimPrefix(trimmed, ">")))
			i++
		} else {
			break
		}
	}

	doc := etree.NewDocument()
	docRoot := doc.CreateElement("document")

	meta := docRoot.CreateElement("meta")
	meta.CreateElement("title").CreateAttr("value", title)
	for _, label := range tags {
		meta.CreateElement("tag").CreateAttr("label", label)
	}

	body := docRoot.CreateElement("body")
	if err := parseContent(lines[i:], body, filePath); err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	return doc, nil
}

func parseContent(lines []string, body *etree.Element, filePath string) error {
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		switch {
		case strings.HasPrefix(trimmed, "```"):
			codeBlock, nextIdx, err := parseCodeBlock(lines, i, filePath)
			if err != nil {
				return err
			}
			if codeBlock != nil {
				body.AddChild(codeBlock)
			}
			i = nextIdx

		case strings.HasPrefix(trimmed, "# "):
			body.CreateElement("bold").CreateText(strings.TrimPrefix(trimmed, "# "))
			i++

		case strings.HasPrefix(trimmed, "- "):
			body.CreateElement("item").CreateText(strings.TrimPrefix(trimmed, "- "))
			i++

		case strings.HasPrefix(trimmed, "> "):
			linkContent := strings.TrimPrefix(trimmed, "> ")
			parts := strings.Fields(linkContent)
			if len(parts) >= 1 {
				link := body.CreateElement("link")
				link.CreateAttr("href", parts[0])
				if len(parts) > 1 {
					link.CreateText(strings.Join(parts[1:], " "))
				} else {
					link.CreateText(parts[0])
				}
			}
			i++

		case trimmed != "":
			textLines := []string{trimmed}
			i++
			for i < len(lines) {
				next := strings.TrimSpace(lines[i])
				if next == "" ||
					strings.HasPrefix(next, "# ") ||
					strings.HasPrefix(next, "- ") ||
					strings.HasPrefix(next, "> ") ||
					strings.HasPrefix(next, "```") {
					break
				}
				textLines = append(textLines, next)
				i++
			}
			body.CreateElement("text").CreateText(strings.Join(textLines, "\n"))

		default:
			i++
		}
	}

	return nil
}

func parseCodeBlock(lines []string, startIdx int, filePath string) (*etree.Element, int, error) {
	endIdx := startIdx + 1
	for endIdx < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[endIdx]), "```") {
			break
		}
		endIdx++
	}

	if endIdx >= len(lines) {
		return nil, startIdx, fmt.Errorf("unclosed code block at line %d", startIdx+1)
	}

	codeContent := strings.Join(lines[startIdx+1:endIdx], "\n")

	htmlContent, err := processWithPandoc(codeContent)
	if err != nil {
		code := etree.NewElement("code")
		code.CreateText(codeContent)
		return code, endIdx + 1, nil
	}

	code := etree.NewElement("code")
	code.AddChild(htmlContent.Root().Copy())
	return code, endIdx + 1, nil
}

func processWithPandoc(markdown string) (*etree.Document, error) {
	tmpFile, err := os.CreateTemp("", "pandoc-input-*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(markdown); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("pandoc", tmpFile.Name(), "-f", "markdown", "-t", "html")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pandoc failed: %s", string(output))
	}

	doc := etree.NewDocument()
	doc.ReadFromBytes(output)
	return doc, nil
}
