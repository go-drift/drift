package plugin

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestGenerateBridgeSourceCarriesBuildTag(t *testing.T) {
	plugins := []ConfiguredPlugin{
		{Package: "github.com/foo/drift-splash/plugin", Config: yaml.Node{}},
		{Package: "github.com/foo/drift-camera/plugin", Config: yaml.Node{}},
	}
	src, err := GenerateBridgeSource(plugins)
	if err != nil {
		t.Fatalf("GenerateBridgeSource: %v", err)
	}
	body := string(src)
	if !strings.HasPrefix(body, "//go:build drift_tool") {
		t.Errorf("missing build tag, body starts with: %q", body[:min(80, len(body))])
	}
	if !strings.Contains(body, "splash \"github.com/foo/drift-splash/plugin\"") {
		t.Errorf("missing splash alias import")
	}
	if !strings.Contains(body, "camera \"github.com/foo/drift-camera/plugin\"") {
		t.Errorf("missing camera alias import")
	}
	if !strings.Contains(body, "driftplugin.Bind(\"github.com/foo/drift-splash/plugin\", splash.Plugin)") {
		t.Errorf("missing splash Bind")
	}
	if !strings.Contains(body, "driftplugin.Bind(\"github.com/foo/drift-camera/plugin\", camera.Plugin)") {
		t.Errorf("missing camera Bind")
	}
}

