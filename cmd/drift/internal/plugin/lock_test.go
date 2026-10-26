package plugin

import (
	b64 "encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// syncLock reconciles dir the way an ejected build does: the lock from the
// previous build, then SyncEjectedLock, committed. Returns the deleted
// paths, the dropped edits, and the planning or commit error.
func syncLock(t *testing.T, dir string, ops []protocol.Op) (deleted, dropped []string, err error) {
	t.Helper()
	prev, err := ReadLock(dir)
	if err != nil {
		return nil, nil, err
	}
	cs := mutate.NewChangeset()
	deleted, dropped, err = SyncEjectedLock(cs, dir, "android", ops, prev)
	if err != nil {
		return nil, nil, err
	}
	if err := cs.Commit(); err != nil {
		return nil, nil, err
	}
	return deleted, dropped, nil
}

func TestSyncEjectedLockCleansUpRemovedPlugins(t *testing.T) {
	dir := t.TempDir()
	base := protocol.Base{Pkg: "github.com/acme/camera"}
	src := &protocol.OpAddKotlinSource{Base: base, Package: "com.acme.camera", RelPath: "CameraPlugin.kt", Content: b64.StdEncoding.EncodeToString([]byte("class CameraPlugin\n"))}
	drawable := &protocol.OpAddFile{Base: base, Location: protocol.FileAndroidDrawable, Name: "camera_icon", Content: b64.StdEncoding.EncodeToString([]byte("png"))}
	perm := &protocol.OpAndroidManifestAddPermission{Base: base, Name: "android.permission.CAMERA"}
	// Plugins record ops for every platform; an Android project's lock
	// must ignore the iOS ones.
	plist := &protocol.OpPlistSet{Base: base, PlistEntry: protocol.PlistEntry{File: protocol.PlistInfo, Key: "NSCameraUsageDescription"}, Value: "camera"}
	ops := []protocol.Op{src, drawable, perm, plist}

	writeFiles := func() {
		for _, op := range ops {
			files, err := ownedFiles(op, project{dir: dir, platform: "android"})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				writeTree(t, "/", map[string]string{f.Path: string(f.Content)})
			}
		}
	}
	writeFiles()
	if _, _, err := syncLock(t, dir, ops); err != nil {
		t.Fatalf("first build: %v", err)
	}
	lock := filepath.Join(dir, filepath.FromSlash(LockFile))
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("lock not written: %v", err)
	}

	// Unchanged ops: nothing to clean up.
	if deleted, _, err := syncLock(t, dir, ops); err != nil || len(deleted) != 0 {
		t.Fatalf("rebuild: deleted=%v err=%v", deleted, err)
	}

	// The user edits the drawable, then removes the plugin.
	drawablePath := filepath.Join(dir, "app/src/main/res/drawable-nodpi/camera_icon.png")
	if err := os.WriteFile(drawablePath, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := os.ReadFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	deleted, dropped, err := syncLock(t, dir, nil)
	if err != nil {
		t.Fatalf("build after removal: %v", err)
	}
	srcPath := filepath.Join(dir, "app/src/main/java/com/acme/camera/CameraPlugin.kt")
	if len(deleted) != 1 || deleted[0] != srcPath {
		t.Errorf("deleted = %v, want only the unmodified source", deleted)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/src/main/java/com/acme")); !os.IsNotExist(err) {
		t.Errorf("empty package directories should be removed: %v", err)
	}
	if _, err := os.Stat(drawablePath); err != nil {
		t.Errorf("modified drawable should be kept: %v", err)
	}
	// Only the manifest edit: an Android project must not list iOS edits.
	if len(dropped) != 1 || !strings.Contains(dropped[0], "android.permission.CAMERA") || !strings.Contains(dropped[0], "(from github.com/acme/camera)") {
		t.Fatalf("dropped = %v, want only the manifest edit", dropped)
	}

	// The lock now records no ops, so the next build lists nothing.
	if deleted, dropped, err := syncLock(t, dir, nil); err != nil || len(deleted) != 0 || len(dropped) != 0 {
		t.Errorf("next build: deleted=%v dropped=%v err=%v; want nothing", deleted, dropped, err)
	}

	// A fresh checkout with the stale lock committed builds the same way.
	if err := os.WriteFile(lock, stale, 0o644); err != nil {
		t.Fatal(err)
	}
	_, again, err := syncLock(t, dir, nil)
	if err != nil {
		t.Fatalf("build from the stale lock: %v", err)
	}
	if !reflect.DeepEqual(again, dropped) {
		t.Errorf("build from the stale lock: dropped = %v, want %v", again, dropped)
	}
}

