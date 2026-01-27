package main

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/beevik/etree"
)

func KeyIDToHex(id int) string {
	return fmt.Sprintf("0x%04x", id)
}

// Helper function to recursively copy element children
func copyElementChildren(src *etree.Element, dst *etree.Element) {
	for _, child := range src.Child {
		if elem, ok := child.(*etree.Element); ok {
			newElem := dst.CreateElement(elem.Tag)
			for _, attr := range elem.Attr {
				newElem.CreateAttr(attr.Key, attr.Value)
			}
			copyElementChildren(elem, newElem)
		} else if text, ok := child.(*etree.CharData); ok {
			dst.CreateText(string(text.Data))
		}
	}
}

func Build(source *Source, taxonomy *Taxonomy) error {
	const xmlOutputPath = "./output/xml"
	const staticsInputPath = "./input/statics"
	const stylesInputPath = "./input/styles"

	// Delete all folders in output prior to building
	outputParent := filepath.Dir(xmlOutputPath)
	entries, err := os.ReadDir(outputParent)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				// Remove all directories in output
				if err := os.RemoveAll(filepath.Join(outputParent, entry.Name())); err != nil {
					return fmt.Errorf("failed to remove output directory %s: %w", entry.Name(), err)
				}
			}
		}
	}
	if err := os.MkdirAll(xmlOutputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Process posts - create folders and index.xml for each post
	for _, post := range source.Posts {
		if err := buildPost(post, xmlOutputPath, taxonomy); err != nil {
			return fmt.Errorf("failed to build post %s: %w", post.Name, err)
		}
	}

	// Process tags - create folders and index.xml for each tag
	for _, tag := range taxonomy.Tags {
		if err := buildTag(tag, xmlOutputPath, source); err != nil {
			return fmt.Errorf("failed to build tag %s: %w", tag.Label, err)
		}
	}

	// Create home catalog index.xml
	if err := buildHomeCatalog(source, taxonomy, xmlOutputPath); err != nil {
		return fmt.Errorf("failed to build home catalog: %w", err)
	}

	// Copy static files
	if err := copyStatics(staticsInputPath, xmlOutputPath); err != nil {
		return fmt.Errorf("failed to copy static files: %w", err)
	}

	// Apply XSL transformations if stylesheets exist
	if err := applyStylesheets(xmlOutputPath, stylesInputPath); err != nil {
		return fmt.Errorf("failed to apply stylesheets: %w", err)
	}

	return nil
}

