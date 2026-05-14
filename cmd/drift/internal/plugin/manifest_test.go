package plugin

import (
	"testing"
)

func TestParseManifestSourceOrder(t *testing.T) {
	yaml := `app:
  name: Showcase

plugins:
  - package: github.com/foo/drift-splash/plugin
    config:
      image: assets/logo.png
      background_color: "#FFFFFF"
  - package: github.com/foo/drift-camera/plugin
    config:
      enable_microphone: true
`
	plugins, err := ParseManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Package != "github.com/foo/drift-splash/plugin" {
		t.Errorf("first package wrong: %q", plugins[0].Package)
	}
	if plugins[1].Package != "github.com/foo/drift-camera/plugin" {
		t.Errorf("second package wrong: %q", plugins[1].Package)
	}
	cfgMap, err := plugins[0].ConfigMap()
	if err != nil {
		t.Fatalf("ConfigMap: %v", err)
	}
	if cfgMap["image"] != "assets/logo.png" {
		t.Errorf("config did not decode: %+v", cfgMap)
	}
}

func TestParseManifestMissingPackageErrors(t *testing.T) {
	yaml := `plugins:
  - config: {}
`
	if _, err := ParseManifest([]byte(yaml)); err == nil {
		t.Error("expected error for missing package")
	}
}

func TestParseManifestEmptyOK(t *testing.T) {
	plugins, err := ParseManifest([]byte(`app:
  name: x
`))
	if err != nil || len(plugins) != 0 {
		t.Errorf("expected empty plugins, got %v %v", plugins, err)
	}
}

func TestParseManifestEmptyPluginsList(t *testing.T) {
	plugins, err := ParseManifest([]byte(`plugins: []
`))
	if err != nil || len(plugins) != 0 {
		t.Errorf("expected empty plugins, got %v %v", plugins, err)
	}
}

func TestConfigYAMLRoundTrips(t *testing.T) {
	yaml := `plugins:
  - package: github.com/a/b/plugin
    config:
      key: value
`
	plugins, err := ParseManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	got, err := plugins[0].ConfigYAML()
	if err != nil {
		t.Fatalf("ConfigYAML: %v", err)
	}
	if got == "" {
		t.Errorf("ConfigYAML returned empty")
	}
}
