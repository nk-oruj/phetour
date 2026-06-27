package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func applyStylesheets(xmlOutputPath string, stylesInputPath string) error {
	if _, err := os.Stat(stylesInputPath); os.IsNotExist(err) {
		return nil
	}

	var xslFiles []string
	err := filepath.Walk(stylesInputPath, func(path string, info fs.FileInfo, err error) error {
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

	for _, xslFile := range xslFiles {
		baseName := filepath.Base(xslFile)
		styleName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		styleOutputPath := filepath.Join(filepath.Dir(xmlOutputPath), styleName)
		if err := transformXMLDirectory(xmlOutputPath, styleOutputPath, xslFile, styleName); err != nil {
			return fmt.Errorf("failed to transform with stylesheet %s: %w", xslFile, err)
		}
	}

	return nil
}

func transformXMLDirectory(srcPath, dstPath, xslFile, styleName string) error {
	if err := os.MkdirAll(dstPath, 0755); err != nil {
		return fmt.Errorf("failed to create style output directory: %w", err)
	}

	return filepath.Walk(srcPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dstPath, relPath), 0755)
		}

		dstFile := filepath.Join(dstPath, relPath)

		if strings.ToLower(filepath.Ext(path)) != ".xml" {
			return copyFile(path, dstFile)
		}

		dstFile = strings.TrimSuffix(dstFile, ".xml") + "." + styleName
		if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}

		return transformWithXsltproc(path, dstFile, xslFile)
	})
}

func transformWithXsltproc(xmlPath, dstPath, xslPath string) error {
	cmd := exec.Command("xsltproc", "-o", dstPath, xslPath, xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errStr := strings.ToLower(string(output))
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "not recognized") ||
			strings.Contains(errStr, "command not found") {
			cmd = exec.Command("msxsl.exe", xmlPath, xslPath, "-o", dstPath)
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("XSLT transformation failed (xsltproc/msxsl unavailable): %s", string(output))
			}
		} else {
			return fmt.Errorf("XSLT transformation failed: %s", string(output))
		}
	}
	return nil
}
