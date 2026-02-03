package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
)

// WriteXtool writes the SwiftPM project files for xtool-based iOS builds.
// If settings.Ejected is true, this returns early without writing anything.
// For ejected builds, bridge files and libraries are handled separately by
// workspace.Prepare and the compile command.
func WriteXtool(root string, settings Settings) error {
	if settings.Ejected {
		return nil
	}

	xtoolDir := filepath.Join(root, "xtool")
	sourcesDir := filepath.Join(xtoolDir, "Sources", "Runner")
	resourcesDir := filepath.Join(sourcesDir, "Resources")
	cdriftDir := filepath.Join(xtoolDir, "Libraries", "CDrift")
	cskiaDir := filepath.Join(xtoolDir, "Libraries", "CSkia")

	// Create directory structure
	dirs := []string{
		xtoolDir,
		sourcesDir,
		resourcesDir,
		cdriftDir,
		cskiaDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create template data
	tmplData := templates.NewTemplateData(templates.TemplateInput{
		AppName:        settings.AppName,
		AndroidPackage: settings.AppID,
		IOSBundleID:    settings.Bundle,
		Orientation:    settings.Orientation,
		AllowHTTP:      settings.AllowHTTP,
	})

	// Helper to write template file
	writeTemplateFile := func(templatePath, destPath string) error {
		content, err := templates.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", templatePath, err)
		}

		if err := os.WriteFile(destPath, []byte(processed), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}

		return nil
	}

	// Write Package.swift
	if err := writeTemplateFile("xtool/Package.swift.tmpl", filepath.Join(xtoolDir, "Package.swift")); err != nil {
		return err
	}

	// Write xtool.yml (required by xtool for iOS app configuration)
	if err := writeTemplateFile("xtool/xtool.yml.tmpl", filepath.Join(xtoolDir, "xtool.yml")); err != nil {
		return err
	}

	// Write Swift files from ios templates (xtool handles app entry point)
	iosFiles, err := templates.GetIOSFiles()
	if err != nil {
		return fmt.Errorf("failed to list ios templates: %w", err)
	}

	for _, file := range iosFiles {
		baseName := templates.FileName(file)

		// Skip AppDelegate - we use the xtool-specific version
		if baseName == "AppDelegate.swift" {
			continue
		}

		// Handle Swift files
		if strings.HasSuffix(baseName, ".swift") || strings.HasSuffix(baseName, ".swift.tmpl") {
			content, err := templates.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read template %s: %w", file, err)
			}

			processed, err := templates.ProcessTemplate(string(content), tmplData)
			if err != nil {
				return fmt.Errorf("failed to process template %s: %w", file, err)
			}

			destName := baseName
			if strings.HasSuffix(destName, ".tmpl") {
				destName = strings.TrimSuffix(destName, ".tmpl")
			}
			destFile := filepath.Join(sourcesDir, destName)
			if err := os.WriteFile(destFile, []byte(processed), 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", destFile, err)
			}
		}

		// Handle storyboard
		if baseName == "LaunchScreen.storyboard" {
			content, err := templates.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read template %s: %w", file, err)
			}
			destFile := filepath.Join(resourcesDir, baseName)
			if err := os.WriteFile(destFile, content, 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", destFile, err)
			}
		}
	}

	// Write Info.plist from xtool template (customized for SwiftPM)
	if err := writeTemplateFile("xtool/Info.plist.tmpl", filepath.Join(resourcesDir, "Info.plist")); err != nil {
		return err
	}

	// Write xtool-specific AppDelegate.swift (without @main attribute for library target)
	appDelegateContent, err := templates.ReadFile("xtool/AppDelegate.swift")
	if err != nil {
		return fmt.Errorf("failed to read xtool AppDelegate template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sourcesDir, "AppDelegate.swift"), appDelegateContent, 0o644); err != nil {
		return fmt.Errorf("failed to write AppDelegate.swift: %w", err)
	}

	// Write DriftApp.swift (SwiftUI entry point that wraps UIKit)
	driftAppContent, err := templates.ReadFile("xtool/DriftApp.swift")
	if err != nil {
		return fmt.Errorf("failed to read xtool DriftApp.swift template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sourcesDir, "DriftApp.swift"), driftAppContent, 0o644); err != nil {
		return fmt.Errorf("failed to write DriftApp.swift: %w", err)
	}

	// Write module maps for C libraries
	if err := writeCDriftModuleMap(cdriftDir); err != nil {
		return err
	}

	if err := writeCSkiaModuleMap(cskiaDir); err != nil {
		return err
	}

	return nil
}

func writeCDriftModuleMap(dir string) error {
	moduleMap := `module CDrift {
    header "libdrift.h"
    link "drift"
    export *
}
`
	return os.WriteFile(filepath.Join(dir, "module.modulemap"), []byte(moduleMap), 0o644)
}

func writeCSkiaModuleMap(dir string) error {
	moduleMap := `module CSkia {
    link "drift_skia"
    export *
}
`
	return os.WriteFile(filepath.Join(dir, "module.modulemap"), []byte(moduleMap), 0o644)
}
