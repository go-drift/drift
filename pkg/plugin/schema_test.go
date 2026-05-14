package plugin

import (
	"reflect"
	"testing"
)

type schemaConfig struct {
	Image           string   `yaml:"image"            drift:"required,asset"`
	BackgroundColor string   `yaml:"background_color" drift:"default=#FFFFFF,hex"`
	Fullscreen      bool     `yaml:"fullscreen"       drift:"default=false"`
	Tags            []string `yaml:"tags"`
}

func TestSchemaForReflectsFields(t *testing.T) {
	s := schemaFor("github.com/test/plugin", "splash", reflect.TypeFor[schemaConfig]())
	if s.Name != "splash" || s.Package != "github.com/test/plugin" {
		t.Fatalf("plugin metadata wrong: %+v", s)
	}
	if len(s.Fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(s.Fields))
	}

	want := map[string]struct {
		ftype    string
		required bool
		def      string
		vals     []string
	}{
		"image":            {ftype: "string", required: true, vals: []string{"asset"}},
		"background_color": {ftype: "string", def: "#FFFFFF", vals: []string{"hex"}},
		"fullscreen":       {ftype: "bool", def: "false"},
		"tags":             {ftype: "[]string"},
	}
	for _, f := range s.Fields {
		w, ok := want[f.Name]
		if !ok {
			t.Errorf("unexpected field %q", f.Name)
			continue
		}
		if f.Type != w.ftype {
			t.Errorf("%s: type %q, want %q", f.Name, f.Type, w.ftype)
		}
		if f.Required != w.required {
			t.Errorf("%s: required %v, want %v", f.Name, f.Required, w.required)
		}
		if f.Default != w.def {
			t.Errorf("%s: default %q, want %q", f.Name, f.Default, w.def)
		}
		if !reflect.DeepEqual(f.Validators, w.vals) {
			if len(f.Validators)+len(w.vals) != 0 {
				t.Errorf("%s: validators %v, want %v", f.Name, f.Validators, w.vals)
			}
		}
	}
}

func TestValidateConfigDetectsProblems(t *testing.T) {
	s := schemaFor("github.com/test/plugin", "splash", reflect.TypeFor[schemaConfig]())
	diags := s.ValidateConfig(map[string]any{
		"background_color": "#000",
		"unknown_key":      "oops",
	})
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d (%v)", len(diags), diags)
	}
	var sawMissing, sawUnknown bool
	for _, d := range diags {
		switch d.Field {
		case "image":
			sawMissing = true
		case "unknown_key":
			sawUnknown = true
		}
	}
	if !sawMissing || !sawUnknown {
		t.Errorf("diagnostics did not cover both cases: %v", diags)
	}
}
