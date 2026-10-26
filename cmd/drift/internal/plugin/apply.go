package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/cmd/drift/internal/templates"
	pkgerrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// Apply plans into cs the mutations the validated op list makes to the
// rendered project at buildDir, writing nothing: the caller commits cs
// once every step of the build has planned, so a failing op leaves the
// project untouched. Returns the files whose bytes the commit changes.
//
// prev is the ejected project's lock from its previous build (ReadLock),
// nil for a managed build. Apply never overwrites a file Drift did not
// write: it fails with a *ForeignFileError if an op would replace a file
// that prev does not record as Drift's, unchanged.
//
// Apply skips ops that do not match the target platform; plugins may
// unconditionally emit ops for every platform.
func Apply(cs *mutate.Changeset, ops []protocol.Op, buildDir, platform string, prev *Lock) ([]string, error) {
	if err := checkOwnedWrites(ops, buildDir, platform, prev); err != nil {
		return nil, err
	}
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
		ios, err := applyIOSOps(cs, bag, buildDir, platform)
		if err != nil {
			return changedSorted(changed), err
		}
		record(ios...)
	}
	if platform == "android" {
		droid, err := applyAndroidOps(cs, bag, buildDir, prev.gradleReplacements(ops))
		if err != nil {
			return changedSorted(changed), err
		}
		record(droid...)
	}

	return changedSorted(changed), nil
}

type opBag struct {
	plists        map[protocol.PlistFile][]protocol.PlistOp
	iosAssets     []*protocol.OpIOSAssetsAddImageSet
	iosLaunch     []*protocol.OpIOSReplaceLaunchScreen
	iosSources    []*protocol.OpAddIOSSource
	iosBundle     []*protocol.OpIOSAddBundleResource
	iosSPM        []*protocol.OpIOSAddPackageDependency
	iosPlugins    []*protocol.OpIOSPlugin
	manifest      mutate.ManifestOps
	colors        []*protocol.OpAndroidColorSet
	strings       []*protocol.OpAndroidStringSet
	styles        []*protocol.OpAndroidStyleSet
	drawables     []*protocol.OpAndroidWriteDrawable
	resXML        []*protocol.OpAndroidWriteResourceXML
	kotlinSources []*protocol.OpAddKotlinSource
	gradleDeps    []*protocol.OpAndroidGradleAddDependency
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
	case protocol.PlistOp:
		if bag.plists == nil {
			bag.plists = map[protocol.PlistFile][]protocol.PlistOp{}
		}
		bag.plists[v.PlistFile()] = append(bag.plists[v.PlistFile()], v)
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
		bag.manifest.Permissions = append(bag.manifest.Permissions, v)
	case *protocol.OpAndroidManifestAddIntentFilter:
		bag.manifest.IntentFilters = append(bag.manifest.IntentFilters, v)
	case *protocol.OpAndroidManifestSetActivityAttr:
		bag.manifest.ActivityAttrs = append(bag.manifest.ActivityAttrs, v)
	case *protocol.OpAndroidManifestAddMetaData:
		bag.manifest.MetaData = append(bag.manifest.MetaData, v)
	case *protocol.OpAndroidManifestAddService:
		bag.manifest.Services = append(bag.manifest.Services, v)
	case *protocol.OpAndroidManifestAddReceiver:
		bag.manifest.Receivers = append(bag.manifest.Receivers, v)
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

func applyIOSOps(cs *mutate.Changeset, bag *opBag, buildDir, platform string) ([]string, error) {
	var changed []string

	for _, file := range []protocol.PlistFile{protocol.PlistInfo, protocol.PlistEntitlements} {
		ops := bag.plists[file]
		if len(ops) == 0 {
			continue
		}
		path := filepath.Join(buildDir, filepath.FromSlash(iosPlistRelPath(platform, file)))
		ch, err := mutate.ApplyPlist(cs, path, ops)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
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
		paths, err := mutate.WriteIOSAssets(cs, iosAssetCatalog(buildDir), bag.iosAssets)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.iosLaunch) > 0 {
		path, ch, err := mutate.ReplaceLaunchScreen(cs, iosLaunchScreenPath(buildDir, platform), bag.iosLaunch[0])
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	// Always synced, even with zero files, so resources from a removed
	// plugin are pruned.
	paths, err := mutate.WriteIOSBundleResources(cs, iosBundleResourcesDir(buildDir, platform), bundleFiles)
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
		ch, err := mutate.SetXtoolResources(cs, ymlPath, resources)
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
	spmPaths, err := mutate.WritePluginPackage(cs, iosSPMPackageRoot(buildDir), pkg)
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

// replace maps Gradle lines to the lines Drift wrote for them at an earlier
// version (Lock.gradleReplacements).
func applyAndroidOps(cs *mutate.Changeset, bag *opBag, buildDir string, replace map[string]string) ([]string, error) {
	var changed []string
	manifestPath := filepath.Join(buildDir, "app", "src", "main", "AndroidManifest.xml")
	if !bag.manifest.Empty() {
		ch, err := mutate.ApplyAndroidManifest(cs, manifestPath, bag.manifest)
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
			return mutate.WriteAndroidColors(cs, filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginColorsFile)), bag.colors)
		},
		func() (string, bool, error) {
			return mutate.WriteAndroidStrings(cs, filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginStringsFile)), bag.strings)
		},
		func() (string, bool, error) {
			return mutate.WriteAndroidStyles(cs, filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginStylesFile)), bag.styles)
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
		paths, err := mutate.WriteAndroidDrawables(cs, androidDrawableDir(buildDir), bag.drawables)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.resXML) > 0 {
		paths, err := mutate.WriteAndroidResourceXML(cs, androidResDir(buildDir), bag.resXML)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.kotlinSources) > 0 {
		paths, err := mutate.WriteKotlinSources(cs, androidJavaRoot(buildDir), bag.kotlinSources)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.gradleDeps) > 0 {
		gradlePath := filepath.Join(buildDir, "app", "build.gradle")
		path, ch, err := mutate.ApplyGradleAddDependencies(cs, gradlePath, bag.gradleDeps, replace)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	if len(bag.appModule) > 0 {
		paths, err := mutate.WriteAndroidAppModuleFiles(cs, androidAppDir(buildDir), bag.appModule)
		if err != nil {
			return changed, err
		}
		changed = append(changed, paths...)
	}

	if len(bag.gradlePlugins) > 0 {
		appGradle := filepath.Join(buildDir, "app", "build.gradle")
		path, ch, err := mutate.ApplyGradleApplyPlugins(cs, appGradle, bag.gradlePlugins)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
		projectGradle := filepath.Join(buildDir, "build.gradle")
		path, ch, err = mutate.ApplyGradleProjectPlugins(cs, projectGradle, bag.gradlePlugins, replace)
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
	case *protocol.OpAndroidAddAppModuleFile:
		return one(mutate.AppModuleFile(androidAppDir(buildDir), v))
	}
	return nil, nil
}

