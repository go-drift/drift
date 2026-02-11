package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/icongen"
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

	// Write iOS template files (Info.plist, Swift sources, LaunchScreen.storyboard)
	isIOSFile := func(name string) bool {
		return strings.HasSuffix(name, ".swift") ||
			strings.HasSuffix(name, ".swift.tmpl") ||
			name == "LaunchScreen.storyboard" ||
			name == "Info.plist.tmpl"
	}
	if err := templates.CopyTree("ios", iosDir, tmplData, isIOSFile); err != nil {
		return err
	}

	// Generate app icon assets
	assetDir := filepath.Join(iosDir, "Assets.xcassets")
	iconSrc, err := icongen.LoadSource(settings.ProjectRoot, settings.Icon)
	if err != nil {
		return fmt.Errorf("failed to load icon: %w", err)
	}
	if err := iconSrc.GenerateIOS(assetDir); err != nil {
		return fmt.Errorf("failed to generate iOS icons: %w", err)
	}

	// Write Xcode project files
	xcodeprojDir := filepath.Join(root, "ios", "Runner.xcodeproj")
	if err := templates.CopyTree("xcodeproj", xcodeprojDir, tmplData, nil); err != nil {
		return err
	}

	return nil
}
