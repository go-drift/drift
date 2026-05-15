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
}

func bundleByPlatform(ops []driftplugin.Op, platform string) *opBag {
	bag := &opBag{}
	for _, op := range ops {
		if !opAppliesTo(op, platform) {
			continue
		}
		switch op.Platform() {
		case "ios":
			bundleIOSOp(bag, op)
		case "android":
			bundleAndroidOp(bag, op)
		default:
			reportUnknownOp(op)
		}
	}
	return bag
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

func bundleIOSOp(bag *opBag, op driftplugin.Op) {
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
	case *driftplugin.OpRegistrantIOS:
		// Consumed by WriteRegistrant, not Apply.
	default:
		reportUnknownOp(op)
	}
}

func bundleAndroidOp(bag *opBag, op driftplugin.Op) {
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
	case *driftplugin.OpRegistrantAndroid, *driftplugin.OpAndroidPreActivityRegistrant:
		// Consumed by WriteRegistrant, not Apply.
	default:
		reportUnknownOp(op)
	}
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

	if len(bag.iosAssets) > 0 {
		paths, err := mutate.WriteIOSAssets(iosAssetsDir(buildDir, platform), bag.iosAssets)
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

	return changed, nil
}

// iosInfoPlistPath returns Info.plist for managed iOS, xtool, and ejected builds.
func iosInfoPlistPath(buildDir, platform string) string {
	if platform == "xtool" {
		return filepath.Join(buildDir, "Sources", "Runner", "Resources", "Info.plist")
	}
	return filepath.Join(buildDir, "Runner", "Info.plist")
}

func iosAssetsDir(buildDir, platform string) string {
	if platform == "xtool" {
		// xtool currently has no Assets.xcassets; treat as ios for ops that need it.
		return filepath.Join(buildDir, "Sources", "Runner", "Resources", "Assets.xcassets")
	}
	return filepath.Join(buildDir, "Runner", "Assets.xcassets")
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

func changedSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
