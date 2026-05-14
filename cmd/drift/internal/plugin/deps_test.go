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
	_, err := CheckPluginDeps(plugins, resolver)
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

func TestCheckPluginDepsOK(t *testing.T) {
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/p/plugin"}}
	resolver := PackageResolver(func(pkg string) (*PackageInfo, error) {
		return &PackageInfo{
			ImportPath: pkg,
			Module:     &ModuleInfo{Path: "github.com/foo/p", Version: "v1.2.3"},
		}, nil
	})
	infos, err := CheckPluginDeps(plugins, resolver)
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
	_, err := CheckPluginDeps(plugins, resolver)
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
}
