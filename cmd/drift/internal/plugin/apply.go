package plugin

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/cmd/drift/internal/templates"
	pkgerrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// Apply mutates the rendered project at buildDir according to the validated
// op list. Returns the list of files whose bytes actually changed.
//
// Apply skips ops that do not match the target platform; plugins may
// unconditionally emit ops for every platform.
func Apply(ops []protocol.Op, buildDir, platform string) ([]string, error) {
	bag := bundleByPlatform(ops, platform)

	changed := make(map[string]bool)
	record := func(paths ...string) {
		for _, p := range paths {
			if p == "" {
				continue
			}
			changed[p] = true
		}
	}

	if platform == "ios" || platform == "xtool" {
		ios, err := applyIOSOps(bag, buildDir, platform)
		if err != nil {
			return changedSorted(changed), err
		}
		record(ios...)
	}
	if platform == "android" {
		droid, err := applyAndroidOps(bag, buildDir)
		if err != nil {
			return changedSorted(changed), err
		}
		record(droid...)
	}

	return changedSorted(changed), nil
}

type opBag struct {
	infoString    []*protocol.OpInfoPlistSetString
	infoBool      []*protocol.OpInfoPlistSetBool
	infoArray     []*protocol.OpInfoPlistSetStringArray
	infoAppend    []*protocol.OpInfoPlistAppendArrayItem
	infoDict      []*protocol.OpInfoPlistSetDict
	iosAssets     []*protocol.OpIOSAssetsAddImageSet
	iosLaunch     []*protocol.OpIOSReplaceLaunchScreen
	iosSources    []*protocol.OpAddIOSSource
	iosBundle     []*protocol.OpIOSAddBundleResource
	iosSPM        []*protocol.OpIOSAddPackageDependency
	iosPlugins    []*protocol.OpIOSPlugin
	addPerm       []*protocol.OpAndroidManifestAddPermission
	addIntent     []*protocol.OpAndroidManifestAddIntentFilter
	setActAttr    []*protocol.OpAndroidManifestSetActivityAttr
	addMeta       []*protocol.OpAndroidManifestAddMetaData
	colors        []*protocol.OpAndroidColorSet
	strings       []*protocol.OpAndroidStringSet
	styles        []*protocol.OpAndroidStyleSet
	drawables     []*protocol.OpAndroidWriteDrawable
	resXML        []*protocol.OpAndroidWriteResourceXML
	kotlinSources []*protocol.OpAddKotlinSource
	gradleDeps    []*protocol.OpAndroidGradleAddDependency
	androidAssets []*protocol.OpAndroidAddAsset
	appModule     []*protocol.OpAndroidAddAppModuleFile
	gradlePlugins []*protocol.OpAndroidGradleApplyPlugin
}

func bundleByPlatform(ops []protocol.Op, platform string) *opBag {
	bag := &opBag{}
	for _, op := range ops {
		if !opAppliesTo(op, platform) {
			continue
		}
		if !bundleOp(bag, op) {
			reportUnknownOp(op)
		}
	}
	return bag
}

// bundleOp files op into the bag. Returns false for an op type Apply does
// not know, which means a new op was added to pkg/plugin without teaching
// Apply about it (TestApplyKnowsEveryOpType guards this).
func bundleOp(bag *opBag, op protocol.Op) bool {
	switch op.Platform() {
	case "ios":
		return bundleIOSOp(bag, op)
	case "android":
		return bundleAndroidOp(bag, op)
	default:
		return false
	}
}

func opAppliesTo(op protocol.Op, platform string) bool {
	switch op.Platform() {
	case "ios":
		return platform == "ios" || platform == "xtool"
	case "android":
		return platform == "android"
	default:
		return true
	}
}

func bundleIOSOp(bag *opBag, op protocol.Op) bool {
	switch v := op.(type) {
	case *protocol.OpInfoPlistSetString:
		bag.infoString = append(bag.infoString, v)
	case *protocol.OpInfoPlistSetBool:
		bag.infoBool = append(bag.infoBool, v)
	case *protocol.OpInfoPlistSetStringArray:
		bag.infoArray = append(bag.infoArray, v)
	case *protocol.OpInfoPlistAppendArrayItem:
		bag.infoAppend = append(bag.infoAppend, v)
	case *protocol.OpInfoPlistSetDict:
		bag.infoDict = append(bag.infoDict, v)
	case *protocol.OpIOSAssetsAddImageSet:
		bag.iosAssets = append(bag.iosAssets, v)
	case *protocol.OpIOSReplaceLaunchScreen:
		bag.iosLaunch = append(bag.iosLaunch, v)
	case *protocol.OpAddIOSSource:
		bag.iosSources = append(bag.iosSources, v)
	case *protocol.OpIOSAddBundleResource:
		bag.iosBundle = append(bag.iosBundle, v)
	case *protocol.OpIOSAddPackageDependency:
		bag.iosSPM = append(bag.iosSPM, v)
	case *protocol.OpIOSPlugin:
		// Rendered by WriteRegistrant; Apply only checks it has a module.
		bag.iosPlugins = append(bag.iosPlugins, v)
	default:
		return false
	}
	return true
}

