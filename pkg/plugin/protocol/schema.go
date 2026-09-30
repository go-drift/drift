package protocol

import (
	"fmt"
	"sort"
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
