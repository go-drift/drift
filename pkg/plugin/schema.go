package plugin

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// PluginSchema describes one plugin's Config struct: fields, types, defaults,
// validators. Returned by the bridge `schema` command and consumed by the CLI
// for drift plugin sync validation.
type PluginSchema struct {
	Package string        `json:"package"`
	Name    string        `json:"name"`
	Fields  []SchemaField `json:"fields"`
}

// SchemaField describes one Config field.
type SchemaField struct {
	// Name is the yaml-mapped name (e.g. "background_color").
	Name string `json:"name"`
	// GoField is the Go struct field name (e.g. "BackgroundColor").
	GoField string `json:"go_field"`
	// Type is a friendly type label ("string", "bool", "int", "[]string").
	Type string `json:"type"`
	// Required is true if the field has the `required` validator tag.
	Required bool `json:"required"`
	// Default is the default literal from `default=...`, if set.
	Default string `json:"default,omitempty"`
	// Validators is the list of drift: validator names other than `required`
	// and `default=...` (e.g. "asset", "hex").
	Validators []string `json:"validators,omitempty"`
}

func schemaFor(pkgPath, name string, t reflect.Type) PluginSchema {
	if t == nil {
		return PluginSchema{Package: pkgPath, Name: name}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var fields []SchemaField
	if t.Kind() == reflect.Struct {
		fields = walkStruct(t)
	}
	return PluginSchema{Package: pkgPath, Name: name, Fields: fields}
}

func walkStruct(t reflect.Type) []SchemaField {
	var out []SchemaField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		yamlName := parseYAMLName(f)
		if yamlName == "-" {
			continue
		}
		field := SchemaField{
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

func applyDriftTag(f *SchemaField, tag string) {
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

// ValidationDiagnostic captures one schema-vs-yaml mismatch.
type ValidationDiagnostic struct {
	Plugin  string `json:"plugin"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (d ValidationDiagnostic) String() string {
	if d.Field != "" {
		return fmt.Sprintf("%s.%s: %s", d.Plugin, d.Field, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Plugin, d.Message)
}

// ValidateConfig checks a parsed config map (the result of decoding the yaml
// `config:` block) against the plugin schema. Returns one diagnostic per
// problem; an empty slice means OK.
func (s PluginSchema) ValidateConfig(config map[string]any) []ValidationDiagnostic {
	var diags []ValidationDiagnostic

	known := make(map[string]SchemaField, len(s.Fields))
	for _, f := range s.Fields {
		known[f.Name] = f
	}

	// Required-field check.
	for _, f := range s.Fields {
		if !f.Required {
			continue
		}
		if _, ok := config[f.Name]; !ok {
			diags = append(diags, ValidationDiagnostic{
				Plugin:  s.Package,
				Field:   f.Name,
				Message: "required field missing",
			})
		}
	}

	// Unknown-key check.
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := known[k]; !ok {
			diags = append(diags, ValidationDiagnostic{
				Plugin:  s.Package,
				Field:   k,
				Message: "unknown config key",
			})
		}
	}
	return diags
}
