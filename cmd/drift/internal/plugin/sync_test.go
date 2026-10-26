package plugin

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The plugin guide's walkthrough: an app has just required a local plugin
// (go mod edit -require -replace) and listed it in drift.yaml. The plugin's
// go.mod, from go mod init, names a newer go version than the app's, so Go
// wants go.mod updated before it reads the module graph. sync --tidy must
// tidy first, or resolving the plugin fails with the "go mod tidy" error
// tidy exists to fix.
func TestSyncTidyResolvesANewlyRequiredPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a plugin bridge")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	_, here, _, _ := runtime.Caller(0)
	drift := filepath.Join(filepath.Dir(here), "..", "..", "..", "..")
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"hello/go.mod": "module example.com/hello\n\ngo 1.24.0\n\nrequire github.com/go-drift/drift v0.26.1\n\nreplace github.com/go-drift/drift => " + drift + "\n",
		"hello/plugin/plugin.go": `package plugin

import driftplugin "github.com/go-drift/drift/pkg/plugin"

type Config struct{}

type hello struct{}

func (hello) Name() string                                   { return "hello" }
func (hello) Build(ctx *driftplugin.BuildCtx, _ Config) error { return nil }

var Plugin driftplugin.Plugin[Config] = hello{}
`,
		"app/go.mod": "module example.com/app\n\ngo 1.22.0\n\nrequire (\n\tgithub.com/go-drift/drift v0.26.1\n\texample.com/hello v0.0.0\n)\n\n" +
			"replace (\n\tgithub.com/go-drift/drift => " + drift + "\n\texample.com/hello => ../hello\n)\n",
		"app/drift.yaml": "app:\n  name: app\n  id: com.example.app\n\nplugins:\n  - package: example.com/hello/plugin\n",
		"app/main.go":    "package main\n\nfunc main() {}\n",
	})
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = filepath.Join(dir, "hello")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tidy plugin: %v\n%s", err, out)
	}

	res, err := Sync(SyncOptions{ProjectRoot: filepath.Join(dir, "app"), CLIVersion: "dev", Tidy: true})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("diagnostics: %v", res.Diagnostics)
	}
}
