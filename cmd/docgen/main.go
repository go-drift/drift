// Package main provides a documentation generator for Drift.
// It copies hand-written guides and generates API documentation
// from Go source code using gomarkdoc.
package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Package represents a Go package to document.
type Package struct {
	Name     string
	Path     string
	Position int
}

// Packages to document (public-facing), in order.
var packages = []Package{
	{Name: "core", Path: "pkg/core", Position: 1},
	{Name: "drift", Path: "pkg/drift", Position: 2},
	{Name: "widgets", Path: "pkg/widgets", Position: 3},
	{Name: "layout", Path: "pkg/layout", Position: 4},
	{Name: "graphics", Path: "pkg/graphics", Position: 5},
	{Name: "theme", Path: "pkg/theme", Position: 6},
	{Name: "animation", Path: "pkg/animation", Position: 7},
	{Name: "navigation", Path: "pkg/navigation", Position: 8},
	{Name: "gestures", Path: "pkg/gestures", Position: 9},
	{Name: "focus", Path: "pkg/focus", Position: 10},
	{Name: "platform", Path: "pkg/platform", Position: 11},
	{Name: "errors", Path: "pkg/errors", Position: 12},
	{Name: "validation", Path: "pkg/validation", Position: 13},
	{Name: "accessibility", Path: "pkg/accessibility", Position: 14},
}

func main() {
	// Find repository root (where go.mod is)
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding repo root: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Repository root: %s\n", root)

	// Ensure gomarkdoc is installed
	if err := ensureGomarkdoc(); err != nil {
		fmt.Fprintf(os.Stderr, "Error ensuring gomarkdoc: %v\n", err)
		os.Exit(1)
	}

	// Create output directories
	docsDir := filepath.Join(root, "website", "docs")
	apiDir := filepath.Join(docsDir, "api")

	if err := os.MkdirAll(apiDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating api directory: %v\n", err)
		os.Exit(1)
	}

	// Copy hand-written docs from website-docs/ to website/docs/
	websiteDocsDir := filepath.Join(root, "website-docs")
	if err := copyDir(websiteDocsDir, docsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying website-docs: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Copied website-docs/ to website/docs/")

	// Create API category file
	if err := writeAPICategoryFile(apiDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing API category file: %v\n", err)
		os.Exit(1)
	}

	// Generate API docs for each package
	for _, pkg := range packages {
		pkgPath := filepath.Join(root, pkg.Path)
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			fmt.Printf("Skipping %s (not found)\n", pkg.Name)
			continue
		}

		fmt.Printf("Generating docs for %s...\n", pkg.Name)
		if err := generatePackageDocs(root, pkg, apiDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating docs for %s: %v\n", pkg.Name, err)
			os.Exit(1)
		}
	}

	fmt.Println("\nDocumentation generated successfully!")
	fmt.Println("Run 'cd website && npm start' to preview")
}

func findRepoRoot() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

func ensureGomarkdoc() error {
	// Check if gomarkdoc is installed
	if _, err := exec.LookPath("gomarkdoc"); err == nil {
		return nil
	}

	fmt.Println("Installing gomarkdoc...")
	cmd := exec.Command("go", "install", "github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyDir(src, dst string) error {
	// Check if source exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Warning: %s does not exist, skipping copy\n", src)
			return nil
		}
		return err
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func writeAPICategoryFile(apiDir string) error {
	content := `{
  "label": "API Reference",
  "position": 100,
  "link": {
    "type": "generated-index",
    "description": "API documentation generated from Go source code."
  }
}
`
	return os.WriteFile(filepath.Join(apiDir, "_category_.json"), []byte(content), 0644)
}

func generatePackageDocs(root string, pkg Package, apiDir string) error {
	pkgPath := "./" + pkg.Path

	// Run gomarkdoc with darwin tag to include platform-specific code
	// (darwin works for both macOS and iOS targets)
	cmd := exec.Command("gomarkdoc", "--tags", "darwin", pkgPath)
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Log warning but don't fail - some packages may have complex build constraints
		fmt.Printf("  Warning: skipping %s (gomarkdoc error)\n", pkg.Name)
		return nil
	}

	// Get the output and add Docusaurus frontmatter
	content := stdout.String()
	if content == "" {
		fmt.Printf("  Warning: no documentation generated for %s\n", pkg.Name)
		return nil
	}

	// Process the markdown content
	content = processMarkdown(pkg, content)

	// Add frontmatter
	frontmatter := fmt.Sprintf(`---
id: %s
title: %s
sidebar_position: %d
---

`, pkg.Name, formatTitle(pkg.Name), pkg.Position)

	finalContent := frontmatter + content

	// Write to file
	outputPath := filepath.Join(apiDir, pkg.Name+".md")
	return os.WriteFile(outputPath, []byte(finalContent), 0644)
}