// The committed bridge is gofmt-clean (imports sorted) while Bind keeps
// drift.yaml order, which decides claim priority.
func TestGenerateBridgeSourceIsGofmtClean(t *testing.T) {
	src, err := GenerateBridgeSource([]ConfiguredPlugin{
		{Package: "github.com/foo/drift-splash/plugin"},
		{Package: "github.com/foo/drift-camera/plugin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != string(src) {
		t.Errorf("bridge source is not gofmt-clean:\n%s", src)
	}
	body := string(src)
	if strings.Index(body, `camera "`) > strings.Index(body, `splash "`) {
		t.Error("imports not sorted")
	}
	if strings.Index(body, "splash.Plugin") > strings.Index(body, "camera.Plugin") {
		t.Error("Bind order is not drift.yaml order")
	}
}

func TestBridgeSourceDeterministic(t *testing.T) {
	plugins := []ConfiguredPlugin{
		{Package: "github.com/foo/drift-a/plugin"},
		{Package: "github.com/foo/drift-b/plugin"},
	}
	a, _ := GenerateBridgeSource(plugins)
	b, _ := GenerateBridgeSource(plugins)
	if string(a) != string(b) {
		t.Error("bridge generation must be byte-deterministic")
	}
}

func TestAliasCollisionResolved(t *testing.T) {
	plugins := []ConfiguredPlugin{
		{Package: "github.com/foo/drift-splash/plugin"},
		{Package: "github.com/bar/drift-splash/plugin"},
	}
	src, err := GenerateBridgeSource(plugins)
	if err != nil {
		t.Fatalf("GenerateBridgeSource: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "splash \"github.com/foo/drift-splash/plugin\"") {
		t.Errorf("first alias should be splash")
	}
	if !strings.Contains(body, "splash_2 \"github.com/bar/drift-splash/plugin\"") {
		t.Errorf("second alias should be splash_2; body:\n%s", body)
	}
}

func TestLastSegmentTrimsDriftPrefix(t *testing.T) {
	cases := map[string]string{
		"github.com/foo/drift-splash/plugin": "splash",
		"github.com/foo/foo-plugin/plugin":   "foo-plugin",
		"foo":                                "foo",
		"":                                   "",
	}
	for in, want := range cases {
		got := lastSegment(in)
		if got != want {
			t.Errorf("lastSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeAliasReplacesHyphens(t *testing.T) {
	cases := map[string]string{
		"foo-plugin": "foo_plugin",
		"camera":     "camera",
		"":           "plugin",
	}
	for in, want := range cases {
		got := sanitizeAlias(in)
		if got != want {
			t.Errorf("sanitizeAlias(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBridgeCacheKeyCoversBuildInputs(t *testing.T) {
	dir := t.TempDir()
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/p/plugin"}}
	infos := []*PackageInfo{{Module: &ModuleInfo{Path: "github.com/foo/p", Version: "v1"}}}
	src, _ := GenerateBridgeSource(plugins)
	env := goToolchain{Version: "go1.24.0"}

	key := func(env goToolchain) string {
		t.Helper()
		k, err := bridgeCacheKey(dir, "v0.1.0", env, plugins, infos, src)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	base := key(env)
	if key(env) != base {
		t.Fatal("key is not deterministic")
	}

	steps := []struct {
		name   string
		mutate func()
		env    goToolchain
	}{
		{"go.sum", func() { mustWrite(t, dir+"/go.sum", "sum") }, env},
		{"go.mod", func() { mustWrite(t, dir+"/go.mod", "module x\nreplace a => a v1.0.1\n") }, env},
		{"go version", func() {}, goToolchain{Version: "go1.25.0"}},
		{"GOFLAGS", func() {}, goToolchain{Version: "go1.25.0", Flags: "-tags=foo"}},
	}
	prev := base
	for _, s := range steps {
		s.mutate()
		got := key(s.env)
		if got == prev {
			t.Errorf("cache key did not change with %s", s.name)
		}
		prev = got
	}
}

func TestGraphHasLocalModules(t *testing.T) {
	cases := []struct {
		name string
		list string
		want bool
	}{
		{"pinned only", `{"Path":"app","Main":true}{"Path":"github.com/go-drift/drift","Version":"v0.3.0"}`, false},
		{"version replace", `{"Path":"app","Main":true}{"Path":"a","Version":"v1.0.0","Replace":{"Path":"b","Version":"v1.1.0"}}`, false},
		{"directory replace of drift", `{"Path":"app","Main":true}{"Path":"github.com/go-drift/drift","Version":"v0.0.0","Replace":{"Path":"../drift"}}`, true},
		{"multi-module workspace", `{"Path":"app","Main":true}{"Path":"plug","Main":true}`, true},
	}
	for _, c := range cases {
		got, err := graphHasLocalModules([]byte(c.list))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := writeFile(path, content); err != nil {
		t.Fatal(err)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func TestAliasesAvoidReservedNames(t *testing.T) {
	cases := map[string]string{
		"github.com/x/drift-go/plugin":     "go_plugin",
		"github.com/x/map/plugin":          "map_plugin",
		"github.com/x/chan/plugin":         "chan_plugin",
		"github.com/x/init/plugin":         "init_plugin",
		"github.com/x/main/plugin":         "main_plugin",
		"github.com/x/string/plugin":       "string_plugin",
		"github.com/x/driftplugin/plugin":  "driftplugin_2",
		"github.com/x/drift-camera/plugin": "camera",
	}
	for pkg, want := range cases {
		got := assignAliases([]ConfiguredPlugin{{Package: pkg}})[pkg]
		if got != want {
			t.Errorf("alias for %s = %q, want %q", pkg, got, want)
		}
	}
}

// The generated bridge must be valid Go whatever the plugin paths are.
func TestGeneratedBridgeParses(t *testing.T) {
	plugins := []ConfiguredPlugin{
		{Package: "github.com/x/drift-go/plugin"},
		{Package: "github.com/y/go/plugin"},
		{Package: "github.com/x/_/plugin"},
		{Package: "github.com/x/9lives/plugin"},
	}
	src, err := GenerateBridgeSource(plugins)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0); err != nil {
		t.Fatalf("generated bridge does not parse: %v\n%s", err, src)
	}
}

// The cache keeps the maxCachedBridges most recently used bridges, keyed
// and local alike, and a directory still being built counts as just used.
func TestPruneBridgeCacheKeepsMostRecentlyUsed(t *testing.T) {
	root := t.TempDir()
	base := time.Now().Add(-time.Hour)
	bridge := func(dir string, age int) string {
		t.Helper()
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(path, "bridge")
		if err := os.WriteFile(bin, nil, 0o755); err != nil {
			t.Fatal(err)
		}
		used := base.Add(-time.Duration(age) * time.Minute)
		if err := os.Chtimes(bin, used, used); err != nil {
			t.Fatal(err)
		}
		return path
	}
	var oldest []string
	for i := range maxCachedBridges + 2 {
		dir := fmt.Sprintf("key%02d", i)
		if i%2 == 1 {
			dir = filepath.Join("local", dir)
		}
		path := bridge(dir, i)
		if i >= maxCachedBridges-1 {
			oldest = append(oldest, path)
		}
	}
	// A build in progress: a fresh directory with only a temporary file.
	building := filepath.Join(root, "building")
	if err := os.MkdirAll(building, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := pruneBridgeCache(root, "bridge"); err != nil {
		t.Fatal(err)
	}
	for _, path := range oldest {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s kept, want removed (least recently used)", path)
		}
	}
	for _, path := range []string{building, filepath.Join(root, "key00"), filepath.Join(root, "local")} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s removed, want kept", path)
		}
	}
	var kept int
	for _, parent := range []string{root, filepath.Join(root, "local")} {
		dirs, _ := os.ReadDir(parent)
		for _, d := range dirs {
			if d.Name() != "local" {
				kept++
			}
		}
	}
	if kept != maxCachedBridges {
		t.Errorf("%d bridges kept, want %d", kept, maxCachedBridges)
	}
}
