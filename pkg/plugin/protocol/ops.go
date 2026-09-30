package protocol

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Op is the closed interface for all plugin-emitted build ops.
type Op interface {
	// Type returns the JSON discriminator (e.g. "info_plist.set_string").
	Type() string
	// Targets names every location the op writes. Two ops touching the
	// same target must agree (see Target); the CLI's conflict check keys
	// on targets, not op types, so different op types writing one file or
	// plist key are caught.
	Targets() []Target
	// PluginPackage returns the package path of the plugin that emitted this op.
	PluginPackage() string
	// PluginID returns the friendly identifier of the plugin that emitted
	// this op (the value of Plugin.Name()).
	PluginID() string
	// Validate reports the first invalid field. Recorders run it before
	// recording an op and DecodeOps runs it on every op from the bridge,
	// so the CLI never applies an invalid op.
	Validate() error
	// Platform returns the platform target ("ios" or "android"). Empty means
	// "applies to whatever platform is being built", but in practice the
	// platform target is derived from the op type itself.
	Platform() string
}

// Base carries the common fields all ops share. Embedded in every concrete op.
//
// The internal field is named Pkg/Ident rather than Plugin/PluginID because
// Go forbids a method and a field on the same receiver from sharing a name,
// and the public accessors are PluginPackage()/PluginID().
type Base struct {
	Pkg   string `json:"plugin"`
	Ident string `json:"plugin_id,omitempty"`
}

func (b Base) PluginPackage() string { return b.Pkg }
func (b Base) PluginID() string      { return b.Ident }

// Target is one location an op writes, such as a plist key, a resource
// name or a file. Key names the location. Member, when set, names one
// element of a set stored at Key (a permission, a registrant, a Gradle
// dependency); an op that owns the whole location leaves it empty. Content
// is a hash of what the op writes there.
//
// Conflict rule, per Key: owners must agree on Content; members with the
// same Member must agree on Content; and a Key cannot have both an owner
// and members. Agreeing ops collapse.
type Target struct {
	Key     string
	Member  string
	Content string
}

// String renders the target for diagnostics.
func (t Target) String() string {
	if t.Member == "" {
		return t.Key
	}
	return t.Key + " [" + t.Member + "]"
}

func owner(key string, content ...string) Target {
	return Target{Key: key, Content: hashBytes(content...)}
}

func member(key, m string, content ...string) Target {
	return Target{Key: key, Member: m, Content: hashBytes(content...)}
}

// StyleItem is one <item name="..."> entry in a style.
type StyleItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ---- iOS Info.plist -----------------------------------------------------

