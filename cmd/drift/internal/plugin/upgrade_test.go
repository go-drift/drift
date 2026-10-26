package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/cmd/drift/internal/scaffold"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ejectedAndroid returns an Android project laid out like an ejected one
// (the scaffold's Gradle files and manifest).
func ejectedAndroid(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := scaffold.WriteAndroid(root, scaffold.Settings{AppName: "App", AppID: "com.example.app", Bundle: "com.example.app", Orientation: "portrait"}); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "android")
}

// ejectedBuild applies ops to dir the way an ejected build does: Apply and
// SyncEjectedLock plan, then one commit.
func ejectedBuild(t *testing.T, dir string, ops ...protocol.Op) error {
	t.Helper()
	prev, err := ReadLock(dir)
	if err != nil {
		return err
	}
	cs := mutate.NewChangeset()
	if _, err := Apply(cs, ops, dir, "android", prev); err != nil {
		return err
	}
	_, dropped, err := SyncEjectedLock(cs, dir, "android", ops, prev)
	if err != nil {
		return err
	}
	if err := cs.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return dropped
}

func lockKeys(t *testing.T, dir string) []string {
	t.Helper()
	l, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, o := range l.Ops {
		keys = append(keys, o.Key)
	}
	return keys
}

func dep(plugin, coord string) *protocol.OpAndroidGradleAddDependency {
	return &protocol.OpAndroidGradleAddDependency{Base: protocol.Base{Pkg: plugin}, Configuration: "implementation", Coord: coord}
}

func gradlePlugin(version string) *protocol.OpAndroidGradleApplyPlugin {
	return &protocol.OpAndroidGradleApplyPlugin{Base: protocol.Base{Pkg: "github.com/acme/push"}, ID: "com.acme.gradle", Version: version}
}

// A plugin upgrade that bumps its Gradle dependency and Gradle plugin
// versions updates Drift's lines in place: no "removed" report, the same
// lock keys, and no old line left beside the new one.
func TestEjectedPluginUpgradeUpdatesGradleVersionsInPlace(t *testing.T) {
	dir := ejectedAndroid(t)
	if err := ejectedBuild(t, dir, dep("github.com/acme/push", "com.acme:push:1.0"), gradlePlugin("1.0")); err != nil {
		t.Fatal(err)
	}
	keys := lockKeys(t, dir)

	if err := ejectedBuild(t, dir, dep("github.com/acme/push", "com.acme:push:2.0"), gradlePlugin("2.0")); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if got := lockKeys(t, dir); !reflect.DeepEqual(got, keys) {
		t.Errorf("lock keys changed: %v, was %v", got, keys)
	}
	app := readFile(t, filepath.Join(dir, "app/build.gradle"))
	if !strings.Contains(app, `implementation "com.acme:push:2.0"`) || strings.Contains(app, "com.acme:push:1.0") {
		t.Errorf("app/build.gradle should hold only the new dependency:\n%s", app)
	}
	project := readFile(t, filepath.Join(dir, "build.gradle"))
	if !strings.Contains(project, `id "com.acme.gradle" version "2.0" apply false`) || strings.Contains(project, `version "1.0"`) {
		t.Errorf("build.gradle should declare only the new version:\n%s", project)
	}
}

// A line the user changed is theirs: Drift adds its own beside it, and a
// Gradle plugin version the user set is still a mismatch.
func TestEjectedPluginUpgradeKeepsUserChangedLines(t *testing.T) {
	dir := ejectedAndroid(t)
	if err := ejectedBuild(t, dir, dep("github.com/acme/push", "com.acme:push:1.0"), gradlePlugin("1.0")); err != nil {
		t.Fatal(err)
	}
	edit := func(rel, old, new string) {
		path := filepath.Join(dir, rel)
		src := readFile(t, path)
		if !strings.Contains(src, old) {
			t.Fatalf("%s lacks %q", rel, old)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(src, old, new, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	edit("app/build.gradle", `"com.acme:push:1.0"`, `"com.acme:push:1.5"`)

	if err := ejectedBuild(t, dir, dep("github.com/acme/push", "com.acme:push:2.0"), gradlePlugin("1.0")); err != nil {
		t.Fatal(err)
	}
	app := readFile(t, filepath.Join(dir, "app/build.gradle"))
	if !strings.Contains(app, "com.acme:push:1.5") || !strings.Contains(app, "com.acme:push:2.0") {
		t.Errorf("want the user's line kept and Drift's added:\n%s", app)
	}

	edit("build.gradle", `version "1.0"`, `version "1.5"`)
	err := ejectedBuild(t, dir, dep("github.com/acme/push", "com.acme:push:2.0"), gradlePlugin("2.0"))
	if err == nil || !strings.Contains(err.Error(), "declares version 1.5") {
		t.Errorf("want a version mismatch for the user's declaration, got %v", err)
	}
}

// Plugins pinning different versions of one artifact both keep their line
// (Gradle resolves to the highest); rebuilding changes nothing.
func TestEjectedPluginsPinningDifferentVersionsKeepBoth(t *testing.T) {
	dir := ejectedAndroid(t)
	ops := []protocol.Op{dep("github.com/acme/a", "androidx.core:core:1.12.0"), dep("github.com/acme/b", "androidx.core:core:1.13.0")}
	for range 2 {
		if err := ejectedBuild(t, dir, ops...); err != nil {
			t.Fatal(err)
		}
	}
	app := readFile(t, filepath.Join(dir, "app/build.gradle"))
	if strings.Count(app, "androidx.core:core:1.12.0") != 1 || strings.Count(app, "androidx.core:core:1.13.0") != 1 {
		t.Errorf("want each pinned version once:\n%s", app)
	}

	// a moves to b's version: one line remains, not two identical ones.
	if err := ejectedBuild(t, dir, dep("github.com/acme/a", "androidx.core:core:1.13.0"), dep("github.com/acme/b", "androidx.core:core:1.13.0")); err != nil {
		t.Fatal(err)
	}
	app = readFile(t, filepath.Join(dir, "app/build.gradle"))
	if strings.Count(app, "androidx.core:core:1.13.0") != 1 || strings.Contains(app, "1.12.0") {
		t.Errorf("want a single 1.13.0 line:\n%s", app)
	}
}

func TestGradleDependencyArtifact(t *testing.T) {
	for coord, want := range map[string]string{
		"g:a":              "g:a",
		"g:a:1.0":          "g:a",
		"g:a:1.0:sources":  "g:a:sources",
		"com.google:x:2.3": "com.google:x",
	} {
		if got := dep("p", coord).Artifact(); got != want {
			t.Errorf("Artifact(%q) = %q, want %q", coord, got, want)
		}
	}
}

// The lock records the line each versioned Gradle op wrote.
func TestLockRecordsGradleLines(t *testing.T) {
	dir := ejectedAndroid(t)
	if err := ejectedBuild(t, dir, dep("github.com/acme/push", "com.acme:push:1.0"), gradlePlugin("1.0")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(LockFile)))
	if err != nil {
		t.Fatal(err)
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, o := range l.Ops {
		lines = append(lines, strings.TrimSpace(o.Line))
	}
	want := []string{`implementation "com.acme:push:1.0"`, `id "com.acme.gradle" version "1.0" apply false`}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lock lines = %q, want %q", lines, want)
	}
}
