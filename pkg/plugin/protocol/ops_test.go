package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

// fixtureOps returns one concrete op per known op type.
func fixtureOps() []Op {
	base := Base{Pkg: "github.com/test/plugin", Ident: "test"}
	return []Op{
		&OpInfoPlistSetString{Base: base, Key: "Foo", Value: "bar"},
		&OpInfoPlistSetBool{Base: base, Key: "Baz", Value: true},
		&OpInfoPlistSetStringArray{Base: base, Key: "Arr", Values: []string{"a", "b"}},
		&OpInfoPlistAppendArrayItem{Base: base, Key: "Arr2", Value: "x"},
		&OpInfoPlistSetDict{Base: base, Key: "Dict", Value: map[string]any{"k": "v"}},
		&OpIOSAssetsAddImageSet{Base: base, Name: "Logo", Image: "AA=="},
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<document/>"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "Foo.swift", Content: "AA=="},
		&OpRegistrantIOS{Base: base, Symbol: "Foo.register"},
		&OpIOSAppDelegateRegistrant{Base: base, Callback: IOSCallbackDidFinishLaunching, Symbol: "FooPlugin.didFinishLaunching"},
		&OpIOSAddBundleResource{Base: base, Path: "GoogleService-Info.plist", Content: "AA=="},
		&OpAndroidAddAsset{Base: base, Path: "models/model.tflite", Content: "AA=="},
		&OpAndroidAddAppModuleFile{Base: base, Name: "google-services.json", Content: "AA=="},
		&OpAndroidGradleApplyPlugin{Base: base, ID: "com.google.gms.google-services", Version: "4.4.0"},
		&OpIOSAddPackageDependency{
			Base:        base,
			URL:         "https://github.com/firebase/firebase-ios-sdk",
			Requirement: spmFrom("10.0.0"),
			Products:    []string{"FirebaseAnalytics", "FirebaseAuth"},
		},
		&OpAndroidManifestAddPermission{Base: base, Name: "android.permission.CAMERA"},
		&OpAndroidManifestAddIntentFilter{Base: base, Activity: ".MainActivity", XML: "<intent-filter/>"},
		&OpAndroidManifestSetActivityAttr{Base: base, Activity: ".MainActivity", Attr: "android:theme", Value: "@style/X"},
		&OpAndroidManifestAddMetaData{Base: base, Parent: "application", Name: "foo", Value: "bar"},
		&OpAndroidColorSet{Base: base, Name: "splash_bg", Value: "#fff"},
		&OpAndroidStringSet{Base: base, Name: "hello", Value: "world"},
		&OpAndroidStyleSet{Base: base, Name: "X", Parent: "Y", Items: []StyleItem{{Name: "a", Value: "b"}}},
		&OpAndroidWriteDrawable{Base: base, Name: "icon", Content: "AA=="},
		&OpAndroidWriteResourceXML{Base: base, RelPath: "raw/foo.xml", Content: "<x/>"},
		&OpAddKotlinSource{Base: base, Package: "com.foo", RelPath: "Foo.kt", Content: "AA=="},
		&OpRegistrantAndroid{Base: base, Symbol: "com.foo.Foo.register"},
		&OpAndroidPreActivityRegistrant{Base: base, Symbol: "com.foo.Foo.preCreate"},
		&OpAndroidGradleAddDependency{Base: base, Configuration: "implementation", Coord: "androidx.core:core-splashscreen:1.0.1"},
	}
}

func TestOpsCoverAllConstructors(t *testing.T) {
	fixtures := fixtureOps()
	seen := make(map[string]bool, len(fixtures))
	for _, op := range fixtures {
		seen[op.Type()] = true
	}
	for _, key := range OpTypes() {
		if !seen[key] {
			t.Errorf("fixtureOps missing entry for %q", key)
		}
	}
	if len(seen) != len(OpTypes()) {
		t.Errorf("fixtureOps has duplicates: %d unique types, %d constructors", len(seen), len(OpTypes()))
	}
}