// An op whose content changes (a plugin upgrade) is an update, not a
// removal: its file stays.
func TestSyncEjectedLockContentChangeIsNotRemoval(t *testing.T) {
	dir := t.TempDir()
	mk := func(body string) []protocol.Op {
		return []protocol.Op{&protocol.OpAddFile{Base: protocol.Base{Pkg: "p"}, Location: protocol.FileAndroidAppModule, Name: "google-services.json", Content: b64.StdEncoding.EncodeToString([]byte(body))}}
	}
	path := filepath.Join(dir, "app/google-services.json")
	writeTree(t, dir, map[string]string{"app/google-services.json": "v1"})
	if _, _, err := syncLock(t, dir, mk("v1")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"app/google-services.json": "v2"})
	if deleted, _, err := syncLock(t, dir, mk("v2")); err != nil || len(deleted) != 0 {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("updated file should remain: %v", err)
	}
}

// The lock records ownedFiles; they must be exactly what Apply writes, or
// cleanup would miss files or delete the wrong ones.
func TestOwnedFilesMatchApply(t *testing.T) {
	dir := t.TempDir()
	seedAndroidScaffold(t, dir)
	enc := func(s string) string { return b64.StdEncoding.EncodeToString([]byte(s)) }
	base := protocol.Base{Pkg: "p"}
	ops := []protocol.Op{
		&protocol.OpAddKotlinSource{Base: base, Package: "com.p", RelPath: "sub/P.kt", Content: enc("class P")},
		&protocol.OpAddFile{Base: base, Location: protocol.FileAndroidDrawable, Name: "p_icon", Content: enc("png")},
		&protocol.OpAddFile{Base: base, Location: protocol.FileAndroidDrawable, Name: "p_photo.webp", Content: enc("webp")},
		&protocol.OpAddFile{Base: base, Location: protocol.FileAndroidResource, Name: "xml/p_config.xml", Content: protocol.EncodeContent([]byte("<config/>"))},
		&protocol.OpAddFile{Base: base, Location: protocol.FileAndroidAppModule, Name: "p-services.json", Content: enc("{}")},
	}
	if _, err := apply(t, ops, dir, "android", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, op := range ops {
		files, err := ownedFiles(op, project{dir: dir, platform: "android"})
		if err != nil || len(files) != 1 {
			t.Fatalf("%s: ownedFiles = %v, %v", op.Type(), files, err)
		}
		got, err := os.ReadFile(files[0].Path)
		if err != nil {
			t.Errorf("%s: Apply did not write %s: %v", op.Type(), files[0].Path, err)
			continue
		}
		if string(got) != string(files[0].Content) {
			t.Errorf("%s: %s content differs", op.Type(), files[0].Path)
		}
	}
}

// A CLI upgrade that renames an op type changes its key, but the op still
// owns its file and makes its edit: nothing is deleted or reported.
func TestSyncEjectedLockRenamedOpIsNotRemoval(t *testing.T) {
	dir := t.TempDir()
	base := protocol.Base{Pkg: "p"}
	file := &protocol.OpAddFile{Base: base, Location: protocol.FileAndroidAppModule, Name: "p-services.json", Content: b64.StdEncoding.EncodeToString([]byte("{}"))}
	perm := &protocol.OpAndroidManifestAddPermission{Base: base, Name: "android.permission.CAMERA"}
	ops := []protocol.Op{file, perm}
	writeTree(t, dir, map[string]string{"app/p-services.json": "{}"})
	if _, _, err := syncLock(t, dir, ops); err != nil {
		t.Fatal(err)
	}

	// The lock as an older CLI wrote it, under other op type names.
	lock := filepath.Join(dir, filepath.FromSlash(LockFile))
	data, err := os.ReadFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.NewReplacer(file.Type(), "android.app_module.write", perm.Type(), "android.manifest.permission").Replace(string(data))
	if renamed == string(data) {
		t.Fatal("test did not rename the op types")
	}
	if err := os.WriteFile(lock, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}

	deleted, dropped, err := syncLock(t, dir, ops)
	if err != nil || len(deleted) != 0 || len(dropped) != 0 {
		t.Fatalf("deleted=%v dropped=%v err=%v; want nothing deleted and no edits listed", deleted, dropped, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/p-services.json")); err != nil {
		t.Errorf("file owned by the renamed op was removed: %v", err)
	}
}

// An op that keeps its key but writes another path leaves the old file
// behind unless the lock removes it.
func TestSyncEjectedLockRemovesMovedFile(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) []protocol.Op {
		return []protocol.Op{&protocol.OpAddFile{Base: protocol.Base{Pkg: "p"}, Location: protocol.FileAndroidResource, Name: name, Content: protocol.EncodeContent([]byte("<config/>"))}}
	}
	writeTree(t, dir, map[string]string{"app/src/main/res/xml/p_old.xml": "<config/>"})
	if _, _, err := syncLock(t, dir, mk("xml/p_old.xml")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"app/src/main/res/xml/p_new.xml": "<config/>"})
	deleted, _, err := syncLock(t, dir, mk("xml/p_new.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || !strings.HasSuffix(deleted[0], "p_old.xml") {
		t.Errorf("deleted = %v, want the old path", deleted)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/src/main/res/xml/p_new.xml")); err != nil {
		t.Errorf("new file removed: %v", err)
	}
}

// Drift deletes files the lock names, so a lock naming a path outside the
// project fails the build before anything is deleted.
func TestReadLockRejectsPathsOutsideTheProject(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "platform", "android")
	outside := filepath.Join(root, "precious.txt")
	writeTree(t, root, map[string]string{"precious.txt": "x"})
	for _, path := range []string{"../../precious.txt", outside, "app/../../x", ""} {
		data := fmt.Sprintf(`{"version":1,"ops":[{"key":"k","plugin":"p","files":[{"path":%q,"sha256":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881"}]}]}`, filepath.ToSlash(path))
		writeTree(t, dir, map[string]string{LockFile: data})
		if _, err := ReadLock(dir); err == nil || !strings.Contains(err.Error(), "not inside the project") {
			t.Errorf("path %q: err = %v, want a path error", path, err)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("file outside the project touched: %v", err)
	}
}

func TestReadLockExplainsAParseFailure(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{LockFile: "<<<<<<< HEAD\n{}"})
	if _, err := ReadLock(dir); err == nil || !strings.Contains(err.Error(), "or delete it") {
		t.Errorf("err = %v, want the recovery steps", err)
	}
}

