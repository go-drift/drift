package mutate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

const baseAppGradle = `plugins {
    id "com.android.application"
    id "org.jetbrains.kotlin.android"
}

android {
    namespace "com.example.app"
    compileSdk 34
}

dependencies {
    implementation "androidx.core:core-ktx:1.12.0"
}
`

const baseProjectGradle = `plugins {
    id "com.android.application" version "8.2.2" apply false
    id "org.jetbrains.kotlin.android" version "1.9.22" apply false
}
`

func writeAppGradle(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "build.gradle")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return p
}

func op(id, version string) *protocol.OpAndroidGradleApplyPlugin {
	return &protocol.OpAndroidGradleApplyPlugin{
		Base:    protocol.Base{Pkg: "p"},
		ID:      id,
		Version: version,
	}
}

func TestApplyGradleApplyPluginsInsertsAfterAndroidBlock(t *testing.T) {
	path := writeAppGradle(t, baseAppGradle)
	_, changed, err := ApplyGradleApplyPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	body, _ := os.ReadFile(path)
	s := string(body)
	if !strings.Contains(s, `apply plugin: "com.google.gms.google-services"`) {
		t.Errorf("apply line missing:\n%s", s)
	}
	// Insert point: must appear AFTER the android block's `}`.
	androidClose := strings.Index(s, "}\n\ndependencies")
	applyIdx := strings.Index(s, `apply plugin: "com.google.gms.google-services"`)
	if applyIdx < 0 || applyIdx < androidClose {
		t.Errorf("apply line should land after android close-brace and before dependencies:\n%s", s)
	}
}

func TestApplyGradleApplyPluginsIdempotent(t *testing.T) {
	path := writeAppGradle(t, baseAppGradle)
	ops := []*protocol.OpAndroidGradleApplyPlugin{op("foo.plugin", "1.0")}
	if _, _, err := ApplyGradleApplyPlugins(path, ops); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, changed, err := ApplyGradleApplyPlugins(path, ops)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if changed {
		t.Errorf("expected idempotent rerun, got changed=true")
	}
}

// An ejected project may apply the plugin itself, here inside an
// `if (file(...).exists())` block. Mutator must detect the live
// (non-commented) line and skip insertion.
func TestApplyGradleApplyPluginsSkipsWhenPresentInConditional(t *testing.T) {
	conditional := baseAppGradle + `
if (file("google-services.json").exists()) {
    apply plugin: "com.google.gms.google-services"
}
`
	path := writeAppGradle(t, conditional)
	_, changed, err := ApplyGradleApplyPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if changed {
		t.Errorf("expected skip; existing conditional already carries the apply line")
	}
}

// A trailing comment mentioning the apply line must not count as present.
func TestApplyGradleApplyPluginsIgnoresTrailingComment(t *testing.T) {
	commented := strings.Replace(baseAppGradle, "compileSdk 34", `compileSdk 34 // apply plugin: "foo.plugin"`, 1)
	path := writeAppGradle(t, commented)
	_, changed, err := ApplyGradleApplyPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{op("foo.plugin", "")})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Errorf("trailing comment should not suppress insertion")
	}
}

// Commented-out apply lines must NOT count as present (otherwise a user
// who commented out a stale Firebase entry would silently lose plugin
// integration).
func TestApplyGradleApplyPluginsIgnoresCommentedLines(t *testing.T) {
	commented := strings.Replace(baseAppGradle, "android {", `// apply plugin: "com.google.gms.google-services"
android {`, 1)
	path := writeAppGradle(t, commented)
	_, changed, err := ApplyGradleApplyPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Errorf("commented line should not suppress insertion")
	}
}

func TestApplyGradleProjectPluginsInsertsIntoExistingBlock(t *testing.T) {
	path := writeAppGradle(t, baseProjectGradle)
	_, changed, err := ApplyGradleProjectPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true")
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `id "com.google.gms.google-services" version "4.4.0" apply false`) {
		t.Errorf("project-level declaration missing:\n%s", body)
	}
}

func TestApplyGradleProjectPluginsIdempotent(t *testing.T) {
	// Pre-declared at version 4.4.0; mutator must skip.
	preDeclared := baseProjectGradle[:len(baseProjectGradle)-2] +
		`    id "com.google.gms.google-services" version "4.4.0" apply false
}
`
	path := writeAppGradle(t, preDeclared)
	_, changed, err := ApplyGradleProjectPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if changed {
		t.Errorf("expected skip; id already declared")
	}
}