func TestOpsRoundTrip(t *testing.T) {
	for _, op := range fixtureOps() {
		raw, err := MarshalOp(op)
		if err != nil {
			t.Fatalf("marshal %s: %v", op.Type(), err)
		}
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &head); err != nil || head.Type != op.Type() {
			t.Fatalf("type field missing for %s: %s", op.Type(), string(raw))
		}
		decoded, err := UnmarshalOp(raw)
		if err != nil {
			t.Fatalf("unmarshal %s: %v", op.Type(), err)
		}
		if decoded.Type() != op.Type() {
			t.Errorf("type changed: %s -> %s", op.Type(), decoded.Type())
		}
		if decoded.Identity() != op.Identity() {
			t.Errorf("%s identity changed: %q -> %q", op.Type(), op.Identity(), decoded.Identity())
		}
		if decoded.ContentHash() != op.ContentHash() {
			t.Errorf("%s content hash changed: %q -> %q", op.Type(), op.ContentHash(), decoded.ContentHash())
		}
		if decoded.MergeClass() != op.MergeClass() {
			t.Errorf("%s merge class changed", op.Type())
		}
	}
}

func TestOpsMergeClassesDeclared(t *testing.T) {
	for _, op := range fixtureOps() {
		switch op.MergeClass() {
		case ClassIdempotent, ClassAdditive, ClassExclusive:
			// OK
		default:
			t.Errorf("%s has unrecognised merge class %d", op.Type(), op.MergeClass())
		}
	}
}

func TestMarshalOpListRoundTrip(t *testing.T) {
	ops := fixtureOps()
	raw, err := MarshalOpList(ops)
	if err != nil {
		t.Fatalf("marshal list: %v", err)
	}
	decoded, err := UnmarshalOpList(raw)
	if err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(decoded) != len(ops) {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), len(ops))
	}
	for i := range ops {
		if decoded[i].Type() != ops[i].Type() {
			t.Errorf("op %d type drift: %s -> %s", i, ops[i].Type(), decoded[i].Type())
		}
	}
}

func TestUnmarshalUnknownType(t *testing.T) {
	if _, err := UnmarshalOp([]byte(`{"type":"not.a.real.op"}`)); err == nil {
		t.Error("expected error for unknown op type")
	}
	if _, err := UnmarshalOp([]byte(`{}`)); err == nil {
		t.Error("expected error for missing type field")
	}
}

func TestIdempotentIdentityIgnoresValue(t *testing.T) {
	a := &OpInfoPlistSetString{Base: Base{Pkg: "a"}, Key: "K", Value: "v1"}
	b := &OpInfoPlistSetString{Base: Base{Pkg: "b"}, Key: "K", Value: "v2"}
	if a.Identity() != b.Identity() {
		t.Errorf("idempotent identity should ignore value: %q vs %q", a.Identity(), b.Identity())
	}
	if a.ContentHash() == b.ContentHash() {
		t.Errorf("idempotent ContentHash should differ on value")
	}
}

func TestValidateSPMRequirement(t *testing.T) {
	good := []SPMRequirement{
		{Kind: "from", Value: "10.0.0"},
		{Kind: "exact", Value: "1.2.3"},
		{Kind: "branch", Value: "main"},
		{Kind: "revision", Value: "abc123"},
		{Kind: "upToNextMajor", Value: "10.0.0"},
		{Kind: "upToNextMinor", Value: "10.5.0"},
		{Kind: "range", Value: "10.0.0", Upper: "11.0.0"},
	}
	for _, r := range good {
		if err := validateSPMRequirement(r); err != nil {
			t.Errorf("expected %+v valid: %v", r, err)
		}
	}
	bad := []SPMRequirement{
		{Kind: "", Value: "10.0.0"},
		{Kind: "from", Value: ""},
		{Kind: "exatc", Value: "1.0"},                  // typo
		{Kind: "range", Value: "1.0.0"},                // missing Upper
		{Kind: "from", Value: "1.0.0", Upper: "2.0.0"}, // Upper outside range
		{Kind: "branch", Value: `main"; evil`},         // breaks the Swift literal
	}
	for _, r := range bad {
		if err := validateSPMRequirement(r); err == nil {
			t.Errorf("expected %+v invalid", r)
		}
	}
}

