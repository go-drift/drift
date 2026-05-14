package plugin

import (
	"encoding/json"
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
		&OpIOSReplaceLaunchScreen{Base: base, Content: "<storyboard/>"},
		&OpAddIOSSource{Base: base, Group: "Cam", RelPath: "Foo.swift", Content: "AA=="},
		&OpRegistrantIOS{Base: base, Symbol: "Foo.register"},
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