func bundleAndroidOp(bag *opBag, op protocol.Op) bool {
	switch v := op.(type) {
	case *protocol.OpAndroidManifestAddPermission:
		bag.addPerm = append(bag.addPerm, v)
	case *protocol.OpAndroidManifestAddIntentFilter:
		bag.addIntent = append(bag.addIntent, v)
	case *protocol.OpAndroidManifestSetActivityAttr:
		bag.setActAttr = append(bag.setActAttr, v)
	case *protocol.OpAndroidManifestAddMetaData:
		bag.addMeta = append(bag.addMeta, v)
	case *protocol.OpAndroidColorSet:
		bag.colors = append(bag.colors, v)
	case *protocol.OpAndroidStringSet:
		bag.strings = append(bag.strings, v)
	case *protocol.OpAndroidStyleSet:
		bag.styles = append(bag.styles, v)
	case *protocol.OpAndroidWriteDrawable:
		bag.drawables = append(bag.drawables, v)
	case *protocol.OpAndroidWriteResourceXML:
		bag.resXML = append(bag.resXML, v)
	case *protocol.OpAddKotlinSource:
		bag.kotlinSources = append(bag.kotlinSources, v)
	case *protocol.OpAndroidGradleAddDependency:
		bag.gradleDeps = append(bag.gradleDeps, v)
	case *protocol.OpAndroidAddAsset:
		bag.androidAssets = append(bag.androidAssets, v)
	case *protocol.OpAndroidAddAppModuleFile:
		bag.appModule = append(bag.appModule, v)
	case *protocol.OpAndroidGradleApplyPlugin:
		bag.gradlePlugins = append(bag.gradlePlugins, v)
	case *protocol.OpAndroidPlugin:
		// Consumed by WriteRegistrant, not Apply.
	default:
		return false
	}
	return true
}

// reportUnknownOp surfaces unrecognised op types via the drift error reporter,
// mirroring the boundary-parser convention in pkg/platform/stream.go.
func reportUnknownOp(op protocol.Op) {
	pkgerrors.Report(&pkgerrors.DriftError{
		Op:   "plugin.Apply",
		Kind: pkgerrors.KindParsing,
		Err:  fmt.Errorf("unknown op %T", op),
	})
}

func applyIOSOps(bag *opBag, buildDir, platform string) ([]string, error) {
	var changed []string

	infoPlistPath := iosInfoPlistPath(buildDir, platform)
	if len(bag.infoString)+len(bag.infoBool)+len(bag.infoArray)+len(bag.infoAppend)+len(bag.infoDict) > 0 {
		ch, err := mutate.ApplyInfoPlist(infoPlistPath, bag.infoString, bag.infoBool, bag.infoArray, bag.infoAppend, bag.infoDict)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, infoPlistPath)
		}
	}

	// Files for the root of the app bundle. On xtool, image sets join them as
	// loose PNGs because xtool cannot compile asset catalogs.
	bundleFiles, err := mutate.IOSBundleFiles(bag.iosBundle)
	if err != nil {
		return changed, err
	}
	if platform == "xtool" {
		// FUTURE(xtool#219): write image sets with mutate.WriteIOSAssets into
		// a Drift-owned Assets.xcassets (listed under xtool.yml
		// `assetCatalogs:`) like xcodeproj builds, and drop this conversion.
		// https://github.com/xtool-org/xtool/pull/219
		images, err := mutate.ImageSetBundleFiles(bag.iosAssets)
		if err != nil {
			return changed, err
		}
		bundleFiles = append(bundleFiles, images...)
	} else if len(bag.iosAssets) > 0 {
		paths, err := mutate.WriteIOSAssets(iosAssetCatalog(buildDir), bag.iosAssets)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.iosLaunch) > 0 {
		path, ch, err := mutate.ReplaceLaunchScreen(iosLaunchScreenPath(buildDir, platform), bag.iosLaunch[0])
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	// Always synced, even with zero files, so resources from a removed
	// plugin are pruned.
	paths, err := mutate.WriteIOSBundleResources(iosBundleResourcesDir(buildDir, platform), bundleFiles)
	if err != nil {
		return changed, err
	}
	changed = append(changed, paths...)

	if platform == "xtool" {
		resources := make([]string, 0, len(bundleFiles))
		for _, f := range bundleFiles {
			resources = append(resources, xtoolPluginResourcesDir+"/"+f.Name)
		}
		ymlPath := filepath.Join(buildDir, "xtool.yml")
		ch, err := mutate.SetXtoolResources(ymlPath, resources)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, ymlPath)
		}
	}

	// The Drift/Plugins package always regenerates, even with zero ops: it
	// holds the plugin API module the app imports, and the xtool
	// Package.swift and xcodeproj local-package reference expect it.
	pkg, err := pluginPackage(bag)
	if err != nil {
		return changed, err
	}
	spmPaths, err := mutate.WritePluginPackage(iosSPMPackageRoot(buildDir), pkg)
	if err != nil {
		return changed, err
	}
	changed = append(changed, spmPaths...)

	return changed, nil
}