// Two ops on the same URL with divergent requirements produce different
// content hashes, so Validate (in conflict.go) flags them as additive
// collisions rather than silently picking one.
func TestSPMAddPackageDivergentRequirementsCollide(t *testing.T) {
	a := &OpIOSAddPackageDependency{
		Base:        Base{Pkg: "a"},
		URL:         "https://github.com/firebase/firebase-ios-sdk",
		Requirement: spmFrom("10.0.0"),
	}
	b := &OpIOSAddPackageDependency{
		Base:        Base{Pkg: "b"},
		URL:         "https://github.com/firebase/firebase-ios-sdk",
		Requirement: spmFrom("11.0.0"),
	}
	if a.Identity() != b.Identity() {
		t.Errorf("same URL must share Identity for collision detection")
	}
	if a.ContentHash() == b.ContentHash() {
		t.Errorf("divergent requirements must produce different ContentHash")
	}
}

func TestValidateSPMDependency(t *testing.T) {
	req := spmFrom("10.0.0")
	good := []struct {
		url      string
		products []string
	}{
		{"https://github.com/firebase/firebase-ios-sdk", []string{"FirebaseCore"}},
		{"https://github.com/firebase/firebase-ios-sdk.git", []string{"FirebaseCore", "FirebaseAuth"}},
		{"git@github.com:getsentry/sentry-cocoa.git", []string{"Sentry"}},
	}
	for _, c := range good {
		if err := validateSPMDependency(c.url, req, c.products); err != nil {
			t.Errorf("expected %q valid: %v", c.url, err)
		}
	}
	bad := []struct {
		name     string
		url      string
		products []string
	}{
		{"empty url", "", []string{"X"}},
		{"local path", "../Foo", []string{"X"}},
		{"http", "http://example.com/foo", []string{"X"}},
		{"no products", "https://github.com/a/b", nil},
		{"empty product", "https://github.com/a/b", []string{""}},
		{"quote in product", "https://github.com/a/b", []string{`X"`}},
		{"quote in url", `https://github.com/a/b"`, []string{"X"}},
		{"no identity", "https://", []string{"X"}},
	}
	for _, c := range bad {
		if err := validateSPMDependency(c.url, req, c.products); err == nil {
			t.Errorf("%s: expected %q invalid", c.name, c.url)
		}
	}
}

