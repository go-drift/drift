package plugin

import (
	"os"
	"strings"
	"testing"

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

func TestBridgeCacheKeyChangesWithGoSum(t *testing.T) {
	dir := t.TempDir()
	plugins := []ConfiguredPlugin{{Package: "github.com/foo/p/plugin"}}
	infos := []*PackageInfo{{Module: &ModuleInfo{Path: "github.com/foo/p", Version: "v1"}}}
	src, _ := GenerateBridgeSource(plugins)

	// No go.sum yet; should still produce a stable key.
	key1, err := BridgeCacheKey(dir, "v0.1.0", plugins, infos, src)
	if err != nil {
		t.Fatalf("key1: %v", err)
	}

	// Write a go.sum: key must change.
	if err := writeFile(dir+"/go.sum", "fakesum"); err != nil {
		t.Fatalf("write go.sum: %v", err)
	}
	key2, err := BridgeCacheKey(dir, "v0.1.0", plugins, infos, src)
	if err != nil {
		t.Fatalf("key2: %v", err)
	}
	if key1 == key2 {
		t.Errorf("cache key did not change when go.sum changed: %s", key1)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
