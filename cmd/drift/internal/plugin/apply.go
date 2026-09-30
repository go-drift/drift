package plugin

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	pkgerrors "github.com/go-drift/drift/pkg/errors"
	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// Apply mutates the rendered project at buildDir according to the validated
// op list. Returns the list of files whose bytes actually changed.
//
// Apply skips ops that do not match the target platform; plugins may
// unconditionally emit ops for every platform.
func Apply(ops []driftplugin.Op, buildDir, platform string) ([]string, error) {
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
	infoString    []*driftplugin.OpInfoPlistSetString
	infoBool      []*driftplugin.OpInfoPlistSetBool
	infoArray     []*driftplugin.OpInfoPlistSetStringArray
	infoAppend    []*driftplugin.OpInfoPlistAppendArrayItem
	infoDict      []*driftplugin.OpInfoPlistSetDict
	iosAssets     []*driftplugin.OpIOSAssetsAddImageSet
	iosLaunch     []*driftplugin.OpIOSReplaceLaunchScreen
	iosSources    []*driftplugin.OpAddIOSSource
	iosBundle     []*driftplugin.OpIOSAddBundleResource
	iosSPM        []*driftplugin.OpIOSAddPackageDependency
	addPerm       []*driftplugin.OpAndroidManifestAddPermission
	addIntent     []*driftplugin.OpAndroidManifestAddIntentFilter
	setActAttr    []*driftplugin.OpAndroidManifestSetActivityAttr
	addMeta       []*driftplugin.OpAndroidManifestAddMetaData
	colors        []*driftplugin.OpAndroidColorSet
	strings       []*driftplugin.OpAndroidStringSet
	styles        []*driftplugin.OpAndroidStyleSet
	drawables     []*driftplugin.OpAndroidWriteDrawable
	resXML        []*driftplugin.OpAndroidWriteResourceXML
	kotlinSources []*driftplugin.OpAddKotlinSource
	gradleDeps    []*driftplugin.OpAndroidGradleAddDependency
	androidAssets []*driftplugin.OpAndroidAddAsset
	appModule     []*driftplugin.OpAndroidAddAppModuleFile
	gradlePlugins []*driftplugin.OpAndroidGradleApplyPlugin
}

