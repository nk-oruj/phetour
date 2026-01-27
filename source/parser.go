package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/beevik/etree"
)

// ParseCustomSyntax parses a custom syntax file into an XML document
func ParseCustomSyntax(content string, filePath string) (*etree.Document, error) {
	lines := strings.Split(content, "\n")

	if len(lines) < 2 {
		return nil, fmt.Errorf("file must have at least 2 lines for metadata")
	}

	// Parse metadata from first two lines
	title, tags, err := parseMetadata(lines[0], lines[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Create document structure
	doc := etree.NewDocument()
	documentRoot := doc.CreateElement("document")

	// Add meta section
	meta := documentRoot.CreateElement("meta")
	titleElem := meta.CreateElement("title")
	titleElem.CreateAttr("value", title)

	// Add tags to meta
	for _, tagLabel := range tags {
		tag := meta.CreateElement("tag")
		tag.CreateAttr("label", tagLabel)
	}

	// Parse content (skip first two lines)
	contentLines := lines[2:]
	body := documentRoot.CreateElement("body")

	err = parseContent(contentLines, body, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	return doc, nil
}

// parseMetadata parses title and tags from metadata lines
func parseMetadata(titleLine, tagsLine string) (string, []string, error) {
	// Parse title: "title: 'title'" or "title: \"title\""
	titleMatch := regexp.MustCompile(`title:\s*['"]([^'"]+)['"]`).FindStringSubmatch(titleLine)
	if len(titleMatch) < 2 {
		return "", nil, fmt.Errorf("invalid title format, expected: title: 'title'")
	}
	title := titleMatch[1]

	// Parse tags: "tags: ['tag1', 'tag2']" or "tags: [\"tag1\", \"tag2\"]"
	tagsMatch := regexp.MustCompile(`tags:\s*\[(.*?)\]`).FindStringSubmatch(tagsLine)
	if len(tagsMatch) < 2 {
		return "", nil, fmt.Errorf("invalid tags format, expected: tags: ['tag1', 'tag2']")
	}

	// Extract individual tags
	tagsStr := tagsMatch[1]
	tagRegex := regexp.MustCompile(`['"]([^'"]+)['"]`)
	tagMatches := tagRegex.FindAllStringSubmatch(tagsStr, -1)

	tags := make([]string, 0, len(tagMatches))
	for _, match := range tagMatches {
		if len(match) >= 2 {
			tags = append(tags, match[1])
		}
	}

	return title, tags, nil
}

// parseContent parses the content lines into body elements
func parseContent(lines []string, body *etree.Element, filePath string) error {
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check for code block start
		if strings.HasPrefix(trimmed, "```") {
			codeBlock, nextIdx, err := parseCodeBlock(lines, i, filePath)
			if err != nil {
				return err
			}
			if codeBlock != nil {
				body.AddChild(codeBlock)
			}
			i = nextIdx
			continue
		}

		// Check for bold/title: "# "
		if strings.HasPrefix(trimmed, "# ") {
			bold := body.CreateElement("bold")
			bold.CreateText(strings.TrimPrefix(trimmed, "# "))
			i++
			continue
		}

		// Check for item: "- "
		if strings.HasPrefix(trimmed, "- ") {
			item := body.CreateElement("item")
			item.CreateText(strings.TrimPrefix(trimmed, "- "))
			i++
			continue
		}

		// Check for link: "> href text"
		if strings.HasPrefix(trimmed, "> ") {
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
			continue
		}

		// Plain text - collect consecutive plain text lines
		if trimmed != "" {
			textLines := []string{trimmed}
			i++
			// Collect following plain text lines
			for i < len(lines) {
				nextLine := strings.TrimSpace(lines[i])
				// Stop if we hit a special prefix or empty line
				if nextLine == "" ||
					strings.HasPrefix(nextLine, "# ") ||
					strings.HasPrefix(nextLine, "- ") ||
					strings.HasPrefix(nextLine, "> ") ||
					strings.HasPrefix(nextLine, "```") {
					break
				}
				textLines = append(textLines, nextLine)
				i++
			}
			// Create text element with all collected lines
			text := body.CreateElement("text")
			text.CreateText(strings.Join(textLines, "\n"))
			continue
		}

		// Empty line - skip
		i++
	}

	return nil
}

// parseCodeBlock parses a code block and processes it with pandoc
// Returns: code element, next line index, error
func parseCodeBlock(lines []string, startIdx int, filePath string) (*etree.Element, int, error) {
	// Find the closing ```
	endIdx := startIdx + 1
	for endIdx < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[endIdx]), "```") {
			break
		}
		endIdx++
	}

	if endIdx >= len(lines) {
		return nil, startIdx, fmt.Errorf("unclosed code block starting at line %d", startIdx+1)
	}

	// Extract code content (excluding the ``` markers)
	codeLines := lines[startIdx+1 : endIdx]
	codeContent := strings.Join(codeLines, "\n")

	// Process with pandoc to convert markdown to HTML
	htmlContent, err := processWithPandoc(codeContent)
	if err != nil {
		// If pandoc fails, just use the raw content
		code := etree.NewElement("code")
		code.CreateText(codeContent)
		return code, endIdx + 1, nil
	}

	// Create code element with HTML content
	code := etree.NewElement("code")
	code.AddChild(htmlContent.Root().Copy())

	// mainDoc := etree.NewDocument()
	// mainDoc.AddChild(htmlContent.Root().Copy())
	// mainDoc.Indent(4)
	// mainDoc.WriteTo(os.Stdout)

	return code, endIdx + 1, nil
}

// processWithPandoc converts markdown content to HTML using pandoc
func processWithPandoc(markdown string) (*etree.Document, error) {
	// Create a temporary file for pandoc input
	tmpFile, err := os.CreateTemp("", "pandoc-input-*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write markdown content
	if _, err := tmpFile.WriteString(markdown); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	fullCmd := "pandoc " + tmpFile.Name() + " -f markdown -t html"

	// Run pandoc: markdown -> HTML
	cmd := exec.Command("sh", "-c", fullCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pandoc conversion failed: %s", string(output))
	}

	outputXML := etree.NewDocument()
	outputXML.ReadFromBytes(output)

	return outputXML, nil
}