func buildPost(post Post, outputPath string, taxonomy *Taxonomy) error {
	identityHex := KeyIDToHex(post.Key)
	postDir := filepath.Join(outputPath, identityHex)

	if err := os.MkdirAll(postDir, 0755); err != nil {
		return fmt.Errorf("failed to create post directory: %w", err)
	}

	// Create a new document with proper root element
	doc := etree.NewDocument()
	documentRoot := doc.CreateElement("document")

	// Get meta element from original document
	metaOrig := post.Content.SelectElement("meta")
	if metaOrig == nil {
		// Check if wrapped in document
		docRoot := post.Content.Root()
		if docRoot != nil && docRoot.Tag == "document" {
			metaOrig = docRoot.SelectElement("meta")
		}
	}

	// Copy meta section to output (for HTML head title)
	if metaOrig != nil {
		meta := documentRoot.CreateElement("meta")
		titleOrig := metaOrig.SelectElement("title")
		if titleOrig != nil {
			title := meta.CreateElement("title")
			titleValue := titleOrig.SelectAttrValue("value", "")
			title.CreateAttr("value", titleValue)
		}
		// Copy tags to meta (for reference, but they'll be in body as links)
		for _, tagOrig := range metaOrig.SelectElements("tag") {
			tag := meta.CreateElement("tag")
			tagLabel := tagOrig.SelectAttrValue("label", "")
			tag.CreateAttr("label", tagLabel)
			// Find and add tag ID
			var tagKey int
			for _, t := range taxonomy.Tags {
				if t.Label == tagLabel {
					tagKey = t.Key
					break
				}
			}
			if tagKey > 0 {
				tag.CreateAttr("id", KeyIDToHex(tagKey))
			}
		}
	}

	// Create body element
	body := documentRoot.CreateElement("body")

	// First: Add bold tag with title from meta
	if metaOrig != nil {
		titleOrig := metaOrig.SelectElement("title")
		if titleOrig != nil {
			titleValue := titleOrig.SelectAttrValue("value", "")
			if titleValue != "" {
				bold := body.CreateElement("bold")
				bold.CreateText(titleValue)
			}
		}

		// Then: Add link tags for each tag in meta
		for _, tagOrig := range metaOrig.SelectElements("tag") {
			tagLabel := tagOrig.SelectAttrValue("label", "")
			if tagLabel != "" {
				// Find the tag key for this label
				var tagKey int
				for _, t := range taxonomy.Tags {
					if t.Label == tagLabel {
						tagKey = t.Key
						break
					}
				}
				if tagKey > 0 {
					link := body.CreateElement("link")
					link.CreateAttr("href", "/"+KeyIDToHex(tagKey)+"/")
					link.CreateText(KeyIDToHex(tagKey) + " - " + tagLabel)
				}
			}
		}
	}

	// Then: Append all elements from original body
	bodyOrig := post.Content.SelectElement("body")
	if bodyOrig == nil {
		// Check if wrapped in document
		docRoot := post.Content.Root()
		if docRoot != nil && docRoot.Tag == "document" {
			bodyOrig = docRoot.SelectElement("body")
		}
	}

	if bodyOrig != nil {
		// Copy all children from original body (bold, text, code, item, link)
		for _, child := range bodyOrig.Child {
			if elem, ok := child.(*etree.Element); ok {
				// Only copy allowed tags: bold, text, code, item, link
				if elem.Tag == "bold" || elem.Tag == "text" || elem.Tag == "code" || elem.Tag == "item" || elem.Tag == "link" {
					newElem := body.CreateElement(elem.Tag)
					// Copy attributes (especially href for link)
					for _, attr := range elem.Attr {
						newElem.CreateAttr(attr.Key, attr.Value)
					}
					// Recursively copy children
					copyElementChildren(elem, newElem)
				}
			} else if text, ok := child.(*etree.CharData); ok {
				// Preserve text nodes
				body.CreateText(string(text.Data))
			}
		}
	} else {
		// Fallback: try to extract from raw XML if body element doesn't exist
		var buf bytes.Buffer
		_, err := post.Content.WriteTo(&buf)
		if err == nil {
			xmlContent := buf.String()
			metaEnd := strings.Index(xmlContent, "</meta>")
			if metaEnd != -1 {
				bodyContent := strings.TrimSpace(xmlContent[metaEnd+7:])
				// Remove any document/body wrappers that might have been added
				bodyContent = strings.TrimPrefix(bodyContent, "<body>")
				bodyContent = strings.TrimSuffix(bodyContent, "</body>")
				bodyContent = strings.TrimPrefix(bodyContent, "<document>")
				bodyContent = strings.TrimSuffix(bodyContent, "</document>")
				bodyContent = strings.TrimSpace(bodyContent)

				if bodyContent != "" {
					// Try to parse as XML elements
					tempDoc := etree.NewDocument()
					tempXML := "<temp>" + bodyContent + "</temp>"
					err = tempDoc.ReadFromString(tempXML)
					if err == nil {
						tempRoot := tempDoc.Root()
						for _, child := range tempRoot.Child {
							if elem, ok := child.(*etree.Element); ok {
								// Only copy allowed tags
								if elem.Tag == "bold" || elem.Tag == "text" || elem.Tag == "code" || elem.Tag == "item" || elem.Tag == "link" {
									newElem := body.CreateElement(elem.Tag)
									for _, attr := range elem.Attr {
										newElem.CreateAttr(attr.Key, attr.Value)
									}
									copyElementChildren(elem, newElem)
								}
							} else if text, ok := child.(*etree.CharData); ok {
								if strings.TrimSpace(string(text.Data)) != "" {
									body.CreateText(string(text.Data))
								}
							}
						}
					} else {
						// If parsing fails, wrap in text tag
						textElem := body.CreateElement("text")
						textElem.CreateText(bodyContent)
					}
				}
			}
		}
	}

	// Write index.xml
	indexPath := filepath.Join(postDir, "index.xml")
	doc.Indent(4)
	if err := doc.WriteToFile(indexPath); err != nil {
		return fmt.Errorf("failed to write post index.xml: %w", err)
	}

	return nil
}

