package plugin

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
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

	ops []protocol.Op

	// errs collects recorder failures: invalid op input and embed.FS walk
	// errors. Recorders are fluent (no error return), so failures surface
	// via Err() after Build returns and abort the build.
	errs []error

	// IOS records ops for the iOS build target. Methods are no-ops on
	// non-iOS builds; plugins may unconditionally call them and the CLI
	// drops ops whose target platform does not match the build.
	IOS *IOSScope
	// Android records ops for the Android build target.
	Android *AndroidScope
	// Xtool aliases the iOS surface; xtool reuses the iOS scaffold.
	Xtool *IOSScope
}

// Err returns every error recorders captured during Build (invalid input,
// unreadable embed.FS), joined, or nil. The bridge runtime consults it
// after Plugin.Build returns; plugin unit tests should check it too.
// Invalid ops are reported here and never recorded.
func (b *BuildCtx) Err() error { return errors.Join(b.errs...) }

// NewTestCtx returns a BuildCtx suitable for plugin author unit tests. Ops
// recorded via the returned ctx can be inspected with Ops().
func NewTestCtx() *BuildCtx {
	return newBuildCtx("test/plugin", "test", "/test/project", "/test/build", "all")
}

// NewTestCtxAt returns a BuildCtx whose projectRoot is the supplied path so
// ResolveAsset reads from a real on-disk directory. Use this when a Build
// implementation needs ResolveAsset to succeed during testing.
func NewTestCtxAt(projectRoot string) *BuildCtx {
	return newBuildCtx("test/plugin", "test", projectRoot, projectRoot, "all")
}

// Ops returns the ops recorded so far in the context.
func (b *BuildCtx) Ops() []protocol.Op {
	out := make([]protocol.Op, len(b.ops))
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
		b: b,
		Colors: &AndroidValuesScope{b: b, newOp: func(base protocol.Base, name, value string) protocol.Op {
			return &protocol.OpAndroidColorSet{Base: base, Name: name, Value: value}
		}},
		Strings: &AndroidValuesScope{b: b, newOp: func(base protocol.Base, name, value string) protocol.Op {
			return &protocol.OpAndroidStringSet{Base: base, Name: name, Value: value}
		}},
		Styles: &AndroidStylesScope{b: b},
	}
	b.Android.Drawables = &AndroidDrawablesScope{b: b}
	b.Android.Sources = &AndroidSourcesScope{b: b}

	b.Xtool = b.IOS
	return b
}

// push records op if it is valid; otherwise the error joins Err().
func (b *BuildCtx) push(op protocol.Op) {
	if err := op.Validate(); err != nil {
		b.errs = append(b.errs, fmt.Errorf("%s: %w", op.Type(), err))
		return
	}
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
	s.b.push(&protocol.OpRegistrantIOS{
		Base:   newBase(s.b),
		Symbol: symbol,
	})
}

// AddPackageDependency records a SwiftPM dependency for the iOS app to
// link. Both build paths (xcodeproj and xtool) consume the same sidecar
// Drift/Plugins/Package.swift; one call produces one `.package(url:...)`
// declaration plus one `.product(name:package:)` per entry in products.
// Products are referenced against the package identity derived from the
// URL (e.g. "firebase-ios-sdk"), matching SwiftPM's own resolution rule.
func (s *IOSScope) AddPackageDependency(url string, req SPMRequirement, products []string) {
	s.b.push(&protocol.OpIOSAddPackageDependency{
		Base:        newBase(s.b),
		URL:         url,
		Requirement: req,
		Products:    append([]string(nil), products...),
	})
}

// AppDelegateRegistrant records a Swift static-function symbol to be called
// from the generated DriftPluginRegistrant.<callback>(...) method. callback
// names a fixed app-level hook (see IOSAppDelegateCallbacks); the plugin's
// symbol must implement the signature documented on that constant.
// Multiple plugins may register on the same callback; the codegen fans out
// in drift.yaml order.
func (s *IOSScope) AppDelegateRegistrant(callback IOSAppDelegateCallback, symbol string) {
	s.b.push(&protocol.OpIOSAppDelegateRegistrant{
		Base:     newBase(s.b),
		Callback: callback,
		Symbol:   symbol,
	})
}

// AddBundleResource records a file to copy into the root of the iOS app
// bundle, where Bundle.main and UIImage(named:) find it. Use for files that
// SDKs look up in the main bundle (Firebase's GoogleService-Info.plist),
// images, fonts, JSON data, or ML models. name is a plain file name: both
// iOS build paths flatten resources into the bundle root.
func (s *IOSScope) AddBundleResource(name string, content []byte) {
	s.b.push(&protocol.OpIOSAddBundleResource{
		Base:    newBase(s.b),
		Path:    name,
		Content: protocol.EncodeContent(content),
	})
}

