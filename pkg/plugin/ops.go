package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// MergeClass declares how two ops with the same identity merge.
type MergeClass int

const (
	// ClassIdempotent: same identity + same payload collapses; same identity +
	// different payload is a hard conflict.
	ClassIdempotent MergeClass = iota
	// ClassAdditive: ops merge into a deduped set; identity covers the full
	// payload so a payload difference manifests as two distinct entries.
	ClassAdditive
	// ClassExclusive: at most one op may target the identity. Identical
	// content hashes collapse; divergent content hashes conflict.
	ClassExclusive
)

func (c MergeClass) String() string {
	switch c {
	case ClassIdempotent:
		return "idempotent"
	case ClassAdditive:
		return "additive"
	case ClassExclusive:
		return "exclusive"
	default:
		return "unknown"
	}
}

// Op is the closed interface for all plugin-emitted build ops.
type Op interface {
	// Type returns the JSON discriminator (e.g. "info_plist.set_string").
	Type() string
	// MergeClass returns the merge class for this op type.
	MergeClass() MergeClass
	// Identity returns the merge identity. Two ops with equal identity
	// collide; the MergeClass dictates whether the collision is fatal or
	// collapses.
	Identity() string
	// ContentHash returns a stable hash of the full payload, used for
	// exclusive-op overlap detection.
	ContentHash() string
	// PluginPackage returns the package path of the plugin that emitted this op.
	PluginPackage() string
	// PluginID returns the friendly identifier of the plugin that emitted
	// this op (the value of Plugin.Name()).
	PluginID() string
	// Platform returns the platform target ("ios" or "android"). Empty means
	// "applies to whatever platform is being built", but in practice the
	// platform target is derived from the op type itself.
	Platform() string
}

// Base carries the common fields all ops share. Embedded in every concrete op.
// MergeClass is fixed by the op type itself (each concrete *OpFoo overrides
// MergeClass) so it doesn't appear here.
//
// The internal field is named Pkg/Ident rather than Plugin/PluginID because
// Go forbids a method and a field on the same receiver from sharing a name,
// and the public accessors are PluginPackage()/PluginID().
type Base struct {
	Pkg   string `json:"plugin"`
	Ident string `json:"plugin_id,omitempty"`
}

func newBase(b *BuildCtx, _ MergeClass) Base {
	return Base{
		Pkg:   b.pluginPackage,
		Ident: b.pluginName,
	}
}

func (b Base) PluginPackage() string { return b.Pkg }
func (b Base) PluginID() string      { return b.Ident }

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

func (o *OpInfoPlistSetString) Type() string           { return "info_plist.set_string" }
func (o *OpInfoPlistSetString) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpInfoPlistSetString) Identity() string       { return o.Type() + "|" + o.Key }
func (o *OpInfoPlistSetString) ContentHash() string    { return hashBytes(o.Key, o.Value) }
func (o *OpInfoPlistSetString) Platform() string       { return "ios" }

type OpInfoPlistSetBool struct {
	Base
	Key   string `json:"key"`
	Value bool   `json:"value"`
}

func (o *OpInfoPlistSetBool) Type() string           { return "info_plist.set_bool" }
func (o *OpInfoPlistSetBool) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpInfoPlistSetBool) Identity() string       { return o.Type() + "|" + o.Key }
func (o *OpInfoPlistSetBool) ContentHash() string {
	return hashBytes(o.Key, fmt.Sprintf("%t", o.Value))
}
func (o *OpInfoPlistSetBool) Platform() string { return "ios" }