type OpInfoPlistSetString struct {
	Base
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (o *OpInfoPlistSetString) Type() string     { return "info_plist.set_string" }
func (o *OpInfoPlistSetString) Platform() string { return "ios" }
func (o *OpInfoPlistSetString) Targets() []Target {
	return []Target{owner("plist:"+o.Key, o.Type(), o.Value)}
}
func (o *OpInfoPlistSetString) Validate() error {
	return checkNonEmpty("plist key", o.Key)
}

type OpInfoPlistSetBool struct {
	Base
	Key   string `json:"key"`
	Value bool   `json:"value"`
}

func (o *OpInfoPlistSetBool) Type() string     { return "info_plist.set_bool" }
func (o *OpInfoPlistSetBool) Platform() string { return "ios" }
func (o *OpInfoPlistSetBool) Targets() []Target {
	return []Target{owner("plist:"+o.Key, o.Type(), strconv.FormatBool(o.Value))}
}
func (o *OpInfoPlistSetBool) Validate() error {
	return checkNonEmpty("plist key", o.Key)
}

type OpInfoPlistSetStringArray struct {
	Base
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func (o *OpInfoPlistSetStringArray) Type() string     { return "info_plist.set_string_array" }
func (o *OpInfoPlistSetStringArray) Platform() string { return "ios" }
func (o *OpInfoPlistSetStringArray) Targets() []Target {
	return []Target{owner("plist:"+o.Key, append([]string{o.Type()}, o.Values...)...)}
}
func (o *OpInfoPlistSetStringArray) Validate() error {
	return checkNonEmpty("plist key", o.Key)
}

type OpInfoPlistAppendArrayItem struct {
	Base
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (o *OpInfoPlistAppendArrayItem) Type() string     { return "info_plist.append_array_item" }
func (o *OpInfoPlistAppendArrayItem) Platform() string { return "ios" }
func (o *OpInfoPlistAppendArrayItem) Targets() []Target {
	return []Target{member("plist:"+o.Key, o.Value)}
}
func (o *OpInfoPlistAppendArrayItem) Validate() error {
	return checkNonEmpty("plist key", o.Key)
}

type OpInfoPlistSetDict struct {
	Base
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

func (o *OpInfoPlistSetDict) Type() string     { return "info_plist.set_dict" }
func (o *OpInfoPlistSetDict) Platform() string { return "ios" }
func (o *OpInfoPlistSetDict) Targets() []Target {
	return []Target{owner("plist:"+o.Key, o.Type(), canonicalJSON(o.Value))}
}
func (o *OpInfoPlistSetDict) Validate() error {
	if err := checkNonEmpty("plist key", o.Key); err != nil {
		return err
	}
	return checkPlistValue(o.Key, map[string]any(o.Value))
}

// ---- iOS assets / storyboards / sources ---------------------------------

type OpIOSAssetsAddImageSet struct {
	Base
	Name  string `json:"name"`
	Image string `json:"image"` // base64
}

func (o *OpIOSAssetsAddImageSet) Type() string     { return "ios.assets.add_image_set" }
func (o *OpIOSAssetsAddImageSet) Platform() string { return "ios" }
func (o *OpIOSAssetsAddImageSet) Targets() []Target {
	// xtool builds emit the set as a loose <Name>.png in the bundle root,
	// so it also claims that bundle name on every iOS path.
	return []Target{
		owner("ios-asset:"+o.Name, o.Image),
		owner("ios-bundle:"+o.Name+".png", o.Type(), o.Image),
	}
}
func (o *OpIOSAssetsAddImageSet) Validate() error {
	// xtool builds emit image sets as loose <Name>.png bundle files, so the
	// name must also be a valid bundle file name on every iOS path.
	if err := validateBundleFileName(o.Name + ".png"); err != nil {
		return fmt.Errorf("image set name: %w", err)
	}
	return checkContent("image set image", o.Image, true)
}

type OpIOSReplaceLaunchScreen struct {
	Base
	Content string `json:"content"`
}

func (o *OpIOSReplaceLaunchScreen) Type() string     { return "ios.storyboards.replace_launch_screen" }
func (o *OpIOSReplaceLaunchScreen) Platform() string { return "ios" }
func (o *OpIOSReplaceLaunchScreen) Targets() []Target {
	return []Target{owner("ios:launch-screen", o.Content)}
}
func (o *OpIOSReplaceLaunchScreen) Validate() error {
	return checkXMLRoot("launch screen storyboard", o.Content, "document")
}

type OpAddIOSSource struct {
	Base
	Group   string `json:"group"`
	RelPath string `json:"rel_path"`
	Content string `json:"content"` // base64
}

func (o *OpAddIOSSource) Type() string     { return "ios.source.add" }
func (o *OpAddIOSSource) Platform() string { return "ios" }
func (o *OpAddIOSSource) Targets() []Target {
	file := o.Group + "/" + o.RelPath
	// Every plugin Swift file compiles into one module, where swiftc
	// requires unique file basenames.
	return []Target{
		owner("ios-src:"+file, o.Content),
		owner("ios-swift-basename:"+path.Base(o.RelPath), file, o.Content),
	}
}
func (o *OpAddIOSSource) Validate() error {
	if err := checkMatch(sourceGroupRe, "source group", o.Group); err != nil {
		return err
	}
	if err := checkRelPath("source path", o.RelPath); err != nil {
		return err
	}
	return checkContent("source content", o.Content, false)
}

type OpRegistrantIOS struct {
	Base
	Symbol string `json:"symbol"`
}

func (o *OpRegistrantIOS) Type() string     { return "ios.registrant" }
func (o *OpRegistrantIOS) Platform() string { return "ios" }
func (o *OpRegistrantIOS) Targets() []Target {
	return []Target{member("ios:plugins", o.Symbol)}
}
func (o *OpRegistrantIOS) Validate() error {
	return checkMatch(dottedIdentRe, "registrant symbol", o.Symbol)
}

// SPMRequirementKind names one of SwiftPM's `.package(url:...)` version
// requirement forms.
type SPMRequirementKind string

const (
	SPMFrom          SPMRequirementKind = "from"
	SPMExact         SPMRequirementKind = "exact"
	SPMBranch        SPMRequirementKind = "branch"
	SPMRevision      SPMRequirementKind = "revision"
	SPMUpToNextMajor SPMRequirementKind = "upToNextMajor"
	SPMUpToNextMinor SPMRequirementKind = "upToNextMinor"
	SPMRange         SPMRequirementKind = "range"
)

// SPMRequirement names a SwiftPM dependency version requirement. Value
// carries the version, branch, or revision; Upper is the exclusive upper
// bound and is only meaningful for SPMRange.
//
// Typed kind + string Value instead of a single "from:10.0.0"-style encoded
// string: SwiftPM has seven requirement forms and string-encoded forms
// invite parse drift between recorder, wire codec, and mutator.
type SPMRequirement struct {
	Kind  SPMRequirementKind `json:"kind"`
	Value string             `json:"value"`
	Upper string             `json:"upper,omitempty"`
}

// validateSPMRequirement errors on unknown Kind, missing Value, a range
// without an Upper bound, or characters that cannot be emitted inside a
// Swift string literal verbatim.
func validateSPMRequirement(req SPMRequirement) error {
	switch req.Kind {
	case SPMFrom, SPMExact, SPMBranch, SPMRevision, SPMUpToNextMajor, SPMUpToNextMinor, SPMRange:
	default:
		return fmt.Errorf("unknown SPM requirement kind %q", req.Kind)
	}
	if req.Value == "" {
		return fmt.Errorf("SPM requirement %q has empty value", req.Kind)
	}
	if req.Kind == SPMRange && req.Upper == "" {
		return fmt.Errorf("SPM range requirement missing upper bound")
	}
	if req.Kind != SPMRange && req.Upper != "" {
		return fmt.Errorf("SPM requirement %q must not set an upper bound", req.Kind)
	}
	if err := validateSwiftLiteral("SPM requirement value", req.Value); err != nil {
		return err
	}
	return validateSwiftLiteral("SPM requirement upper bound", req.Upper)
}

// validateSPMDependency checks a whole package dependency: a remote git URL
// (https:// or git@ form), a valid requirement, and at least one product.
// Every string must be safe to emit inside a Swift string literal verbatim.
func validateSPMDependency(url string, req SPMRequirement, products []string) error {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "git@") {
		return fmt.Errorf("SPM package URL %q must start with https:// or git@", url)
	}
	if err := validateSwiftLiteral("SPM package URL", url); err != nil {
		return err
	}
	if SPMPackageIdentity(url) == "" {
		return fmt.Errorf("SPM package URL %q has no package name segment", url)
	}
	if err := validateSPMRequirement(req); err != nil {
		return err
	}
	if len(products) == 0 {
		return fmt.Errorf("SPM package %q lists no products; the dependency would be unused", url)
	}
	for _, p := range products {
		if p == "" {
			return fmt.Errorf("SPM package %q lists an empty product name", url)
		}
		if err := validateSwiftLiteral("SPM product name", p); err != nil {
			return err
		}
	}
	return nil
}

// SPMPackageIdentity returns the SwiftPM package identity for a remote URL:
// the last path segment minus `.git`. `.product(name:package:)` references
// this identity (e.g. "firebase-ios-sdk"), not the manifest's `name:`
// (e.g. "Firebase"), so no override exists.
func SPMPackageIdentity(url string) string {
	trimmed := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	if i := strings.LastIndexAny(trimmed, "/:"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// validateSwiftLiteral rejects characters that would need escaping inside a
// Swift string literal. Values here are URLs, versions, and identifiers, so
// rejecting beats escaping: the generator can quote verbatim.
func validateSwiftLiteral(what, v string) error {
	if strings.ContainsAny(v, "\"\\\n\r") {
		return fmt.Errorf("%s %q contains a quote, backslash, or newline", what, v)
	}
	return nil
}

// canonicalString returns a deterministic, comparison-friendly encoding of
// the requirement for hashing and equality checks.
func (r SPMRequirement) canonicalString() string {
	return string(r.Kind) + ":" + r.Value + ":" + r.Upper
}

// OpIOSAddPackageDependency records a SwiftPM dependency for the iOS app
// to link. Both build paths (xcodeproj and xtool) consume the same
// sidecar Drift/Plugins/Package.swift; this op contributes one
// `.package(url:...)` declaration plus product references in that sidecar.
//
// Plugins may share a package and each ask for their own products; the
// sidecar declares the package once with the union of products. The URL
// and Requirement must match across plugins: SwiftPM cannot reconcile two
// requirements for one package identity, so a mismatch is a conflict that
// reaches the user rather than silently picking one.
type OpIOSAddPackageDependency struct {
	Base
	URL         string         `json:"url"`
	Requirement SPMRequirement `json:"requirement"`
	Products    []string       `json:"products"`
}

func (o *OpIOSAddPackageDependency) Type() string     { return "ios.spm.add_package" }
func (o *OpIOSAddPackageDependency) Platform() string { return "ios" }
func (o *OpIOSAddPackageDependency) Targets() []Target {
	id := SPMPackageIdentity(o.URL)
	// One requirement per package identity (SwiftPM cannot reconcile two),
	// while each plugin may ask for its own products from it.
	ts := []Target{owner("spm:"+id, o.URL, o.Requirement.canonicalString())}
	for _, p := range o.Products {
		ts = append(ts, member("spm-products:"+id, p))
	}
	return ts
}
func (o *OpIOSAddPackageDependency) Validate() error {
	return validateSPMDependency(o.URL, o.Requirement, o.Products)
}

// IOSAppDelegateCallback identifies an app-level iOS event that plugins can
// hook. Each variant has a fixed plugin-side Swift signature, listed on the
// constant; the plugin's static function must match it exactly. See
// cmd/drift/internal/plugin/registrant.go for the per-callback emission rules.
//
// The app is scene-based, so URL and user-activity events are delivered to
// the scene (SceneDelegate on xcodeproj builds, SwiftUI modifiers on xtool),
// not to UIApplicationDelegate. Both call sites route through the generated
// DriftPluginRegistrant, so plugins see one event regardless of build path.
type IOSAppDelegateCallback string

const (
	// func(application: UIApplication, launchOptions: [UIApplication.LaunchOptionsKey: Any]?)
	IOSCallbackDidFinishLaunching IOSAppDelegateCallback = "didFinishLaunching"
	// func(url: URL) -> Bool
	//
	// Return true to claim the URL. The first claiming plugin (in drift.yaml
	// order) stops dispatch, and Drift's own deep-link channel does
	// not see the URL. Unclaimed URLs reach the Drift deep-link channel.
	IOSCallbackOpenURL IOSAppDelegateCallback = "openURL"
	// func(userActivity: NSUserActivity) -> Bool
	//
	// Same claim semantics as IOSCallbackOpenURL. Unclaimed activities with a
	// webpageURL (universal links) reach the Drift deep-link channel.
	IOSCallbackContinueUserActivity IOSAppDelegateCallback = "continueUserActivity"
	// func(application: UIApplication, deviceToken: Data)
	IOSCallbackDidRegisterForRemoteNotifications IOSAppDelegateCallback = "didRegisterForRemoteNotifications"
	// func(application: UIApplication, error: Error)
	IOSCallbackDidFailToRegisterForRemoteNotifs IOSAppDelegateCallback = "didFailToRegisterForRemoteNotifications"
	// func(userInfo: [AnyHashable: Any], completion: @escaping (UIBackgroundFetchResult) -> Void)
	//
	// The plugin must call completion exactly once. Results from all plugins
	// are merged (.newData beats .noData beats .failed); a plugin that never
	// calls completion is cut off by a 25-second safety timeout.
	IOSCallbackDidReceiveRemoteNotification IOSAppDelegateCallback = "didReceiveRemoteNotification"
)

// IOSAppDelegateCallbacks lists every callback in struct-tag order. Codegen
// iterates this slice so generated methods always appear in the same order.
var IOSAppDelegateCallbacks = []IOSAppDelegateCallback{
	IOSCallbackDidFinishLaunching,
	IOSCallbackOpenURL,
	IOSCallbackContinueUserActivity,
	IOSCallbackDidRegisterForRemoteNotifications,
	IOSCallbackDidFailToRegisterForRemoteNotifs,
	IOSCallbackDidReceiveRemoteNotification,
}

var validIOSCallbacks = func() map[IOSAppDelegateCallback]struct{} {
	m := make(map[IOSAppDelegateCallback]struct{}, len(IOSAppDelegateCallbacks))
	for _, c := range IOSAppDelegateCallbacks {
		m[c] = struct{}{}
	}
	return m
}()

// validateIOSAppDelegateCallback errors when c is not a known callback. Used
// by recorders to parse-at-boundary (per the project's no-documented-footguns
// convention) so typos in plugin Build code surface immediately instead of
// silently producing no codegen.
func validateIOSAppDelegateCallback(c IOSAppDelegateCallback) error {
	if _, ok := validIOSCallbacks[c]; !ok {
		return fmt.Errorf("unknown iOS AppDelegate callback %q", string(c))
	}
	return nil
}

// validateBundleFileName checks an iOS bundle-resource name. Both iOS build
// paths copy resources flat into the app bundle root (Xcode's resources
// phase and xtool's `resources:` list both flatten), so the name must be a
// single file name. Rejected:
//   - empty, or containing a path separator
//   - Info.plist, which the app bundle already owns
//   - source files, which Xcode would compile into the app target
//   - types that need Xcode's compilers (storyboards, xibs, asset catalogs,
//     Core Data models, localisation folders): xtool copies them raw, so they
//     would only work on xcodeproj builds
func validateBundleFileName(name string) error {
	if name == "" {
		return fmt.Errorf("bundle resource name is empty")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("bundle resource %q must be a plain file name; resources land flat in the bundle root", name)
	}
	if name == "Info.plist" {
		return fmt.Errorf("bundle resource %q is reserved by the app bundle", name)
	}
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".swift", ".m", ".mm", ".h", ".c", ".cc", ".cpp":
		return fmt.Errorf("bundle resource %q is a source file; use IOS.Sources instead", name)
	case ".storyboard", ".xib", ".xcassets", ".xcdatamodeld", ".lproj":
		return fmt.Errorf("bundle resource %q needs Xcode compilation, which xtool builds cannot do", name)
	}
	return nil
}

// appModuleReservedNames are files and directories in the Android app
// module that the scaffold owns. A plugin file with one of these names
// would clobber the build.
var appModuleReservedNames = map[string]bool{
	"build.gradle":       true,
	"build.gradle.kts":   true,
	"proguard-rules.pro": true,
	"src":                true,
	"libs":               true,
	"build":              true,
}

// validateAppModuleFileName checks a file name for OpAndroidAddAppModuleFile:
// a single file name (no separators) that does not collide with files the
// scaffold owns in the app module.
func validateAppModuleFileName(name string) error {
	if name == "" {
		return fmt.Errorf("app module file name is empty")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("app module file %q must be a plain file name", name)
	}
	if appModuleReservedNames[name] {
		return fmt.Errorf("app module file %q is reserved by the Drift scaffold", name)
	}
	return nil
}

// OpIOSAppDelegateRegistrant records a Swift static-function symbol to be
// called from the generated DriftPluginRegistrant.<callback>(...) method.
// Each callback has a fixed signature (documented on the
// IOSAppDelegateCallback constants); plugin authors implement against it and
// the codegen renders the call. Multiple plugins can hook the same callback.
type OpIOSAppDelegateRegistrant struct {
	Base
	Callback IOSAppDelegateCallback `json:"callback"`
	Symbol   string                 `json:"symbol"`
}

func (o *OpIOSAppDelegateRegistrant) Type() string     { return "ios.app_delegate_registrant" }
func (o *OpIOSAppDelegateRegistrant) Platform() string { return "ios" }
func (o *OpIOSAppDelegateRegistrant) Targets() []Target {
	return []Target{member("ios:app-delegate:"+string(o.Callback), o.Symbol)}
}
func (o *OpIOSAppDelegateRegistrant) Validate() error {
	if err := validateIOSAppDelegateCallback(o.Callback); err != nil {
		return err
	}
	return checkMatch(dottedIdentRe, "app delegate registrant symbol", o.Symbol)
}

// OpIOSAddBundleResource records a file to copy into the root of the iOS
// app bundle, where Bundle.main and UIImage(named:) find it. Path is a plain
// file name (see validateBundleFileName).
//
// Two plugins writing divergent content to the same bundle name conflict.
type OpIOSAddBundleResource struct {
	Base
	Path    string `json:"path"`              // bundle-root file name
	Content string `json:"content,omitempty"` // base64
}

func (o *OpIOSAddBundleResource) Type() string     { return "ios.bundle.add_resource" }
func (o *OpIOSAddBundleResource) Platform() string { return "ios" }
func (o *OpIOSAddBundleResource) Targets() []Target {
	return []Target{owner("ios-bundle:"+o.Path, o.Type(), o.Content)}
}
func (o *OpIOSAddBundleResource) Validate() error {
	if err := validateBundleFileName(o.Path); err != nil {
		return err
	}
	return checkContent("bundle resource content", o.Content, false)
}

// ---- Android manifest ---------------------------------------------------

type OpAndroidManifestAddPermission struct {
	Base
	Name string `json:"name"`
}

func (o *OpAndroidManifestAddPermission) Type() string     { return "android.manifest.add_permission" }
func (o *OpAndroidManifestAddPermission) Platform() string { return "android" }
func (o *OpAndroidManifestAddPermission) Targets() []Target {
	return []Target{member("manifest:uses-permission", o.Name)}
}
func (o *OpAndroidManifestAddPermission) Validate() error {
	return checkMatch(dottedIdentRe, "permission name", o.Name)
}

type OpAndroidManifestAddIntentFilter struct {
	Base
	Activity string `json:"activity"`
	XML      string `json:"xml"`
}

func (o *OpAndroidManifestAddIntentFilter) Type() string     { return "android.manifest.add_intent_filter" }
func (o *OpAndroidManifestAddIntentFilter) Platform() string { return "android" }
func (o *OpAndroidManifestAddIntentFilter) Targets() []Target {
	return []Target{member("manifest:activity:"+o.Activity+":intent-filter", hashBytes(o.XML))}
}
func (o *OpAndroidManifestAddIntentFilter) Validate() error {
	if err := checkMatch(activityNameRe, "activity", o.Activity); err != nil {
		return err
	}
	return checkXMLRoot("intent filter", o.XML, "intent-filter")
}

type OpAndroidManifestSetActivityAttr struct {
	Base
	Activity string `json:"activity"`
	Attr     string `json:"attr"`
	Value    string `json:"value"`
}

func (o *OpAndroidManifestSetActivityAttr) Type() string     { return "android.manifest.set_activity_attr" }
func (o *OpAndroidManifestSetActivityAttr) Platform() string { return "android" }
func (o *OpAndroidManifestSetActivityAttr) Targets() []Target {
	return []Target{owner("manifest:activity:"+o.Activity+"@"+o.Attr, o.Value)}
}
func (o *OpAndroidManifestSetActivityAttr) Validate() error {
	if err := checkMatch(activityNameRe, "activity", o.Activity); err != nil {
		return err
	}
	if err := checkMatch(attrNameRe, "activity attribute", o.Attr); err != nil {
		return err
	}
	return checkNonEmpty("activity attribute value", o.Value)
}

type OpAndroidManifestAddMetaData struct {
	Base
	Parent string `json:"parent"` // "application" or "activity:<name>"
	Name   string `json:"name"`
	Value  string `json:"value"`
}

func (o *OpAndroidManifestAddMetaData) Type() string     { return "android.manifest.add_meta_data" }
func (o *OpAndroidManifestAddMetaData) Platform() string { return "android" }
func (o *OpAndroidManifestAddMetaData) Targets() []Target {
	return []Target{owner("manifest:"+o.Parent+":meta-data:"+o.Name, o.Value)}
}
func (o *OpAndroidManifestAddMetaData) Validate() error {
	if err := checkMatch(metaParentRe, "meta-data parent", o.Parent); err != nil {
		return err
	}
	return checkNonEmpty("meta-data name", o.Name)
}

// ---- Android resources --------------------------------------------------

type OpAndroidColorSet struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (o *OpAndroidColorSet) Type() string     { return "android.color.set" }
func (o *OpAndroidColorSet) Platform() string { return "android" }
func (o *OpAndroidColorSet) Targets() []Target {
	return []Target{owner("android-res:color/"+o.Name, o.Type(), o.Value)}
}
func (o *OpAndroidColorSet) Validate() error {
	if err := checkMatch(valueResNameRe, "color name", o.Name); err != nil {
		return err
	}
	return checkNonEmpty("color value", o.Value)
}

type OpAndroidStringSet struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (o *OpAndroidStringSet) Type() string     { return "android.string.set" }
func (o *OpAndroidStringSet) Platform() string { return "android" }
func (o *OpAndroidStringSet) Targets() []Target {
	return []Target{owner("android-res:string/"+o.Name, o.Value)}
}
func (o *OpAndroidStringSet) Validate() error {
	return checkMatch(valueResNameRe, "string name", o.Name)
}

type OpAndroidStyleSet struct {
	Base
	Name   string      `json:"name"`
	Parent string      `json:"parent"`
	Items  []StyleItem `json:"items"`
}

func (o *OpAndroidStyleSet) Type() string     { return "android.style.set" }
func (o *OpAndroidStyleSet) Platform() string { return "android" }
func (o *OpAndroidStyleSet) Targets() []Target {
	parts := []string{o.Parent}
	for _, it := range o.Items {
		parts = append(parts, it.Name, it.Value)
	}
	return []Target{owner("android-res:style/"+o.Name, parts...)}
}
func (o *OpAndroidStyleSet) Validate() error {
	if err := checkMatch(valueResNameRe, "style name", o.Name); err != nil {
		return err
	}
	seen := make(map[string]bool, len(o.Items))
	for _, it := range o.Items {
		if err := checkMatch(attrNameRe, "style item name", it.Name); err != nil {
			return err
		}
		if seen[it.Name] {
			return fmt.Errorf("style %q sets item %q twice", o.Name, it.Name)
		}
		seen[it.Name] = true
	}
	return nil
}

type OpAndroidWriteDrawable struct {
	Base
	Name    string `json:"name"`
	Content string `json:"content"` // base64
}

func (o *OpAndroidWriteDrawable) Type() string     { return "android.drawable.write" }
func (o *OpAndroidWriteDrawable) Platform() string { return "android" }
func (o *OpAndroidWriteDrawable) Targets() []Target {
	// Resource names ignore the file extension: icon.png and icon.webp are
	// both R.drawable.icon.
	name, _, _ := strings.Cut(o.Name, ".")
	return []Target{owner("android-res:drawable/"+name, o.Type(), o.Name, o.Content)}
}
func (o *OpAndroidWriteDrawable) Validate() error {
	if err := checkMatch(drawableNameRe, "drawable name", o.Name); err != nil {
		return err
	}
	return checkContent("drawable content", o.Content, true)
}

type OpAndroidWriteResourceXML struct {
	Base
	RelPath string `json:"rel_path"`
	Content string `json:"content"`
}

func (o *OpAndroidWriteResourceXML) Type() string     { return "android.resource.write_xml" }
func (o *OpAndroidWriteResourceXML) Platform() string { return "android" }
func (o *OpAndroidWriteResourceXML) Targets() []Target {
	ts := []Target{owner("android-res-file:"+o.RelPath, o.Content)}
	// Outside values*/ the file itself is a resource named by its stem
	// (drawable/splash.xml is R.drawable.splash), so it also claims that
	// resource in its configuration directory.
	dir, file, _ := strings.Cut(o.RelPath, "/")
	if dir != "values" && !strings.HasPrefix(dir, "values-") {
		ts = append(ts, owner("android-res:"+dir+"/"+strings.TrimSuffix(file, ".xml"), o.Type(), o.Content))
	}
	return ts
}
func (o *OpAndroidWriteResourceXML) Validate() error {
	if err := checkRelPath("resource path", o.RelPath); err != nil {
		return err
	}
	dir, file, ok := strings.Cut(o.RelPath, "/")
	if !ok || strings.Contains(file, "/") {
		return fmt.Errorf("resource path %q must be <dir>/<file>.xml", o.RelPath)
	}
	if err := checkMatch(resDirRe, "resource directory", dir); err != nil {
		return err
	}
	if err := checkMatch(resFileRe, "resource file", file); err != nil {
		return err
	}
	switch o.RelPath {
	case AndroidPluginColorsFile, AndroidPluginStringsFile, AndroidPluginStylesFile:
		return fmt.Errorf("resource path %q is owned by Drift; use Resources.Colors, Strings or Styles", o.RelPath)
	}
	return checkNonEmpty("resource content", o.Content)
}

// OpAndroidAddAsset records a file to drop into the Android app's
// `assets/` directory. Gradle's app/src/main/assets convention auto-bundles
// the directory; no manifest edits needed. The app reads these at runtime
// via AssetManager (fonts, ML models, JSON data). Build-time config such as
// google-services.json belongs in OpAndroidAddAppModuleFile instead.
//
// Two plugins writing divergent content to the same asset path conflict.
type OpAndroidAddAsset struct {
	Base
	Path    string `json:"path"`              // assets/-relative
	Content string `json:"content,omitempty"` // base64
}

func (o *OpAndroidAddAsset) Type() string     { return "android.assets.add" }
func (o *OpAndroidAddAsset) Platform() string { return "android" }
func (o *OpAndroidAddAsset) Targets() []Target {
	return []Target{owner("android-asset:"+o.Path, o.Content)}
}
func (o *OpAndroidAddAsset) Validate() error {
	if err := checkRelPath("asset path", o.Path); err != nil {
		return err
	}
	return checkContent("asset content", o.Content, false)
}

// OpAndroidAddAppModuleFile records a file to drop into the Android app
// module directory (app/<name>), next to app/build.gradle. This is where
// Gradle plugins look for build-time config, e.g. the google-services
// plugin reads app/google-services.json. Name is a plain file name (see
// validateAppModuleFileName).
//
// Two plugins writing divergent content to the same name conflict.
type OpAndroidAddAppModuleFile struct {
	Base
	Name    string `json:"name"`
	Content string `json:"content,omitempty"` // base64
}

func (o *OpAndroidAddAppModuleFile) Type() string     { return "android.app_module.add_file" }
func (o *OpAndroidAddAppModuleFile) Platform() string { return "android" }
func (o *OpAndroidAddAppModuleFile) Targets() []Target {
	return []Target{owner("android-app-file:"+o.Name, o.Content)}
}
func (o *OpAndroidAddAppModuleFile) Validate() error {
	if err := validateAppModuleFileName(o.Name); err != nil {
		return err
	}
	return checkContent("app module file content", o.Content, false)
}

// ---- Android sources / registrant ---------------------------------------

type OpAddKotlinSource struct {
	Base
	Package string `json:"package"`
	RelPath string `json:"rel_path"`
	Content string `json:"content"` // base64
}

func (o *OpAddKotlinSource) Type() string     { return "android.source.add" }
func (o *OpAddKotlinSource) Platform() string { return "android" }
func (o *OpAddKotlinSource) Targets() []Target {
	return []Target{owner("android-src:"+strings.ReplaceAll(o.Package, ".", "/")+"/"+o.RelPath, o.Content)}
}
func (o *OpAddKotlinSource) Validate() error {
	if err := checkMatch(dottedIdentRe, "Kotlin package", o.Package); err != nil {
		return err
	}
	if err := checkRelPath("source path", o.RelPath); err != nil {
		return err
	}
	return checkContent("source content", o.Content, false)
}

type OpRegistrantAndroid struct {
	Base
	Symbol string `json:"symbol"`
}

func (o *OpRegistrantAndroid) Type() string     { return "android.registrant" }
func (o *OpRegistrantAndroid) Platform() string { return "android" }
func (o *OpRegistrantAndroid) Targets() []Target {
	return []Target{member("android:plugins", o.Symbol)}
}
func (o *OpRegistrantAndroid) Validate() error {
	return checkMatch(dottedIdentRe, "registrant symbol", o.Symbol)
}

// OpAndroidPreActivityRegistrant records a Kotlin symbol to be called from
// the generated DriftPluginRegistrant.preActivityCreate(activity) body. The
// hook runs before super.onCreate, giving plugins (e.g. the splash plugin's
// Android 12+ controller) a place to call APIs like installSplashScreen()
// that require the Activity but must run pre-super.onCreate.
//
// Shape mirrors OpRegistrantAndroid: a set of symbols.
type OpAndroidPreActivityRegistrant struct {
	Base
	Symbol string `json:"symbol"`
}

func (o *OpAndroidPreActivityRegistrant) Type() string     { return "android.pre_activity_registrant" }
func (o *OpAndroidPreActivityRegistrant) Platform() string { return "android" }
func (o *OpAndroidPreActivityRegistrant) Targets() []Target {
	return []Target{member("android:pre-activity", o.Symbol)}
}
func (o *OpAndroidPreActivityRegistrant) Validate() error {
	return checkMatch(dottedIdentRe, "pre-activity registrant symbol", o.Symbol)
}

// OpAndroidGradleAddDependency records a single dependency to be inserted
// into the app's Gradle dependencies block. Configuration is the Gradle
// dependency configuration name ("implementation", "api", "testImplementation",
// etc.); Coord is the full Gradle coordinate including the version
// ("group:artifact:version").
//
// A set per configuration keyed on the full coord. Two plugins requesting
// the exact same coord+config collapse to a single dependency line; two
// plugins requesting the same artifact at different versions land as
// distinct entries, which Gradle then reconciles via standard conflict
// resolution.
type OpAndroidGradleAddDependency struct {
	Base
	Configuration string `json:"configuration"`
	Coord         string `json:"coord"`
}

func (o *OpAndroidGradleAddDependency) Type() string     { return "android.gradle.add_dependency" }
func (o *OpAndroidGradleAddDependency) Platform() string { return "android" }
func (o *OpAndroidGradleAddDependency) Targets() []Target {
	return []Target{member("gradle-dep:"+o.Configuration, o.Coord)}
}
func (o *OpAndroidGradleAddDependency) Validate() error {
	if err := checkMatch(identRe, "gradle configuration", o.Configuration); err != nil {
		return err
	}
	return checkGradleCoord(o.Coord)
}

// OpAndroidGradleApplyPlugin records a Gradle plugin id to apply in
// app/build.gradle, inserted after the android { } block (plugins such as
// com.google.gms.google-services need the android block configured first).
//
// When Version is set, the id is also declared in the project-level
// build.gradle `plugins { }` block as `id "<id>" version "<v>" apply false`,
// which puts the plugin on the build classpath. Leave Version empty for
// plugins that are already on the classpath.
//
// Same id + same Version collapse; same id + different Version conflict.
// Gradle cannot load one plugin id at two versions, so version skew must
// reach the user.
type OpAndroidGradleApplyPlugin struct {
	Base
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

func (o *OpAndroidGradleApplyPlugin) Type() string     { return "android.gradle.apply_plugin" }
func (o *OpAndroidGradleApplyPlugin) Platform() string { return "android" }
func (o *OpAndroidGradleApplyPlugin) Targets() []Target {
	return []Target{owner("gradle-plugin:"+o.ID, o.Version)}
}
func (o *OpAndroidGradleApplyPlugin) Validate() error {
	if err := checkMatch(gradleIDRe, "gradle plugin id", o.ID); err != nil {
		return err
	}
	if o.Version == "" {
		return nil
	}
	return checkMatch(gradleTokRe, "gradle plugin version", o.Version)
}

// ---- Dispatch tables ----------------------------------------------------

// opConstructors maps JSON discriminators to fresh-instance constructors so
// json.Unmarshal can target the correct concrete type.
var opConstructors = map[string]func() Op{
	"info_plist.set_string":                 func() Op { return &OpInfoPlistSetString{} },
	"info_plist.set_bool":                   func() Op { return &OpInfoPlistSetBool{} },
	"info_plist.set_string_array":           func() Op { return &OpInfoPlistSetStringArray{} },
	"info_plist.append_array_item":          func() Op { return &OpInfoPlistAppendArrayItem{} },
	"info_plist.set_dict":                   func() Op { return &OpInfoPlistSetDict{} },
	"ios.assets.add_image_set":              func() Op { return &OpIOSAssetsAddImageSet{} },
	"ios.storyboards.replace_launch_screen": func() Op { return &OpIOSReplaceLaunchScreen{} },
	"ios.source.add":                        func() Op { return &OpAddIOSSource{} },
	"ios.registrant":                        func() Op { return &OpRegistrantIOS{} },
	"ios.app_delegate_registrant":           func() Op { return &OpIOSAppDelegateRegistrant{} },
	"ios.bundle.add_resource":               func() Op { return &OpIOSAddBundleResource{} },
	"ios.spm.add_package":                   func() Op { return &OpIOSAddPackageDependency{} },
	"android.assets.add":                    func() Op { return &OpAndroidAddAsset{} },
	"android.app_module.add_file":           func() Op { return &OpAndroidAddAppModuleFile{} },
	"android.gradle.apply_plugin":           func() Op { return &OpAndroidGradleApplyPlugin{} },
	"android.manifest.add_permission":       func() Op { return &OpAndroidManifestAddPermission{} },
	"android.manifest.add_intent_filter":    func() Op { return &OpAndroidManifestAddIntentFilter{} },
	"android.manifest.set_activity_attr":    func() Op { return &OpAndroidManifestSetActivityAttr{} },
	"android.manifest.add_meta_data":        func() Op { return &OpAndroidManifestAddMetaData{} },
	"android.color.set":                     func() Op { return &OpAndroidColorSet{} },
	"android.string.set":                    func() Op { return &OpAndroidStringSet{} },
	"android.style.set":                     func() Op { return &OpAndroidStyleSet{} },
	"android.drawable.write":                func() Op { return &OpAndroidWriteDrawable{} },
	"android.resource.write_xml":            func() Op { return &OpAndroidWriteResourceXML{} },
	"android.source.add":                    func() Op { return &OpAddKotlinSource{} },
	"android.pre_activity_registrant":       func() Op { return &OpAndroidPreActivityRegistrant{} },
	"android.gradle.add_dependency":         func() Op { return &OpAndroidGradleAddDependency{} },
	"android.registrant":                    func() Op { return &OpRegistrantAndroid{} },
}

// OpTypes lists every known op discriminator in deterministic order. Used by
// tests to assert round-tripping covers every type.
func OpTypes() []string {
	out := make([]string, 0, len(opConstructors))
	for k := range opConstructors {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// NewOp constructs a zero-value Op for the given JSON discriminator.
func NewOp(opType string) (Op, error) {
	ctor, ok := opConstructors[opType]
	if !ok {
		return nil, fmt.Errorf("unknown op type %q", opType)
	}
	return ctor(), nil
}

// MarshalOp produces the wire JSON for an op (an object with a type field).
func MarshalOp(o Op) ([]byte, error) {
	payload, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	// Insert "type" field. Easier to round-trip via map[string]json.RawMessage.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}
	tt, _ := json.Marshal(o.Type())
	fields["type"] = tt
	return marshalSortedMap(fields)
}

// UnmarshalOp decodes a wire JSON object into the matching concrete Op.
func UnmarshalOp(data []byte) (Op, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("op decode: %w", err)
	}
	if head.Type == "" {
		return nil, fmt.Errorf("op decode: missing type field")
	}
	op, err := NewOp(head.Type)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, op); err != nil {
		return nil, fmt.Errorf("op decode %s: %w", head.Type, err)
	}
	return op, nil
}

// DecodeOps converts a slice of raw JSON ops into typed, validated Ops: the
// boundary parser for a bridge Response.Ops list. Every op it returns has
// passed Validate, so mutators need not re-check their input.
func DecodeOps(raws []json.RawMessage) ([]Op, error) {
	out := make([]Op, 0, len(raws))
	for i, raw := range raws {
		op, err := UnmarshalOp(raw)
		if err != nil {
			return nil, fmt.Errorf("op %d: %w", i, err)
		}
		if err := op.Validate(); err != nil {
			return nil, fmt.Errorf("op %d (%s from %s): %w", i, op.Type(), op.PluginPackage(), err)
		}
		out = append(out, op)
	}
	return out, nil
}

// MarshalOpList encodes a slice of ops as a JSON array.
func MarshalOpList(ops []Op) ([]byte, error) {
	parts := make([]json.RawMessage, len(ops))
	for i, op := range ops {
		raw, err := MarshalOp(op)
		if err != nil {
			return nil, err
		}
		parts[i] = raw
	}
	return json.Marshal(parts)
}

// UnmarshalOpList decodes a JSON array of ops.
func UnmarshalOpList(data []byte) ([]Op, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("op list decode: %w", err)
	}
	out := make([]Op, len(raws))
	for i, raw := range raws {
		op, err := UnmarshalOp(raw)
		if err != nil {
			return nil, fmt.Errorf("op list[%d]: %w", i, err)
		}
		out[i] = op
	}
	return out, nil
}

// canonicalJSON returns a deterministic JSON encoding of v. Maps with any
// value type are normalised by round-tripping through encoding/json into the
// generic any form before sorting, so a nested map[string]string hashes the
// same way a semantically identical map[string]any does.
func canonicalJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return ""
	}
	b, err := canonicalEncode(generic)
	if err != nil {
		return ""
	}
	return string(b)
}

// canonicalEncode expects v to be the generic form produced by
// json.Unmarshal into any: map[string]any, []any, string, float64, bool, nil.
// Anything else falls through to json.Marshal (e.g. json.Number); callers
// should normalise first via the JSON round-trip in canonicalJSON.
func canonicalEncode(v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return []byte("null"), nil
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			b.Write(kb)
			b.WriteByte(':')
			vb, err := canonicalEncode(t[k])
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte('}')
		return []byte(b.String()), nil
	case []any:
		var b strings.Builder
		b.WriteByte('[')
		for i, item := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			vb, err := canonicalEncode(item)
			if err != nil {
				return nil, err
			}
			b.Write(vb)
		}
		b.WriteByte(']')
		return []byte(b.String()), nil
	default:
		return json.Marshal(t)
	}
}

// marshalSortedMap encodes a map[string]json.RawMessage with sorted keys for
// stable output.
func marshalSortedMap(m map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		b.Write(m[k])
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// DecodeContent returns the decoded base64 payload of an op carrying file
// bytes. Callers must know which ops have a Content field; this is a
// convenience for mutators.
func DecodeContent(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// EncodeContent is the inverse of DecodeContent: the base64 wire form of a
// file payload carried in an op's Content field.
func EncodeContent(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
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
