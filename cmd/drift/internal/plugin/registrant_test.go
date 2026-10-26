package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func readRegistrant(t *testing.T, dir, platform string) string {
	t.Helper()
	rel := map[string]string{
		"android": "app/src/main/java/com/drift/runner/DriftPluginRegistrant.kt",
		"ios":     "Runner/DriftPluginRegistrant.swift",
		"xtool":   "Sources/Runner/DriftPluginRegistrant.swift",
	}[platform]
	body, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	return string(body)
}

func TestWriteRegistrantAndroidListsPluginsInSourceOrder(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpAndroidPlugin{Base: protocol.Base{Pkg: "example.com/zeta/plugin", Ident: "zeta"}, Class: "com.zeta.ZetaPlugin"},
		&protocol.OpAndroidManifestAddPermission{Base: protocol.Base{Pkg: "example.com/zeta/plugin", Ident: "zeta"}, Name: "android.permission.CAMERA"},
		&protocol.OpAndroidPlugin{Base: protocol.Base{Pkg: protocol.DriftModulePath + "/plugins/alpha/plugin", Ident: "alpha"}, Class: "com.alpha.AlphaPlugin"},
	}
	if _, err := writeRegistrant(t, dir, "android", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	s := readRegistrant(t, dir, "android")
	want := "    fun makePlugins(): List<DriftPluginEntry> = listOf(\n" +
		"        DriftPluginEntry(\"zeta\", false, com.zeta.ZetaPlugin()),\n" +
		"        DriftPluginEntry(\"alpha\", true, com.alpha.AlphaPlugin()),\n" +
		"    )\n"
	for _, w := range []string{"package com.drift.runner", "internal object DriftPluginRegistrant", want} {
		if !strings.Contains(s, w) {
			t.Errorf("registrant missing %q:\n%s", w, s)
		}
	}
}

func TestWriteRegistrantIOSListsPluginsInSourceOrder(t *testing.T) {
	ops := []protocol.Op{
		&protocol.OpIOSPlugin{Base: protocol.Base{Pkg: "example.com/zeta/plugin", Ident: "zeta"}, Class: "ZetaPlugin"},
		&protocol.OpIOSPlugin{Base: protocol.Base{Pkg: protocol.DriftModulePath + "/plugins/alpha/plugin", Ident: "alpha"}, Class: "AlphaPlugin"},
	}
	for _, platform := range []string{"ios", "xtool"} {
		dir := t.TempDir()
		if _, err := writeRegistrant(t, dir, platform, ops); err != nil {
			t.Fatalf("%s: WriteRegistrant: %v", platform, err)
		}
		s := readRegistrant(t, dir, platform)
		want := "import DriftPluginAPI\n" +
			"import DriftPlugin_zeta\n" +
			"import DriftPlugin_alpha\n" +
			"\n" +
			"enum DriftPluginRegistrant {\n" +
			"    /// Creates one instance of every configured plugin, in drift.yaml order.\n" +
			"    static func makePlugins() -> [DriftPluginEntry] {\n" +
			"        return [\n" +
			"            DriftPluginEntry(name: \"zeta\", firstParty: false, plugin: DriftPlugin_zeta.ZetaPlugin()),\n" +
			"            DriftPluginEntry(name: \"alpha\", firstParty: true, plugin: DriftPlugin_alpha.AlphaPlugin()),\n" +
			"        ]\n" +
			"    }\n"
		if !strings.Contains(s, want) {
			t.Errorf("%s: registrant wrong, want:\n%s\ngot:\n%s", platform, want, s)
		}
	}
}

func TestWriteRegistrantIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeRegistrant(t, dir, "android", nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := writeRegistrant(t, dir, "android", nil)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("expected no changes on idempotent rewrite, got %v", changed)
	}
}

