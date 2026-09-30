package mutate

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

const scaffoldXtoolYml = `bundleID: com.example.app
displayName: Example
version: 1
buildNumber: 1
deploymentTarget: 16.0
deviceFamily:
  - iphone
  - ipad
infoPath: Sources/Runner/Resources/Info.plist
iconPath: Sources/Runner/Resources/AppIcon.png
`

func writeXtoolYml(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "xtool.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readXtoolYml(t *testing.T, p string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("result is not valid YAML: %v\n%s", err, data)
	}
	return out
}

func TestSetXtoolResourcesSetsSortedListAndPreservesKeys(t *testing.T) {
	p := writeXtoolYml(t, scaffoldXtoolYml)
	changed, err := SetXtoolResources(p, []string{"PluginResources/b.json", "PluginResources/a.png"})
	if err != nil || !changed {
		t.Fatalf("set: changed=%v err=%v", changed, err)
	}
	got := readXtoolYml(t, p)
	res, _ := got["resources"].([]any)
	if len(res) != 2 || res[0] != "PluginResources/a.png" || res[1] != "PluginResources/b.json" {
		t.Errorf("resources = %v", got["resources"])
	}
	if got["bundleID"] != "com.example.app" || got["iconPath"] != "Sources/Runner/Resources/AppIcon.png" {
		t.Errorf("other keys not preserved: %v", got)
	}
}

func TestSetXtoolResourcesIdempotentAndReplaces(t *testing.T) {
	p := writeXtoolYml(t, scaffoldXtoolYml)
	if _, err := SetXtoolResources(p, []string{"PluginResources/a.png"}); err != nil {
		t.Fatal(err)
	}
	if changed, err := SetXtoolResources(p, []string{"PluginResources/a.png"}); err != nil || changed {
		t.Errorf("rerun: changed=%v err=%v", changed, err)
	}
	if _, err := SetXtoolResources(p, []string{"PluginResources/c.png"}); err != nil {
		t.Fatal(err)
	}
	res, _ := readXtoolYml(t, p)["resources"].([]any)
	if len(res) != 1 || res[0] != "PluginResources/c.png" {
		t.Errorf("expected replacement, got %v", res)
	}
}

func TestSetXtoolResourcesEmpty(t *testing.T) {
	p := writeXtoolYml(t, scaffoldXtoolYml)
	// No resources and no key: file untouched byte-for-byte.
	if changed, err := SetXtoolResources(p, nil); err != nil || changed {
		t.Fatalf("empty on clean file: changed=%v err=%v", changed, err)
	}
	if data, _ := os.ReadFile(p); string(data) != scaffoldXtoolYml {
		t.Errorf("clean file was rewritten:\n%s", data)
	}
	// Emptying a populated list removes the key.
	if _, err := SetXtoolResources(p, []string{"PluginResources/a.png"}); err != nil {
		t.Fatal(err)
	}
	if changed, err := SetXtoolResources(p, nil); err != nil || !changed {
		t.Fatalf("clear: changed=%v err=%v", changed, err)
	}
	if _, ok := readXtoolYml(t, p)["resources"]; ok {
		t.Errorf("resources key should be removed")
	}
}

func TestSetXtoolResourcesRejectsNonMapping(t *testing.T) {
	p := writeXtoolYml(t, "- not\n- a mapping\n")
	if _, err := SetXtoolResources(p, []string{"x"}); err == nil {
		t.Errorf("expected error for non-mapping document")
	}
}
