package mutate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// bundleResourceWarnBytes is the soft threshold at which the bundle mutator
// prints a stderr breadcrumb. No hard cap: hard caps pick an arbitrary
// number and force a Drift patch when legitimate use (medium ML models,
// video previews) wants more headroom. The breadcrumb pays for itself the
// first time a plugin author lands an OOM via the JSON bridge: they see
// "you injected a 47 MB asset" instead of an opaque stack trace.
const bundleResourceWarnBytes = 5 * 1024 * 1024 // 5 MB

// warnSink lets tests capture the soft-warning output without grabbing
// os.Stderr global state. Production code uses the default.
var warnSink io.Writer = os.Stderr

// BundleFile is one decoded file destined for the root of the iOS app
// bundle. Name is a plain file name (driftplugin.ValidateBundleFileName).
type BundleFile struct {
	Name    string
	Content []byte
	Plugin  string // emitting plugin package, for diagnostics
}

// IOSBundleFiles decodes OpIOSAddBundleResource ops into BundleFiles.
func IOSBundleFiles(ops []*driftplugin.OpIOSAddBundleResource) ([]BundleFile, error) {
	out := make([]BundleFile, 0, len(ops))
	for _, op := range ops {
		f, err := decodeBundleFile(op.Path, op.Content, op.PluginPackage())
		if err != nil {
			return nil, fmt.Errorf("iOS bundle resource %s: %w", op.Path, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// ImageSetBundleFiles converts OpIOSAssetsAddImageSet ops into loose
// `<Name>.png` bundle files. UIImage(named: "<Name>") resolves a loose PNG
// in the main bundle the same way it resolves an asset-catalog image set,
// so plugin Swift code is unchanged. Used on xtool builds, which cannot
// compile asset catalogs.
//
// FUTURE(xtool#219): once xtool compiles asset catalogs, xtool builds can
// write image sets with WriteIOSAssets like xcodeproj builds and this
// conversion goes away. https://github.com/xtool-org/xtool/pull/219
func ImageSetBundleFiles(ops []*driftplugin.OpIOSAssetsAddImageSet) ([]BundleFile, error) {
	out := make([]BundleFile, 0, len(ops))
	for _, op := range ops {
		f, err := decodeBundleFile(op.Name+".png", op.Image, op.PluginPackage())
		if err != nil {
			return nil, fmt.Errorf("iOS image set %s: %w", op.Name, err)
		}
		out = append(out, f)
	}
	return out, nil
}

func decodeBundleFile(name, content, plugin string) (BundleFile, error) {
	// ValidateBundleFileName in the recorder is the boundary check; re-run
	// it for ops decoded from JSON without going through the recorder.
	if err := driftplugin.ValidateBundleFileName(name); err != nil {
		return BundleFile{}, err
	}
	decoded, err := driftplugin.DecodeContent(content)
	if err != nil {
		return BundleFile{}, fmt.Errorf("decode content: %w", err)
	}
	return BundleFile{Name: name, Content: decoded, Plugin: plugin}, nil
}

// WriteIOSBundleResources makes dir contain exactly files: each file is
// written (only when its bytes differ) and anything else in dir is removed,
// so a plugin dropped from the project, including mid-watch-session, takes
// its resources with it. dir is owned by Drift; nothing else may write
// there. With no files, dir is removed entirely.
//
// Two files with the same name are an error: both build paths flatten
// resources into the bundle root, so the second would shadow the first.
// Returns written and removed paths.
func WriteIOSBundleResources(dir string, files []BundleFile) ([]string, error) {
	want := make(map[string]BundleFile, len(files))
	for _, f := range files {
		if prev, dup := want[f.Name]; dup {
			return nil, fmt.Errorf("iOS bundle resource %q is provided by both %s and %s", f.Name, pluginLabel(prev.Plugin), pluginLabel(f.Plugin))
		}
		want[f.Name] = f
	}

	var changed []string
	removed, err := pruneDir(dir, want)
	if err != nil {
		return nil, err
	}
	changed = append(changed, removed...)

	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f := want[name]
		maybeWarnLargeResource(f.Plugin, f.Name, len(f.Content))
		dest := filepath.Join(dir, name)
		ch, err := writeIfDifferent(dest, f.Content)
		if err != nil {
			return changed, fmt.Errorf("write iOS bundle resource %s: %w", name, err)
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

// pruneDir removes every entry in dir whose name is not in keep, and dir
// itself when keep is empty. A missing dir is not an error.
func pruneDir(dir string, keep map[string]BundleFile) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var removed []string
	for _, e := range entries {
		if _, ok := keep[e.Name()]; ok && e.Type().IsRegular() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(p); err != nil {
			return removed, fmt.Errorf("remove stale %s: %w", p, err)
		}
		removed = append(removed, p)
	}
	if len(keep) == 0 {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("remove %s: %w", dir, err)
		}
	}
	return removed, nil
}

// WriteAndroidAssets writes each OpAndroidAddAsset verbatim under assetsRoot
// (typically app/src/main/assets/) at the op's relative path. Gradle's
// build pipeline picks up the directory automatically; no manifest edits.
func WriteAndroidAssets(assetsRoot string, ops []*driftplugin.OpAndroidAddAsset) ([]string, error) {
	var changed []string
	for _, op := range ops {
		if err := driftplugin.ValidateAssetRelPath(op.Path); err != nil {
			return changed, fmt.Errorf("Android asset: %w", err)
		}
		content, err := driftplugin.DecodeContent(op.Content)
		if err != nil {
			return changed, fmt.Errorf("Android asset %s: decode content: %w", op.Path, err)
		}
		maybeWarnLargeResource(op.PluginPackage(), op.Path, len(content))
		dest := filepath.Join(assetsRoot, filepath.FromSlash(op.Path))
		ch, err := writeIfDifferent(dest, content)
		if err != nil {
			return changed, fmt.Errorf("write Android asset %s: %w", op.Path, err)
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

// WriteAndroidAppModuleFiles writes each OpAndroidAddAppModuleFile into the
// app module directory (appDir, i.e. <project>/app), where Gradle plugins
// such as google-services read their config.
func WriteAndroidAppModuleFiles(appDir string, ops []*driftplugin.OpAndroidAddAppModuleFile) ([]string, error) {
	var changed []string
	for _, op := range ops {
		if err := driftplugin.ValidateAppModuleFileName(op.Name); err != nil {
			return changed, fmt.Errorf("Android app module file: %w", err)
		}
		content, err := driftplugin.DecodeContent(op.Content)
		if err != nil {
			return changed, fmt.Errorf("Android app module file %s: decode content: %w", op.Name, err)
		}
		dest := filepath.Join(appDir, op.Name)
		ch, err := writeIfDifferent(dest, content)
		if err != nil {
			return changed, fmt.Errorf("write Android app module file %s: %w", op.Name, err)
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

// maybeWarnLargeResource emits a single stderr line when a written
// resource exceeds bundleResourceWarnBytes.
func maybeWarnLargeResource(pluginPackage, path string, size int) {
	if size < bundleResourceWarnBytes {
		return
	}
	fmt.Fprintf(warnSink, "drift: plugin %s wrote %d-byte resource %q; large assets stress the JSON plugin bridge\n", pluginLabel(pluginPackage), size, path)
}

// pluginLabel quotes a plugin package for diagnostics, tolerating the empty
// package that synthetic test fixtures use.
func pluginLabel(pkg string) string {
	if pkg == "" {
		return "<unknown>"
	}
	return fmt.Sprintf("%q", pkg)
}
