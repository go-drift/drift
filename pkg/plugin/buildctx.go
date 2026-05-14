package plugin

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// BuildCtx is passed to Plugin.Build. It records ops on platform-specific
// surfaces and exposes shared helpers like ResolveAsset. Plugins MUST NOT
// touch disk directly during Build; all mutations flow through op recorders
// so the CLI can validate the full op list before applying it.
type BuildCtx struct {
	pluginPackage string
	pluginName    string
	projectRoot   string
	buildDir      string
	platform      string

	ops []Op

	// deferredErr captures the first error from a recorder that cannot
	// return an error to its caller (e.g. Sources.AddFS walking an
	// embed.FS). Surfaced via Err() after Build returns.
	deferredErr error

	// IOS records ops for the iOS build target. Methods are no-ops on
	// non-iOS builds; plugins may unconditionally call them and the CLI
	// drops ops whose target platform does not match the build.
	IOS *IOSScope
	// Android records ops for the Android build target.
	Android *AndroidScope
	// Xtool aliases the iOS surface; xtool reuses the iOS scaffold.
	Xtool *IOSScope
}

// Err returns the first deferred error captured by void recorders during
// Build. The bridge runtime consults this after Plugin.Build returns so
// silent failures (e.g. a malformed embed.FS) still abort the build.
func (b *BuildCtx) Err() error { return b.deferredErr }

// NewTestCtx returns a BuildCtx suitable for plugin author unit tests. Ops
// recorded via the returned ctx can be inspected with Ops().
func NewTestCtx() *BuildCtx {
	return newBuildCtx("test/plugin", "test", "/test/project", "/test/build", "all")
}

// Ops returns the ops recorded so far in the context.
func (b *BuildCtx) Ops() []Op {
	out := make([]Op, len(b.ops))
	copy(out, b.ops)
	return out
}

// Plugin returns the package path of the plugin currently being built.
func (b *BuildCtx) Plugin() string { return b.pluginPackage }

// PluginName returns the friendly name of the plugin currently being built.
func (b *BuildCtx) PluginName() string { return b.pluginName }

// Platform returns the target platform string ("android", "ios", or "xtool").
// "all" is reserved for unit-test contexts; plugins should not assume it.
func (b *BuildCtx) Platform() string { return b.platform }

// ProjectRoot returns the absolute path of the user's project root.
func (b *BuildCtx) ProjectRoot() string { return b.projectRoot }

// BuildDir returns the absolute build directory (managed or ejected).
func (b *BuildCtx) BuildDir() string { return b.buildDir }

// ResolveAsset reads an asset at a path relative to the project root. Plugin
// authors use this to bundle user-provided images, fonts, etc.
func (b *BuildCtx) ResolveAsset(rel string) ([]byte, error) {
	if rel == "" {
		return nil, fmt.Errorf("ResolveAsset: empty path")
	}
	if filepath.IsAbs(rel) {
		return nil, fmt.Errorf("ResolveAsset: %q must be project-relative", rel)
	}
	abs := filepath.Join(b.projectRoot, filepath.FromSlash(rel))
	return os.ReadFile(abs)
}

func newBuildCtx(pluginPackage, pluginName, projectRoot, buildDir, platform string) *BuildCtx {
	b := &BuildCtx{
		pluginPackage: pluginPackage,
		pluginName:    pluginName,
		projectRoot:   projectRoot,
		buildDir:      buildDir,
		platform:      platform,
	}
	b.IOS = &IOSScope{b: b}
	b.IOS.Info = &IOSInfoScope{b: b}
	b.IOS.Assets = &IOSAssetsScope{b: b}
	b.IOS.Storyboards = &IOSStoryboardsScope{b: b}
	b.IOS.Sources = &IOSSourcesScope{b: b}

	b.Android = &AndroidScope{b: b}
	b.Android.Manifest = &AndroidManifestScope{b: b}
	b.Android.Resources = &AndroidResourcesScope{
		b:       b,
		Colors:  &AndroidValuesScope{b: b, kind: "color"},
		Strings: &AndroidValuesScope{b: b, kind: "string"},
		Styles:  &AndroidStylesScope{b: b},
	}
	b.Android.Drawables = &AndroidDrawablesScope{b: b}
	b.Android.Sources = &AndroidSourcesScope{b: b}

	b.Xtool = b.IOS
	return b
}

func (b *BuildCtx) push(op Op) {
	b.ops = append(b.ops, op)
}

// IOSScope groups iOS-targeted ops.
type IOSScope struct {
	b *BuildCtx

	Info        *IOSInfoScope
	Assets      *IOSAssetsScope
	Storyboards *IOSStoryboardsScope
	Sources     *IOSSourcesScope
}

// Registrant records an iOS registrant entry that the generated
// DriftPluginRegistrant.swift will call as `symbol(host: host)`.
func (s *IOSScope) Registrant(symbol string) {
	s.b.push(&OpRegistrantIOS{
		Base:   newBase(s.b, ClassAdditive),
		Symbol: symbol,
	})
}