// pluginPackage assembles the Drift/Plugins package for bag, and checks
// that every plugin naming an iOS plugin class ships the Swift sources its
// module needs.
func pluginPackage(bag *opBag) (mutate.PluginPackage, error) {
	api := map[string][]byte{}
	entries, err := templates.FS.ReadDir(pluginAPITemplates)
	if err != nil {
		return mutate.PluginPackage{}, fmt.Errorf("read plugin API templates: %w", err)
	}
	for _, e := range entries {
		content, err := templates.ReadFile(pluginAPITemplates + "/" + e.Name())
		if err != nil {
			return mutate.PluginPackage{}, err
		}
		api[e.Name()] = content
	}

	withSources := map[string]bool{}
	for _, op := range bag.iosSources {
		withSources[op.PluginID()] = true
	}
	for _, op := range bag.iosPlugins {
		if !withSources[op.PluginID()] {
			return mutate.PluginPackage{}, fmt.Errorf("plugin %s names iOS plugin class %s but ships no Swift sources (IOS.Sources) to define it",
				op.PluginPackage(), op.Class)
		}
	}
	return mutate.PluginPackage{API: api, Deps: bag.iosSPM, Sources: bag.iosSources}, nil
}

// pluginAPITemplates is the template dir holding the DriftPluginAPI
// module's sources.
const pluginAPITemplates = "plugin-api/ios"

