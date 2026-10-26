package mutate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
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
	ops := []*protocol.OpAndroidGradleAddDependency{
		{
			Base:          protocol.Base{Pkg: "github.com/foo/splash"},
			Configuration: "implementation",
			Coord:         "androidx.core:core-splashscreen:1.0.1",
		},
	}
	wrote, changed, err := applyGradleAddDependencies(t, path, ops, nil)
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
	ops := []*protocol.OpAndroidGradleAddDependency{
		{
			Base:          protocol.Base{Pkg: "github.com/foo/splash"},
			Configuration: "implementation",
			Coord:         "androidx.core:core-splashscreen:1.0.1",
		},
	}
	if _, _, err := applyGradleAddDependencies(t, path, ops, nil); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	_, changed, err := applyGradleAddDependencies(t, path, ops, nil)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if changed {
		t.Errorf("expected idempotent re-apply, got changed=true")
	}
}

func TestApplyGradleAddDependencies_DedupesIdenticalInOpList(t *testing.T) {
	path := writeGradle(t, baseGradle)
	ops := []*protocol.OpAndroidGradleAddDependency{
		{Base: protocol.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
		{Base: protocol.Base{Pkg: "b"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	if _, _, err := applyGradleAddDependencies(t, path, ops, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if got := strings.Count(string(body), `com.foo:bar:1.0.0`); got != 1 {
		t.Errorf("expected single dep entry, got %d:\n%s", got, body)
	}
}

func TestApplyGradleAddDependencies_PreservesExistingFormat(t *testing.T) {
	path := writeGradle(t, baseGradle)
	ops := []*protocol.OpAndroidGradleAddDependency{
		{Base: protocol.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	if _, _, err := applyGradleAddDependencies(t, path, ops, nil); err != nil {
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
	ops := []*protocol.OpAndroidGradleAddDependency{
		{Base: protocol.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	_, _, err := applyGradleAddDependencies(t, gradlePath, ops, nil)
	if !errors.Is(err, errKotlinDSL) {
		t.Errorf("expected Kotlin DSL error, got %v", err)
	}
}

func TestApplyGradleAddDependencies_EmptyOpsIsNoOp(t *testing.T) {
	path := writeGradle(t, baseGradle)
	_, changed, err := applyGradleAddDependencies(t, path, nil, nil)
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
	ops := []*protocol.OpAndroidGradleAddDependency{
		{Base: protocol.Base{Pkg: "a"}, Configuration: "implementation", Coord: "com.foo:bar:1.0.0"},
	}
	_, _, err := applyGradleAddDependencies(t, path, ops, nil)
	if err == nil {
		t.Fatal("expected error for missing dependencies block")
	}
	if !strings.Contains(err.Error(), "dependencies") {
		t.Errorf("error should name the missing dependencies block; got %v", err)
	}
}

// An ejected app's build.gradle is the user's: braces, block names and
// dependency lines inside comments and strings are not code, so edits land
// where they would in the same file without them.
const hostileGradle = `plugins {
    id "com.android.application"
}

/*
dependencies {
    implementation "old:commented:1.0"
}
*/
android {
    namespace "com.example.app" // not "}" either
    // a closing } in a comment
    buildConfigField "String", "BRACES", '"}}{"'
    defaultConfig {
        manifestPlaceholders = [label: """multi
line } string"""]
    }
}

dependencies {
    implementation "androidx.appcompat:appcompat:1.6.1"
    // implementation "androidx.core:core-ktx:1.12.0" }
    /* implementation "com.example:unused:1.0" */
}
`

func TestGradleMutatorsIgnoreCommentsAndStrings(t *testing.T) {
	path := writeGradle(t, hostileGradle)
	_, _, err := applyGradleAddDependencies(t, path, []*protocol.OpAndroidGradleAddDependency{
		{Base: protocol.Base{Pkg: "p"}, Configuration: "implementation", Coord: "androidx.core:core-ktx:1.12.0"},
		{Base: protocol.Base{Pkg: "p"}, Configuration: "implementation", Coord: "old:commented:1.0"},
	}, nil)
	if err != nil {
		t.Fatalf("add dependencies: %v", err)
	}
	if _, _, err := applyGradleApplyPlugins(t, path, []*protocol.OpAndroidGradleApplyPlugin{op("com.google.gms.google-services", "")}); err != nil {
		t.Fatalf("apply plugins: %v", err)
	}
	body, _ := os.ReadFile(path)
	want := strings.Replace(hostileGradle, `        manifestPlaceholders = [label: """multi
line } string"""]
    }
}
`, `        manifestPlaceholders = [label: """multi
line } string"""]
    }
}

apply plugin: "com.google.gms.google-services"
`, 1)
	want = strings.Replace(want, `    /* implementation "com.example:unused:1.0" */
}`, `    /* implementation "com.example:unused:1.0" */
    implementation "androidx.core:core-ktx:1.12.0"
    implementation "old:commented:1.0"
}`, 1)
	if string(body) != want {
		t.Errorf("got:\n%s\nwant:\n%s", body, want)
	}
}

func TestMaskGroovy(t *testing.T) {
	for _, tc := range []struct{ src, comments, all string }{
		{`a // b }`, `a       `, `a       `},
		{"a /* {\n} */ b", "a     \n     b", "a     \n     b"},
		{`x "a}" 'b{' y`, `x "a}" 'b{' y`, `x           y`},
		{`x "a\"}" y`, `x "a\"}" y`, `x        y`},
		{`x """a"}""" y`, `x """a"}""" y`, `x           y`},
		{`x "//" }`, `x "//" }`, `x      }`},
		{`x '/*' }`, `x '/*' }`, `x      }`},
		{"x 'open\n}", "x 'open\n}", "x      \n}"},
		{"x /* open", "x        ", "x        "},
	} {
		if got := string(maskGroovy([]byte(tc.src), false)); got != tc.comments {
			t.Errorf("maskGroovy(%q, false) = %q, want %q", tc.src, got, tc.comments)
		}
		if got := string(maskGroovy([]byte(tc.src), true)); got != tc.all {
			t.Errorf("maskGroovy(%q, true) = %q, want %q", tc.src, got, tc.all)
		}
	}
}
