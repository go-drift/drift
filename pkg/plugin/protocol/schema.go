package protocol

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginSchema describes one plugin's Config struct: fields, types, defaults,
// validators. Returned by the bridge `schema` command. The bridge (on build)
// and the CLI (on drift plugin sync) both check config against it with
// ValidateConfig, so the two paths cannot disagree.
type PluginSchema struct {
	Package string        `json:"package"`
	Name    string        `json:"name"`
	Fields  []SchemaField `json:"fields"`
}

// SchemaField describes one Config field.
type SchemaField struct {
	// Name is the yaml-mapped name (e.g. "background_color").
	Name string `json:"name"`
	// Type is a friendly type label: "string", "bool", "int", "float",
	// "struct", or "[]"/"map[..]" compositions of those.
	Type string `json:"type"`
	// Required is true if the field has the `required` tag: the key must be
	// present in the config mapping (an explicit zero value counts).
	Required bool `json:"required"`
	// Default is the literal from `default=...`, applied when the key is
	// absent. Empty means no default.
	Default string `json:"default,omitempty"`
	// Validators lists the value checks from the drift tag other than
	// `required` and `default=` (see ValidatorHex, ValidatorAsset).
	Validators []string `json:"validators,omitempty"`
	// Fields describes the nested mapping for Type "struct" and "[]struct".
	Fields []SchemaField `json:"fields,omitempty"`
}

// Value validators accepted in a `drift:"..."` tag.
const (
	// ValidatorHex requires a colour string "#RRGGBB" or "#RRGGBBAA".
	ValidatorHex = "hex"
	// ValidatorAsset requires a project-relative path to an existing file.
	ValidatorAsset = "asset"
)

