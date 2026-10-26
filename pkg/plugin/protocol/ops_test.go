package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// fixtureOps returns one concrete op per known op type.
func fixtureOps() []Op {
	base := Base{Pkg: "github.com/test/plugin", Ident: "test"}
	return []Op{
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Foo"}, Value: "bar"},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Baz"}, Value: true},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Arr"}, Value: []any{"a", "b"}},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Arr2"}, Value: "x"},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "CFBundleURLTypes"}, Value: map[string]any{"CFBundleURLSchemes": []any{"x"}}},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Dict"}, Value: map[string]any{"k": "v"}},
		&OpIOSAssetsAddImageSet{Base: base, Name: "Logo", Image: "AA=="},
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<document/>"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "Foo.swift", Content: "AA=="},
		&OpIOSPlugin{Base: base, Class: "FooPlugin"},
		&OpAddFile{Base: base, Location: FileIOSBundle, Name: "GoogleService-Info.plist", Content: "AA=="},
		&OpAddFile{Base: base, Location: FileIOSPluginConfig, Name: "DriftPluginConfig-test.json", Content: enc(`{"greeting":"hi"}`)},
		&OpAddFile{Base: base, Location: FileAndroidPluginConfig, Name: "test.json", Content: enc(`{"greeting":"hi"}`)},
		&OpAddFile{Base: base, Location: FileAndroidAppModule, Name: "google-services.json", Content: "AA=="},
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
		&OpAndroidManifestAddComponent{Base: base, XML: `<service android:name="com.foo.PushService" android:exported="false"/>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<receiver android:name="com.foo.AlarmReceiver" android:exported="false"/>`},
		&OpAndroidColorSet{Base: base, Name: "splash_bg", Value: "#FFFFFF"},
		&OpAndroidStringSet{Base: base, Name: "hello", Value: "world"},
		&OpAndroidStyleSet{Base: base, Name: "X", Parent: "Y", Items: []StyleItem{{Name: "a", Value: "b"}}},
		&OpAddFile{Base: base, Location: FileAndroidDrawable, Name: "icon", Content: "AA=="},
		&OpAddFile{Base: base, Location: FileAndroidResource, Name: "raw/foo.xml", Content: enc("<x/>")},
		&OpAddKotlinSource{Base: base, Package: "com.foo", RelPath: "Foo.kt", Content: "AA=="},
		&OpAndroidPlugin{Base: base, Class: "com.foo.FooPlugin"},
		&OpAndroidGradleAddDependency{Base: base, Configuration: "implementation", Coord: "androidx.core:core-splashscreen:1.0.1"},
	}
}

