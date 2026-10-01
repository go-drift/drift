package plugin

import (
	b64 "encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

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
	if _, err := SyncEjectedLock(dir, "android", ops); err != nil {
		t.Fatalf("first build: %v", err)
	}
	lock := filepath.Join(dir, filepath.FromSlash(LockFile))
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("lock not written: %v", err)
	}

	// Unchanged ops: nothing to clean up.
	if deleted, err := SyncEjectedLock(dir, "android", ops); err != nil || len(deleted) != 0 {
		t.Fatalf("rebuild: deleted=%v err=%v", deleted, err)
	}

	// The user edits the drawable, then removes the plugin.
	drawablePath := filepath.Join(dir, "app/src/main/res/drawable-nodpi/camera_icon.png")
	if err := os.WriteFile(drawablePath, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	deleted, err := SyncEjectedLock(dir, "android", nil)
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
	if _, err := SyncEjectedLock(dir, "android", nil); err != nil {
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
	if _, err := SyncEjectedLock(dir, "android", mk("v1")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"app/google-services.json": "v2"})
	if deleted, err := SyncEjectedLock(dir, "android", mk("v2")); err != nil || len(deleted) != 0 {
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
		&protocol.OpAndroidAddAsset{Base: base, Path: "models/p.bin", Content: enc("model")},
		&protocol.OpAndroidAddAppModuleFile{Base: base, Name: "p-services.json", Content: enc("{}")},
	}
	if _, err := Apply(ops, dir, "android"); err != nil {
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
