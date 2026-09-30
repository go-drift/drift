package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
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

type darkCfg struct {
	Image string `yaml:"image" drift:"required,asset"`
	Color string `yaml:"color" drift:"default=#000000,hex"`
}

type itemCfg struct {
	Name string `yaml:"name" drift:"required"`
}

type baseCfg struct {
	Label string `yaml:"label" drift:"default=hi"`
}

type nestedCfg struct {
	Image   string    `yaml:"image" drift:"required,asset"`
	Color   string    `yaml:"color" drift:"default=#FFFFFF,hex"`
	Fade    int       `yaml:"fade"  drift:"default=200"`
	Flag    bool      `yaml:"flag"  drift:"required"`
	Dark    *darkCfg  `yaml:"dark"`
	Items   []itemCfg `yaml:"items"`
	baseCfg `yaml:",inline"`
}

type nestedPlugin struct{ got *nestedCfg }

func (nestedPlugin) Name() string { return "nested" }
func (p nestedPlugin) Build(_ *BuildCtx, cfg nestedCfg) error {
	*p.got = cfg
	return nil
}

func TestSchemaForNested(t *testing.T) {
	s := schemaFor("p", "nested", reflect.TypeFor[nestedCfg]())
	byName := map[string]protocol.SchemaField{}
	for _, f := range s.Fields {
		byName[f.Name] = f
	}
	if f := byName["dark"]; f.Type != "struct" || len(f.Fields) != 2 || !f.Fields[0].Required {
		t.Errorf("dark: %+v", f)
	}
	if f := byName["items"]; f.Type != "[]struct" || len(f.Fields) != 1 {
		t.Errorf("items: %+v", f)
	}
	if f, ok := byName["label"]; !ok || f.Default != "hi" {
		t.Errorf("inline field label not flattened: %+v", s.Fields)
	}
}

func TestBindBuildConfig(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.png", "dark.png"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("png"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, c nestedCfg)
	}{
		{
			name: "defaults and explicit false",
			yaml: "image: a.png\nflag: false\n",
			check: func(t *testing.T, c nestedCfg) {
				if c.Color != "#FFFFFF" || c.Fade != 200 || c.Flag || c.Dark != nil || c.Label != "hi" {
					t.Errorf("resolved %+v", c)
				}
			},
		},
		{
			name: "nested defaults apply when the parent is present",
			yaml: "image: a.png\nflag: true\ndark:\n  image: dark.png\n",
			check: func(t *testing.T, c nestedCfg) {
				if c.Dark == nil || c.Dark.Color != "#000000" {
					t.Errorf("dark %+v", c.Dark)
				}
			},
		},
		{name: "missing required", yaml: "image: a.png\n", wantErr: "p.flag: required field missing"},
		{name: "nested required", yaml: "image: a.png\nflag: true\ndark: {}\n", wantErr: "dark.image: required field missing"},
		{name: "required in list", yaml: "image: a.png\nflag: true\nitems: [{name: x}, {}]\n", wantErr: "items[1].name: required field missing"},
		{name: "unknown nested key", yaml: "image: a.png\nflag: true\ndark: {image: dark.png, bogus: 1}\n", wantErr: "dark.bogus: unknown config key"},
		{name: "hex", yaml: "image: a.png\nflag: true\ncolor: red\n", wantErr: "not a hex colour"},
		{name: "asset missing", yaml: "image: nope.png\nflag: true\n", wantErr: "not found under the project root"},
		{name: "asset escapes root", yaml: "image: ../a.png\nflag: true\n", wantErr: "`..`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got nestedCfg
			b := Bind[nestedCfg]("p", nestedPlugin{got: &got})
			err := b.Build(NewTestCtxAt(root), []byte(c.yaml))
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			c.check(t, got)
		})
	}
}

type badSchemaPlugin[T any] struct{}

func (badSchemaPlugin[T]) Name() string                 { return "bad" }
func (badSchemaPlugin[T]) Build(_ *BuildCtx, _ T) error { return nil }

func expectBindPanic[T any](t *testing.T, want string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(fmt.Sprint(r), want) {
			t.Errorf("panic = %v, want it to contain %q", r, want)
		}
	}()
	Bind[T]("p", badSchemaPlugin[T]{})
}

func TestBindPanicsOnMalformedSchema(t *testing.T) {
	expectBindPanic[struct {
		A string `yaml:"a" drift:"colour"`
	}](t, `unknown validator "colour"`)
	expectBindPanic[struct {
		A string `yaml:"a" drift:"required,default=x"`
	}](t, "both required and defaulted")
	expectBindPanic[struct {
		A int `yaml:"a" drift:"hex"`
	}](t, "needs a string field")
	expectBindPanic[struct {
		A int `yaml:"a" drift:"default=abc"`
	}](t, "not a valid int")
	expectBindPanic[struct {
		A string `yaml:"a" drift:"default=blue,hex"`
	}](t, "not a hex colour")
}
