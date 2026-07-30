package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runBuild(force bool) error {
	fmt.Println("Build: fully regenerate output/")
	proceed, err := confirmPlan(force)
	if err != nil {
		return err
	}
	if !proceed {
		fmt.Println("Cancelled.")
		return nil
	}

	keylock, err := LoadKeylock()
	if err != nil {
		return err
	}
	taxonomy := NewTaxonomy(keylock)
	source, err := LoadSource(keylock, taxonomy)
	if err != nil {
		return err
	}
	if err := Build(source, taxonomy); err != nil {
		return err
	}
	if err := keylock.Save(); err != nil {
		return fmt.Errorf("failed to save lock file: %w", err)
	}
	return nil
}

func Build(source *Source, taxonomy *Taxonomy) error {
	const xmlOutputPath = "./output/xml"
	const staticsInputPath = "./input/statics"
	const stylesInputPath = "./input/styles"

	outputParent := filepath.Dir(xmlOutputPath)
	if entries, err := os.ReadDir(outputParent); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				if err := os.RemoveAll(filepath.Join(outputParent, entry.Name())); err != nil {
					return fmt.Errorf("failed to remove output directory %s: %w", entry.Name(), err)
				}
			}
		}
	}

	if err := os.MkdirAll(xmlOutputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, post := range source.Posts {
		if err := buildPost(post, xmlOutputPath, taxonomy); err != nil {
			return fmt.Errorf("failed to build post %s: %w", post.Name, err)
		}
	}

	for _, tag := range taxonomy.Tags {
		if err := buildTag(tag, xmlOutputPath, source); err != nil {
			return fmt.Errorf("failed to build tag %s: %w", tag.Label, err)
		}
	}

	if err := buildHomeCatalog(taxonomy, xmlOutputPath); err != nil {
		return fmt.Errorf("failed to build home catalog: %w", err)
	}

	if err := copyStatics(staticsInputPath, xmlOutputPath); err != nil {
		return fmt.Errorf("failed to copy static files: %w", err)
	}

	if err := applyStylesheets(xmlOutputPath, stylesInputPath); err != nil {
		return fmt.Errorf("failed to apply stylesheets: %w", err)
	}

	return nil
}
