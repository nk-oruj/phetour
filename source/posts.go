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

func LoadSource(keylock *Keylock, taxonomy *Taxonomy) (*Source, error) {
	source := &Source{Posts: []Post{}}

	err := filepath.Walk(postsPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name()[0] == '~' {
			return nil
		}

		post, err := loadPost(path, info.Name(), keylock, taxonomy)
		if err != nil {
			return fmt.Errorf("failed loading post %s: %w", path, err)
		}

		source.Posts = append(source.Posts, post)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed reading posts folder: %w", err)
	}

	return source, nil
}

func loadPost(path string, name string, keylock *Keylock, taxonomy *Taxonomy) (Post, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return Post{}, fmt.Errorf("failed reading file: %w", err)
	}

	document, err := readPostDocument(string(contentBytes), path)
	if err != nil {
		return Post{}, fmt.Errorf("failed parsing document: %w", err)
	}

	key := keylock.AssureKey("POST:" + name)

	title, tags, err := extractPostMeta(document, key, taxonomy)
	if err != nil {
		return Post{}, fmt.Errorf("failed reading meta: %w", err)
	}

	return Post{
		Name:    name,
		Title:   title,
		Key:     key,
		Content: document,
		Tags:    tags,
	}, nil
}

func readPostDocument(content string, path string) (*etree.Document, error) {
	var firstLine string
	for _, line := range strings.Split(content, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			firstLine = trimmed
			break
		}
	}

	if strings.HasPrefix(firstLine, "#") {
		return parseDocument(content, path)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(content); err != nil {
		return nil, fmt.Errorf("failed to parse as XML: %w", err)
	}
	return doc, nil
}

func extractPostMeta(content *etree.Document, key int, taxonomy *Taxonomy) (string, []int, error) {
	meta := content.Root().SelectElement("meta")
	if meta == nil {
		return "", nil, fmt.Errorf("no meta element found")
	}

	titleElem := meta.SelectElement("title")
	if titleElem == nil {
		return "", nil, fmt.Errorf("no title element found")
	}

	titleValue := titleElem.SelectAttrValue("value", "")
	if titleValue == "" {
		return "", nil, fmt.Errorf("title value is empty")
	}

	var labelKeys []int
	for _, tagElem := range meta.SelectElements("tag") {
		tagLabel := tagElem.SelectAttrValue("label", "")
		if tagLabel == "" {
			return "", nil, fmt.Errorf("tag element with empty label found")
		}
		t := taxonomy.AssureTag(tagLabel)
		t.AssureMention(key)
		labelKeys = append(labelKeys, t.Key)
	}

	return titleValue, labelKeys, nil
}
