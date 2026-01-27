package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

const (
	postsPath = "./input/posts"
)

type Post struct {
	Name    string
	Title   string
	Key     int
	Content *etree.Document
	Tags    []int
}

type Source struct {
	Posts []Post
}

func GetSource(keylock *Keylock, taxonomy *Taxonomy) (*Source, error) {

	source := &Source{Posts: []Post{}}

	err := filepath.Walk(postsPath, func(path string, info fs.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if info.Name()[0] == '~' {
			return nil
		}

		name := info.Name()

		post, err := GetPostDocument(path, name, keylock, taxonomy)
		if err != nil {
			return fmt.Errorf("failed reading post document %s: %w", path, err)
		}

		source.Posts = append(source.Posts, post)

		return nil

	})

	if err != nil {
		return nil, fmt.Errorf("failed reading post document folder: %w", err)
	}

	return source, nil

}

func GetPostDocument(path string, name string, keylock *Keylock, taxonomy *Taxonomy) (Post, error) {

	// Read file content
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return Post{}, fmt.Errorf("failed reading post document %s: %w", path, err)
	}

	contentStr := string(contentBytes)
	document := etree.NewDocument()

	// Try to parse as custom syntax first (check if first line looks like metadata)
	firstLine := strings.TrimSpace(strings.Split(contentStr, "\n")[0])
	if strings.HasPrefix(firstLine, "title:") {
		// Custom syntax format
		document, err = ParseCustomSyntax(contentStr, path)
		if err != nil {
			return Post{}, fmt.Errorf("failed parsing custom syntax document %s: %w", path, err)
		}
	} else {
		// Try to parse as XML - if it fails, it might be the old format with loose content
		err = document.ReadFromString(contentStr)
		if err != nil {
			// Old format: has <meta>...</meta> followed by loose content
			// Wrap it in a document structure to make it valid XML
			metaEnd := strings.Index(contentStr, "</meta>")
			if metaEnd != -1 {
				// Create valid XML by wrapping in <document><body>
				wrapped := "<document>" + contentStr[:metaEnd+7] + "<body>" + contentStr[metaEnd+7:] + "</body></document>"
				err = document.ReadFromString(wrapped)
				if err != nil {
					return Post{}, fmt.Errorf("failed parsing post document %s (even after wrapping): %w", path, err)
				}
			} else {
				return Post{}, fmt.Errorf("failed parsing post document %s: %w", path, err)
			}
		}
	}

	key := keylock.AssureKey("POST:" + name)

	title, tags, err := ProcessPostMeta(document, key, taxonomy)
	if err != nil {
		return Post{}, fmt.Errorf("failed reading post document meta: %w", err)
	}

	return Post{
		Name:    name,
		Title:   title,
		Key:     key,
		Content: document,
		Tags:    tags,
	}, nil

}

func ProcessPostMeta(content *etree.Document, key int, taxonomy *Taxonomy) (string, []int, error) {

	// Check if meta is inside document wrapper or at root
	var meta *etree.Element
	docRoot := content.Root()
	if docRoot != nil && docRoot.Tag == "document" {
		meta = docRoot.SelectElement("meta")
	} else {
		meta = content.SelectElement("meta")
	}
	if meta == nil {
		return "", nil, fmt.Errorf("no meta tag found")
	}

	title := meta.SelectElement("title")
	if title == nil {
		return "", nil, fmt.Errorf("no title tag found")
	}

	titleValue := title.SelectAttrValue("value", "")
	if titleValue == "" {
		return "", nil, fmt.Errorf("no value in title tag found")
	}

	labelKeys := []int{}

	tags := meta.SelectElements("tag")

	for _, tag := range tags {
		tagLabel := tag.SelectAttrValue("label", "")
		if tagLabel == "" {
			return "", nil, fmt.Errorf("no label found in a tag")
		}

		labelKey := taxonomy.AssureLabelFromDocument(tagLabel, key)
		labelKeys = append(labelKeys, labelKey)
	}

	return titleValue, labelKeys, nil

}
