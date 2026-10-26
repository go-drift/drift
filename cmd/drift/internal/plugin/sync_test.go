package plugin

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// writeSyncProject writes an app requiring a local plugin module
// example.com/hello, whose package files are pluginFiles (relative to the
// module root), and returns the app's root. drift.yaml lists plugins.
func writeSyncProject(t *testing.T, pluginFiles map[string]string, plugins string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds a plugin bridge")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	_, here, _, _ := runtime.Caller(0)
	drift := filepath.Join(filepath.Dir(here), "..", "..", "..", "..")
	dir := t.TempDir()
	files := map[string]string{
		"hello/go.mod": "module example.com/hello\n\ngo 1.24.0\n\nrequire github.com/go-drift/drift v0.26.1\n\nreplace github.com/go-drift/drift => " + drift + "\n",
		"app/go.mod": "module example.com/app\n\ngo 1.22.0\n\nrequire (\n\tgithub.com/go-drift/drift v0.26.1\n\texample.com/hello v0.0.0\n)\n\n" +
			"replace (\n\tgithub.com/go-drift/drift => " + drift + "\n\texample.com/hello => ../hello\n)\n",
		"app/drift.yaml": "app:\n  name: app\n  id: com.example.app\n\nplugins:\n" + plugins,
		"app/main.go":    "package main\n\nfunc main() {}\n",
	}
	for rel, src := range pluginFiles {
		files["hello/"+rel] = src
	}
	writeTree(t, dir, files)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = filepath.Join(dir, "hello")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tidy plugin: %v\n%s", err, out)
	}
	return filepath.Join(dir, "app")
}

// The plugin guide's walkthrough: an app has just required a local plugin
// (go mod edit -require -replace) and listed it in drift.yaml. The plugin's
// go.mod, from go mod init, names a newer go version than the app's, so Go
// wants go.mod updated before it reads the module graph. sync --tidy must
// tidy first, or resolving the plugin fails with the "go mod tidy" error
// tidy exists to fix.
func TestSyncTidyResolvesANewlyRequiredPlugin(t *testing.T) {
	app := writeSyncProject(t, map[string]string{
		"plugin/plugin.go": `package plugin

import driftplugin "github.com/go-drift/drift/pkg/plugin"

type Config struct{}

type hello struct{}

func (hello) Name() string                                   { return "hello" }
func (hello) Build(ctx *driftplugin.BuildCtx, _ Config) error { return nil }

var Plugin driftplugin.Plugin[Config] = hello{}
`,
	}, "  - package: example.com/hello/plugin\n")

	res, err := Sync(SyncOptions{ProjectRoot: app, CLIVersion: "dev", Tidy: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("diagnostics: %v", res.Diagnostics)
	}
}

// Config the schema accepts but a plugin's Build rejects fails sync as it
// would fail the build, one diagnostic per plugin: a failing plugin does
// not hide the next. Build checks the config for every platform ("all")
// against the app's id.
func TestSyncReportsBuildErrors(t *testing.T) {
	app := writeSyncProject(t, map[string]string{
		"blank/plugin.go": `package blank

import (
	"errors"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

type Config struct {
	Description string ` + "`yaml:\"description\" drift:\"required\"`" + `
}

type blank struct{}

func (blank) Name() string { return "blank" }
func (blank) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	if strings.TrimSpace(cfg.Description) == "" {
		return errors.New("description must not be blank")
	}
	return nil
}

var Plugin driftplugin.Plugin[Config] = blank{}
`,
		"env/plugin.go": `package env

import (
	"fmt"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

type Config struct{}

type env struct{}

func (env) Name() string { return "env" }
func (env) Build(ctx *driftplugin.BuildCtx, _ Config) error {
	return fmt.Errorf("platform %q app %q", ctx.Platform(), ctx.AppID())
}

var Plugin driftplugin.Plugin[Config] = env{}
`,
	}, "  - package: example.com/hello/blank\n    config:\n      description: \"  \"\n  - package: example.com/hello/env\n")

	res, err := Sync(SyncOptions{ProjectRoot: app, CLIVersion: "dev", Tidy: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	want := []protocol.ValidationDiagnostic{
		{Plugin: "example.com/hello/blank", Message: "description must not be blank"},
		{Plugin: "example.com/hello/env", Message: `platform "all" app "com.example.app"`},
	}
	if len(res.Diagnostics) != len(want) {
		t.Fatalf("diagnostics = %v, want %v", res.Diagnostics, want)
	}
	for i, d := range res.Diagnostics {
		if d != want[i] {
			t.Errorf("diagnostic %d = %v, want %v", i, d, want[i])
		}
	}
}

// Build runs only on schema-valid config: a schema diagnostic is reported
// alone, without Build's error.
func TestSyncSkipsBuildOnSchemaErrors(t *testing.T) {
	app := writeSyncProject(t, map[string]string{
		"plugin/plugin.go": `package plugin

import (
	"errors"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

type Config struct {
	Description string ` + "`yaml:\"description\" drift:\"required\"`" + `
}

type hello struct{}

func (hello) Name() string { return "hello" }
func (hello) Build(ctx *driftplugin.BuildCtx, _ Config) error {
	return errors.New("build ran")
}

var Plugin driftplugin.Plugin[Config] = hello{}
`,
	}, "  - package: example.com/hello/plugin\n")

	res, err := Sync(SyncOptions{ProjectRoot: app, CLIVersion: "dev", Tidy: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Field != "description" {
		t.Errorf("diagnostics = %v, want one for the missing description", res.Diagnostics)
	}
}