func buildTag(tag Tag, outputPath string, source *Source) error {
	identityHex := KeyIDToHex(tag.Key)
	tagDir := filepath.Join(outputPath, identityHex)

	if err := os.MkdirAll(tagDir, 0755); err != nil {
		return fmt.Errorf("failed to create tag directory: %w", err)
	}

	// Create tag catalog XML with document structure
	doc := etree.NewDocument()
	documentRoot := doc.CreateElement("document")

	// Add meta section with title (for HTML head)
	meta := documentRoot.CreateElement("meta")
	title := meta.CreateElement("title")
	title.CreateAttr("value", tag.Label)

	// Create body element
	body := documentRoot.CreateElement("body")

	// First: Add bold tag with tag label
	bold := body.CreateElement("bold")
	bold.CreateText(tag.Label)

	// Then: Add link tags for each document mentioning the tag
	for _, mentionID := range tag.Mentions {
		// Find the post for this mention
		var postTitle string
		for _, post := range source.Posts {
			if post.Key == mentionID {
				postTitle = post.Title
				break
			}
		}
		if postTitle != "" {
			link := body.CreateElement("link")
			link.CreateAttr("href", "/"+KeyIDToHex(mentionID)+"/")
			link.CreateText(fmt.Sprintf("%s - %s", KeyIDToHex(mentionID), postTitle))
		}
	}

	// Write index.xml
	indexPath := filepath.Join(tagDir, "index.xml")
	doc.Indent(4)
	if err := doc.WriteToFile(indexPath); err != nil {
		return fmt.Errorf("failed to write tag index.xml: %w", err)
	}

	return nil
}

func buildHomeCatalog(source *Source, taxonomy *Taxonomy, outputPath string) error {
	// Create home catalog XML with document structure
	doc := etree.NewDocument()
	documentRoot := doc.CreateElement("document")

	// Add meta section with title (for HTML head)
	meta := documentRoot.CreateElement("meta")
	title := meta.CreateElement("title")
	title.CreateAttr("value", "փետուր")

	// Create body element
	body := documentRoot.CreateElement("body")

	slices.SortFunc(source.Posts, func(a, b Post) int { return -cmp.Compare(a.Key, b.Key) })

	// Add link tags for processed post documents
	for _, post := range source.Posts {
		link := body.CreateElement("link")
		link.CreateAttr("href", "/"+KeyIDToHex(post.Key)+"/")
		link.CreateText(fmt.Sprintf("%s - %s", KeyIDToHex(post.Key), post.Title))
	}

	// Add separator text element
	text := body.CreateElement("text")
	text.CreateText("")

	slices.SortFunc(taxonomy.Tags, func(a, b Tag) int { return -cmp.Compare(a.Key, b.Key) })

	// Add link tags for tags
	for _, tag := range taxonomy.Tags {
		link := body.CreateElement("link")
		link.CreateAttr("href", "/"+KeyIDToHex(tag.Key)+"/")
		link.CreateText(fmt.Sprintf("%s - %s", KeyIDToHex(tag.Key), tag.Label))
	}

	// Write index.xml
	indexPath := filepath.Join(outputPath, "index.xml")
	doc.Indent(4)
	if err := doc.WriteToFile(indexPath); err != nil {
		return fmt.Errorf("failed to write home catalog: %w", err)
	}

	return nil
}