func TestApplyGradleProjectPluginsSkipsEmptyVersion(t *testing.T) {
	path := writeAppGradle(t, baseProjectGradle)
	_, changed, err := ApplyGradleProjectPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("on.gradle.portal", ""),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if changed {
		t.Errorf("plugin without version should be skipped (assumed already on classpath)")
	}
}

// A project that declares google-services 4.4.0 itself; a plugin asking for
// another version must fail loudly rather than silently get 4.4.0.
func TestApplyGradleProjectPluginsRejectsVersionMismatch(t *testing.T) {
	preDeclared := baseProjectGradle[:len(baseProjectGradle)-2] +
		`    id "com.google.gms.google-services" version "4.4.0" apply false
}
`
	path := writeAppGradle(t, preDeclared)
	_, _, err := ApplyGradleProjectPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.2"),
	})
	if err == nil || !strings.Contains(err.Error(), "4.4.2") {
		t.Fatalf("expected version-mismatch error naming both versions, got %v", err)
	}
}

// A declaration only present in a trailing comment must not count.
func TestApplyGradleProjectPluginsIgnoresTrailingComment(t *testing.T) {
	commented := strings.Replace(baseProjectGradle, `"1.9.22" apply false`,
		`"1.9.22" apply false // id "foo.plugin" version "9.9"`, 1)
	path := writeAppGradle(t, commented)
	_, changed, err := ApplyGradleProjectPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("foo.plugin", "1.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Errorf("commented declaration should not count as present")
	}
}

// Firebase-shaped integration: AddGradleDependency for the runtime SDKs,
// ApplyGradlePlugin for google-services, ApplyGradleProjectPlugins to
// declare the plugin classpath at the project level. End state matches
// Firebase's canonical Android setup.
func TestFirebaseShapedIntegration(t *testing.T) {
	app := writeAppGradle(t, baseAppGradle)
	project := writeAppGradle(t, baseProjectGradle)

	depOps := []*protocol.OpAndroidGradleAddDependency{
		{Base: protocol.Base{Pkg: "fb"}, Configuration: "implementation", Coord: "com.google.firebase:firebase-bom:33.0.0"},
		{Base: protocol.Base{Pkg: "fb"}, Configuration: "implementation", Coord: "com.google.firebase:firebase-analytics-ktx"},
	}
	if _, _, err := ApplyGradleAddDependencies(app, depOps); err != nil {
		t.Fatalf("add deps: %v", err)
	}
	pluginOps := []*protocol.OpAndroidGradleApplyPlugin{
		op("com.google.gms.google-services", "4.4.0"),
	}
	if _, _, err := ApplyGradleApplyPlugins(app, pluginOps); err != nil {
		t.Fatalf("apply plugins: %v", err)
	}
	if _, _, err := ApplyGradleProjectPlugins(project, pluginOps); err != nil {
		t.Fatalf("project plugins: %v", err)
	}

	appBody, _ := os.ReadFile(app)
	projectBody, _ := os.ReadFile(project)

	required := map[string]string{
		"firebase-bom dep":             `implementation "com.google.firebase:firebase-bom:33.0.0"`,
		"firebase-analytics-ktx dep":   `implementation "com.google.firebase:firebase-analytics-ktx"`,
		"apply google-services plugin": `apply plugin: "com.google.gms.google-services"`,
	}
	for name, want := range required {
		if !strings.Contains(string(appBody), want) {
			t.Errorf("app/build.gradle missing %s (%q):\n%s", name, want, appBody)
		}
	}
	if !strings.Contains(string(projectBody), `id "com.google.gms.google-services" version "4.4.0" apply false`) {
		t.Errorf("project build.gradle missing google-services declaration:\n%s", projectBody)
	}
}

// A build.gradle that ends immediately at the android block's `}` with no
// trailing newline is rare in the wild but legal Groovy. The mutator must
// synthesise the leading newline rather than splicing the apply line onto
// the close brace.
func TestApplyGradleApplyPluginsHandlesNoTrailingNewline(t *testing.T) {
	noTrailing := `plugins {
    id "com.android.application"
}

android {
    namespace "com.example.app"
}`
	path := writeAppGradle(t, noTrailing)
	_, changed, err := ApplyGradleApplyPlugins(path, []*protocol.OpAndroidGradleApplyPlugin{
		op("foo.plugin", "1.0"),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Fatalf("expected change")
	}
	body, _ := os.ReadFile(path)
	s := string(body)
	if !strings.Contains(s, "}\n\napply plugin: \"foo.plugin\"") {
		t.Errorf("apply line not separated from close-brace by a newline:\n%s", s)
	}
}
