package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func copyStatics(srcPath string, dstPath string) error {
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
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
		if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		return copyFile(path, dstFile)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}