// A lock write that fails (a crash, a full disk) leaves the previous lock
// whole, and the next build recovers.
func TestSyncEjectedLockRecoversFromAFailedWrite(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	mk := func(body string) []protocol.Op {
		return []protocol.Op{&protocol.OpAddFile{Base: protocol.Base{Pkg: "p"}, Location: protocol.FileAndroidAppModule, Name: "p.json", Content: b64.StdEncoding.EncodeToString([]byte(body))}}
	}
	writeTree(t, dir, map[string]string{"app/p.json": "v1"})
	if _, _, err := syncLock(t, dir, mk("v1")); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, filepath.FromSlash(LockFile))
	before, _ := os.ReadFile(lock)

	lockDir := filepath.Dir(lock)
	if err := os.Chmod(lockDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(lockDir, 0o755) })
	writeTree(t, dir, map[string]string{"app/p.json": "v2"})
	if _, _, err := syncLock(t, dir, mk("v2")); err == nil {
		t.Fatal("lock write into a read-only directory succeeded")
	}
	if after, _ := os.ReadFile(lock); string(after) != string(before) {
		t.Fatalf("failed write changed the lock:\n%s", after)
	}
	if err := os.Chmod(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := syncLock(t, dir, mk("v2")); err != nil {
		t.Fatalf("next build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/p.json")); err != nil {
		t.Errorf("plugin file lost: %v", err)
	}
}