// IOSInfoScope records Info.plist mutations.
type IOSInfoScope struct{ b *BuildCtx }

func (s *IOSInfoScope) SetString(key, value string) {
	s.b.push(&OpInfoPlistSetString{
		Base:  newBase(s.b, ClassIdempotent),
		Key:   key,
		Value: value,
	})
}

func (s *IOSInfoScope) SetBool(key string, value bool) {
	s.b.push(&OpInfoPlistSetBool{
		Base:  newBase(s.b, ClassIdempotent),
		Key:   key,
		Value: value,
	})
}

func (s *IOSInfoScope) SetStringArray(key string, values []string) {
	s.b.push(&OpInfoPlistSetStringArray{
		Base:   newBase(s.b, ClassExclusive),
		Key:    key,
		Values: append([]string(nil), values...),
	})
}

func (s *IOSInfoScope) AppendArrayItem(key, value string) {
	s.b.push(&OpInfoPlistAppendArrayItem{
		Base:  newBase(s.b, ClassAdditive),
		Key:   key,
		Value: value,
	})
}

func (s *IOSInfoScope) SetDict(key string, dict map[string]any) {
	s.b.push(&OpInfoPlistSetDict{
		Base:  newBase(s.b, ClassExclusive),
		Key:   key,
		Value: copyDict(dict),
	})
}

// IOSAssetsScope records additions to Runner/Assets.xcassets.
type IOSAssetsScope struct{ b *BuildCtx }

// AddImageSet adds a single-resolution image set named `name` containing img.
// The image is treated as the 1x universal entry.
func (s *IOSAssetsScope) AddImageSet(name string, img []byte) {
	s.b.push(&OpIOSAssetsAddImageSet{
		Base:  newBase(s.b, ClassExclusive),
		Name:  name,
		Image: encodeBytes(img),
	})
}

// IOSStoryboardsScope records storyboard mutations.
type IOSStoryboardsScope struct{ b *BuildCtx }

// ReplaceLaunchScreen replaces Runner/LaunchScreen.storyboard with the
// supplied content. This op is exclusive: two plugins that try to replace
// the launch screen with divergent content conflict.
func (s *IOSStoryboardsScope) ReplaceLaunchScreen(content string) {
	s.b.push(&OpIOSReplaceLaunchScreen{
		Base:    newBase(s.b, ClassExclusive),
		Content: content,
	})
}

// IOSSourcesScope records Swift sources to drop into the iOS target.
type IOSSourcesScope struct{ b *BuildCtx }

// AddFS walks the supplied embed.FS rooted at root (slash-separated) and
// records one OpAddIOSSource per file. Files land under
// Runner/Plugins/<group>/<relpath> in the generated project tree.
func (s *IOSSourcesScope) AddFS(group string, sources embed.FS, root string) {
	s.b.walkEmbedFS(sources, root, func(rel string, content []byte) {
		s.b.push(&OpAddIOSSource{
			Base:    newBase(s.b, ClassExclusive),
			Group:   group,
			RelPath: rel,
			Content: encodeBytes(content),
		})
	})
}

// AddFile records a single Swift source file at Runner/Plugins/<group>/<rel>.
func (s *IOSSourcesScope) AddFile(group, rel string, content []byte) {
	s.b.push(&OpAddIOSSource{
		Base:    newBase(s.b, ClassExclusive),
		Group:   group,
		RelPath: rel,
		Content: encodeBytes(content),
	})
}

// AndroidScope groups Android-targeted ops.
type AndroidScope struct {
	b *BuildCtx

	Manifest  *AndroidManifestScope
	Resources *AndroidResourcesScope
	Drawables *AndroidDrawablesScope
	Sources   *AndroidSourcesScope
}

// Registrant records an Android registrant entry that the generated
// DriftPluginRegistrant.kt will call as `<symbol>(host)`. Symbol is the
// fully-qualified Kotlin identifier, e.g. com.foo.camera.CameraPlugin.register.
func (s *AndroidScope) Registrant(symbol string) {
	s.b.push(&OpRegistrantAndroid{
		Base:   newBase(s.b, ClassAdditive),
		Symbol: symbol,
	})
}

// AndroidManifestScope records AndroidManifest.xml mutations.
type AndroidManifestScope struct{ b *BuildCtx }

func (s *AndroidManifestScope) AddPermission(name string) {
	s.b.push(&OpAndroidManifestAddPermission{
		Base: newBase(s.b, ClassAdditive),
		Name: name,
	})
}

func (s *AndroidManifestScope) AddIntentFilter(activity string, xml string) {
	s.b.push(&OpAndroidManifestAddIntentFilter{
		Base:     newBase(s.b, ClassAdditive),
		Activity: activity,
		XML:      xml,
	})
}

func (s *AndroidManifestScope) SetActivityAttr(activity, attr, value string) {
	s.b.push(&OpAndroidManifestSetActivityAttr{
		Base:     newBase(s.b, ClassIdempotent),
		Activity: activity,
		Attr:     attr,
		Value:    value,
	})
}

