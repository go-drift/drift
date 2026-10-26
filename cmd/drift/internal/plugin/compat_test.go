package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/semver"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// pluginModule writes a plugin module's go.mod requiring drift at
// driftVersion and returns the configured plugin and its go list info.
func pluginModule(t *testing.T, path, version, driftVersion string) (ConfiguredPlugin, *PackageInfo) {
	t.Helper()
	dir := t.TempDir()
	gomod := "module " + path + "\n\ngo 1.24.0\n\nrequire " + protocol.DriftModulePath + " " + driftVersion + "\n"
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := path + "/plugin"
	return ConfiguredPlugin{Package: pkg}, &PackageInfo{
		ImportPath: pkg,
		Module:     &ModuleInfo{Path: path, Version: version, GoMod: goModPath},
	}
}

func TestCheckPluginCompat(t *testing.T) {
	const cli = "v0.28.1"
	tests := []struct {
		name    string
		drift   string
		wantNew bool
		wantOld bool
	}{
		{"same version", cli, false, false},
		{"older patch, same minor", "v0.28.0", false, false},
		{"at the floor", templates.PluginAPIFloor, false, false},
		{"newer patch, same minor", "v0.28.2", false, false},
		{"pseudo-version past the CLI, same minor", "v0.28.2-0.20261026120000-abcdef123456", false, false},
		{"newer minor", "v0.29.0", true, false},
		{"pseudo-version of the next minor", "v0.29.0-0.20261026120000-abcdef123456", true, false},
		{"newer major", "v1.0.0", true, false},
		{"below the floor", "v0.26.1", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, info := pluginModule(t, "example.com/p", "v1.0.0", tt.drift)
			err := checkPluginCompat(cli, []ConfiguredPlugin{p}, []*PackageInfo{info})
			if !tt.wantNew && !tt.wantOld {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var skew *VersionSkewError
			if !errors.As(err, &skew) {
				t.Fatalf("want *VersionSkewError, got %T %v", err, err)
			}
			if (len(skew.TooNew) == 1) != tt.wantNew || (len(skew.TooOld) == 1) != tt.wantOld {
				t.Errorf("TooNew = %v, TooOld = %v", skew.TooNew, skew.TooOld)
			}
			msg := err.Error()
			for _, want := range []string{cli, tt.drift, "example.com/p/plugin", "example.com/p@v1.0.0"} {
				if !strings.Contains(msg, want) {
					t.Errorf("message lacks %q:\n%s", want, msg)
				}
			}
		})
	}
}

func TestCheckPluginCompatReportsEveryPluginAndTheNewestCLI(t *testing.T) {
	a, ai := pluginModule(t, "example.com/a", "v1.0.0", "v0.29.0")
	b, bi := pluginModule(t, "example.com/b", "v1.0.0", "v0.30.1")
	c, ci := pluginModule(t, "example.com/c", "v1.0.0", "v0.1.0")
	ok, oki := pluginModule(t, "example.com/ok", "v1.0.0", "v0.28.0")
	err := checkPluginCompat("v0.28.0",
		[]ConfiguredPlugin{a, b, ok, c}, []*PackageInfo{ai, bi, oki, ci})
	var skew *VersionSkewError
	if !errors.As(err, &skew) {
		t.Fatalf("want *VersionSkewError, got %T %v", err, err)
	}
	if len(skew.TooNew) != 2 || len(skew.TooOld) != 1 {
		t.Fatalf("TooNew = %v, TooOld = %v", skew.TooNew, skew.TooOld)
	}
	msg := err.Error()
	if !strings.Contains(msg, "go install github.com/go-drift/drift/cmd/drift@v0.30.1") {
		t.Errorf("message should suggest the newest required CLI:\n%s", msg)
	}
	if strings.Contains(msg, "example.com/ok") {
		t.Errorf("message names a compatible plugin:\n%s", msg)
	}
}

