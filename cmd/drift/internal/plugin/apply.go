package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/cmd/drift/internal/templates"
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
// that prev does not record as Drift's, unchanged, and that does not hold
// the template's content (templateDefault).
//
// Each op's opKind decides what it writes: the whole files it owns as
// they are, and edits to shared files and regenerated output through the
// mutator its batch feeds. Apply skips ops that do not match the target
// platform; plugins may unconditionally emit ops for every platform.
func Apply(cs *mutate.Changeset, ops []protocol.Op, buildDir, platform string, prev *Lock) ([]string, error) {
	p := project{dir: buildDir, platform: platform}
	if err := checkOwnedWrites(ops, p, prev); err != nil {
		return nil, err
	}

	changed := make(map[string]bool)
	record := func(paths ...string) {
		for _, path := range paths {
			changed[path] = true
		}
	}

	var b batch
	for _, op := range ops {
		if !opAppliesTo(op, platform) {
			continue
		}
		if stage := kindOf(op).stage; stage != nil {
			stage(&b, op)
		}
		files, err := ownedFiles(op, p)
		if err != nil {
			return changedSorted(changed), err
		}
		paths, err := mutate.WriteOwned(cs, files...)
		record(paths...)
		if err != nil {
			return changedSorted(changed), err
		}
	}

	var shared []string
	var err error
	switch platform {
	case "ios", "xtool":
		shared, err = applyIOSBatch(cs, &b, p)
	case "android":
		shared, err = applyAndroidBatch(cs, &b, p, prev.gradleReplacements(ops))
	}
	record(shared...)
	return changedSorted(changed), err
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

// applyIOSBatch runs the iOS mutators of shared files and regenerated
// output with b's ops.
func applyIOSBatch(cs *mutate.Changeset, b *batch, p project) ([]string, error) {
	var changed []string

	for _, file := range []protocol.PlistFile{protocol.PlistInfo, protocol.PlistEntitlements} {
		ops := b.plists[file]
		if len(ops) == 0 {
			continue
		}
		path := p.abs(p.plist(file))
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
	bundleFiles, err := mutate.AddedFiles(slices.Concat(b.files[protocol.FileIOSBundle], b.files[protocol.FileIOSPluginConfig]))
	if err != nil {
		return changed, err
	}
	if p.xtool() {
		// FUTURE(xtool#219): write image sets as owned files into a
		// Drift-owned Assets.xcassets (listed under xtool.yml
		// `assetCatalogs:`) like xcodeproj builds, and drop this conversion.
		// https://github.com/xtool-org/xtool/pull/219
		images, err := mutate.ImageSetBundleFiles(b.imageSets)
		if err != nil {
			return changed, err
		}
		bundleFiles = append(bundleFiles, images...)
	} else if len(b.imageSets) > 0 {
		// The image sets themselves are owned files; their catalog needs a
		// root.
		paths, err := mutate.EnsureAssetCatalog(cs, p.abs(p.assetCatalog()))
		changed = append(changed, paths...)
		if err != nil {
			return changed, err
		}
	}

	// Always synced, even with zero files, so resources from a removed
	// plugin are pruned.
	paths, err := mutate.WriteIOSBundleResources(cs, p.abs(p.bundleResources()), bundleFiles)
	if err != nil {
		return changed, err
	}
	changed = append(changed, paths...)

	if p.xtool() {
		resources := make([]string, 0, len(bundleFiles))
		for _, f := range bundleFiles {
			resources = append(resources, p.bundleResources()+"/"+f.Name)
		}
		ymlPath := p.abs(p.xtoolYML())
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
	pkg, err := pluginPackage(b)
	if err != nil {
		return changed, err
	}
	spmPaths, err := mutate.WritePluginPackage(cs, p.abs(p.spmPackage()), pkg)
	if err != nil {
		return changed, err
	}
	changed = append(changed, spmPaths...)

	return changed, nil
}

// pluginPackage assembles the Drift/Plugins package for bag, and checks
// that every plugin naming an iOS plugin class ships the Swift sources its
// module needs.
func pluginPackage(b *batch) (mutate.PluginPackage, error) {
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
	for _, op := range b.iosSources {
		withSources[op.PluginID()] = true
	}
	for _, op := range b.iosPlugins {
		if !withSources[op.PluginID()] {
			return mutate.PluginPackage{}, fmt.Errorf("plugin %s names iOS plugin class %s but ships no Swift sources (IOS.Sources) to define it",
				op.PluginPackage(), op.Class)
		}
	}
	return mutate.PluginPackage{API: api, Deps: b.spm, Sources: b.iosSources}, nil
}

// pluginAPITemplates is the template dir holding the DriftPluginAPI
// module's sources.
const pluginAPITemplates = "plugin-api/ios"

// applyAndroidBatch runs the Android mutators of shared files and
// regenerated output with b's ops. replace maps Gradle lines to the lines
// Drift wrote for them at an earlier version (Lock.gradleReplacements).
func applyAndroidBatch(cs *mutate.Changeset, b *batch, p project, replace map[string]string) ([]string, error) {
	var changed []string
	if !b.manifest.Empty() {
		manifestPath := p.abs(p.manifest())
		ch, err := mutate.ApplyAndroidManifest(cs, manifestPath, b.manifest)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, manifestPath)
		}
	}

	// Drift-owned values files are always written (or removed when empty),
	// so a dropped plugin's entries do not linger.
	resDir := p.abs(p.resDir())
	for _, w := range []func() (string, bool, error){
		func() (string, bool, error) {
			return mutate.WriteAndroidColors(cs, filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginColorsFile)), b.colors)
		},
		func() (string, bool, error) {
			return mutate.WriteAndroidStrings(cs, filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginStringsFile)), b.strings)
		},
		func() (string, bool, error) {
			return mutate.WriteAndroidStyles(cs, filepath.Join(resDir, filepath.FromSlash(protocol.AndroidPluginStylesFile)), b.styles)
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

	if len(b.gradleDeps) > 0 {
		path, ch, err := mutate.ApplyGradleAddDependencies(cs, p.abs(p.appGradle()), b.gradleDeps, replace)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	// Always synced, even with zero configs, so a removed plugin's goes.
	configs, err := mutate.AddedFiles(b.files[protocol.FileAndroidPluginConfig])
	if err != nil {
		return changed, err
	}
	paths, err := mutate.WriteAndroidPluginConfigs(cs, p.abs(p.pluginConfigs()), configs)
	if err != nil {
		return changed, err
	}
	changed = append(changed, paths...)

	if len(b.gradlePlugs) > 0 {
		path, ch, err := mutate.ApplyGradleApplyPlugins(cs, p.abs(p.appGradle()), b.gradlePlugs)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
		path, ch, err = mutate.ApplyGradleProjectPlugins(cs, p.abs(p.projectGradle()), b.gradlePlugs, replace)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, path)
		}
	}

	return changed, nil
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
// records that content as what Drift wrote there, or it is the template's
// (templateDefault).
func checkOwnedWrites(ops []protocol.Op, p project, prev *Lock) error {
	var foreign []ForeignFile
	for _, op := range ops {
		if !opAppliesTo(op, p.platform) {
			continue
		}
		files, err := ownedFiles(op, p)
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
			def, err := templateDefault(p, f.Path)
			if err != nil {
				return err
			}
			if def != nil && bytes.Equal(existing, def) {
				continue
			}
			rel, err := p.rel(f.Path)
			if err != nil {
				return err
			}
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

// templateDefault returns the project template's content for path if path
// is a template file a plugin op replaces whole (the launch screen), or
// nil. While the file holds that content it is Drift's: an op may replace
// it, and once no op does, SyncEjectedLock restores it rather than
// deleting it, since the project references it (UILaunchStoryboardName).
func templateDefault(p project, path string) ([]byte, error) {
	if (p.platform == "ios" || p.xtool()) && path == p.abs(p.launchScreen()) {
		return templates.DefaultLaunchScreen()
	}
	return nil, nil
}

func changedSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
