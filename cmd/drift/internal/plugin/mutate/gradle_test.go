package mutate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

const baseGradle = `plugins {
    id "com.android.application"
}

android {
    namespace "com.example.app"
}

dependencies {
    implementation "androidx.appcompat:appcompat:1.6.1"
}
`

func writeGradle(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "build.gradle")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed gradle: %v", err)
	}
	return path
}

func TestApplyGradleAddDependencies_InsertsIntoExistingBlock(t *testing.T) {
	path := writeGradle(t, baseGradle)
	ops := []*driftplugin.OpAndroidGradleAddDependency{
		{
			Base:          driftplugin.Base{Pkg: "github.com/foo/splash"},
			Configuration: "implementation",
			Coord:         "androidx.core:core-splashscreen:1.0.1",
		},
	}
	wrote, changed, err := ApplyGradleAddDependencies(path, ops)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if wrote != path || !changed {
		t.Fatalf("expected changed=true at path %s; got path=%s changed=%v", path, wrote, changed)
	}
	body, _ := os.ReadFile(path)
	s := string(body)
	if !strings.Contains(s, `implementation "androidx.core:core-splashscreen:1.0.1"`) {
		t.Errorf("missing inserted dep:\n%s", s)
	}
	if !strings.Contains(s, `implementation "androidx.appcompat:appcompat:1.6.1"`) {
		t.Errorf("dropped existing dep:\n%s", s)
	}
}

func TestApplyGradleAddDependencies_IdempotentReapply(t *testing.T) {
	path := writeGradle(t, baseGradle)
	ops := []*driftplugin.OpAndroidGradleAddDependency{
		{
			Base:          driftplugin.Base{Pkg: "github.com/foo/splash"},
			Configuration: "implementation",
			Coord:         "androidx.core:core-splashscreen:1.0.1",
		},
	}
	if _, _, err := ApplyGradleAddDependencies(path, ops); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	_, changed, err := ApplyGradleAddDependencies(path, ops)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if changed {
		t.Errorf("expected idempotent re-apply, got changed=true")
	}
}

func TestApplyGradleAddDependencies_DedupesIdenticalInOpList(t *testing.T) {
	path := writeGradle(t, baseGradle)
	ops := []*driftplugin.OpAndroidGradleAddDependency{
		{Base: driftplugin.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
		{Base: driftplugin.Base{Pkg: "b"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	if _, _, err := ApplyGradleAddDependencies(path, ops); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if got := strings.Count(string(body), `com.foo:bar:1.0.0`); got != 1 {
		t.Errorf("expected single dep entry, got %d:\n%s", got, body)
	}
}

func TestApplyGradleAddDependencies_PreservesExistingFormat(t *testing.T) {
	path := writeGradle(t, baseGradle)
	ops := []*driftplugin.OpAndroidGradleAddDependency{
		{Base: driftplugin.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	if _, _, err := ApplyGradleAddDependencies(path, ops); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	// Final closing brace must stay on its own line, not get trailing content.
	if !strings.HasSuffix(string(body), "}\n") {
		t.Errorf("file should end with closing brace + newline; got:\n%q", body)
	}
}

func TestApplyGradleAddDependencies_RejectsKotlinDSL(t *testing.T) {
	dir := t.TempDir()
	gradlePath := filepath.Join(dir, "build.gradle")
	ktsPath := filepath.Join(dir, "build.gradle.kts")
	// Both files present: simulates a stale Groovy build.gradle next to a
	// new Kotlin DSL one. The mutator must refuse rather than silently
	// edit the wrong file.
	if err := os.WriteFile(gradlePath, []byte(baseGradle), 0o644); err != nil {
		t.Fatalf("seed groovy: %v", err)
	}
	if err := os.WriteFile(ktsPath, []byte("// kotlin DSL\n"), 0o644); err != nil {
		t.Fatalf("seed kts: %v", err)
	}
	ops := []*driftplugin.OpAndroidGradleAddDependency{
		{Base: driftplugin.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	_, _, err := ApplyGradleAddDependencies(gradlePath, ops)
	if !errors.Is(err, errKotlinDSL) {
		t.Errorf("expected Kotlin DSL error, got %v", err)
	}
}

func TestApplyGradleAddDependencies_EmptyOpsIsNoOp(t *testing.T) {
	path := writeGradle(t, baseGradle)
	_, changed, err := ApplyGradleAddDependencies(path, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if changed {
		t.Errorf("empty ops list must not change file")
	}
}

// TestApplyGradleAddDependencies_MissingDependenciesBlock guards the error
// path for a malformed scaffold (or a user-edited build.gradle that lost
// the top-level `dependencies { }` block). The scaffold template ships the
// block, so this is defensive — but silent fallthrough would land us back
// in "missing dep at runtime" territory, which the explicit error
// prevents.
func TestApplyGradleAddDependencies_MissingDependenciesBlock(t *testing.T) {
	const noBlock = `plugins {
    id "com.android.application"
}

android {
    namespace "com.example.app"
}
`
	path := writeGradle(t, noBlock)
	ops := []*driftplugin.OpAndroidGradleAddDependency{
		{Base: driftplugin.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	_, _, err := ApplyGradleAddDependencies(path, ops)
	if err == nil {
		t.Fatal("expected error for missing dependencies block")
	}
	if !strings.Contains(err.Error(), "dependencies") {
		t.Errorf("error should name the missing dependencies block; got %v", err)
	}
}
