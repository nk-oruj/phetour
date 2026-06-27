package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/beevik/etree"
)

func KeyIDToHex(id int) string {
	return fmt.Sprintf("0x%04x", id)
}

func copyElementChildren(src, dst *etree.Element) {
	for _, child := range src.Child {
		if elem, ok := child.(*etree.Element); ok {
			newElem := dst.CreateElement(elem.Tag)
			for _, attr := range elem.Attr {
				newElem.CreateAttr(attr.Key, attr.Value)
			}
			copyElementChildren(elem, newElem)
		} else if charData, ok := child.(*etree.CharData); ok {
			dst.CreateText(string(charData.Data))
		}
	}
}

func buildPost(post Post, outputPath string, taxonomy *Taxonomy) error {
	postDir := filepath.Join(outputPath, KeyIDToHex(post.Key))
	if err := os.MkdirAll(postDir, 0755); err != nil {
		return fmt.Errorf("failed to create post directory: %w", err)
	}

	doc := etree.NewDocument()
	docRoot := doc.CreateElement("document")

	srcRoot := post.Content.Root()
	srcMeta := srcRoot.SelectElement("meta")

	meta := docRoot.CreateElement("meta")
	meta.CreateElement("title").CreateAttr("value", post.Title)
	for _, srcTag := range srcMeta.SelectElements("tag") {
		tagLabel := srcTag.SelectAttrValue("label", "")
		tag := meta.CreateElement("tag")
		tag.CreateAttr("label", tagLabel)
		for _, t := range taxonomy.Tags {
			if t.Label == tagLabel {
				tag.CreateAttr("id", KeyIDToHex(t.Key))
				break
			}
		}
	}

	body := docRoot.CreateElement("body")
	body.CreateElement("bold").CreateText(post.Title)

	for _, srcTag := range srcMeta.SelectElements("tag") {
		tagLabel := srcTag.SelectAttrValue("label", "")
		for _, t := range taxonomy.Tags {
			if t.Label == tagLabel {
				link := body.CreateElement("link")
				link.CreateAttr("href", "/"+KeyIDToHex(t.Key)+"/")
				link.CreateText(KeyIDToHex(t.Key) + " - " + tagLabel)
				break
			}
		}
	}

	srcBody := srcRoot.SelectElement("body")
	for _, child := range srcBody.Child {
		if elem, ok := child.(*etree.Element); ok {
			switch elem.Tag {
			case "bold", "text", "code", "item", "link":
				newElem := body.CreateElement(elem.Tag)
				for _, attr := range elem.Attr {
					newElem.CreateAttr(attr.Key, attr.Value)
				}
				copyElementChildren(elem, newElem)
			}
		} else if charData, ok := child.(*etree.CharData); ok {
			body.CreateText(string(charData.Data))
		}
	}

	doc.Indent(4)
	if err := doc.WriteToFile(filepath.Join(postDir, "index.xml")); err != nil {
		return fmt.Errorf("failed to write post index.xml: %w", err)
	}

	return nil
}

func buildTag(tag Tag, outputPath string, source *Source) error {
	tagDir := filepath.Join(outputPath, KeyIDToHex(tag.Key))
	if err := os.MkdirAll(tagDir, 0755); err != nil {
		return fmt.Errorf("failed to create tag directory: %w", err)
	}

	doc := etree.NewDocument()
	docRoot := doc.CreateElement("document")
	docRoot.CreateElement("meta").CreateElement("title").CreateAttr("value", tag.Label)

	body := docRoot.CreateElement("body")
	body.CreateElement("bold").CreateText(tag.Label)

	slices.SortFunc(tag.Mentions, func(a, b int) int { return -cmp.Compare(a, b) })

	for _, mentionID := range tag.Mentions {
		for _, post := range source.Posts {
			if post.Key == mentionID {
				link := body.CreateElement("link")
				link.CreateAttr("href", "/"+KeyIDToHex(mentionID)+"/")
				link.CreateText(fmt.Sprintf("%s - %s", KeyIDToHex(mentionID), post.Title))
				break
			}
		}
	}

	doc.Indent(4)
	if err := doc.WriteToFile(filepath.Join(tagDir, "index.xml")); err != nil {
		return fmt.Errorf("failed to write tag index.xml: %w", err)
	}

	return nil
}

func buildHomeCatalog(source *Source, taxonomy *Taxonomy, outputPath string) error {
	doc := etree.NewDocument()
	docRoot := doc.CreateElement("document")
	docRoot.CreateElement("meta").CreateElement("title").CreateAttr("value", "փետուր")

	body := docRoot.CreateElement("body")

	slices.SortFunc(source.Posts, func(a, b Post) int { return -cmp.Compare(a.Key, b.Key) })

	for _, post := range source.Posts {
		link := body.CreateElement("link")
		link.CreateAttr("href", "/"+KeyIDToHex(post.Key)+"/")
		link.CreateText(fmt.Sprintf("%s - %s", KeyIDToHex(post.Key), post.Title))
	}

	body.CreateElement("text").CreateText("")

	slices.SortFunc(taxonomy.Tags, func(a, b Tag) int { return -cmp.Compare(a.Key, b.Key) })

	for _, tag := range taxonomy.Tags {
		link := body.CreateElement("link")
		link.CreateAttr("href", "/"+KeyIDToHex(tag.Key)+"/")
		link.CreateText(fmt.Sprintf("%s - %s", KeyIDToHex(tag.Key), tag.Label))
	}

	doc.Indent(4)
	if err := doc.WriteToFile(filepath.Join(outputPath, "index.xml")); err != nil {
		return fmt.Errorf("failed to write home catalog: %w", err)
	}

	return nil
}