// The placeholder registrant templates must be byte-identical to what the
// codegen emits for zero plugins. Otherwise every build rewrites the file on
// a fresh scaffold and produces a spurious diff.
func TestRegistrantPlaceholdersMatchEmptyCodegen(t *testing.T) {
	cases := []struct{ platform, template string }{
		{"ios", "ios/DriftPluginRegistrant.swift"},
		{"android", "android/runner/DriftPluginRegistrant.kt"},
	}
	for _, c := range cases {
		tmpl, err := templates.ReadFile(c.template)
		if err != nil {
			t.Fatalf("read %s: %v", c.template, err)
		}
		dir := t.TempDir()
		if _, err := writeRegistrant(t, dir, c.platform, nil); err != nil {
			t.Fatalf("WriteRegistrant: %v", err)
		}
		if got := readRegistrant(t, dir, c.platform); got != string(tmpl) {
			t.Errorf("%s placeholder and empty codegen differ.\n--- template ---\n%s\n--- generated ---\n%s", c.platform, tmpl, got)
		}
	}
}

// EnsureRunnerSupport must write exactly the bytes the scaffold copies, or
// every ejected build would fight the scaffold's copy. A rerun is a no-op.
func TestEnsureRunnerSupportMatchesTemplates(t *testing.T) {
	type file struct{ template, written string }
	cases := map[string][]file{}
	for _, f := range AndroidRunnerSupportFiles {
		cases["android"] = append(cases["android"], file{f.TemplatePath, "app/src/main/java/com/drift/runner/" + f.Name})
	}
	for _, name := range IOSRunnerSupportFiles {
		cases["ios"] = append(cases["ios"], file{"ios/" + name, "Runner/" + name})
		cases["xtool"] = append(cases["xtool"], file{"ios/" + name, "Sources/Runner/" + name})
	}
	for platform, files := range cases {
		dir := t.TempDir()
		changed, err := ensureRunnerSupport(t, dir, platform)
		if err != nil {
			t.Fatalf("%s: %v", platform, err)
		}
		if len(changed) != len(files) {
			t.Errorf("%s: wrote %d files, want %d", platform, len(changed), len(files))
		}
		for _, f := range files {
			want, err := templates.ReadFile(f.template)
			if err != nil {
				t.Fatalf("read %s: %v", f.template, err)
			}
			got, err := os.ReadFile(filepath.Join(dir, f.written))
			if err != nil {
				t.Fatalf("%s: %v", platform, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("%s: %s differs from %s", platform, f.written, f.template)
			}
		}
		if changed, err := ensureRunnerSupport(t, dir, platform); err != nil || len(changed) != 0 {
			t.Errorf("%s: rerun should be a no-op, got %v, %v", platform, changed, err)
		}
	}
}

// The ejected-project checks demand exactly the wiring the templates have.
// If a template stops making a call, the check would reject projects the
// scaffold itself produces. xtool is never ejected, but its templates must
// make the same calls (DriftApp.swift standing in for SceneDelegate.swift).
func TestTemplatesSatisfyEjectedWiring(t *testing.T) {
	iosTemplates := map[string][]string{
		"Runner/AppDelegate.swift":         {"ios/AppDelegate.swift", "xtool/AppDelegate.swift"},
		"Runner/SceneDelegate.swift":       {"ios/SceneDelegate.swift", "xtool/DriftApp.swift"},
		"Runner/PlatformChannel.swift":     {"ios/PlatformChannel.swift"},
		"Runner/DriftViewController.swift": {"ios/DriftViewController.swift"},
		"Runner.xcodeproj/project.pbxproj": {"xcodeproj/project.pbxproj.tmpl"},
	}
	check := func(calls []wiringCall, templatesFor func(string) []string) {
		for _, c := range calls {
			paths := templatesFor(c.File)
			if len(paths) == 0 {
				t.Errorf("no template mapped for %s", c.File)
			}
			for _, p := range paths {
				body, err := templates.ReadFile(p)
				if err != nil {
					t.Fatalf("read %s: %v", p, err)
				}
				if !strings.Contains(string(body), c.Call) {
					t.Errorf("%s does not contain %q", p, c.Call)
				}
			}
		}
	}
	check(iosWiring, func(f string) []string { return iosTemplates[f] })
	androidTemplates := map[string][]string{
		"app/src/main/AndroidManifest.xml": {"android/AndroidManifest.xml.tmpl"},
		"DriftApplication.kt":              {"android/java/DriftApplication.kt"},
		"MainActivity.kt":                  {"android/java/MainActivity.kt"},
	}
	check(androidWiring, func(f string) []string { return androidTemplates[f] })
}

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func templateText(t *testing.T, path string) string {
	t.Helper()
	b, err := templates.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCheckEjectedIOS(t *testing.T) {
	dir := t.TempDir()
	// A project ejected before plugin hooks existed.
	writeTree(t, dir, map[string]string{
		"Runner/AppDelegate.swift":         "NotificationHandler.start()\n",
		"Runner/SceneDelegate.swift":       "DeepLinkHandler.handle(url: url, source: \"open_url\")\n",
		"Runner/PlatformChannel.swift":     "",
		"Runner/DriftViewController.swift": "",
		"Runner.xcodeproj/project.pbxproj": "// no package refs\n",
	})
	entitlementOp := []protocol.Op{&protocol.OpPlistSet{
		Base:       protocol.Base{Pkg: "p"},
		PlistEntry: protocol.PlistEntry{File: protocol.PlistEntitlements, Key: "aps-environment"},
		Value:      "development",
	}}
	err := CheckEjectedIOS(dir, entitlementOp)
	if err == nil {
		t.Fatal("expected wiring error")
	}
	for _, want := range []string{"Runner/AppDelegate.swift", "DriftPlugins.shared.launch(", "DriftPlugins.shared.willPresentNotification(", "DriftPlugins.shared.attach(self, overlayView:", "PBXFileSystemSynchronizedRootGroup", "Drift/Plugins", "Runner.entitlements", "CODE_SIGN_ENTITLEMENTS"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q:\n%v", want, err)
		}
	}

	// A project ejected from the current templates is fully wired.
	writeTree(t, dir, map[string]string{
		"Runner/AppDelegate.swift":         templateText(t, "ios/AppDelegate.swift"),
		"Runner/SceneDelegate.swift":       templateText(t, "ios/SceneDelegate.swift"),
		"Runner/PlatformChannel.swift":     templateText(t, "ios/PlatformChannel.swift"),
		"Runner/DriftViewController.swift": templateText(t, "ios/DriftViewController.swift"),
		"Runner.xcodeproj/project.pbxproj": templateText(t, "xcodeproj/project.pbxproj.tmpl"),
		"Runner.entitlements":              templateText(t, "ios/Runner.entitlements"),
	})
	if err := CheckEjectedIOS(dir, entitlementOp); err != nil {
		t.Errorf("template-wired project should pass: %v", err)
	}
}

func TestCheckEjectedAndroid(t *testing.T) {
	dir := t.TempDir()
	mainActivity := "app/src/main/java/com/example/app/MainActivity.kt"
	writeTree(t, dir, map[string]string{mainActivity: "class MainActivity\n"})
	gradleOp := []protocol.Op{&protocol.OpAndroidGradleAddDependency{Base: protocol.Base{Pkg: "p"}, Configuration: "implementation", Coord: "a:b:1"}}

	err := CheckEjectedAndroid(dir, gradleOp)
	if err == nil {
		t.Fatal("expected wiring error")
	}
	for _, want := range []string{"MainActivity.kt", "DriftApplication.kt", "DriftPlugins.register(", "DriftPlugins.attach(", ".DriftApplication", "app/build.gradle"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q:\n%v", want, err)
		}
	}

	writeTree(t, dir, map[string]string{
		mainActivity: templateText(t, "android/java/MainActivity.kt"),
		"app/src/main/java/com/example/app/DriftApplication.kt": templateText(t, "android/java/DriftApplication.kt"),
		"app/src/main/AndroidManifest.xml":                      templateText(t, "android/AndroidManifest.xml.tmpl"),
		"app/build.gradle":                                      "android {}\n",
	})
	if err := CheckEjectedAndroid(dir, gradleOp); err != nil {
		t.Errorf("template-wired project should pass: %v", err)
	}
}
