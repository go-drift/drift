package templates

import (
	"encoding/xml"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"howett.net/plist"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// The app templates are the baseline every Drift app ships. Permissions,
// usage strings, background modes, entitlements and heavy dependencies
// come from plugins listed in drift.yaml, never from the templates (Play
// and App Review reject apps that declare what they do not use). The
// exceptions are the normal (install-time, promptless) permissions core
// features need: network access and vibration (platform.Haptics). These
// tests keep it that way.

// renderAll renders path with every TemplateData combination that changes
// its conditional sections.
func renderAll(t *testing.T, path string) []string {
	t.Helper()
	raw, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, allowHTTP := range []bool{false, true} {
		for _, orientation := range []string{"portrait", "landscape", "all"} {
			data := NewTemplateData(TemplateInput{
				AppName:        "guard_app",
				AndroidPackage: "com.example.guard_app",
				IOSBundleID:    "com.example.guardApp",
				Orientation:    orientation,
				AllowHTTP:      allowHTTP,
			})
			rendered, err := ProcessTemplate(string(raw), data)
			if err != nil {
				t.Fatalf("render %s: %v", path, err)
			}
			out = append(out, rendered)
		}
	}
	return out
}

func TestManifestDeclaresOnlyCorePermissions(t *testing.T) {
	for _, manifest := range renderAll(t, "android/AndroidManifest.xml.tmpl") {
		var permissions, forbidden []string
		decoder := xml.NewDecoder(strings.NewReader(manifest))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			switch start.Name.Local {
			case "uses-permission", "uses-permission-sdk-23":
				for _, attr := range start.Attr {
					if attr.Name.Local == "name" {
						permissions = append(permissions, attr.Value)
					}
				}
			case "permission", "uses-feature", "receiver", "service":
				forbidden = append(forbidden, start.Name.Local)
			}
		}
		slices.Sort(permissions)
		want := []string{"android.permission.ACCESS_NETWORK_STATE", "android.permission.INTERNET", "android.permission.VIBRATE"}
		if !slices.Equal(permissions, want) {
			t.Errorf("manifest permissions = %v, want %v", permissions, want)
		}
		if len(forbidden) > 0 {
			t.Errorf("manifest declares %v; these come from plugins", forbidden)
		}
	}
}

var plistKey = regexp.MustCompile(`<key>([^<]+)</key>`)

func TestInfoPlistsDeclareNoCapabilities(t *testing.T) {
	forbidden := regexp.MustCompile(`^(NS\w+UsageDescription|UIBackgroundModes|BGTaskSchedulerPermittedIdentifiers)$`)
	for _, path := range []string{"ios/Info.plist.tmpl", "xtool/Info.plist.tmpl"} {
		for _, plist := range renderAll(t, path) {
			for _, match := range plistKey.FindAllStringSubmatch(plist, -1) {
				if forbidden.MatchString(match[1]) {
					t.Errorf("%s declares %s; it comes from a plugin", path, match[1])
				}
			}
		}
	}
}

// Plugins may not set the keys the Info.plist templates write; the list
// the op validation reserves must be exactly those keys.
func TestTemplateInfoKeysAreReserved(t *testing.T) {
	keys := map[string]bool{}
	for _, path := range []string{"ios/Info.plist.tmpl", "xtool/Info.plist.tmpl"} {
		for _, rendered := range renderAll(t, path) {
			var root map[string]any
			if _, err := plist.Unmarshal([]byte(rendered), &root); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for key := range root {
				keys[key] = true
			}
		}
	}
	got := slices.Sorted(maps.Keys(keys))
	if want := protocol.TemplateInfoKeys(); !slices.Equal(got, want) {
		t.Errorf("template Info.plist keys = %v\nprotocol.TemplateInfoKeys() = %v", got, want)
	}
}

func TestEntitlementsAreEmpty(t *testing.T) {
	raw, err := ReadFile("ios/Runner.entitlements")
	if err != nil {
		t.Fatal(err)
	}
	if keys := plistKey.FindAllString(string(raw), -1); len(keys) > 0 {
		t.Errorf("Runner.entitlements declares %v; entitlements come from plugins", keys)
	}
}

func TestGradleDependsOnlyOnAppCompat(t *testing.T) {
	dependency := regexp.MustCompile(`(?m)^\s*(?:implementation|api|runtimeOnly)\s+["']([^:"']+:[^:"']+)`)
	for _, gradle := range renderAll(t, "android/app.build.gradle.tmpl") {
		var modules []string
		for _, match := range dependency.FindAllStringSubmatch(gradle, -1) {
			modules = append(modules, match[1])
		}
		if want := []string{"androidx.appcompat:appcompat"}; !slices.Equal(modules, want) {
			t.Errorf("app dependencies = %v, want %v; SDKs come from plugins", modules, want)
		}
	}
}