func TestCheckPluginCompatSkips(t *testing.T) {
	p, info := pluginModule(t, "example.com/p", "v1.0.0", "v0.99.0")

	if err := checkPluginCompat("", []ConfiguredPlugin{p}, []*PackageInfo{info}); err != nil {
		t.Errorf("development CLI: %v", err)
	}

	main := *info.Module
	main.Main = true
	if err := checkPluginCompat("v0.28.0", []ConfiguredPlugin{p}, []*PackageInfo{{Module: &main}}); err != nil {
		t.Errorf("plugin in the main module: %v", err)
	}

	// A first-party plugin is in drift itself, whose go.mod does not
	// require drift: the bridge check covers it.
	firstParty := ConfiguredPlugin{Package: protocol.DriftModulePath + "/plugins/splash/plugin"}
	drift := &ModuleInfo{Path: protocol.DriftModulePath, Version: "v0.99.0", GoMod: filepath.Join(t.TempDir(), "missing.mod")}
	if err := checkPluginCompat("v0.28.0", []ConfiguredPlugin{firstParty}, []*PackageInfo{{Module: drift}}); err != nil {
		t.Errorf("first-party plugin: %v", err)
	}
}

func TestCheckPluginCompatChecksLocalPlugins(t *testing.T) {
	// A replaced plugin still declares its drift version in its go.mod.
	p, info := pluginModule(t, "example.com/p", "v1.0.0", "v0.99.0")
	info.Module.Replace = &Replace{Path: "../p"}
	err := checkPluginCompat("v0.28.0", []ConfiguredPlugin{p}, []*PackageInfo{info})
	if !strings.Contains(err.Error(), "example.com/p => ../p") {
		t.Errorf("want skew naming the replaced module, got %v", err)
	}
}

// TestHelloRequiresAServedPluginAPI keeps examples/plugins/hello, the
// in-repo third-party plugin, usable with a release CLI: raising
// templates.PluginAPIFloor must raise the drift its go.mod requires too.
func TestHelloRequiresAServedPluginAPI(t *testing.T) {
	mod := &ModuleInfo{Path: "github.com/go-drift/drift/examples/plugins/hello", GoMod: "../../../../examples/plugins/hello/go.mod"}
	req, err := requiredDrift(mod)
	if err != nil {
		t.Fatal(err)
	}
	if semver.Compare(req, templates.PluginAPIFloor) < 0 {
		t.Errorf("%s requires drift %s, older than templates.PluginAPIFloor %s", mod.GoMod, req, templates.PluginAPIFloor)
	}
}

func TestCheckPluginCompatRejectsPluginWithoutDrift(t *testing.T) {
	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module example.com/p\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info := &PackageInfo{Module: &ModuleInfo{Path: "example.com/p", Version: "v1.0.0", GoMod: goMod}}
	err := checkPluginCompat("v0.28.0", []ConfiguredPlugin{{Package: "example.com/p/plugin"}}, []*PackageInfo{info})
	if err == nil || !strings.Contains(err.Error(), "does not require") {
		t.Errorf("want missing-requirement error, got %v", err)
	}
}

func TestCheckBridgeDrift(t *testing.T) {
	tests := []struct {
		name    string
		cli     string
		bridge  string
		wantErr string // substring; empty means no error
	}{
		{"same version", "v0.28.0", "v0.28.0", ""},
		{"same minor", "v0.28.0", "v0.28.3", ""},
		{"pseudo-version on the same minor", "v0.28.0", "v0.28.4-0.20261026120000-abcdef123456", ""},
		{"local drift", "v0.28.0", protocol.DevelVersion, ""},
		{"development CLI", "", "v0.30.0", ""},
		{"newer minor", "v0.28.0", "v0.29.0", "drift v0.29.0, but this CLI is v0.28.0"},
		{"older minor", "v0.28.0", "v0.27.5", "drift v0.27.5, but this CLI is v0.28.0"},
		{"no version reported", "", "", "reported no drift version"},
		{"invalid version", "v0.28.0", "latest", "invalid drift version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkBridgeDrift(tt.cli, tt.bridge)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
