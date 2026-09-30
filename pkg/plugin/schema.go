package plugin

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func schemaFor(pkgPath, name string, t reflect.Type) protocol.PluginSchema {
	if t == nil {
		return protocol.PluginSchema{Package: pkgPath, Name: name}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var fields []protocol.SchemaField
	if t.Kind() == reflect.Struct {
		fields = walkStruct(t)
	}
	return protocol.PluginSchema{Package: pkgPath, Name: name, Fields: fields}
}

// walkStruct describes t's yaml-visible fields, descending into nested
// structs (by value or pointer) and slices of structs, and flattening
// `yaml:",inline"` structs into the parent level as yaml.v3 does.
func walkStruct(t reflect.Type) []protocol.SchemaField {
	var out []protocol.SchemaField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		yamlName, inline := parseYAMLTag(f)
		// yaml.v3 skips unexported fields, except an embedded struct marked
		// inline, whose exported fields it still promotes.
		if yamlName == "-" || (!f.IsExported() && !(f.Anonymous && inline)) {
			continue
		}
		if inline {
			if st := structType(f.Type); st != nil {
				out = append(out, walkStruct(st)...)
				continue
			}
		}
		field := protocol.SchemaField{
			Name: yamlName,
			Type: friendlyType(f.Type),
		}
		if st := structType(f.Type); st != nil {
			field.Fields = walkStruct(st)
		} else if f.Type.Kind() == reflect.Slice || f.Type.Kind() == reflect.Array {
			if st := structType(f.Type.Elem()); st != nil {
				field.Fields = walkStruct(st)
			}
		}
		applyDriftTag(&field, f.Tag.Get("drift"))
		out = append(out, field)
	}
	return out
}

// structType returns the struct type t names directly or through
// pointers, or nil.
func structType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		return t
	}
	return nil
}

func parseYAMLTag(f reflect.StructField) (name string, inline bool) {
	tag := f.Tag.Get("yaml")
	parts := strings.Split(tag, ",")
	for _, opt := range parts[1:] {
		if opt == "inline" {
			inline = true
		}
	}
	if parts[0] == "" {
		return strings.ToLower(f.Name), inline
	}
	return parts[0], inline
}

func friendlyType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "int"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Slice, reflect.Array:
		return "[]" + friendlyType(t.Elem())
	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", friendlyType(t.Key()), friendlyType(t.Elem()))
	case reflect.Struct:
		return "struct"
	case reflect.Pointer:
		return friendlyType(t.Elem())
	default:
		return t.Kind().String()
	}
}

func applyDriftTag(f *protocol.SchemaField, tag string) {
	if tag == "" {
		return
	}
	for part := range splitCSV(tag) {
		switch {
		case part == "required":
			f.Required = true
		case strings.HasPrefix(part, "default="):
			f.Default = strings.TrimPrefix(part, "default=")
		default:
			f.Validators = append(f.Validators, part)
		}
	}
	sort.Strings(f.Validators)
}

func splitCSV(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for p := range strings.SplitSeq(s, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !yield(p) {
				return
			}
		}
	}
}
