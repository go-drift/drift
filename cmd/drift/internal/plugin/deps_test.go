package plugin

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckPluginDepsMissing(t *testing.T) {
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/missing/plugin"}}
	resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
		return &PackageInfo{ImportPath: pkg, Error: "package not found"}, nil
	})
	_, err := CheckPluginDeps("dev", plugins, resolver)
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

// TestMissingPluginErrorHint checks the `go get` hint: a release CLI names
// its own version for first-party plugins, which are tagged in lockstep with
// drift, since @latest may need a newer CLI; third-party plugins and any
// plugin under a development CLI get @latest.
func TestMissingPluginErrorHint(t *testing.T) {
	const (
		splash = "github.com/go-drift/drift/plugins/splash"
		hello  = "example.com/hello"
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
			want: `Plugin package not found: github.com/go-drift/drift/plugins/splash

Run:
  go get github.com/go-drift/drift/plugins/splash@v0.28.0

After installing, commit drift.yaml and tools/drift-plugins/main.go together.`,
		},
		{
			name: "release CLI, first- and third-party",
			cli:  "0.28.0",
			pkgs: []string{splash, hello},
			want: `Plugin packages not found:
  - github.com/go-drift/drift/plugins/splash
  - example.com/hello

Run:
  go get github.com/go-drift/drift/plugins/splash@v0.28.0
  go get example.com/hello@latest

If a plugin's latest version requires a drift newer than this CLI (v0.28.0), get an earlier version of it.

After installing, commit drift.yaml and tools/drift-plugins/main.go together.`,
		},
		{
			name: "development CLI",
			cli:  "dev+abc123",
			pkgs: []string{splash, hello},
			want: `Plugin packages not found:
  - github.com/go-drift/drift/plugins/splash
  - example.com/hello

Run:
  go get github.com/go-drift/drift/plugins/splash@latest
  go get example.com/hello@latest

After installing, commit drift.yaml and tools/drift-plugins/main.go together.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins := make([]ConfiguredPlugin, len(tt.pkgs))
			for i, p := range tt.pkgs {
				plugins[i] = ConfiguredPlugin{Package: p}
			}
			resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
				return &PackageInfo{ImportPath: pkg, Error: "package not found"}, nil
			})
			_, err := CheckPluginDeps(tt.cli, plugins, resolver)
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

func TestCheckPluginDepsOK(t *testing.T) {
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/p/plugin"}}
	resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
		return &PackageInfo{
			ImportPath: pkg,
			Module:     &ModuleInfo{Path: "github.com/foo/p", Version: "v1.2.3"},
		}, nil
	})
	infos, err := CheckPluginDeps("dev", plugins, resolver)
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
		return &PackageInfo{ImportPath: pkg, Error: "package not found"}, nil
	})
	_, err := CheckPluginDeps("dev", plugins, resolver)
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
