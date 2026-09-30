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

func walkStruct(t reflect.Type) []protocol.SchemaField {
	var out []protocol.SchemaField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		yamlName := parseYAMLName(f)
		if yamlName == "-" {
			continue
		}
		field := protocol.SchemaField{
			Name:    yamlName,
			GoField: f.Name,
			Type:    friendlyType(f.Type),
		}
		applyDriftTag(&field, f.Tag.Get("drift"))
		out = append(out, field)
	}
	return out
}

func parseYAMLName(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" {
		return strings.ToLower(f.Name)
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return strings.ToLower(f.Name)
	}
	return parts[0]
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