// ForeignFile is a file a plugin op would overwrite although Drift did not
// write it, or it changed since Drift did.
type ForeignFile struct {
	Path   string // slash-separated, relative to the project
	Plugin string
}

// ForeignFileError lists every ForeignFile in a build.
type ForeignFileError struct {
	Files []ForeignFile
}

func (e *ForeignFileError) Error() string {
	var sb strings.Builder
	sb.WriteString("plugins would overwrite files Drift did not write, or that changed since it did:\n")
	for _, f := range e.Files {
		fmt.Fprintf(&sb, "  - %s (from %s)\n", f.Path, f.Plugin)
	}
	sb.WriteString("In an ejected project, move your file aside (or undo your change) and build again to let the plugin write it, " +
		"or remove the plugin from drift.yaml. Otherwise the plugin collides with a file in Drift's project template and must write another path.")
	return sb.String()
}

// checkOwnedWrites returns a *ForeignFileError naming every whole file an
// op would write over an existing file holding other content, unless prev
// records that content as what Drift wrote there.
func checkOwnedWrites(ops []protocol.Op, buildDir, platform string, prev *Lock) error {
	var foreign []ForeignFile
	for _, op := range ops {
		if !opAppliesTo(op, platform) {
			continue
		}
		files, err := OwnedFiles(op, buildDir, platform)
		if err != nil {
			return err
		}
		for _, f := range files {
			existing, err := os.ReadFile(f.Path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if bytes.Equal(existing, f.Content) {
				continue
			}
			rel, err := filepath.Rel(buildDir, f.Path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if prev.wrote(rel, existing) {
				continue
			}
			foreign = append(foreign, ForeignFile{Path: rel, Plugin: op.PluginPackage()})
		}
	}
	if len(foreign) > 0 {
		return &ForeignFileError{Files: foreign}
	}
	return nil
}

func androidAppDir(buildDir string) string { return filepath.Join(buildDir, "app") }
func androidResDir(buildDir string) string {
	return filepath.Join(buildDir, "app", "src", "main", "res")
}

// androidDrawableDir holds plugin bitmaps. They come at one pixel size, so
// they go in drawable-nodpi: in drawable/ Android treats them as mdpi and
// scales them up on denser screens (a 2000px image becomes ~5500px on a
// 420dpi phone).
func androidDrawableDir(buildDir string) string {
	return filepath.Join(androidResDir(buildDir), "drawable-nodpi")
}
func androidJavaRoot(buildDir string) string {
	return filepath.Join(buildDir, "app", "src", "main", "java")
}
func iosAssetCatalog(buildDir string) string {
	return filepath.Join(buildDir, "Runner", "Assets.xcassets")
}

// iosPlistRelPath returns the project-relative, slash-separated path of an
// iOS plist for managed iOS, xtool, and ejected builds. The entitlements
// sit at the project root on both paths, outside the sources so neither
// build copies them into the bundle (xcodeproj: CODE_SIGN_ENTITLEMENTS;
// xtool: entitlementsPath in xtool.yml).
func iosPlistRelPath(platform string, file protocol.PlistFile) string {
	switch file {
	case protocol.PlistInfo:
		if platform == "xtool" {
			return "Sources/Runner/Resources/Info.plist"
		}
		return "Runner/Info.plist"
	case protocol.PlistEntitlements:
		return "Runner.entitlements"
	}
	panic(fmt.Sprintf("plugin: unknown plist file %q", file))
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