func enc(s string) string { return EncodeContent([]byte(s)) }

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
		&OpPlistSet{Base: Base{Pkg: "p", Ident: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: "V"},
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
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: ""}, Value: "V"},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: "Info.plist", Key: "K"}, Value: "V"},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{Key: "K"}, Value: "V"},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: struct{}{}},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: ""}},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}},
		&OpPlistAppendArrayItem{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: map[string]any{"k": nil}},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "CFBundleIdentifier"}, Value: "com.other"},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "CFBundleURLTypes"}, Value: []any{}},
		&OpPlistSet{Base: base, PlistEntry: PlistEntry{File: PlistInfo, Key: "Dict"}, Value: map[string]any{"k": nil}},
		&OpIOSAssetsAddImageSet{Base: base, Name: "../Logo", Image: "AA=="},
		&OpIOSAssetsAddImageSet{Base: base, Name: "Logo", Image: ""},
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<storyboard/>"},
		&OpAddIOSSource{Base: base, Group: "a b", RelPath: "Foo.swift"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "../Foo.swift"},
		&OpIOSPlugin{Base: base, Class: "Foo.register"},
		&OpAddFile{Base: base, Location: FileIOSBundle, Name: "a/b.plist"},
		&OpAddFile{Base: base, Location: FileIOSBundle, Name: "b.plist", Content: "not base64"},
		&OpAddFile{Base: base, Location: FileIOSPluginConfig, Name: "DriftPluginConfig-test.json", Content: enc(`["not", "an", "object"]`)},
		&OpAddFile{Base: base, Location: FileIOSPluginConfig, Name: "DriftPluginConfig-other.json", Content: enc(`{}`)},
		&OpAddFile{Base: base, Location: FileAndroidPluginConfig, Name: "test.json", Content: enc(`null`)},
		&OpAddFile{Base: base, Location: FileAndroidPluginConfig, Name: "other.json", Content: enc(`{}`)},
		&OpAddFile{Base: base, Location: FileAndroidAppModule, Name: "build.gradle"},
		&OpAddFile{Base: base, Location: "ios.documents", Name: "a.txt"},
		&OpAndroidGradleApplyPlugin{Base: base, ID: ""},
		&OpIOSAddPackageDependency{Base: base, URL: "http://x/y", Requirement: spmFrom("1.0.0"), Products: []string{"P"}},
		&OpAndroidManifestAddPermission{Base: base, Name: "not a permission"},
		&OpAndroidManifestAddPermission{Base: base, Name: "android.permission.READ_EXTERNAL_STORAGE", MaxSDK: -1},
		&OpAndroidManifestAddIntentFilter{Base: base, Activity: ".MainActivity", XML: "<activity/>"},
		&OpAndroidManifestSetActivityAttr{Base: base, Activity: ".MainActivity", Attr: "android theme", Value: "x"},
		&OpAndroidManifestAddMetaData{Base: base, Parent: "service", Name: "foo"},
		&OpAndroidManifestAddComponent{Base: base, XML: `<activity android:name="com.foo.A"/>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<service android:name="com.foo.S"/><service android:name="com.foo.T"/>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<service android:exported="false"/>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<service android:name=".PushService" android:exported="false"/>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<service android:name="com.foo.S"><intent-filter><action android:name="x"/></intent-filter></service>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<receiver android:name=".AlarmReceiver"/>`},
		&OpAndroidManifestAddComponent{Base: base, XML: `<receiver android:name="com.foo.R"><intent-filter><action android:name="x"/></intent-filter></receiver>`},
		&OpAndroidColorSet{Base: base, Name: "splash-bg", Value: "#FFFFFF"},
		&OpAndroidColorSet{Base: base, Name: "splash_bg", Value: "#fff"},
		&OpAndroidStringSet{Base: base, Name: "1hello"},
		&OpAndroidStyleSet{Base: base, Name: "X", Items: []StyleItem{{Name: "a", Value: "1"}, {Name: "a", Value: "2"}}},
		&OpAddFile{Base: base, Location: FileAndroidDrawable, Name: "Icon", Content: "AA=="},
		&OpAddFile{Base: base, Location: FileAndroidDrawable, Name: "icon"},
		&OpAddFile{Base: base, Location: FileAndroidResource, Name: "../raw/foo.xml", Content: enc("<x/>")},
		&OpAddFile{Base: base, Location: FileAndroidResource, Name: AndroidPluginColorsFile, Content: enc("<resources/>")},
		&OpAddKotlinSource{Base: base, Package: "com.foo", RelPath: "/abs/Foo.kt"},
		&OpAddKotlinSource{Base: base, Package: "com.drift.runner", RelPath: "Foo.kt", Content: "AA=="},
		&OpAddKotlinSource{Base: base, Package: "com.drift.runner.extra", RelPath: "Foo.kt", Content: "AA=="},
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
	raw, err := MarshalOp(&OpAddFile{Base: Base{Pkg: "p", Ident: "p"}, Location: FileAndroidAppModule, Name: "../escape"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeOps([]json.RawMessage{raw})
	if err == nil || !strings.Contains(err.Error(), "file.add") {
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
	generic := &OpPlistSet{Base: Base{Pkg: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: map[string]any{
		"inner": map[string]any{"b": "2", "a": "1"},
	}}
	typed := &OpPlistSet{Base: Base{Pkg: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: map[string]any{
		"inner": map[string]string{"b": "2", "a": "1"},
	}}
	if !reflect.DeepEqual(generic.Targets(), typed.Targets()) {
		t.Errorf("targets differ: %v vs %v", generic.Targets(), typed.Targets())
	}
}

func TestDecodeOpsRejectsInvalidPluginName(t *testing.T) {
	raw, err := MarshalOp(&OpPlistSet{Base: Base{Pkg: "p", Ident: "Bad-Name"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: "V"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeOps([]json.RawMessage{raw}); err == nil || !strings.Contains(err.Error(), "plugin name") {
		t.Fatalf("expected plugin name error, got %v", err)
	}
}

// One key in Info.plist and the entitlements are different targets.
func TestPlistTargetsIncludeFile(t *testing.T) {
	info := &OpPlistSet{PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: "a"}
	ent := &OpPlistSet{PlistEntry: PlistEntry{File: PlistEntitlements, Key: "K"}, Value: "b"}
	if info.Targets()[0].Key == ent.Targets()[0].Key {
		t.Errorf("both target %q", info.Targets()[0].Key)
	}
}

// A service is keyed by its class, so two plugins declaring one service
// differently conflict, and different services do not.
func TestAddServiceTargetsName(t *testing.T) {
	a := &OpAndroidManifestAddComponent{XML: `<service android:name="com.a.S" android:exported="false"/>`}
	b := &OpAndroidManifestAddComponent{XML: `<service android:name="com.b.S" android:exported="false"/>`}
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

// An integer and a float of equal value are different plist values
// (<integer> vs <real>), so two plugins writing them conflict.
func TestSetDictTargetDistinguishesIntFromFloat(t *testing.T) {
	op := func(v any) *OpPlistSet {
		return &OpPlistSet{Base: Base{Pkg: "p"}, PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: map[string]any{"n": v}}
	}
	if reflect.DeepEqual(op(2).Targets(), op(2.0).Targets()) {
		t.Error("SetDict {n: 2} and {n: 2.0} have the same target content")
	}
}

// A plist value crosses the wire as the kind it was recorded as: a string
// stays a string, an integer an integer and a real a real, at any depth.
func TestPlistValueRoundTripKeepsValueKinds(t *testing.T) {
	base, entry := Base{Pkg: "p", Ident: "p"}, PlistEntry{File: PlistInfo, Key: "K"}
	for _, tc := range []struct{ recorded, decoded any }{
		{"s", "s"},
		{true, true},
		{[]any{"a", "b"}, []any{"a", "b"}},
		{2, int64(2)},
		{2.0, 2.0},
		{map[string]any{"n": 2, "r": 2.5, "a": []any{1.0}}, map[string]any{"n": int64(2), "r": 2.5, "a": []any{1.0}}},
	} {
		for _, op := range []Op{
			&OpPlistSet{Base: base, PlistEntry: entry, Value: tc.recorded},
			&OpPlistAppendArrayItem{Base: base, PlistEntry: entry, Value: tc.recorded},
		} {
			raw, err := MarshalOp(op)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := UnmarshalOp(raw)
			if err != nil {
				t.Fatal(err)
			}
			var got any
			switch d := decoded.(type) {
			case *OpPlistSet:
				got = d.Value
			case *OpPlistAppendArrayItem:
				got = d.Value
			}
			if !reflect.DeepEqual(got, tc.decoded) {
				t.Errorf("%s: %#v decoded as %#v, want %#v", op.Type(), tc.recorded, got, tc.decoded)
			}
		}
	}
}

// An appended string is its own member, so the ejected lock keeps the
// members string items had before items could be any plist value; any
// other item is its canonical JSON, the same however its maps are typed.
func TestAppendArrayItemMember(t *testing.T) {
	item := func(v any) string {
		return (&OpPlistAppendArrayItem{PlistEntry: PlistEntry{File: PlistInfo, Key: "K"}, Value: v}).Targets()[0].Member
	}
	if got := item("fetch"); got != "fetch" {
		t.Errorf("string member = %q, want fetch", got)
	}
	a := item(map[string]any{"CFBundleURLSchemes": []any{"x"}, "CFBundleURLName": "x"})
	b := item(map[string]any{"CFBundleURLName": "x", "CFBundleURLSchemes": []string{"x"}})
	if a != b || a != `{"CFBundleURLName":"x","CFBundleURLSchemes":["x"]}` {
		t.Errorf("dict members = %q, %q; want equal canonical JSON", a, b)
	}
}

// A file op's platform is its location's.
func TestAddFilePlatformFollowsLocation(t *testing.T) {
	for loc, want := range map[FileLocation]string{
		FileIOSBundle:           "ios",
		FileIOSPluginConfig:     "ios",
		FileAndroidDrawable:     "android",
		FileAndroidResource:     "android",
		FileAndroidAppModule:    "android",
		FileAndroidPluginConfig: "android",
		"ios.documents":         "",
	} {
		if got := (&OpAddFile{Location: loc}).Platform(); got != want {
			t.Errorf("%s: platform %q, want %q", loc, got, want)
		}
	}
}

// A component is keyed by its element and class.
func TestAddComponentTargetsTagAndName(t *testing.T) {
	op := &OpAndroidManifestAddComponent{XML: `<receiver android:name="com.a.R" android:exported="false"/>`}
	if op.Tag() != "receiver" || op.Name() != "com.a.R" {
		t.Errorf("tag %q, name %q", op.Tag(), op.Name())
	}
	if got := op.Targets()[0].Key; got != "manifest:application:receiver:com.a.R" {
		t.Errorf("target %q", got)
	}
}
