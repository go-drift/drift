package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This test invokes Main via an external-test pattern: it builds a tiny
// bridge binary in a temp dir using `go run` against a generated stub, sends
// a build envelope on stdin, and verifies the response file contents.
//
// Skipped if no go binary is available (e.g. extremely stripped CI).
func TestMainBuildRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	tmp := t.TempDir()
	bridgeFile := filepath.Join(tmp, "bridge.go")
	if err := os.WriteFile(bridgeFile, []byte(stubBridge), 0o644); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	respFile := filepath.Join(tmp, "resp.json")

	cmd := exec.Command("go", "run", bridgeFile, "--response-file="+respFile)
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	env := map[string]any{
		"api_version":  APIVersion,
		"cmd":          "build",
		"platform":     "ios",
		"project_root": tmp,
		"build_dir":    tmp,
		"plugins": []map[string]any{
			{
				"package":     "github.com/example/p",
				"config_yaml": "background_color: \"#FF00FF\"\n",
			},
		},
	}
	envBytes, _ := json.Marshal(env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}

	go func() {
		_, _ = io.WriteString(stdin, string(envBytes))
		stdin.Close()
	}()

	// Capture stderr but tolerate fmt.Println from plugin code on stdout.
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bridge run: %v\nstderr: %s", err, stderr.String())
	}

	data, err := os.ReadFile(respFile)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, data)
	}
	if resp.APIVersion != APIVersion {
		t.Errorf("api version %d, want %d", resp.APIVersion, APIVersion)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
	if len(resp.Ops) != 1 {
		t.Fatalf("expected 1 op, got %d (%s)", len(resp.Ops), string(data))
	}
	var head struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(resp.Ops[0], &head); err != nil {
		t.Fatalf("decode op: %v", err)
	}
	if head.Type != "info_plist.set_string" || head.Value != "#FF00FF" {
		t.Errorf("unexpected op: %+v", head)
	}
	// Confirm stdout noise didn't break the response file.
	if !strings.Contains(stdout.String(), "noisy plugin") {
		t.Errorf("expected plugin stdout noise to be captured; got %q", stdout.String())
	}
}

// stubBridge is a minimal bridge tool that uses pkg/plugin to round-trip a
// fixture plugin. The plugin prints to stdout to verify that plugin noise
// doesn't corrupt the response protocol.
const stubBridge = `package main

import (
	"fmt"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

type Config struct {
	BackgroundColor string ` + "`yaml:\"background_color\"`" + `
}

type Plug struct{}

func (Plug) Name() string { return "stub" }
func (Plug) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	fmt.Println("noisy plugin output that must not corrupt protocol")
	ctx.IOS.Info.SetString("BackgroundColor", cfg.BackgroundColor)
	return nil
}

func main() {
	driftplugin.Main(driftplugin.Bind("github.com/example/p", Plug{}))
}
`

func TestMainPanicsOnDuplicatePackage(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate package")
		}
	}()
	checkBindings([]Binding{
		{Package: "github.com/foo/p", Name: "a"},
		{Package: "github.com/foo/p", Name: "b"},
	})
}

func TestMainPanicsOnDuplicateName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate name")
		}
	}()
	checkBindings([]Binding{
		{Package: "github.com/foo/a", Name: "x"},
		{Package: "github.com/foo/b", Name: "x"},
	})
}

func TestUnknownEnvelopePackageSurfaces(t *testing.T) {
	resp := doBuild(Envelope{
		APIVersion: APIVersion,
		Cmd:        "build",
		Plugins: []EnvelopePlugin{
			{Package: "github.com/unknown/p", ConfigYAML: ""},
		},
	}, nil)
	if resp.Error == "" {
		t.Errorf("expected error for unknown plugin package; got %+v", resp)
	}
	if !strings.Contains(resp.Error, "github.com/unknown/p") {
		t.Errorf("error should name the package; got %q", resp.Error)
	}
}

// Sanity: BuildCtx.Ops returns the recorded ops.
func TestBuildCtxOpsRecord(t *testing.T) {
	ctx := NewTestCtx()
	ctx.IOS.Info.SetString("Foo", "Bar")
	ctx.Android.Manifest.AddPermission("android.permission.CAMERA")
	ops := ctx.Ops()
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ops))
	}
	if ops[0].Type() != "info_plist.set_string" {
		t.Errorf("op 0 type: %s", ops[0].Type())
	}
	if ops[1].Type() != "android.manifest.add_permission" {
		t.Errorf("op 1 type: %s", ops[1].Type())
	}
}

// Sanity check that BindBuild decodes YAML against the plugin's typed Config.
func TestBindBuildDecodesTypedConfig(t *testing.T) {
	type cfg struct {
		BackgroundColor string `yaml:"background_color"`
	}
	var got string
	p := pluginFn[cfg]{
		name: "test",
		build: func(ctx *BuildCtx, c cfg) error {
			got = c.BackgroundColor
			return nil
		},
	}
	b := Bind[cfg]("github.com/test/p", p)
	ctx := newBuildCtx(b.Package, b.Name, "/", "/", "all")
	if err := b.Build(ctx, []byte("background_color: '#abc'\n")); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got != "#abc" {
		t.Errorf("got %q, want %q", got, "#abc")
	}
}

func TestBindBuildRejectsUnknownKeys(t *testing.T) {
	type cfg struct {
		Color string `yaml:"color"`
	}
	p := pluginFn[cfg]{name: "test", build: func(*BuildCtx, cfg) error { return nil }}
	b := Bind[cfg]("github.com/test/p", p)
	ctx := newBuildCtx(b.Package, b.Name, "/", "/", "all")
	err := b.Build(ctx, []byte("color: red\ntpyo: oops\n"))
	if err == nil {
		t.Fatal("expected decode error for unknown key")
	}
	if !strings.Contains(err.Error(), "tpyo") {
		t.Errorf("error should name the unknown key; got %v", err)
	}
}

func TestBindPanicsOnEmptyName(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when Plugin.Name() returns empty")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "empty string") {
			t.Errorf("panic message should mention empty string; got %v", r)
		}
	}()
	type cfg struct{}
	p := pluginFn[cfg]{name: "", build: func(*BuildCtx, cfg) error { return nil }}
	_ = Bind[cfg]("github.com/test/p", p)
}

func TestBindBuildEnforcesRequiredFields(t *testing.T) {
	type cfg struct {
		Image string `yaml:"image" drift:"required"`
		Color string `yaml:"color"`
	}
	p := pluginFn[cfg]{name: "test", build: func(*BuildCtx, cfg) error { return nil }}
	b := Bind[cfg]("github.com/test/p", p)
	ctx := newBuildCtx(b.Package, b.Name, "/", "/", "all")
	err := b.Build(ctx, []byte("color: red\n"))
	if err == nil {
		t.Fatal("expected required-field error")
	}
	if !strings.Contains(err.Error(), "image") {
		t.Errorf("error should name the missing required field; got %v", err)
	}
}

type pluginFn[T any] struct {
	name  string
	build func(*BuildCtx, T) error
}

func (p pluginFn[T]) Name() string                     { return p.name }
func (p pluginFn[T]) Build(ctx *BuildCtx, cfg T) error { return p.build(ctx, cfg) }
func (p pluginFn[T]) String() string                   { return fmt.Sprintf("plugin %s", p.name) }