func bundleByPlatform(ops []driftplugin.Op, platform string) *opBag {
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
func bundleOp(bag *opBag, op driftplugin.Op) bool {
	switch op.Platform() {
	case "ios":
		return bundleIOSOp(bag, op)
	case "android":
		return bundleAndroidOp(bag, op)
	default:
		return false
	}
}

func opAppliesTo(op driftplugin.Op, platform string) bool {
	switch op.Platform() {
	case "ios":
		return platform == "ios" || platform == "xtool"
	case "android":
		return platform == "android"
	default:
		return true
	}
}

func bundleIOSOp(bag *opBag, op driftplugin.Op) bool {
	switch v := op.(type) {
	case *driftplugin.OpInfoPlistSetString:
		bag.infoString = append(bag.infoString, v)
	case *driftplugin.OpInfoPlistSetBool:
		bag.infoBool = append(bag.infoBool, v)
	case *driftplugin.OpInfoPlistSetStringArray:
		bag.infoArray = append(bag.infoArray, v)
	case *driftplugin.OpInfoPlistAppendArrayItem:
		bag.infoAppend = append(bag.infoAppend, v)
	case *driftplugin.OpInfoPlistSetDict:
		bag.infoDict = append(bag.infoDict, v)
	case *driftplugin.OpIOSAssetsAddImageSet:
		bag.iosAssets = append(bag.iosAssets, v)
	case *driftplugin.OpIOSReplaceLaunchScreen:
		bag.iosLaunch = append(bag.iosLaunch, v)
	case *driftplugin.OpAddIOSSource:
		bag.iosSources = append(bag.iosSources, v)
	case *driftplugin.OpIOSAddBundleResource:
		bag.iosBundle = append(bag.iosBundle, v)
	case *driftplugin.OpIOSAddPackageDependency:
		bag.iosSPM = append(bag.iosSPM, v)
	case *driftplugin.OpRegistrantIOS, *driftplugin.OpIOSAppDelegateRegistrant:
		// Consumed by WriteRegistrant, not Apply.
	default:
		return false
	}
	return true
}

func bundleAndroidOp(bag *opBag, op driftplugin.Op) bool {
	switch v := op.(type) {
	case *driftplugin.OpAndroidManifestAddPermission:
		bag.addPerm = append(bag.addPerm, v)
	case *driftplugin.OpAndroidManifestAddIntentFilter:
		bag.addIntent = append(bag.addIntent, v)
	case *driftplugin.OpAndroidManifestSetActivityAttr:
		bag.setActAttr = append(bag.setActAttr, v)
	case *driftplugin.OpAndroidManifestAddMetaData:
		bag.addMeta = append(bag.addMeta, v)
	case *driftplugin.OpAndroidColorSet:
		bag.colors = append(bag.colors, v)
	case *driftplugin.OpAndroidStringSet:
		bag.strings = append(bag.strings, v)
	case *driftplugin.OpAndroidStyleSet:
		bag.styles = append(bag.styles, v)
	case *driftplugin.OpAndroidWriteDrawable:
		bag.drawables = append(bag.drawables, v)
	case *driftplugin.OpAndroidWriteResourceXML:
		bag.resXML = append(bag.resXML, v)
	case *driftplugin.OpAddKotlinSource:
		bag.kotlinSources = append(bag.kotlinSources, v)
	case *driftplugin.OpAndroidGradleAddDependency:
		bag.gradleDeps = append(bag.gradleDeps, v)
	case *driftplugin.OpAndroidAddAsset:
		bag.androidAssets = append(bag.androidAssets, v)
	case *driftplugin.OpAndroidAddAppModuleFile:
		bag.appModule = append(bag.appModule, v)
	case *driftplugin.OpAndroidGradleApplyPlugin:
		bag.gradlePlugins = append(bag.gradlePlugins, v)
	case *driftplugin.OpRegistrantAndroid, *driftplugin.OpAndroidPreActivityRegistrant:
		// Consumed by WriteRegistrant, not Apply.
	default:
		return false
	}
	return true
}

// reportUnknownOp surfaces unrecognised op types via the drift error reporter,
// mirroring the boundary-parser convention in pkg/platform/stream.go.
func reportUnknownOp(op driftplugin.Op) {
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
		paths, err := mutate.WriteIOSAssets(filepath.Join(buildDir, "Runner", "Assets.xcassets"), bag.iosAssets)
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

	if len(bag.iosSources) > 0 {
		paths, err := mutate.WriteIOSSources(iosPluginsDir(buildDir, platform), bag.iosSources)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
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

	// SwiftPM sidecar always regenerates, even with zero ops. The xtool
	// template's .package(path: "Drift/Plugins") and the xcodeproj's
	// local-package reference assume the sidecar exists; an empty op set
	// produces a valid zero-deps wrapper.
	spmPaths, err := mutate.ApplyPluginPackage(iosSPMPackageRoot(buildDir), bag.iosSPM)
	if err != nil {
		return changed, err
	}
	changed = append(changed, spmPaths...)

	return changed, nil
}

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

	resValuesDir := filepath.Join(buildDir, "app", "src", "main", "res", "values")
	if len(bag.colors) > 0 {
		path, ch, err := mutate.ApplyAndroidColors(filepath.Join(resValuesDir, "plugin_colors.xml"), bag.colors)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}
	if len(bag.strings) > 0 {
		path, ch, err := mutate.ApplyAndroidStrings(filepath.Join(resValuesDir, "plugin_strings.xml"), bag.strings)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}
	if len(bag.styles) > 0 {
		path, ch, err := mutate.ApplyAndroidStyles(filepath.Join(resValuesDir, "plugin_styles.xml"), bag.styles)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	if len(bag.drawables) > 0 {
		drawableDir := filepath.Join(buildDir, "app", "src", "main", "res", "drawable")
		paths, err := mutate.WriteAndroidDrawables(drawableDir, bag.drawables)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.resXML) > 0 {
		resRoot := filepath.Join(buildDir, "app", "src", "main", "res")
		paths, err := mutate.WriteAndroidResourceXML(resRoot, bag.resXML)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.kotlinSources) > 0 {
		javaRoot := filepath.Join(buildDir, "app", "src", "main", "java")
		paths, err := mutate.WriteKotlinSources(javaRoot, bag.kotlinSources)
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
		assetsRoot := filepath.Join(buildDir, "app", "src", "main", "assets")
		paths, err := mutate.WriteAndroidAssets(assetsRoot, bag.androidAssets)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.appModule) > 0 {
		paths, err := mutate.WriteAndroidAppModuleFiles(filepath.Join(buildDir, "app"), bag.appModule)
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

func iosPluginsDir(buildDir, platform string) string {
	if platform == "xtool" {
		return filepath.Join(buildDir, "Sources", "Runner", "Plugins")
	}
	return filepath.Join(buildDir, "Runner", "Plugins")
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
