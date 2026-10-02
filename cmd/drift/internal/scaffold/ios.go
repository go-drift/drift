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
	tmplData := templates.NewTemplateData(templates.TemplateInput{
		AppName:        settings.AppName,
		AndroidPackage: settings.AppID,
		IOSBundleID:    settings.Bundle,
		Orientation:    settings.Orientation,
		AllowHTTP:      settings.AllowHTTP,
	})
	return WriteIOSProject(filepath.Join(root, "ios"), tmplData, settings.ProjectRoot, settings.Icon)
}

// WriteIOSProject writes a complete Xcode project into dir: the Runner
// sources, Info.plist and launch screen, app icons, Runner.entitlements and
// Runner.xcodeproj. Managed builds and `drift eject` both write through it,
// so an ejected project has every file the project template names.
func WriteIOSProject(dir string, data *templates.TemplateData, projectRoot, icon string) error {
	runnerDir := filepath.Join(dir, "Runner")

	// Write iOS template files (Info.plist, Swift sources, LaunchScreen.storyboard)
	isIOSFile := func(name string) bool {
		return strings.HasSuffix(name, ".swift") ||
			strings.HasSuffix(name, ".swift.tmpl") ||
			name == "LaunchScreen.storyboard" ||
			name == "Info.plist.tmpl"
	}
	if err := templates.CopyTree("ios", runnerDir, data, isIOSFile); err != nil {
		return err
	}

	// Generate app icon assets
	iconSrc, err := icongen.LoadSource(projectRoot, icon)
	if err != nil {
		return fmt.Errorf("failed to load icon: %w", err)
	}
	if err := iconSrc.GenerateIOS(filepath.Join(runnerDir, "Assets.xcassets")); err != nil {
		return fmt.Errorf("failed to generate iOS icons: %w", err)
	}

	// Entitlements at the project root, outside the synchronized Runner
	// folder so they are never a bundle resource; CODE_SIGN_ENTITLEMENTS
	// names them. Plugins add keys (ctx.IOS.Entitlements).
	if err := copyEntitlements(dir, data); err != nil {
		return err
	}

	// Write Xcode project files
	return templates.CopyTree("xcodeproj", filepath.Join(dir, "Runner.xcodeproj"), data, nil)
}

// copyEntitlements writes the empty Runner.entitlements template into dir,
// the project root of an iOS build path.
func copyEntitlements(dir string, data *templates.TemplateData) error {
	return templates.CopyTree("ios", dir, data, func(name string) bool {
		return name == "Runner.entitlements"
	})
}