type OpInfoPlistSetStringArray struct {
	Base
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func (o *OpInfoPlistSetStringArray) Type() string           { return "info_plist.set_string_array" }
func (o *OpInfoPlistSetStringArray) MergeClass() MergeClass { return ClassExclusive }
func (o *OpInfoPlistSetStringArray) Identity() string       { return o.Type() + "|" + o.Key }
func (o *OpInfoPlistSetStringArray) ContentHash() string {
	return hashBytes(append([]string{o.Key}, o.Values...)...)
}
func (o *OpInfoPlistSetStringArray) Platform() string { return "ios" }

type OpInfoPlistAppendArrayItem struct {
	Base
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (o *OpInfoPlistAppendArrayItem) Type() string           { return "info_plist.append_array_item" }
func (o *OpInfoPlistAppendArrayItem) MergeClass() MergeClass { return ClassAdditive }
func (o *OpInfoPlistAppendArrayItem) Identity() string {
	return o.Type() + "|" + o.Key + "|" + o.Value
}
func (o *OpInfoPlistAppendArrayItem) ContentHash() string { return hashBytes(o.Key, o.Value) }
func (o *OpInfoPlistAppendArrayItem) Platform() string    { return "ios" }

type OpInfoPlistSetDict struct {
	Base
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

func (o *OpInfoPlistSetDict) Type() string           { return "info_plist.set_dict" }
func (o *OpInfoPlistSetDict) MergeClass() MergeClass { return ClassExclusive }
func (o *OpInfoPlistSetDict) Identity() string       { return o.Type() + "|" + o.Key }
func (o *OpInfoPlistSetDict) ContentHash() string {
	return hashBytes(o.Key, canonicalJSON(o.Value))
}
func (o *OpInfoPlistSetDict) Platform() string { return "ios" }

// ---- iOS assets / storyboards / sources ---------------------------------

type OpIOSAssetsAddImageSet struct {
	Base
	Name  string `json:"name"`
	Image string `json:"image"` // base64
}

func (o *OpIOSAssetsAddImageSet) Type() string           { return "ios.assets.add_image_set" }
func (o *OpIOSAssetsAddImageSet) MergeClass() MergeClass { return ClassExclusive }
func (o *OpIOSAssetsAddImageSet) Identity() string       { return o.Type() + "|" + o.Name }
func (o *OpIOSAssetsAddImageSet) ContentHash() string    { return hashBytes(o.Name, o.Image) }
func (o *OpIOSAssetsAddImageSet) Platform() string       { return "ios" }

type OpIOSReplaceLaunchScreen struct {
	Base
	Content string `json:"content"`
}

func (o *OpIOSReplaceLaunchScreen) Type() string           { return "ios.storyboards.replace_launch_screen" }
func (o *OpIOSReplaceLaunchScreen) MergeClass() MergeClass { return ClassExclusive }
func (o *OpIOSReplaceLaunchScreen) Identity() string       { return o.Type() }
func (o *OpIOSReplaceLaunchScreen) ContentHash() string    { return hashBytes(o.Content) }
func (o *OpIOSReplaceLaunchScreen) Platform() string       { return "ios" }

type OpAddIOSSource struct {
	Base
	Group   string `json:"group"`
	RelPath string `json:"rel_path"`
	Content string `json:"content"` // base64
}

func (o *OpAddIOSSource) Type() string           { return "ios.source.add" }
func (o *OpAddIOSSource) MergeClass() MergeClass { return ClassExclusive }
func (o *OpAddIOSSource) Identity() string {
	return o.Type() + "|" + o.Group + "/" + o.RelPath
}
func (o *OpAddIOSSource) ContentHash() string { return hashBytes(o.Group, o.RelPath, o.Content) }
func (o *OpAddIOSSource) Platform() string    { return "ios" }

type OpRegistrantIOS struct {
	Base
	Symbol string `json:"symbol"`
}

func (o *OpRegistrantIOS) Type() string           { return "ios.registrant" }
func (o *OpRegistrantIOS) MergeClass() MergeClass { return ClassAdditive }
func (o *OpRegistrantIOS) Identity() string       { return o.Type() + "|" + o.Symbol }
func (o *OpRegistrantIOS) ContentHash() string    { return hashBytes(o.Symbol) }
func (o *OpRegistrantIOS) Platform() string       { return "ios" }

// ---- Android manifest ---------------------------------------------------

type OpAndroidManifestAddPermission struct {
	Base
	Name string `json:"name"`
}

func (o *OpAndroidManifestAddPermission) Type() string           { return "android.manifest.add_permission" }
func (o *OpAndroidManifestAddPermission) MergeClass() MergeClass { return ClassAdditive }
func (o *OpAndroidManifestAddPermission) Identity() string       { return o.Type() + "|" + o.Name }
func (o *OpAndroidManifestAddPermission) ContentHash() string    { return hashBytes(o.Name) }
func (o *OpAndroidManifestAddPermission) Platform() string       { return "android" }

type OpAndroidManifestAddIntentFilter struct {
	Base
	Activity string `json:"activity"`
	XML      string `json:"xml"`
}

func (o *OpAndroidManifestAddIntentFilter) Type() string           { return "android.manifest.add_intent_filter" }
func (o *OpAndroidManifestAddIntentFilter) MergeClass() MergeClass { return ClassAdditive }
func (o *OpAndroidManifestAddIntentFilter) Identity() string {
	return o.Type() + "|" + o.Activity + "|" + hashBytes(o.XML)
}
func (o *OpAndroidManifestAddIntentFilter) ContentHash() string { return hashBytes(o.Activity, o.XML) }
func (o *OpAndroidManifestAddIntentFilter) Platform() string    { return "android" }

type OpAndroidManifestSetActivityAttr struct {
	Base
	Activity string `json:"activity"`
	Attr     string `json:"attr"`
	Value    string `json:"value"`
}

func (o *OpAndroidManifestSetActivityAttr) Type() string           { return "android.manifest.set_activity_attr" }
func (o *OpAndroidManifestSetActivityAttr) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpAndroidManifestSetActivityAttr) Identity() string {
	return o.Type() + "|" + o.Activity + "|" + o.Attr
}
func (o *OpAndroidManifestSetActivityAttr) ContentHash() string {
	return hashBytes(o.Activity, o.Attr, o.Value)
}
func (o *OpAndroidManifestSetActivityAttr) Platform() string { return "android" }

type OpAndroidManifestAddMetaData struct {
	Base
	Parent string `json:"parent"` // "application" or "activity:<name>"
	Name   string `json:"name"`
	Value  string `json:"value"`
}

func (o *OpAndroidManifestAddMetaData) Type() string           { return "android.manifest.add_meta_data" }
func (o *OpAndroidManifestAddMetaData) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpAndroidManifestAddMetaData) Identity() string {
	return o.Type() + "|" + o.Parent + "|" + o.Name
}
func (o *OpAndroidManifestAddMetaData) ContentHash() string {
	return hashBytes(o.Parent, o.Name, o.Value)
}
func (o *OpAndroidManifestAddMetaData) Platform() string { return "android" }

// ---- Android resources --------------------------------------------------

type OpAndroidColorSet struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (o *OpAndroidColorSet) Type() string           { return "android.color.set" }
func (o *OpAndroidColorSet) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpAndroidColorSet) Identity() string       { return o.Type() + "|" + o.Name }
func (o *OpAndroidColorSet) ContentHash() string    { return hashBytes(o.Name, o.Value) }
func (o *OpAndroidColorSet) Platform() string       { return "android" }

