package plugin

import (
	"fmt"
	"path/filepath"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// project is a native project being built at dir for platform ("ios",
// "xtool" or "android"): managed (regenerated every build) or ejected. It
// knows where every file plugin output touches lives. Each location is a
// project-relative, slash-separated path, the form the ejected lock and
// messages use; abs turns one into a path on disk.
type project struct {
	dir      string
	platform string
}

// abs returns rel's path on disk.
func (p project) abs(rel string) string {
	return filepath.Join(p.dir, filepath.FromSlash(rel))
}

// rel returns path, on disk under the project, project-relative and
// slash-separated.
func (p project) rel(path string) (string, error) {
	rel, err := filepath.Rel(p.dir, path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (p project) xtool() bool { return p.platform == "xtool" }

// fileDir is the directory of an OpAddFile location.
func (p project) fileDir(loc protocol.FileLocation) string {
	switch loc {
	case protocol.FileIOSBundle, protocol.FileIOSPluginConfig:
		return p.bundleResources()
	case protocol.FileAndroidDrawable:
		return p.drawables()
	case protocol.FileAndroidResource:
		return p.resDir()
	case protocol.FileAndroidAppModule:
		return p.appModule()
	case protocol.FileAndroidPluginConfig:
		return p.pluginConfigs()
	}
	panic(fmt.Sprintf("plugin: unknown file location %q", loc))
}

// ---- iOS ----------------------------------------------------------------

// iosSources is where the app target's Swift sources live: Runner/ for
// xcodeproj builds (managed and ejected), Sources/Runner/ for xtool.
func (p project) iosSources() string {
	if p.xtool() {
		return "Sources/Runner"
	}
	return "Runner"
}

// plist returns the path of an iOS plist. The entitlements sit at the
// project root on both paths, outside the sources so neither build copies
// them into the bundle (xcodeproj: CODE_SIGN_ENTITLEMENTS; xtool:
// entitlementsPath in xtool.yml).
func (p project) plist(file protocol.PlistFile) string {
	switch file {
	case protocol.PlistInfo:
		if p.xtool() {
			return "Sources/Runner/Resources/Info.plist"
		}
		return "Runner/Info.plist"
	case protocol.PlistEntitlements:
		return "Runner.entitlements"
	}
	panic(fmt.Sprintf("plugin: unknown plist file %q", file))
}

func (p project) launchScreen() string {
	if p.xtool() {
		return "Sources/Runner/Resources/LaunchScreen.storyboard"
	}
	return "Runner/LaunchScreen.storyboard"
}

func (p project) assetCatalog() string { return "Runner/Assets.xcassets" }

// bundleResources is the Drift-owned directory for files destined for the
// root of the app bundle (protocol.FileIOSBundle, plugin configs, plus
// image sets on xtool). On xcodeproj builds it is Runner/PluginResources:
// the PBXFileSystemSynchronizedRootGroup includes it and Xcode's resources
// phase copies the files flat into the bundle root. Keeping it out of
// Runner/ itself means a resource can never shadow a template source file.
// On xtool builds it is PluginResources/ at the project root, outside
// Sources/Runner so SwiftPM never sees the files (SwiftPM would bundle them
// into Runner_Runner.bundle, invisible to Bundle.main); xtool.yml's
// `resources:` list copies them into the app bundle root instead.
func (p project) bundleResources() string {
	if p.xtool() {
		return "PluginResources"
	}
	return "Runner/PluginResources"
}

// xtoolYML is xtool's project file, which lists the bundle resources.
func (p project) xtoolYML() string { return "xtool.yml" }

// spmPackage is the SwiftPM sidecar package. Both build paths use the same
// layout: Drift/Plugins/ at the iOS project root. The xtool Package.swift
// references it relative (.package(path: "Drift/Plugins")), the xcodeproj
// wires it via an XCLocalSwiftPackageReference with the same relativePath.
func (p project) spmPackage() string { return "Drift/Plugins" }

// ---- Android ------------------------------------------------------------

func (p project) appModule() string     { return "app" }
func (p project) appGradle() string     { return "app/build.gradle" }
func (p project) projectGradle() string { return "build.gradle" }
func (p project) manifest() string      { return "app/src/main/AndroidManifest.xml" }
func (p project) resDir() string        { return "app/src/main/res" }
func (p project) javaRoot() string      { return "app/src/main/java" }

// runnerPackage is the directory of Drift's Android host support files and
// generated registrant (protocol.AndroidRunnerPackage).
func (p project) runnerPackage() string { return p.javaRoot() + "/com/drift/runner" }

// drawables holds plugin bitmaps (protocol.AndroidDrawableDir).
func (p project) drawables() string { return p.resDir() + "/" + protocol.AndroidDrawableDir }

// pluginConfigs is the Drift-owned asset directory of plugin configs
// (protocol.AndroidPluginConfigAssetDir).
func (p project) pluginConfigs() string {
	return "app/src/main/assets/" + protocol.AndroidPluginConfigAssetDir
}