func applyAndroidOps(bag *opBag, buildDir string) ([]string, error) {
	var changed []string
	manifestPath := filepath.Join(buildDir, "app", "src", "main", "AndroidManifest.xml")
	if len(bag.addPerm)+len(bag.addIntent)+len(bag.setActAttr)+len(bag.addMeta) > 0 {
		ch, err := mutate.ApplyAndroidManifest(manifestPath, bag.addPerm, bag.addIntent, bag.setActAttr, bag.addMeta)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, manifestPath)
		}
	}

	// Drift-owned values files are always written (or removed when empty),
	// so a dropped plugin's entries do not linger.
	resDir := androidResDir(buildDir)
	for _, w := range []func() (string, bool, error){
		func() (string, bool, error) {
			return mutate.WriteAndroidColors(filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginColorsFile)), bag.colors)
		},
		func() (string, bool, error) {
			return mutate.WriteAndroidStrings(filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginStringsFile)), bag.strings)
		},
		func() (string, bool, error) {
			return mutate.WriteAndroidStyles(filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginStylesFile)), bag.styles)
		},
	} {
		path, ch, err := w()
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	if len(bag.drawables) > 0 {
		paths, err := mutate.WriteAndroidDrawables(androidDrawableDir(buildDir), bag.drawables)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.resXML) > 0 {
		paths, err := mutate.WriteAndroidResourceXML(androidResDir(buildDir), bag.resXML)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.kotlinSources) > 0 {
		paths, err := mutate.WriteKotlinSources(androidJavaRoot(buildDir), bag.kotlinSources)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.gradleDeps) > 0 {
		gradlePath := filepath.Join(buildDir, "app", "build.gradle")
		path, ch, err := mutate.ApplyGradleAddDependencies(gradlePath, bag.gradleDeps)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	if len(bag.androidAssets) > 0 {
		paths, err := mutate.WriteAndroidAssets(androidAssetsRoot(buildDir), bag.androidAssets)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.appModule) > 0 {
		paths, err := mutate.WriteAndroidAppModuleFiles(androidAppDir(buildDir), bag.appModule)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.gradlePlugins) > 0 {
		appGradle := filepath.Join(buildDir, "app", "build.gradle")
		path, ch, err := mutate.ApplyGradleApplyPlugins(appGradle, bag.gradlePlugins)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
		projectGradle := filepath.Join(buildDir, "build.gradle")
		path, ch, err = mutate.ApplyGradleProjectPlugins(projectGradle, bag.gradlePlugins)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	return changed, nil
}

// OwnedFiles returns the whole files op writes into the project at buildDir
// (see mutate.OwnedFile), computed exactly as Apply writes them. Ops that
// only edit shared files, or whose output Drift regenerates wholesale every
// build (registrants, values files, bundle resources, the Drift/Plugins
// package with plugins' Swift sources), own none.
func OwnedFiles(op protocol.Op, buildDir, platform string) ([]mutate.OwnedFile, error) {
	one := func(f mutate.OwnedFile, err error) ([]mutate.OwnedFile, error) {
		if err != nil {
			return nil, err
		}
		return []mutate.OwnedFile{f}, nil
	}
	switch v := op.(type) {
	case *protocol.OpIOSAssetsAddImageSet:
		if platform == "xtool" {
			return nil, nil // a loose bundle PNG, pruned with the bundle resources
		}
		return mutate.ImageSetFiles(iosAssetCatalog(buildDir), v)
	case *protocol.OpAddKotlinSource:
		return one(mutate.KotlinSourceFile(androidJavaRoot(buildDir), v))
	case *protocol.OpAndroidWriteDrawable:
		return one(mutate.DrawableFile(androidDrawableDir(buildDir), v))
	case *protocol.OpAndroidWriteResourceXML:
		return one(mutate.ResourceXMLFile(androidResDir(buildDir), v), nil)
	case *protocol.OpAndroidAddAsset:
		return one(mutate.AndroidAssetFile(androidAssetsRoot(buildDir), v))
	case *protocol.OpAndroidAddAppModuleFile:
		return one(mutate.AppModuleFile(androidAppDir(buildDir), v))
	}
	return nil, nil
}

func androidAppDir(buildDir string) string { return filepath.Join(buildDir, "app") }
func androidResDir(buildDir string) string {
	return filepath.Join(buildDir, "app", "src", "main", "res")
}
func androidDrawableDir(buildDir string) string {
	return filepath.Join(androidResDir(buildDir), "drawable")
}
func androidJavaRoot(buildDir string) string {
	return filepath.Join(buildDir, "app", "src", "main", "java")
}
func androidAssetsRoot(buildDir string) string {
	return filepath.Join(buildDir, "app", "src", "main", "assets")
}
func iosAssetCatalog(buildDir string) string {
	return filepath.Join(buildDir, "Runner", "Assets.xcassets")
}

// iosInfoPlistPath returns Info.plist for managed iOS, xtool, and ejected builds.
func iosInfoPlistPath(buildDir, platform string) string {
	if platform == "xtool" {
		return filepath.Join(buildDir, "Sources", "Runner", "Resources", "Info.plist")
	}
	return filepath.Join(buildDir, "Runner", "Info.plist")
}

func iosLaunchScreenPath(buildDir, platform string) string {
	if platform == "xtool" {
		return filepath.Join(buildDir, "Sources", "Runner", "Resources", "LaunchScreen.storyboard")
	}
	return filepath.Join(buildDir, "Runner", "LaunchScreen.storyboard")
}

// xtoolPluginResourcesDir is the Drift-owned directory, relative to the
// xtool project root, holding plugin bundle resources. It sits outside
// Sources/Runner so SwiftPM never sees the files (SwiftPM would bundle them
// into Runner_Runner.bundle, invisible to Bundle.main); xtool.yml's
// `resources:` list copies them into the app bundle root instead.
const xtoolPluginResourcesDir = "PluginResources"

// iosBundleResourcesDir is the Drift-owned directory for files destined for
// the root of the app bundle (OpIOSAddBundleResource, plus image sets on
// xtool). On xcodeproj builds it is Runner/PluginResources: the
// PBXFileSystemSynchronizedRootGroup includes it and Xcode's resources phase
// copies the files flat into the bundle root. Keeping it out of Runner/
// itself means a resource can never shadow a template source file. On
// xtool builds, see xtoolPluginResourcesDir.
func iosBundleResourcesDir(buildDir, platform string) string {
	if platform == "xtool" {
		return filepath.Join(buildDir, xtoolPluginResourcesDir)
	}
	return filepath.Join(buildDir, "Runner", "PluginResources")
}

// iosSPMPackageRoot returns the absolute directory for the SwiftPM sidecar
// package. Both build paths use the same layout: Drift/Plugins/ at the
// iOS project root. The xtool Package.swift references it relative
// (.package(path: "Drift/Plugins")), the xcodeproj wires it via an
// XCLocalSwiftPackageReference with the same relativePath.
func iosSPMPackageRoot(buildDir string) string {
	return filepath.Join(buildDir, "Drift", "Plugins")
}

func changedSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
