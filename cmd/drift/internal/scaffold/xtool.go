package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/icongen"
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
	for _, dir := range []string{xtoolDir, sourcesDir, resourcesDir, cdriftDir, cskiaDir} {
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

	// Write Package.swift and xtool.yml
	isProjectFile := func(name string) bool {
		return name == "Package.swift.tmpl" || name == "xtool.yml.tmpl"
	}
	if err := templates.CopyTree("xtool", xtoolDir, tmplData, isProjectFile); err != nil {
		return err
	}

	// Write shared Swift files from ios/ (skip AppDelegate, xtool has its own)
	isSwiftFile := func(name string) bool {
		return name != "AppDelegate.swift" &&
			(strings.HasSuffix(name, ".swift") || strings.HasSuffix(name, ".swift.tmpl"))
	}
	if err := templates.CopyTree("ios", sourcesDir, tmplData, isSwiftFile); err != nil {
		return err
	}

	// Write LaunchScreen.storyboard to resources
	if err := templates.CopyTree("ios", resourcesDir, tmplData, func(name string) bool {
		return name == "LaunchScreen.storyboard"
	}); err != nil {
		return err
	}

	// Generate app icon (xtool uses iconPath in xtool.yml, not Assets.xcassets)
	iconSrc, err := icongen.LoadSource(settings.ProjectRoot, settings.Icon)
	if err != nil {
		return fmt.Errorf("failed to load icon: %w", err)
	}
	if err := iconSrc.GenerateIconPNG(filepath.Join(resourcesDir, "AppIcon.png")); err != nil {
		return fmt.Errorf("failed to generate app icon: %w", err)
	}

	// Write xtool-specific files to their destinations
	if err := templates.CopyTree("xtool", resourcesDir, tmplData, func(name string) bool {
		return name == "Info.plist.tmpl"
	}); err != nil {
		return err
	}
	if err := templates.CopyTree("xtool", sourcesDir, tmplData, func(name string) bool {
		return strings.HasSuffix(name, ".swift")
	}); err != nil {
		return err
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