type OpAndroidStringSet struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (o *OpAndroidStringSet) Type() string           { return "android.string.set" }
func (o *OpAndroidStringSet) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpAndroidStringSet) Identity() string       { return o.Type() + "|" + o.Name }
func (o *OpAndroidStringSet) ContentHash() string    { return hashBytes(o.Name, o.Value) }
func (o *OpAndroidStringSet) Platform() string       { return "android" }

type OpAndroidStyleSet struct {
	Base
	Name   string      `json:"name"`
	Parent string      `json:"parent"`
	Items  []StyleItem `json:"items"`
}

func (o *OpAndroidStyleSet) Type() string           { return "android.style.set" }
func (o *OpAndroidStyleSet) MergeClass() MergeClass { return ClassIdempotent }
func (o *OpAndroidStyleSet) Identity() string       { return o.Type() + "|" + o.Name }
func (o *OpAndroidStyleSet) ContentHash() string {
	parts := []string{o.Name, o.Parent}
	for _, it := range o.Items {
		parts = append(parts, it.Name, it.Value)
	}
	return hashBytes(parts...)
}
func (o *OpAndroidStyleSet) Platform() string { return "android" }

type OpAndroidWriteDrawable struct {
	Base
	Name    string `json:"name"`
	Content string `json:"content"` // base64
}

func (o *OpAndroidWriteDrawable) Type() string           { return "android.drawable.write" }
func (o *OpAndroidWriteDrawable) MergeClass() MergeClass { return ClassExclusive }
func (o *OpAndroidWriteDrawable) Identity() string       { return o.Type() + "|" + o.Name }
func (o *OpAndroidWriteDrawable) ContentHash() string    { return hashBytes(o.Name, o.Content) }
func (o *OpAndroidWriteDrawable) Platform() string       { return "android" }

type OpAndroidWriteResourceXML struct {
	Base
	RelPath string `json:"rel_path"`
	Content string `json:"content"`
}

func (o *OpAndroidWriteResourceXML) Type() string           { return "android.resource.write_xml" }
func (o *OpAndroidWriteResourceXML) MergeClass() MergeClass { return ClassExclusive }
func (o *OpAndroidWriteResourceXML) Identity() string       { return o.Type() + "|" + o.RelPath }
func (o *OpAndroidWriteResourceXML) ContentHash() string    { return hashBytes(o.RelPath, o.Content) }
func (o *OpAndroidWriteResourceXML) Platform() string       { return "android" }

// ---- Android sources / registrant ---------------------------------------

type OpAddKotlinSource struct {
	Base
	Package string `json:"package"`
	RelPath string `json:"rel_path"`
	Content string `json:"content"` // base64
}

func (o *OpAddKotlinSource) Type() string           { return "android.source.add" }
func (o *OpAddKotlinSource) MergeClass() MergeClass { return ClassExclusive }
func (o *OpAddKotlinSource) Identity() string {
	return o.Type() + "|" + o.Package + "/" + o.RelPath
}
func (o *OpAddKotlinSource) ContentHash() string { return hashBytes(o.Package, o.RelPath, o.Content) }
func (o *OpAddKotlinSource) Platform() string    { return "android" }

type OpRegistrantAndroid struct {
	Base
	Symbol string `json:"symbol"`
}

func (o *OpRegistrantAndroid) Type() string           { return "android.registrant" }
func (o *OpRegistrantAndroid) MergeClass() MergeClass { return ClassAdditive }
func (o *OpRegistrantAndroid) Identity() string       { return o.Type() + "|" + o.Symbol }
func (o *OpRegistrantAndroid) ContentHash() string    { return hashBytes(o.Symbol) }
func (o *OpRegistrantAndroid) Platform() string       { return "android" }

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
	return decodeBytes(s)
}