// IOSInfoScope records Info.plist mutations.
type IOSInfoScope struct{ b *BuildCtx }

func (s *IOSInfoScope) SetString(key, value string) {
	s.b.push(&protocol.OpInfoPlistSetString{
		Base:  newBase(s.b),
		Key:   key,
		Value: value,
	})
}

func (s *IOSInfoScope) SetBool(key string, value bool) {
	s.b.push(&protocol.OpInfoPlistSetBool{
		Base:  newBase(s.b),
		Key:   key,
		Value: value,
	})
}

func (s *IOSInfoScope) SetStringArray(key string, values []string) {
	s.b.push(&protocol.OpInfoPlistSetStringArray{
		Base:   newBase(s.b),
		Key:    key,
		Values: append([]string(nil), values...),
	})
}

func (s *IOSInfoScope) AppendArrayItem(key, value string) {
	s.b.push(&protocol.OpInfoPlistAppendArrayItem{
		Base:  newBase(s.b),
		Key:   key,
		Value: value,
	})
}

func (s *IOSInfoScope) SetDict(key string, dict map[string]any) {
	s.b.push(&protocol.OpInfoPlistSetDict{
		Base:  newBase(s.b),
		Key:   key,
		Value: copyDict(dict),
	})
}

// IOSAssetsScope records additions to Runner/Assets.xcassets.
type IOSAssetsScope struct{ b *BuildCtx }

// AddImageSet adds a single-resolution image set named `name` containing img.
// The image is treated as the 1x universal entry.
func (s *IOSAssetsScope) AddImageSet(name string, img []byte) {
	s.b.push(&protocol.OpIOSAssetsAddImageSet{
		Base:  newBase(s.b),
		Name:  name,
		Image: protocol.EncodeContent(img),
	})
}

// IOSStoryboardsScope records storyboard mutations.
type IOSStoryboardsScope struct{ b *BuildCtx }

// ReplaceLaunchScreen replaces Runner/LaunchScreen.storyboard with the
// supplied content. Two plugins that try to replace the launch screen with
// divergent content conflict.
func (s *IOSStoryboardsScope) ReplaceLaunchScreen(content string) {
	s.b.push(&protocol.OpIOSReplaceLaunchScreen{
		Base:    newBase(s.b),
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
		s.b.push(&protocol.OpAddIOSSource{
			Base:    newBase(s.b),
			Group:   group,
			RelPath: rel,
			Content: protocol.EncodeContent(content),
		})
	})
}