var hexColorRe = regexp.MustCompile(`^#([0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

// Check reports whether the schema itself is well formed: known validators
// on string fields only, no field both required and defaulted, defaults
// only on scalar fields and parseable as the field's type, and defaults
// that pass the field's validators (asset excepted: defaults cannot name
// project files). The bridge panics on a malformed schema at startup, so
// a bad drift tag fails loudly before any config is read.
func (s PluginSchema) Check() error {
	return checkFields("", s.Fields)
}

func checkFields(prefix string, fields []SchemaField) error {
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		p := prefix + f.Name
		if seen[f.Name] {
			return fmt.Errorf("%s: declared twice", p)
		}
		seen[f.Name] = true
		for _, v := range f.Validators {
			switch v {
			case ValidatorHex, ValidatorAsset:
				if f.Type != "string" {
					return fmt.Errorf("%s: validator %q needs a string field, not %s", p, v, f.Type)
				}
			default:
				return fmt.Errorf("%s: unknown validator %q (known: %s, %s)", p, v, ValidatorHex, ValidatorAsset)
			}
		}
		if f.Default != "" {
			if f.Required {
				return fmt.Errorf("%s: a field cannot be both required and defaulted", p)
			}
			v, err := parseDefault(f)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			for _, name := range f.Validators {
				if name == ValidatorAsset {
					return fmt.Errorf("%s: an asset field cannot have a default", p)
				}
				if msg := runValidator(name, v, ""); msg != "" {
					return fmt.Errorf("%s: default %q: %s", p, f.Default, msg)
				}
			}
		}
		if err := checkFields(p+".", f.Fields); err != nil {
			return err
		}
	}
	return nil
}

// parseDefault converts a default literal to the value a YAML decode of the
// field would produce. String defaults are taken verbatim, so "#FFFFFF" is
// not read as a YAML comment.
func parseDefault(f SchemaField) (any, error) {
	if f.Type == "string" {
		return f.Default, nil
	}
	var v any
	if err := yaml.Unmarshal([]byte(f.Default), &v); err != nil {
		return nil, fmt.Errorf("default %q: %w", f.Default, err)
	}
	ok := false
	switch f.Type {
	case "bool":
		_, ok = v.(bool)
	case "int":
		_, ok = v.(int)
	case "float":
		switch v.(type) {
		case int, float64:
			ok = true
		}
	default:
		return nil, fmt.Errorf("defaults are only supported on scalar fields, not %s", f.Type)
	}
	if !ok {
		return nil, fmt.Errorf("default %q is not a valid %s", f.Default, f.Type)
	}
	return v, nil
}

// runValidator returns a problem description, or "" if v passes. Asset
// existence is checked under projectRoot.
func runValidator(name string, v any, projectRoot string) string {
	s, ok := v.(string)
	if !ok {
		return "must be a string"
	}
	switch name {
	case ValidatorHex:
		if _, err := ParseColor(s); err != nil {
			return err.Error()
		}
	case ValidatorAsset:
		if err := checkRelPath("asset path", s); err != nil {
			return err.Error()
		}
		info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(s)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Sprintf("asset %q not found under the project root", s)
			}
			return err.Error()
		}
		if info.IsDir() {
			return fmt.Sprintf("asset %q is a directory", s)
		}
	}
	return ""
}

// ValidationDiagnostic captures one schema-vs-yaml mismatch.
type ValidationDiagnostic struct {
	Plugin string `json:"plugin"`
	// Field is the dotted path of the offending key ("dark.image",
	// "items[0].name"); empty for plugin-level problems.
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (d ValidationDiagnostic) String() string {
	if d.Field != "" {
		return fmt.Sprintf("%s.%s: %s", d.Plugin, d.Field, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Plugin, d.Message)
}

// DiagnosticsError joins diagnostics into one error, or returns nil.
func DiagnosticsError(diags []ValidationDiagnostic) error {
	if len(diags) == 0 {
		return nil
	}
	msgs := make([]string, len(diags))
	for i, d := range diags {
		msgs[i] = d.String()
	}
	return errors.New(strings.Join(msgs, "; "))
}

// ValidateConfig checks a decoded config mapping (the drift.yaml `config:`
// block) against the schema, recursing into nested mappings and lists of
// mappings: unknown keys, missing required keys, shape mismatches for
// nested fields, and value validators (asset paths resolve under
// projectRoot). Returns one diagnostic per problem; empty means OK.
func (s PluginSchema) ValidateConfig(config map[string]any, projectRoot string) []ValidationDiagnostic {
	v := configValidator{plugin: s.Package, projectRoot: projectRoot}
	v.mapping("", s.Fields, config)
	return v.diags
}

type configValidator struct {
	plugin      string
	projectRoot string
	diags       []ValidationDiagnostic
}

func (v *configValidator) report(field, format string, args ...any) {
	v.diags = append(v.diags, ValidationDiagnostic{Plugin: v.plugin, Field: field, Message: fmt.Sprintf(format, args...)})
}

func (v *configValidator) mapping(prefix string, fields []SchemaField, config map[string]any) {
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		known[f.Name] = true
		p := prefix + f.Name
		val, present := config[f.Name]
		if !present {
			if f.Required {
				v.report(p, "required field missing")
			}
			continue
		}
		v.value(p, f, val)
	}
	keys := make([]string, 0, len(config))
	for k := range config {
		if !known[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		v.report(prefix+k, "unknown config key")
	}
}

func (v *configValidator) value(p string, f SchemaField, val any) {
	switch f.Type {
	case "struct":
		m, ok := val.(map[string]any)
		if !ok {
			v.report(p, "must be a mapping")
			return
		}
		v.mapping(p+".", f.Fields, m)
		return
	case "[]struct":
		list, ok := val.([]any)
		if !ok {
			v.report(p, "must be a list")
			return
		}
		for i, e := range list {
			ep := fmt.Sprintf("%s[%d]", p, i)
			m, ok := e.(map[string]any)
			if !ok {
				v.report(ep, "must be a mapping")
				continue
			}
			v.mapping(ep+".", f.Fields, m)
		}
		return
	}
	for _, name := range f.Validators {
		if msg := runValidator(name, val, v.projectRoot); msg != "" {
			v.report(p, "%s", msg)
		}
	}
}

// ApplyDefaults returns a copy of config with each absent defaulted key set
// to its default, recursing into nested mappings that are present. The
// schema must have passed Check.
func (s PluginSchema) ApplyDefaults(config map[string]any) map[string]any {
	return applyDefaults(s.Fields, config)
}

func applyDefaults(fields []SchemaField, config map[string]any) map[string]any {
	out := make(map[string]any, len(config)+len(fields))
	for k, val := range config {
		out[k] = val
	}
	for _, f := range fields {
		val, present := out[f.Name]
		if !present {
			if f.Default != "" {
				d, err := parseDefault(f)
				if err != nil {
					panic(fmt.Sprintf("drift plugin: ApplyDefaults on unchecked schema: %v", err))
				}
				out[f.Name] = d
			}
			continue
		}
		switch f.Type {
		case "struct":
			if m, ok := val.(map[string]any); ok {
				out[f.Name] = applyDefaults(f.Fields, m)
			}
		case "[]struct":
			if list, ok := val.([]any); ok {
				cp := make([]any, len(list))
				for i, e := range list {
					if m, ok := e.(map[string]any); ok {
						cp[i] = applyDefaults(f.Fields, m)
					} else {
						cp[i] = e
					}
				}
				out[f.Name] = cp
			}
		}
	}
	return out
}
