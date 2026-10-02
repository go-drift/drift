package protocol

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// fixtureOps returns one concrete op per known op type.
func fixtureOps() []Op {
	base := Base{Pkg: "github.com/test/plugin", Ident: "test"}
	return []Op{
		&OpPlistSetString{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Foo"}, Value: "bar"},
		&OpPlistSetBool{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Baz"}, Value: true},
		&OpPlistSetStringArray{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Arr"}, Values: []string{"a", "b"}},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Arr2"}, Value: "x"},
		&OpPlistSetDict{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Dict"}, Value: map[string]any{"k": "v"}},
		&OpIOSAssetsAddImageSet{Base: base, Name: "Logo", Image: "AA=="},
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<document/>"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "Foo.swift", Content: "AA=="},
		&OpIOSPlugin{Base: base, Class: "FooPlugin"},
		&OpIOSAddBundleResource{Base: base, Path: "GoogleService-Info.plist", Content: "AA=="},
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
		&OpAndroidManifestAddService{Base: base, XML: `<service android:name="com.foo.PushService" android:exported="false"/>`},
		&OpAndroidColorSet{Base: base, Name: "splash_bg", Value: "#FFFFFF"},
		&OpAndroidStringSet{Base: base, Name: "hello", Value: "world"},
		&OpAndroidStyleSet{Base: base, Name: "X", Parent: "Y", Items: []StyleItem{{Name: "a", Value: "b"}}},
		&OpAndroidWriteDrawable{Base: base, Name: "icon", Content: "AA=="},
		&OpAndroidWriteResourceXML{Base: base, RelPath: "raw/foo.xml", Content: "<x/>"},
		&OpAddKotlinSource{Base: base, Package: "com.foo", RelPath: "Foo.kt", Content: "AA=="},
		&OpAndroidPlugin{Base: base, Class: "com.foo.FooPlugin"},
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

// The plugin guide's op reference is hand-written; every op type must
// appear in it.
func TestOpReferenceListsEveryOp(t *testing.T) {
	guide, err := os.ReadFile("../../../website-docs/guides/plugins.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range OpTypes() {
		if !strings.Contains(string(guide), "`"+typ+"`") {
			t.Errorf("website-docs/guides/plugins.md op reference does not list %q", typ)
		}
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
		if !reflect.DeepEqual(decoded.Targets(), op.Targets()) {
			t.Errorf("%s targets changed: %v -> %v", op.Type(), op.Targets(), decoded.Targets())
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

func spmFrom(v string) SPMRequirement { return SPMRequirement{Kind: SPMFrom, Value: v} }

func TestDecodeOpsParsesJSONList(t *testing.T) {
	ops := []Op{
		&OpPlistSetString{Base: Base{Pkg: "p", Ident: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: "V"},
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
		&OpPlistSetString{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: ""}},
		&OpPlistSetString{Base: base, PlistEntry: PlistEntry{File: "Info.plist", Key: "K"}},
		&OpPlistSetString{Base: base, PlistEntry: PlistEntry{Key: "K"}},
		&OpPlistSetBool{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: ""}},
		&OpPlistSetStringArray{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: ""}},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: ""}},
		&OpPlistSetDict{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Dict"}, Value: map[string]any{"k": nil}},
		&OpIOSAssetsAddImageSet{Base: base, Name: "../Logo", Image: "AA=="},
		&OpIOSAssetsAddImageSet{Base: base, Name: "Logo", Image: ""},
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<storyboard/>"},
		&OpAddIOSSource{Base: base, Group: "a b", RelPath: "Foo.swift"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "../Foo.swift"},
		&OpIOSPlugin{Base: base, Class: "Foo.register"},
		&OpIOSAddBundleResource{Base: base, Path: "a/b.plist"},
		&OpAndroidAddAppModuleFile{Base: base, Name: "build.gradle"},
		&OpAndroidGradleApplyPlugin{Base: base, ID: ""},
		&OpIOSAddPackageDependency{Base: base, URL: "http://x/y", Requirement: spmFrom("1.0.0"), Products: []string{"P"}},
		&OpAndroidManifestAddPermission{Base: base, Name: "not a permission"},
		&OpAndroidManifestAddIntentFilter{Base: base, Activity: ".MainActivity", XML: "<activity/>"},
		&OpAndroidManifestSetActivityAttr{Base: base, Activity: ".MainActivity", Attr: "android theme", Value: "x"},
		&OpAndroidManifestAddMetaData{Base: base, Parent: "service", Name: "foo"},
		&OpAndroidManifestAddService{Base: base, XML: `<receiver android:name="com.foo.R"/>`},
		&OpAndroidManifestAddService{Base: base, XML: `<service android:exported="false"/>`},
		&OpAndroidManifestAddService{Base: base, XML: `<service android:name=".PushService" android:exported="false"/>`},
		&OpAndroidManifestAddService{Base: base, XML: `<service android:name="com.foo.S"><intent-filter><action android:name="x"/></intent-filter></service>`},
		&OpAndroidColorSet{Base: base, Name: "splash-bg", Value: "#FFFFFF"},
		&OpAndroidColorSet{Base: base, Name: "splash_bg", Value: "#fff"},
		&OpAndroidStringSet{Base: base, Name: "1hello"},
		&OpAndroidStyleSet{Base: base, Name: "X", Items: []StyleItem{{Name: "a", Value: "1"}, {Name: "a", Value: "2"}}},
		&OpAndroidWriteDrawable{Base: base, Name: "Icon", Content: "AA=="},
		&OpAndroidWriteResourceXML{Base: base, RelPath: "../raw/foo.xml", Content: "<x/>"},
		&OpAndroidWriteResourceXML{Base: base, RelPath: AndroidPluginColorsFile, Content: "<resources/>"},
		&OpAddKotlinSource{Base: base, Package: "com.foo", RelPath: "/abs/Foo.kt"},
		&OpAndroidPlugin{Base: base, Class: "FooPlugin"},
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
	raw, err := MarshalOp(&OpAndroidAddAppModuleFile{Base: Base{Pkg: "p", Ident: "p"}, Name: "../escape"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeOps([]json.RawMessage{raw})
	if err == nil || !strings.Contains(err.Error(), "android.app_module.add_file") {
		t.Fatalf("expected validation error naming the op, got %v", err)
	}
}

func TestEveryOpHasTargets(t *testing.T) {
	for _, op := range fixtureOps() {
		if len(op.Targets()) == 0 {
			t.Errorf("%s declares no targets, so it can never conflict", op.Type())
		}
	}
}

// Two dicts carrying the same logical value built from different concrete
// Go types (map[string]any vs nested map[string]string) must target the
// same content, or identical plugin output would spuriously conflict.
// Without the JSON round trip in canonicalJSON the typed map would fall
// through to json.Marshal with unsorted keys.
func TestSetDictTargetStableAcrossNestedConcreteTypes(t *testing.T) {
	generic := &OpPlistSetDict{Base: Base{Pkg: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: map[string]any{
		"inner": map[string]any{"b": "2", "a": "1"},
	}}
	typed := &OpPlistSetDict{Base: Base{Pkg: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: map[string]any{
		"inner": map[string]string{"b": "2", "a": "1"},
	}}
	if !reflect.DeepEqual(generic.Targets(), typed.Targets()) {
		t.Errorf("targets differ: %v vs %v", generic.Targets(), typed.Targets())
	}
}

func TestDecodeOpsRejectsInvalidPluginName(t *testing.T) {
	raw, err := MarshalOp(&OpPlistSetString{Base: Base{Pkg: "p", Ident: "Bad-Name"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: "V"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeOps([]json.RawMessage{raw}); err == nil || !strings.Contains(err.Error(), "plugin name") {
		t.Fatalf("expected plugin name error, got %v", err)
	}
}

// One key in Info.plist and the entitlements are different targets.
func TestPlistTargetsIncludeFile(t *testing.T) {
	info := &OpPlistSetString{PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: "a"}
	ent := &OpPlistSetString{PlistEntry: PlistEntry{File: PlistEntitlements, Key: "K"}, Value: "b"}
	if info.Targets()[0].Key == ent.Targets()[0].Key {
		t.Errorf("both target %q", info.Targets()[0].Key)
	}
}

// A service is keyed by its class, so two plugins declaring one service
// differently conflict, and different services do not.
func TestAddServiceTargetsName(t *testing.T) {
	a := &OpAndroidManifestAddService{XML: `<service android:name="com.a.S" android:exported="false"/>`}
	b := &OpAndroidManifestAddService{XML: `<service android:name="com.b.S" android:exported="false"/>`}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := a.Targets()[0].Key; got != "manifest:application:service:com.a.S" {
		t.Errorf("target %q", got)
	}
	if a.Targets()[0].Key == b.Targets()[0].Key {
		t.Error("different services share a target")
	}
}
