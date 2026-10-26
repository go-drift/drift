package plugin

import (
	b64 "encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// syncLock reconciles dir the way an ejected build does: the lock from the
// previous build, then SyncEjectedLock.
func syncLock(t *testing.T, dir string, ops []protocol.Op) ([]string, error) {
	t.Helper()
	prev, err := ReadLock(dir)
	if err != nil {
		return nil, err
	}
	return SyncEjectedLock(dir, "android", ops, prev)
}

func TestSyncEjectedLockCleansUpRemovedPlugins(t *testing.T) {
	dir := t.TempDir()
	base := protocol.Base{Pkg: "github.com/acme/camera"}
	src := &protocol.OpAddKotlinSource{Base: base, Package: "com.acme.camera", RelPath: "CameraPlugin.kt", Content: b64.StdEncoding.EncodeToString([]byte("class CameraPlugin\n"))}
	drawable := &protocol.OpAndroidWriteDrawable{Base: base, Name: "camera_icon", Content: b64.StdEncoding.EncodeToString([]byte("png"))}
	perm := &protocol.OpAndroidManifestAddPermission{Base: base, Name: "android.permission.CAMERA"}
	// Plugins record ops for every platform; an Android project's lock
	// must ignore the iOS ones.
	plist := &protocol.OpPlistSetString{Base: base, PlistEntry: protocol.PlistEntry{File: protocol.PlistInfo, Key: "NSCameraUsageDescription"}, Value: "camera"}
	ops := []protocol.Op{src, drawable, perm, plist}

	writeFiles := func() {
		for _, op := range ops {
			files, err := OwnedFiles(op, dir, "android")
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				writeTree(t, "/", map[string]string{f.Path: string(f.Content)})
			}
		}
	}
	writeFiles()
	if _, err := syncLock(t, dir, ops); err != nil {
		t.Fatalf("first build: %v", err)
	}
	lock := filepath.Join(dir, filepath.FromSlash(LockFile))
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("lock not written: %v", err)
	}

	// Unchanged ops: nothing to clean up.
	if deleted, err := syncLock(t, dir, ops); err != nil || len(deleted) != 0 {
		t.Fatalf("rebuild: deleted=%v err=%v", deleted, err)
	}

	// The user edits the drawable, then removes the plugin.
	drawablePath := filepath.Join(dir, "app/src/main/res/drawable-nodpi/camera_icon.png")
	if err := os.WriteFile(drawablePath, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	deleted, err := syncLock(t, dir, nil)
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
	if err == nil || !strings.Contains(err.Error(), "android.permission.CAMERA") || !strings.Contains(err.Error(), "github.com/acme/camera") {
		t.Fatalf("want an error listing the manifest edit, got %v", err)
	}
	if strings.Contains(err.Error(), "Info.plist") {
		t.Errorf("an Android project must not list iOS edits: %v", err)
	}

	// The list is shown once.
	if _, err := syncLock(t, dir, nil); err != nil {
		t.Errorf("second build after removal should pass: %v", err)
	}
}

// An op whose content changes (a plugin upgrade) is an update, not a
// removal: its file stays.
func TestSyncEjectedLockContentChangeIsNotRemoval(t *testing.T) {
	dir := t.TempDir()
	mk := func(body string) []protocol.Op {
		return []protocol.Op{&protocol.OpAndroidAddAppModuleFile{Base: protocol.Base{Pkg: "p"}, Name: "google-services.json", Content: b64.StdEncoding.EncodeToString([]byte(body))}}
	}
	path := filepath.Join(dir, "app/google-services.json")
	writeTree(t, dir, map[string]string{"app/google-services.json": "v1"})
	if _, err := syncLock(t, dir, mk("v1")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"app/google-services.json": "v2"})
	if deleted, err := syncLock(t, dir, mk("v2")); err != nil || len(deleted) != 0 {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("updated file should remain: %v", err)
	}
}

// The lock records OwnedFiles; they must be exactly what Apply writes, or
// cleanup would miss files or delete the wrong ones.
func TestOwnedFilesMatchApply(t *testing.T) {
	dir := t.TempDir()
	seedAndroidScaffold(t, dir)
	enc := func(s string) string { return b64.StdEncoding.EncodeToString([]byte(s)) }
	base := protocol.Base{Pkg: "p"}
	ops := []protocol.Op{
		&protocol.OpAddKotlinSource{Base: base, Package: "com.p", RelPath: "sub/P.kt", Content: enc("class P")},
		&protocol.OpAndroidWriteDrawable{Base: base, Name: "p_icon", Content: enc("png")},
		&protocol.OpAndroidWriteDrawable{Base: base, Name: "p_photo.webp", Content: enc("webp")},
		&protocol.OpAndroidWriteResourceXML{Base: base, RelPath: "xml/p_config.xml", Content: "<config/>"},
		&protocol.OpAndroidAddAppModuleFile{Base: base, Name: "p-services.json", Content: enc("{}")},
	}
	if _, err := Apply(ops, dir, "android", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, op := range ops {
		files, err := OwnedFiles(op, dir, "android")
		if err != nil || len(files) != 1 {
			t.Fatalf("%s: OwnedFiles = %v, %v", op.Type(), files, err)
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
	file := &protocol.OpAndroidAddAppModuleFile{Base: base, Name: "p-services.json", Content: b64.StdEncoding.EncodeToString([]byte("{}"))}
	perm := &protocol.OpAndroidManifestAddPermission{Base: base, Name: "android.permission.CAMERA"}
	ops := []protocol.Op{file, perm}
	writeTree(t, dir, map[string]string{"app/p-services.json": "{}"})
	if _, err := syncLock(t, dir, ops); err != nil {
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

	deleted, err := syncLock(t, dir, ops)
	if err != nil || len(deleted) != 0 {
		t.Fatalf("deleted=%v err=%v; want nothing deleted and no edits listed", deleted, err)
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
		return []protocol.Op{&protocol.OpAndroidWriteResourceXML{Base: protocol.Base{Pkg: "p"}, RelPath: name, Content: "<config/>"}}
	}
	writeTree(t, dir, map[string]string{"app/src/main/res/xml/p_old.xml": "<config/>"})
	if _, err := syncLock(t, dir, mk("xml/p_old.xml")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"app/src/main/res/xml/p_new.xml": "<config/>"})
	deleted, err := syncLock(t, dir, mk("xml/p_new.xml"))
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
		return []protocol.Op{&protocol.OpAndroidAddAppModuleFile{Base: protocol.Base{Pkg: "p"}, Name: "p.json", Content: b64.StdEncoding.EncodeToString([]byte(body))}}
	}
	writeTree(t, dir, map[string]string{"app/p.json": "v1"})
	if _, err := syncLock(t, dir, mk("v1")); err != nil {
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
	if _, err := syncLock(t, dir, mk("v2")); err == nil {
		t.Fatal("lock write into a read-only directory succeeded")
	}
	if after, _ := os.ReadFile(lock); string(after) != string(before) {
		t.Fatalf("failed write changed the lock:\n%s", after)
	}
	if err := os.Chmod(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := syncLock(t, dir, mk("v2")); err != nil {
		t.Fatalf("next build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/p.json")); err != nil {
		t.Errorf("plugin file lost: %v", err)
	}
}