// SetActivityTheme is sugar for SetActivityAttr(activity, "android:theme", theme).
func (s *AndroidManifestScope) SetActivityTheme(activity, theme string) {
	s.SetActivityAttr(activity, "android:theme", theme)
}

func (s *AndroidManifestScope) AddMetaData(parent, name, value string) {
	s.b.push(&OpAndroidManifestAddMetaData{
		Base:   newBase(s.b, ClassIdempotent),
		Parent: parent,
		Name:   name,
		Value:  value,
	})
}

// AndroidResourcesScope groups res/values/* and arbitrary resource writers.
type AndroidResourcesScope struct {
	b       *BuildCtx
	Colors  *AndroidValuesScope
	Strings *AndroidValuesScope
	Styles  *AndroidStylesScope
}

// WriteXML writes an arbitrary resource XML file under res/<relPath>.
// relPath is slash-separated and rooted at res/.
func (s *AndroidResourcesScope) WriteXML(relPath, content string) {
	s.b.push(&OpAndroidWriteResourceXML{
		Base:    newBase(s.b, ClassExclusive),
		RelPath: relPath,
		Content: content,
	})
}

// AndroidValuesScope handles res/values/{colors,strings}.xml.
type AndroidValuesScope struct {
	b    *BuildCtx
	kind string // "color" or "string"
}

func (s *AndroidValuesScope) Set(name, value string) {
	switch s.kind {
	case "color":
		s.b.push(&OpAndroidColorSet{
			Base:  newBase(s.b, ClassIdempotent),
			Name:  name,
			Value: value,
		})
	case "string":
		s.b.push(&OpAndroidStringSet{
			Base:  newBase(s.b, ClassIdempotent),
			Name:  name,
			Value: value,
		})
	default:
		panic(fmt.Sprintf("android values scope: unknown kind %q", s.kind))
	}
}

// AndroidStylesScope handles res/values/styles.xml entries.
type AndroidStylesScope struct{ b *BuildCtx }

// Set records a style with the given parent and key/value items.
func (s *AndroidStylesScope) Set(name, parent string, items map[string]string) {
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]StyleItem, len(keys))
	for i, k := range keys {
		pairs[i] = StyleItem{Name: k, Value: items[k]}
	}
	s.b.push(&OpAndroidStyleSet{
		Base:   newBase(s.b, ClassIdempotent),
		Name:   name,
		Parent: parent,
		Items:  pairs,
	})
}

// AndroidDrawablesScope writes raw bitmaps under res/drawable.
type AndroidDrawablesScope struct{ b *BuildCtx }

func (s *AndroidDrawablesScope) AddBitmap(name string, content []byte) {
	s.b.push(&OpAndroidWriteDrawable{
		Base:    newBase(s.b, ClassExclusive),
		Name:    name,
		Content: encodeBytes(content),
	})
}

// AndroidSourcesScope writes Kotlin sources to the app source tree.
type AndroidSourcesScope struct{ b *BuildCtx }

// AddFS walks the supplied embed.FS rooted at root and records one
// OpAddKotlinSource per file. Files land under
// app/src/main/java/<packagePath>/<relpath> where packagePath is the
// dot-separated Kotlin package converted to slashes.
func (s *AndroidSourcesScope) AddFS(pkg string, sources embed.FS, root string) {
	s.b.walkEmbedFS(sources, root, func(rel string, content []byte) {
		s.b.push(&OpAddKotlinSource{
			Base:    newBase(s.b, ClassExclusive),
			Package: pkg,
			RelPath: rel,
			Content: encodeBytes(content),
		})
	})
}

// AddFile records a single Kotlin file under the given package.
func (s *AndroidSourcesScope) AddFile(pkg, rel string, content []byte) {
	s.b.push(&OpAddKotlinSource{
		Base:    newBase(s.b, ClassExclusive),
		Package: pkg,
		RelPath: rel,
		Content: encodeBytes(content),
	})
}

// walkEmbedFS walks `sources` rooted at `root` and invokes emit for each
// file. The first walk error is stashed on the BuildCtx so the bridge
// runtime can fail the build instead of silently producing an incomplete op
// list. Recorders cannot propagate errors directly because Build's API is
// fluent (no `if err :=` at every call site).
func (b *BuildCtx) walkEmbedFS(sources embed.FS, root string, emit func(rel string, content []byte)) {
	if root == "" {
		root = "."
	}
	walkErr := fs.WalkDir(sources, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := sources.ReadFile(p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, root+"/")
		if rel == p {
			rel = path.Base(p)
		}
		emit(rel, data)
		return nil
	})
	if walkErr != nil && b.deferredErr == nil {
		b.deferredErr = fmt.Errorf("walk embed.FS at %q: %w", root, walkErr)
	}
}

func encodeBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func decodeBytes(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

func copyDict(d map[string]any) map[string]any {
	if d == nil {
		return nil
	}
	out := make(map[string]any, len(d))
	maps.Copy(out, d)
	return out
}

// hashBytes returns a hex sha256 of the input.
func hashBytes(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(p))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