func formatTitle(name string) string {
	// Capitalize first letter and handle special cases
	titles := map[string]string{
		"core":          "Core",
		"drift":         "Drift",
		"widgets":       "Widgets",
		"layout":        "Layout",
		"rendering":     "Rendering",
		"theme":         "Theme",
		"animation":     "Animation",
		"navigation":    "Navigation",
		"gestures":      "Gestures",
		"focus":         "Focus",
		"platform":      "Platform",
		"errors":        "Errors",
		"validation":    "Validation",
		"accessibility": "Accessibility",
	}

	if title, ok := titles[name]; ok {
		return title
	}
	return strings.Title(name)
}

func processMarkdown(pkg Package, content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skipNext := false
	inIndex := false
	inCodeBlock := false

	for i, line := range lines {
		// Skip the first header line since we add our own title
		if i == 0 && strings.HasPrefix(line, "# ") {
			continue
		}

		// Skip the Index section (starts with "## Index", ends at next ## heading)
		if line == "## Index" {
			inIndex = true
			continue
		}
		if inIndex {
			// End of index section when we hit another ## heading
			if strings.HasPrefix(line, "## ") {
				inIndex = false
				// Fall through to process this line
			} else {
				continue
			}
		}

		// Track code block state for language tagging below.
		// A line starting with ``` followed by a language tag opens a block.
		if strings.HasPrefix(line, "```") && line != "```" {
			inCodeBlock = true
		}

		// Skip "import" lines that show the import path
		if strings.HasPrefix(line, "```go") && i+1 < len(lines) && strings.Contains(lines[i+1], "import") {
			skipNext = true
		}
		if skipNext && line == "```" {
			inCodeBlock = false
			skipNext = false
			continue
		}
		if skipNext {
			continue
		}

		// Convert <details><summary>Example</summary> to **Example:**
		if strings.HasPrefix(line, "<details><summary>") && strings.HasSuffix(line, "</summary>") {
			// Extract the summary text
			summary := line[len("<details><summary>") : len(line)-len("</summary>")]
			result = append(result, "")
			result = append(result, fmt.Sprintf("**%s:**", summary))
			result = append(result, "")
			continue
		}

		// Skip </details>, <p>, and </p> tags from gomarkdoc
		if line == "</details>" || line == "<p>" || line == "</p>" {
			continue
		}

		// Tag bare code fences from doc comments as Go for syntax highlighting.
		// gomarkdoc emits ``` (no language) for indented code blocks in doc
		// comments, but ```go for Example function bodies. Docusaurus/Prism
		// only highlights fences with an explicit language tag.
		if line == "```" {
			if inCodeBlock {
				inCodeBlock = false
			} else {
				inCodeBlock = true
				result = append(result, "```go")
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// sortPackages returns packages sorted by name (for stable output).
func sortPackages(pkgs []Package) []Package {
	sorted := make([]Package, len(pkgs))
	copy(sorted, pkgs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}