// AddFile records a single Swift source file at Runner/Plugins/<group>/<rel>.
func (s *IOSSourcesScope) AddFile(group, rel string, content []byte) {
	s.b.push(&protocol.OpAddIOSSource{
		Base:    newBase(s.b),
		Group:   group,
		RelPath: rel,
		Content: protocol.EncodeContent(content),
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
	s.b.push(&protocol.OpRegistrantAndroid{
		Base:   newBase(s.b),
		Symbol: symbol,
	})
}

// PreActivityRegistrant records a Kotlin symbol to be called from the
// generated DriftPluginRegistrant.preActivityCreate(activity) body. Used by
// plugins that need to run before MainActivity.super.onCreate (e.g. calling
// androidx.core.splashscreen.installSplashScreen()). Symbol is the
// fully-qualified Kotlin identifier, e.g.
// com.foo.splash.Android12SplashController.install.
func (s *AndroidScope) PreActivityRegistrant(symbol string) {
	s.b.push(&protocol.OpAndroidPreActivityRegistrant{
		Base:   newBase(s.b),
		Symbol: symbol,
	})
}

// AddGradleDependency records a single Gradle dependency line to be inserted
// into the app's build.gradle dependencies block. Configuration is the
// Gradle dependency configuration ("implementation", "api", etc.); coord is
// the full coordinate including version ("group:artifact:version"). Pin
// versions explicitly; floating versions in generated build files are a CI
// heisenbug factory.
func (s *AndroidScope) AddGradleDependency(configuration, coord string) {
	s.b.push(&protocol.OpAndroidGradleAddDependency{
		Base:          newBase(s.b),
		Configuration: configuration,
		Coord:         coord,
	})
}

// ApplyGradlePlugin records an `apply plugin: "<id>"` line for the app's
// build.gradle, inserted after the android { } block (plugins such as
// Firebase's com.google.gms.google-services need the android block
// configured first). A non-empty version also declares the plugin in the
// project-level build.gradle plugins { } block so Gradle can resolve it;
// pass "" when the plugin is already on the build classpath.
func (s *AndroidScope) ApplyGradlePlugin(id, version string) {
	s.b.push(&protocol.OpAndroidGradleApplyPlugin{
		Base:    newBase(s.b),
		ID:      id,
		Version: version,
	})
}

// AddAsset records a file to drop into the Android app's assets/ directory
// at the assets-relative path. Gradle auto-bundles app/src/main/assets/, so
// no manifest edits are needed. Use for fonts, ML models, or other static
// content the app reads via AssetManager. Build-time config files belong in
// AddAppModuleFile. path is a canonical slash-separated relative path.
func (s *AndroidScope) AddAsset(path string, content []byte) {
	s.b.push(&protocol.OpAndroidAddAsset{
		Base:    newBase(s.b),
		Path:    path,
		Content: protocol.EncodeContent(content),
	})
}

// AddAppModuleFile records a file to drop into the Android app module
// directory, next to app/build.gradle. Use for build-time config that Gradle
// plugins read from there, such as Firebase's google-services.json. name is
// a plain file name; scaffold-owned names (build.gradle, src, ...) are
// rejected.
func (s *AndroidScope) AddAppModuleFile(name string, content []byte) {
	s.b.push(&protocol.OpAndroidAddAppModuleFile{
		Base:    newBase(s.b),
		Name:    name,
		Content: protocol.EncodeContent(content),
	})
}

// AndroidManifestScope records AndroidManifest.xml mutations.
type AndroidManifestScope struct{ b *BuildCtx }

func (s *AndroidManifestScope) AddPermission(name string) {
	s.b.push(&protocol.OpAndroidManifestAddPermission{
		Base: newBase(s.b),
		Name: name,
	})
}

func (s *AndroidManifestScope) AddIntentFilter(activity string, xml string) {
	s.b.push(&protocol.OpAndroidManifestAddIntentFilter{
		Base:     newBase(s.b),
		Activity: activity,
		XML:      xml,
	})
}

func (s *AndroidManifestScope) SetActivityAttr(activity, attr, value string) {
	s.b.push(&protocol.OpAndroidManifestSetActivityAttr{
		Base:     newBase(s.b),
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
	s.b.push(&protocol.OpAndroidManifestAddMetaData{
		Base:   newBase(s.b),
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
	s.b.push(&protocol.OpAndroidWriteResourceXML{
		Base:    newBase(s.b),
		RelPath: relPath,
		Content: content,
	})
}

// AndroidValuesScope records one kind of res/values entry (colors or
// strings) into Drift's plugin-owned values file for that kind.
type AndroidValuesScope struct {
	b     *BuildCtx
	newOp func(base protocol.Base, name, value string) protocol.Op
}

// Set records the value resource name = value.
func (s *AndroidValuesScope) Set(name, value string) {
	s.b.push(s.newOp(newBase(s.b), name, value))
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
	pairs := make([]protocol.StyleItem, len(keys))
	for i, k := range keys {
		pairs[i] = protocol.StyleItem{Name: k, Value: items[k]}
	}
	s.b.push(&protocol.OpAndroidStyleSet{
		Base:   newBase(s.b),
		Name:   name,
		Parent: parent,
		Items:  pairs,
	})
}

// AndroidDrawablesScope writes raw bitmaps under res/drawable.
type AndroidDrawablesScope struct{ b *BuildCtx }

func (s *AndroidDrawablesScope) AddBitmap(name string, content []byte) {
	s.b.push(&protocol.OpAndroidWriteDrawable{
		Base:    newBase(s.b),
		Name:    name,
		Content: protocol.EncodeContent(content),
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
		s.b.push(&protocol.OpAddKotlinSource{
			Base:    newBase(s.b),
			Package: pkg,
			RelPath: rel,
			Content: protocol.EncodeContent(content),
		})
	})
}

// AddFile records a single Kotlin file under the given package.
func (s *AndroidSourcesScope) AddFile(pkg, rel string, content []byte) {
	s.b.push(&protocol.OpAddKotlinSource{
		Base:    newBase(s.b),
		Package: pkg,
		RelPath: rel,
		Content: protocol.EncodeContent(content),
	})
}

// walkEmbedFS walks `sources` rooted at `root` and invokes emit for each
// file. A walk error joins Err() so the bridge runtime can fail the build
// instead of silently producing an incomplete op list. Recorders cannot
// propagate errors directly because Build's API is fluent (no `if err :=`
// at every call site).
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
	if walkErr != nil {
		b.errs = append(b.errs, fmt.Errorf("walk embed.FS at %q: %w", root, walkErr))
	}
}

func copyDict(d map[string]any) map[string]any {
	if d == nil {
		return nil
	}
	out := make(map[string]any, len(d))
	maps.Copy(out, d)
	return out
}

func newBase(b *BuildCtx) protocol.Base {
	return protocol.Base{
		Pkg:   b.pluginPackage,
		Ident: b.pluginName,
	}
}
