package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
)

// WriteIOS writes the iOS project files to root.
// If settings.Ejected is true, this returns early without writing anything.
// For ejected builds, bridge files and libraries are handled separately by
// workspace.Prepare and the compile command.
func WriteIOS(root string, settings Settings) error {
	if settings.Ejected {
		return nil
	}

	iosDir := filepath.Join(root, "ios", "Runner")

	// Create template data
	tmplData := templates.NewTemplateData(templates.TemplateInput{
		AppName:        settings.AppName,
		AndroidPackage: settings.AppID,
		IOSBundleID:    settings.Bundle,
		Orientation:    settings.Orientation,
		AllowHTTP:      settings.AllowHTTP,
	})

	writeTemplateFile := func(templatePath, destPath string, perm os.FileMode) error {
		content, err := templates.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", templatePath, err)
		}

		if err := os.WriteFile(destPath, []byte(processed), perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}

		return nil
	}

	// Info.plist

	if err := os.MkdirAll(iosDir, 0o755); err != nil {
		return fmt.Errorf("failed to create ios directory: %w", err)
	}

	if err := writeTemplateFile("ios/Info.plist.tmpl", filepath.Join(iosDir, "Info.plist"), 0o644); err != nil {
		return err
	}

	// Write Swift files from templates
	iosFiles, err := templates.GetIOSFiles()
	if err != nil {
		return fmt.Errorf("failed to list ios templates: %w", err)
	}

	for _, file := range iosFiles {
		if !strings.HasSuffix(file, ".swift") && !strings.HasSuffix(file, ".swift.tmpl") {
			continue
		}

		content, err := templates.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", file, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", file, err)
		}

		destName := templates.FileName(file)
		if strings.HasSuffix(destName, ".tmpl") {
			destName = strings.TrimSuffix(destName, ".tmpl")
		}
		destFile := filepath.Join(iosDir, destName)
		if err := os.WriteFile(destFile, []byte(processed), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destFile, err)
		}
	}

	// LaunchScreen.storyboard
	if err := writeTemplateFile("ios/LaunchScreen.storyboard", filepath.Join(iosDir, "LaunchScreen.storyboard"), 0o644); err != nil {
		return err
	}

	xcodeprojDir := filepath.Join(root, "ios", "Runner.xcodeproj")
	if err := os.MkdirAll(xcodeprojDir, 0o755); err != nil {
		return fmt.Errorf("failed to create xcode project directory: %w", err)
	}

	xcodeFiles, err := templates.GetXcodeProjectFiles()
	if err != nil {
		return fmt.Errorf("failed to list xcode templates: %w", err)
	}

	for _, file := range xcodeFiles {
		content, err := templates.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", file, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", file, err)
		}

		destName := templates.FileName(file)
		if strings.HasSuffix(destName, ".tmpl") {
			destName = strings.TrimSuffix(destName, ".tmpl")
		}
		destFile := filepath.Join(xcodeprojDir, destName)
		if err := os.WriteFile(destFile, []byte(processed), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destFile, err)
		}
	}

	return nil
}