func TestSPMPackageIdentity(t *testing.T) {
	cases := map[string]string{
		"https://github.com/firebase/firebase-ios-sdk":     "firebase-ios-sdk",
		"https://github.com/firebase/firebase-ios-sdk.git": "firebase-ios-sdk",
		"https://github.com/firebase/firebase-ios-sdk/":    "firebase-ios-sdk",
		"git@github.com:getsentry/sentry-cocoa.git":        "sentry-cocoa",
		"git@example.com:toplevel.git":                     "toplevel",
	}
	for url, want := range cases {
		if got := SPMPackageIdentity(url); got != want {
			t.Errorf("SPMPackageIdentity(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestValidateBundleFileName(t *testing.T) {
	good := []string{
		"foo.json",
		"with spaces and-dashes.png",
		"GoogleService-Info.plist",
		"logo@2x.png",
		"model.mlmodelc",
		"Font.ttf",
	}
	for _, p := range good {
		if err := validateBundleFileName(p); err != nil {
			t.Errorf("expected %q valid: %v", p, err)
		}
	}
	bad := []string{
		"",
		".",
		"..",
		"nested/file.json",
		`win\file.json`,
		"/abs.json",
		"Info.plist",
		"Extra.swift",
		"Header.h",
		"Main.storyboard",
		"View.xib",
		"Assets.xcassets",
		"Model.xcdatamodeld",
		"en.lproj",
	}
	for _, p := range bad {
		if err := validateBundleFileName(p); err == nil {
			t.Errorf("expected %q invalid", p)
		}
	}
}

func TestCheckRelPath(t *testing.T) {
	good := []string{
		"foo.json",
		"deep/nested/path/file.bin",
		"with spaces and-dashes.png",
		"foo..bar.txt",
	}
	for _, p := range good {
		if err := checkRelPath("path", p); err != nil {
			t.Errorf("expected %q valid: %v", p, err)
		}
	}
	bad := []string{
		"",
		"/abs/path",
		`\win\abs`,
		"C:/drive",
		"../escape",
		"a/../b",
		`a\..\b`,
		"a//b",
		"a/./b",
		"a/",
		".",
		`a\b`,
	}
	for _, p := range bad {
		if err := checkRelPath("path", p); err == nil {
			t.Errorf("expected %q invalid", p)
		}
	}
}

func TestValidateAppModuleFileName(t *testing.T) {
	for _, p := range []string{"google-services.json", "agconnect-services.json"} {
		if err := validateAppModuleFileName(p); err != nil {
			t.Errorf("expected %q valid: %v", p, err)
		}
	}
	for _, p := range []string{"", ".", "..", "src", "libs", "build", "build.gradle", "build.gradle.kts", "proguard-rules.pro", "sub/file.json"} {
		if err := validateAppModuleFileName(p); err == nil {
			t.Errorf("expected %q invalid", p)
		}
	}
}

func TestValidateIOSAppDelegateCallback(t *testing.T) {
	for _, c := range IOSAppDelegateCallbacks {
		if err := validateIOSAppDelegateCallback(c); err != nil {
			t.Errorf("known callback %q rejected: %v", c, err)
		}
	}
	if err := validateIOSAppDelegateCallback("didFinishLaunchin"); err == nil {
		t.Error("expected error for typo'd callback")
	}
	if err := validateIOSAppDelegateCallback(""); err == nil {
		t.Error("expected error for empty callback")
	}
}

// Two ops on the same callback with different symbols must produce distinct
// identities so additive merge keeps both. Symbols register independently.
func TestIOSAppDelegateRegistrantIdentityIncludesSymbol(t *testing.T) {
	a := &OpIOSAppDelegateRegistrant{
		Base:     Base{Pkg: "p1"},
		Callback: IOSCallbackOpenURL,
		Symbol:   "PluginA.openURL",
	}
	b := &OpIOSAppDelegateRegistrant{
		Base:     Base{Pkg: "p2"},
		Callback: IOSCallbackOpenURL,
		Symbol:   "PluginB.openURL",
	}
	if a.Identity() == b.Identity() {
		t.Errorf("additive identity should distinguish symbols: %q", a.Identity())
	}
	if a.MergeClass() != ClassAdditive {
		t.Errorf("expected ClassAdditive, got %v", a.MergeClass())
	}
}

func TestAdditiveIdentityIncludesValue(t *testing.T) {
	a := &OpInfoPlistAppendArrayItem{Base: Base{Pkg: "a"}, Key: "K", Value: "x"}
	b := &OpInfoPlistAppendArrayItem{Base: Base{Pkg: "b"}, Key: "K", Value: "y"}
	if a.Identity() == b.Identity() {
		t.Errorf("additive identity should include value: both %q", a.Identity())
	}
}

// Two ops carrying the same logical dict but built from different concrete
// Go types (map[string]any vs nested map[string]string) must hash equal so
// the merge layer treats them as identical exclusive content. Without the
// JSON-roundtrip in canonicalJSON, the typed-map branch would fall through
// to json.Marshal and produce unsorted keys.
func TestSetDictContentHashStableAcrossNestedConcreteTypes(t *testing.T) {
	generic := &OpInfoPlistSetDict{
		Base: Base{Pkg: "p"},
		Key:  "K",
		Value: map[string]any{
			"inner": map[string]any{"b": "2", "a": "1"},
		},
	}
	typed := &OpInfoPlistSetDict{
		Base: Base{Pkg: "p"},
		Key:  "K",
		Value: map[string]any{
			"inner": map[string]string{"b": "2", "a": "1"},
		},
	}
	if generic.ContentHash() != typed.ContentHash() {
		t.Errorf("nested typed/generic maps must produce the same hash; got %q vs %q",
			generic.ContentHash(), typed.ContentHash())
	}
}

func spmFrom(v string) SPMRequirement { return SPMRequirement{Kind: SPMFrom, Value: v} }

func TestDecodeOpsParsesJSONList(t *testing.T) {
	ops := []Op{
		&OpInfoPlistSetString{Base: Base{Pkg: "p"}, Key: "K", Value: "V"},
	}
	raw, err := MarshalOpList(ops)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	decoded, err := DecodeOps(raws)
	if err != nil {
		t.Fatalf("DecodeOps: %v", err)
	}
	if len(decoded) != 1 {
		t.Errorf("expected 1 op, got %d", len(decoded))
	}
}

func TestDecodeOpsRejectsUnknown(t *testing.T) {
	raws := []json.RawMessage{[]byte(`{"type":"not.a.real.op"}`)}
	if _, err := DecodeOps(raws); err == nil || !strings.Contains(err.Error(), "not.a.real.op") {
		t.Errorf("expected unknown-op error, got %v", err)
	}
}

func TestFixtureOpsAreValid(t *testing.T) {
	for _, op := range fixtureOps() {
		if err := op.Validate(); err != nil {
			t.Errorf("%s: fixture should be valid: %v", op.Type(), err)
		}
	}
}

// invalidOps returns, per op type, at least one op that Validate rejects.
// TestOpValidateRejects checks the table covers every constructor.
func invalidOps() []Op {
	base := Base{Pkg: "github.com/test/plugin", Ident: "test"}
	return []Op{
		&OpInfoPlistSetString{Base: base, Key: ""},
		&OpInfoPlistSetBool{Base: base, Key: ""},
		&OpInfoPlistSetStringArray{Base: base, Key: ""},
		&OpInfoPlistAppendArrayItem{Base: base, Key: ""},
		&OpInfoPlistSetDict{Base: base, Key: "Dict", Value: map[string]any{"k": nil}},
		&OpIOSAssetsAddImageSet{Base: base, Name: "../Logo", Image: "AA=="},
		&OpIOSAssetsAddImageSet{Base: base, Name: "Logo", Image: ""},
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<storyboard/>"},
		&OpAddIOSSource{Base: base, Group: "a b", RelPath: "Foo.swift"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "../Foo.swift"},
		&OpRegistrantIOS{Base: base, Symbol: ""},
		&OpIOSAppDelegateRegistrant{Base: base, Callback: "nope", Symbol: "Foo.bar"},
		&OpIOSAddBundleResource{Base: base, Path: "a/b.plist"},
		&OpAndroidAddAsset{Base: base, Path: "models/../x.bin"},
		&OpAndroidAddAppModuleFile{Base: base, Name: "build.gradle"},
		&OpAndroidGradleApplyPlugin{Base: base, ID: ""},
		&OpIOSAddPackageDependency{Base: base, URL: "http://x/y", Requirement: spmFrom("1.0.0"), Products: []string{"P"}},
		&OpAndroidManifestAddPermission{Base: base, Name: "not a permission"},
		&OpAndroidManifestAddIntentFilter{Base: base, Activity: ".MainActivity", XML: "<activity/>"},
		&OpAndroidManifestSetActivityAttr{Base: base, Activity: ".MainActivity", Attr: "android theme", Value: "x"},
		&OpAndroidManifestAddMetaData{Base: base, Parent: "service", Name: "foo"},
		&OpAndroidColorSet{Base: base, Name: "splash-bg", Value: "#fff"},
		&OpAndroidStringSet{Base: base, Name: "1hello"},
		&OpAndroidStyleSet{Base: base, Name: "X", Items: []StyleItem{{Name: "a", Value: "1"}, {Name: "a", Value: "2"}}},
		&OpAndroidWriteDrawable{Base: base, Name: "Icon", Content: "AA=="},
		&OpAndroidWriteResourceXML{Base: base, RelPath: "../raw/foo.xml", Content: "<x/>"},
		&OpAndroidWriteResourceXML{Base: base, RelPath: AndroidPluginColorsFile, Content: "<resources/>"},
		&OpAddKotlinSource{Base: base, Package: "com.foo", RelPath: "/abs/Foo.kt"},
		&OpRegistrantAndroid{Base: base, Symbol: "com.foo.Foo.register(host)"},
		&OpAndroidPreActivityRegistrant{Base: base, Symbol: ""},
		&OpAndroidGradleAddDependency{Base: base, Configuration: "implementation", Coord: "a:b:1.0'); evil('"},
	}
}

func TestOpValidateRejects(t *testing.T) {
	covered := map[string]bool{}
	for _, op := range invalidOps() {
		covered[op.Type()] = true
		if err := op.Validate(); err == nil {
			t.Errorf("%s %+v: expected a validation error", op.Type(), op)
		}
	}
	for _, typ := range OpTypes() {
		if !covered[typ] {
			t.Errorf("invalidOps has no case for %s", typ)
		}
	}
}

func TestDecodeOpsRejectsInvalid(t *testing.T) {
	raw, err := MarshalOp(&OpAndroidAddAsset{Base: Base{Pkg: "p"}, Path: "../escape"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeOps([]json.RawMessage{raw})
	if err == nil || !strings.Contains(err.Error(), "android.assets.add") {
		t.Fatalf("expected validation error naming the op, got %v", err)
	}
}
