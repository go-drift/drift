package plugin

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// notProvided is go list's result for pkg when no module in the graph
// provides it.
func notProvided(pkg string) *PackageInfo {
	return &PackageInfo{ImportPath: pkg, Error: "no required module provides package " + pkg + "; to add it:\n\tgo get " + pkg}
}

func TestCheckPluginDepsMissing(t *testing.T) {
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/missing/plugin"}}
	resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
		return notProvided(pkg), nil
	})
	_, err := CheckPluginDeps("dev", "", plugins, resolver)
	var missing *MissingPluginError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingPluginError, got %T %v", err, err)
	}
	if len(missing.Packages) != 1 || missing.Packages[0] != "github.com/foo/missing/plugin" {
		t.Errorf("MissingPluginError packages wrong: %v", missing.Packages)
	}
	if !strings.Contains(missing.Error(), "go get github.com/foo/missing/plugin@latest") {
		t.Errorf("error did not include go get suggestion: %s", missing.Error())
	}
}

// TestMissingPluginErrorHint checks the hint. A first-party plugin ships in
// drift itself, so a missing one is a wrong path or an app on a drift that
// predates it: a release CLI names its own drift version, since a newer
// drift needs a newer CLI. Third-party plugins get `go get ...@latest`.
func TestMissingPluginErrorHint(t *testing.T) {
	const (
		splash = "github.com/go-drift/drift/plugins/splash/plugin"
		hello  = "example.com/hello/plugin"
	)
	tests := []struct {
		name string
		cli  string
		pkgs []string
		want string
	}{
		{
			name: "release CLI, first-party",
			cli:  "v0.28.0",
			pkgs: []string{splash},
			want: `Plugin package not found: github.com/go-drift/drift/plugins/splash/plugin

First-party plugins ship in github.com/go-drift/drift itself. Check the package path in drift.yaml, or update the app to a drift that has the plugin:
  go get github.com/go-drift/drift@v0.28.0

Then commit drift.yaml and tools/drift-plugins/main.go together.`,
		},
		{
			name: "release CLI, first- and third-party",
			cli:  "0.28.0",
			pkgs: []string{splash, hello},
			want: `Plugin packages not found:
  - github.com/go-drift/drift/plugins/splash/plugin
  - example.com/hello/plugin

Run:
  go get example.com/hello/plugin@latest

If a plugin's latest version requires a drift newer than this CLI (v0.28.0), get an earlier version of it.

First-party plugins ship in github.com/go-drift/drift itself. Check the package path in drift.yaml, or update the app to a drift that has the plugin:
  go get github.com/go-drift/drift@v0.28.0

Then commit drift.yaml and tools/drift-plugins/main.go together.`,
		},
		{
			name: "development CLI",
			cli:  "dev+abc123",
			pkgs: []string{splash, hello},
			want: `Plugin packages not found:
  - github.com/go-drift/drift/plugins/splash/plugin
  - example.com/hello/plugin

Run:
  go get example.com/hello/plugin@latest

First-party plugins ship in github.com/go-drift/drift itself. Check the package path in drift.yaml, or update the app to a drift that has the plugin:
  go get github.com/go-drift/drift@latest

Then commit drift.yaml and tools/drift-plugins/main.go together.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins := make([]ConfiguredPlugin, len(tt.pkgs))
			for i, p := range tt.pkgs {
				plugins[i] = ConfiguredPlugin{Package: p}
			}
			resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
				return notProvided(pkg), nil
			})
			_, err := CheckPluginDeps(tt.cli, "", plugins, resolver)
			var missing *MissingPluginError
			if !errors.As(err, &missing) {
				t.Fatalf("expected MissingPluginError, got %T %v", err, err)
			}
			if got := missing.Error(); got != tt.want {
				t.Errorf("message:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// A plugin module that go.mod requires but go.sum has no entry for is not
// missing: go cannot verify it, and `go mod tidy` adds the entry. `go get
// ...@latest` would change the version the app pins, and for drift itself
// could downgrade it.
func TestCheckPluginDepsMissingGoSum(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	// No network: go reports the missing entry before fetching anything.
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "-mod=readonly")
	t.Setenv("GOWORK", "off")
	app := t.TempDir()
	writeTree(t, app, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.22\n\nrequire example.com/hello v1.0.0\n",
	})
	plugins := []ConfiguredPlugin{{Package: "example.com/hello/plugin"}}
	_, err := CheckPluginDeps("v0.28.0", app, plugins, NewGoListResolver(app))
	if err == nil {
		t.Fatal("expected an error")
	}
	var missing *MissingPluginError
	if errors.As(err, &missing) {
		t.Errorf("reported as not found: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"example.com/hello/plugin", "missing go.sum entry", "go mod tidy"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "go get") {
		t.Errorf("error should not suggest go get:\n%s", msg)
	}
}

func TestCheckPluginDepsOK(t *testing.T) {
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/p/plugin"}}
	resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
		return &PackageInfo{
			ImportPath: pkg,
			Module:     &ModuleInfo{Path: "github.com/foo/p", Version: "v1.2.3"},
		}, nil
	})
	infos, err := CheckPluginDeps("dev", "", plugins, resolver)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(infos) != 1 || infos[0].Module.Version != "v1.2.3" {
		t.Errorf("unexpected infos: %+v", infos)
	}
}

func TestCheckPluginDepsAggregatesMissing(t *testing.T) {
	plugins := []ConfiguredPlugin{
		{Package: "github.com/foo/a/plugin"},
		{Package: "github.com/foo/b/plugin"},
		{Package: "github.com/foo/c/plugin"},
	}
	resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
		if pkg == "github.com/foo/b/plugin" {
			return &PackageInfo{
				ImportPath: pkg,
				Module:     &ModuleInfo{Path: "github.com/foo/b", Version: "v1"},
			}, nil
		}
		return notProvided(pkg), nil
	})
	_, err := CheckPluginDeps("dev", "", plugins, resolver)
	var missing *MissingPluginError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingPluginError, got %v", err)
	}
	if len(missing.Packages) != 2 {
		t.Fatalf("expected both missing entries, got %v", missing.Packages)
	}
	got := strings.Join(missing.Packages, ",")
	if !strings.Contains(got, "/a/") || !strings.Contains(got, "/c/") {
		t.Errorf("missing list should include a and c, got %v", missing.Packages)
	}
}

func TestParseGoListOutput(t *testing.T) {
	raw := []byte(`{
		"ImportPath": "github.com/foo/p/plugin",
		"Module": {
			"Path": "github.com/foo/p",
			"Version": "v0.1.0",
			"GoMod": "/src/local/p/go.mod",
			"Replace": {"Path": "../local/p", "Version": ""}
		}
	}`)
	info, err := parseGoList(raw)
	if err != nil {
		t.Fatalf("parseGoList: %v", err)
	}
	if info.Module == nil || info.Module.Replace == nil {
		t.Fatalf("expected replace info, got %+v", info)
	}
	if info.Module.Replace.Path != "../local/p" {
		t.Errorf("replace path wrong: %q", info.Module.Replace.Path)
	}
	if info.Module.GoMod != "/src/local/p/go.mod" || info.Module.Main {
		t.Errorf("GoMod = %q, Main = %v; want /src/local/p/go.mod, false", info.Module.GoMod, info.Module.Main)
	}
}
