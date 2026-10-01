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
		&protocol.OpAndroidPlugin{Base: protocol.Base{Pkg: "z"}, Class: "com.zeta.ZetaPlugin"},
		&protocol.OpAndroidManifestAddPermission{Base: protocol.Base{Pkg: "z"}, Name: "android.permission.CAMERA"},
		&protocol.OpAndroidPlugin{Base: protocol.Base{Pkg: "a"}, Class: "com.alpha.AlphaPlugin"},
	}
	if _, err := WriteRegistrant(dir, "android", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	s := readRegistrant(t, dir, "android")
	want := "    fun makePlugins(): List<DriftPlugin> = listOf(\n" +
		"        com.zeta.ZetaPlugin(),\n" +
		"        com.alpha.AlphaPlugin(),\n" +
		"    )\n"
	for _, w := range []string{"package com.drift.runner", "object DriftPluginRegistrant", want} {
		if !strings.Contains(s, w) {
			t.Errorf("registrant missing %q:\n%s", w, s)
		}
	}
}

func TestWriteRegistrantIOSListsPluginsInSourceOrder(t *testing.T) {
	ops := []protocol.Op{
		&protocol.OpIOSPlugin{Base: protocol.Base{Pkg: "z", Ident: "zeta"}, Class: "ZetaPlugin"},
		&protocol.OpIOSPlugin{Base: protocol.Base{Pkg: "a", Ident: "alpha"}, Class: "AlphaPlugin"},
	}
	for _, platform := range []string{"ios", "xtool"} {
		dir := t.TempDir()
		if _, err := WriteRegistrant(dir, platform, ops); err != nil {
			t.Fatalf("%s: WriteRegistrant: %v", platform, err)
		}
		s := readRegistrant(t, dir, platform)
		want := "import DriftPluginAPI\n" +
			"import DriftPlugin_zeta\n" +
			"import DriftPlugin_alpha\n" +
			"\n" +
			"enum DriftPluginRegistrant {\n" +
			"    /// Creates one instance of every configured plugin, in drift.yaml order.\n" +
			"    static func makePlugins() -> [DriftPlugin] {\n" +
			"        return [\n" +
			"            DriftPlugin_zeta.ZetaPlugin(),\n" +
			"            DriftPlugin_alpha.AlphaPlugin(),\n" +
			"        ]\n" +
			"    }\n"
		if !strings.Contains(s, want) {
			t.Errorf("%s: registrant wrong, want:\n%s\ngot:\n%s", platform, want, s)
		}
	}
}

func TestWriteRegistrantIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteRegistrant(dir, "android", nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := WriteRegistrant(dir, "android", nil)
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
		if _, err := WriteRegistrant(dir, c.platform, nil); err != nil {
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
		changed, err := EnsureRunnerSupport(dir, platform)
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
		if changed, err := EnsureRunnerSupport(dir, platform); err != nil || len(changed) != 0 {
			t.Errorf("%s: rerun should be a no-op, got %v, %v", platform, changed, err)
		}
	}
}

// Built-in channels register a synchronous MethodHandler that
// PlatformChannel.kt calls as handler(method, args), which needs the
// operator form.
func TestBuiltInMethodHandlerUsesOperatorInvoke(t *testing.T) {
	content := templateText(t, "android/java/PlatformChannel.kt")
	if !strings.Contains(content, "operator fun invoke(method: String, args: Any?)") {
		t.Errorf("PlatformChannel.kt's MethodHandler must declare `operator fun invoke(...)`")
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
	check(androidWiring, func(string) []string { return []string{"android/java/MainActivity.kt"} })
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
	err := CheckEjectedIOS(dir)
	if err == nil {
		t.Fatal("expected wiring error")
	}
	for _, want := range []string{"Runner/AppDelegate.swift", "DriftPlugins.shared.launch(", "Runner/SceneDelegate.swift", "DeepLinkHandler.route(url:", "DriftPlugins.shared.attach(self, overlayView:", "PBXFileSystemSynchronizedRootGroup", "Drift/Plugins"} {
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
	})
	if err := CheckEjectedIOS(dir); err != nil {
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
	for _, want := range []string{"MainActivity.kt", "DriftPlugins.register(", "DriftPlugins.attach(", "app/build.gradle"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q:\n%v", want, err)
		}
	}

	writeTree(t, dir, map[string]string{
		mainActivity:       templateText(t, "android/java/MainActivity.kt"),
		"app/build.gradle": "android {}\n",
	})
	if err := CheckEjectedAndroid(dir, gradleOp); err != nil {
		t.Errorf("template-wired project should pass: %v", err)
	}
}
