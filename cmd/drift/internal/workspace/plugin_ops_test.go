package workspace

import (
	"encoding/base64"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	driftpluginCLI "github.com/go-drift/drift/cmd/drift/internal/plugin"
	"github.com/go-drift/drift/cmd/drift/internal/scaffold"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// Refresh regenerates the native project exactly when the plugin output
// hash changes, so the hash must be stable for equal op lists and differ
// for any change, including dropping every plugin.
func TestPluginOpsHash(t *testing.T) {
	mk := func(ops ...protocol.Op) string {
		t.Helper()
		p, err := newPluginOps(ops)
		if err != nil {
			t.Fatal(err)
		}
		return p.hash
	}
	perm := func(name string) protocol.Op {
		return &protocol.OpAndroidManifestAddPermission{Base: protocol.Base{Pkg: "p"}, Name: name}
	}

	none := mk()
	if none != mk() {
		t.Error("empty op list hash is not stable")
	}
	one := mk(perm("android.permission.CAMERA"))
	if one != mk(perm("android.permission.CAMERA")) {
		t.Error("equal op lists hash differently")
	}
	if one == none {
		t.Error("adding an op did not change the hash")
	}
	if one == mk(perm("android.permission.INTERNET")) {
		t.Error("changing an op did not change the hash")
	}
}

// An ejected build whose last op fails writes nothing: the edits and files
// earlier ops planned, the registrant, the support files and the lock stay
// as they were, so no output escapes the lock. Without the failing op the
// same build lands.
func TestApplyPluginOpsWritesNothingWhenAnOpFails(t *testing.T) {
	root := t.TempDir()
	if err := scaffold.WriteAndroid(root, scaffold.Settings{AppName: "App", AppID: "com.example.app", Bundle: "com.example.app", Orientation: "portrait"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "android")
	ws := &Workspace{Root: root, AndroidDir: dir, Platform: "android", Ejected: true}
	base := protocol.Base{Pkg: "github.com/acme/push"}
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	build := func(ops ...protocol.Op) error {
		t.Helper()
		p, err := newPluginOps(ops)
		if err != nil {
			t.Fatal(err)
		}
		return applyPluginOps(ws, p)
	}

	if err := build(&protocol.OpAddFile{Base: base, Location: protocol.FileAndroidDrawable, Name: "push_icon", Content: enc("png v1")}); err != nil {
		t.Fatalf("first build: %v", err)
	}
	before := snapshotTree(t, dir)

	good := []protocol.Op{
		&protocol.OpAndroidManifestAddPermission{Base: base, Name: "android.permission.POST_NOTIFICATIONS"},
		&protocol.OpAndroidColorSet{Base: base, Name: "push_accent", Value: "#ff0000ff"},
		&protocol.OpAddFile{Base: base, Location: protocol.FileAndroidDrawable, Name: "push_icon", Content: enc("png v2")},
		&protocol.OpAddKotlinSource{Base: base, Package: "com.acme.push", RelPath: "PushPlugin.kt", Content: enc("class PushPlugin\n")},
		&protocol.OpAndroidGradleAddDependency{Base: base, Configuration: "implementation", Coord: "com.acme:push:1.0"},
		&protocol.OpAndroidPlugin{Base: base, Class: "com.acme.push.PushPlugin"},
	}
	// The scaffold declares com.android.application at another version,
	// which the last Gradle mutator rejects.
	bad := &protocol.OpAndroidGradleApplyPlugin{Base: base, ID: "com.android.application", Version: "0.0.1"}
	err := build(append(good, bad)...)
	if err == nil || !strings.Contains(err.Error(), "the project declares version") {
		t.Fatalf("err = %v, want the Gradle plugin version mismatch", err)
	}
	if after := snapshotTree(t, dir); !reflect.DeepEqual(after, before) {
		for path := range after {
			if _, ok := before[path]; !ok {
				t.Errorf("failed build created %s", path)
			} else if after[path] != before[path] {
				t.Errorf("failed build changed %s", path)
			}
		}
		for path := range before {
			if _, ok := after[path]; !ok {
				t.Errorf("failed build removed %s", path)
			}
		}
	}

	if err := build(good...); err != nil {
		t.Fatalf("build without the failing op: %v", err)
	}
	lock, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(driftpluginCLI.LockFile)))
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"app/src/main/java/com/acme/push/PushPlugin.kt", "app/src/main/res/drawable-nodpi/push_icon.png"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s not written: %v", rel, err)
		}
		if !strings.Contains(string(lock), rel) {
			t.Errorf("lock does not record %s:\n%s", rel, lock)
		}
	}
	if gradle, _ := os.ReadFile(filepath.Join(dir, "app", "build.gradle")); !strings.Contains(string(gradle), "com.acme:push:1.0") {
		t.Errorf("app/build.gradle lacks the dependency:\n%s", gradle)
	}
}

// An edit a removed plugin made inside a shared file is a warning, not a
// failure, so the build's outcome does not depend on whether the previous
// lock was committed: a fresh checkout with the stale lock builds and
// warns the same way.
func TestApplyPluginOpsWarnsAboutDroppedEdits(t *testing.T) {
	root := t.TempDir()
	if err := scaffold.WriteAndroid(root, scaffold.Settings{AppName: "App", AppID: "com.example.app", Bundle: "com.example.app", Orientation: "portrait"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "android")
	ws := &Workspace{Root: root, AndroidDir: dir, Platform: "android", Ejected: true}
	build := func(ops ...protocol.Op) (stderr string) {
		t.Helper()
		p, err := newPluginOps(ops)
		if err != nil {
			t.Fatal(err)
		}
		stderr = captureStderr(t, func() { err = applyPluginOps(ws, p) })
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		return stderr
	}

	perm := &protocol.OpAndroidManifestAddPermission{Base: protocol.Base{Pkg: "github.com/acme/camera"}, Name: "android.permission.CAMERA"}
	if out := build(perm); out != "" {
		t.Errorf("first build warned: %q", out)
	}
	lockPath := filepath.Join(dir, filepath.FromSlash(driftpluginCLI.LockFile))
	stale, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	warning := build()
	for _, want := range []string{"plugins no longer make these edits", dir, "android.permission.CAMERA", "(from github.com/acme/camera)"} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning lacks %q:\n%s", want, warning)
		}
	}
	if out := build(); out != "" {
		t.Errorf("build after the lock was updated warned: %q", out)
	}

	if err := os.WriteFile(lockPath, stale, 0o644); err != nil {
		t.Fatal(err)
	}
	if again := build(); again != warning {
		t.Errorf("build from the stale lock warned %q, want %q", again, warning)
	}
}

// captureStderr returns what fn writes to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	out := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		out <- string(data)
	}()
	fn()
	w.Close()
	return <-out
}

// snapshotTree maps every file and directory under dir, relative, to its
// content ("" for a directory).
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			tree[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		tree[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