func copyStatics(srcPath string, dstPath string) error {
	_, err := os.Stat(srcPath)
	if os.IsNotExist(err) {
		// No statics directory, skip
		return nil
	}

	return filepath.Walk(srcPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		dstFile := filepath.Join(dstPath, relPath)
		dstDir := filepath.Dir(dstFile)

		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer srcFile.Close()

		dstFileHandle, err := os.Create(dstFile)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer dstFileHandle.Close()

		_, err = io.Copy(dstFileHandle, srcFile)
		if err != nil {
			return fmt.Errorf("failed to copy file: %w", err)
		}

		return nil
	})
}

func applyStylesheets(xmlOutputPath string, stylesInputPath string) error {
	_, err := os.Stat(stylesInputPath)
	if os.IsNotExist(err) {
		// No styles directory, skip
		return nil
	}

	// Find all .xsl files
	xslFiles := []string{}
	err = filepath.Walk(stylesInputPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".xsl") {
			xslFiles = append(xslFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk styles directory: %w", err)
	}

	if len(xslFiles) == 0 {
		return nil
	}

	// For each XSL file, create a parallel output directory and transform XML files
	for _, xslFile := range xslFiles {
		// Get the base name of the XSL file (without .xsl extension)
		baseName := filepath.Base(xslFile)
		styleName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		// Determine output directory based on XML output path
		// If xmlOutputPath is "./output/xml", style output would be "./output/<styleName>"
		xmlOutputParent := filepath.Dir(xmlOutputPath)
		styleOutputPath := filepath.Join(xmlOutputParent, styleName)

		if err := transformXMLDirectory(xmlOutputPath, styleOutputPath, xslFile, styleName); err != nil {
			return fmt.Errorf("failed to transform with stylesheet %s: %w", xslFile, err)
		}
	}

	return nil
}

func transformXMLDirectory(srcPath string, dstPath string, xslFile string, styleName string) error {
	// Create output directory
	if err := os.MkdirAll(dstPath, 0755); err != nil {
		return fmt.Errorf("failed to create style output directory: %w", err)
	}

	// Walk through all XML files and transform them
	return filepath.Walk(srcPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Create corresponding directory structure
			relPath, err := filepath.Rel(srcPath, path)
			if err != nil {
				return err
			}
			dstDir := filepath.Join(dstPath, relPath)
			return os.MkdirAll(dstDir, 0755)
		}

		// Only process XML files
		if strings.ToLower(filepath.Ext(path)) != ".xml" {
			// Copy non-XML files as-is
			relPath, err := filepath.Rel(srcPath, path)
			if err != nil {
				return err
			}
			dstFile := filepath.Join(dstPath, relPath)

			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer src.Close()

			dst, err := os.Create(dstFile)
			if err != nil {
				return err
			}
			defer dst.Close()

			_, err = io.Copy(dst, src)
			return err
		}

		// Transform XML file
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		// Replace .xml extension with style name
		dstFile := filepath.Join(dstPath, relPath)
		dstFile = strings.TrimSuffix(dstFile, ".xml") + "." + styleName

		// Ensure destination directory exists
		dstDir := filepath.Dir(dstFile)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		return transformXMLFile(path, dstFile, xslFile)
	})
}

func transformXMLFile(xmlPath string, dstPath string, xslPath string) error {
	return transformWithXsltproc(xmlPath, dstPath, xslPath)
}

func transformWithXsltproc(xmlPath, dstPath, xslPath string) error {
	// Try to use xsltproc if available
	// Note: On Windows, user may need to install libxslt or use msxsl.exe
	// Alternative tools: saxon (Java), xalan (Java), msxsl (Windows)

	cmd := exec.Command("xsltproc", "-o", dstPath, xslPath, xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If xsltproc is not available, try msxsl on Windows
		errStr := strings.ToLower(string(output))
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "not recognized") ||
			strings.Contains(errStr, "command not found") {

			// Try msxsl.exe on Windows
			cmd = exec.Command("msxsl.exe", xmlPath, xslPath, "-o", dstPath)
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("XSLT transformation failed. xsltproc/msxsl not found or error occurred: %s. Please install an XSLT processor", string(output))
			}
		} else {
			return fmt.Errorf("XSLT transformation failed: %s", string(output))
		}
	}

	return nil
}
