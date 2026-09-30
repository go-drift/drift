package mutate

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func iosBundleOp(pkg, path string, content []byte) *protocol.OpIOSAddBundleResource {
	return &protocol.OpIOSAddBundleResource{
		Base:    protocol.Base{Pkg: pkg},
		Path:    path,
		Content: base64.StdEncoding.EncodeToString(content),
	}
}

// writeBundleOps decodes ops the way apply.go does and syncs dir to them.
func writeBundleOps(t *testing.T, dir string, ops ...*protocol.OpIOSAddBundleResource) ([]string, error) {
	t.Helper()
	files, err := IOSBundleFiles(ops)
	if err != nil {
		return nil, err
	}
	return WriteIOSBundleResources(dir, files)
}

func androidAssetOp(pkg, path string, content []byte) *protocol.OpAndroidAddAsset {
	return &protocol.OpAndroidAddAsset{
		Base:    protocol.Base{Pkg: pkg},
		Path:    path,
		Content: base64.StdEncoding.EncodeToString(content),
	}
}

func TestWriteIOSBundleResourceVerbatim(t *testing.T) {
	dir := t.TempDir()
	content := []byte("PROJECT_ID=test\nAPI_KEY=abc\n")
	changed, err := writeBundleOps(t, dir, iosBundleOp("github.com/foo/firebase", "GoogleService-Info.plist", content))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("expected 1 file changed, got %d (%v)", len(changed), changed)
	}
	got, err := os.ReadFile(filepath.Join(dir, "GoogleService-Info.plist"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content not written verbatim:\ngot:  %q\nwant: %q", got, content)
	}
}

func TestWriteIOSBundleResourceIdempotent(t *testing.T) {
	dir := t.TempDir()
	op := iosBundleOp("p", "foo.json", []byte(`{"a":1}`))
	if _, err := writeBundleOps(t, dir, op); err != nil {
		t.Fatalf("first write: %v", err)
	}
	changed, err := writeBundleOps(t, dir, op)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("expected zero changes on byte-identical rerun, got %v", changed)
	}
}

func TestWriteIOSBundleResourceOverwriteOnDiff(t *testing.T) {
	dir := t.TempDir()
	first := iosBundleOp("p", "foo.json", []byte("v1"))
	second := iosBundleOp("p", "foo.json", []byte("v2"))
	if _, err := writeBundleOps(t, dir, first); err != nil {
		t.Fatalf("first write: %v", err)
	}
	changed, err := writeBundleOps(t, dir, second)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if len(changed) != 1 {
		t.Errorf("expected one change on byte-different content, got %v", changed)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "foo.json"))
	if !bytes.Equal(got, []byte("v2")) {
		t.Errorf("expected v2 after overwrite, got %q", got)
	}
}

// A plugin dropped from the project must take its resources with it,
// including files nobody asked for that ended up in the Drift-owned dir.
func TestWriteIOSBundleResourcePrunesStale(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeBundleOps(t, dir,
		iosBundleOp("a", "keep.json", []byte("k")),
		iosBundleOp("b", "gone.json", []byte("g")),
	); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "stray"), 0o755); err != nil {
		t.Fatal(err)
	}
	changed, err := writeBundleOps(t, dir, iosBundleOp("a", "keep.json", []byte("k")))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(changed) != 2 {
		t.Errorf("expected 2 removals reported, got %v", changed)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "keep.json" {
		t.Errorf("expected only keep.json to remain, got %v", entries)
	}
}

func TestWriteIOSBundleResourceEmptyRemovesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "PluginResources")
	if _, err := writeBundleOps(t, dir, iosBundleOp("a", "x.json", []byte("x"))); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := WriteIOSBundleResources(dir, nil); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("expected dir removed, stat err = %v", err)
	}
	// Missing dir with no files is a no-op.
	if changed, err := WriteIOSBundleResources(dir, nil); err != nil || len(changed) != 0 {
		t.Errorf("expected no-op, got %v, %v", changed, err)
	}
}

func TestWriteIOSBundleResourceDuplicateNameFails(t *testing.T) {
	dir := t.TempDir()
	images, err := ImageSetBundleFiles([]*protocol.OpIOSAssetsAddImageSet{
		{Base: protocol.Base{Pkg: "splash"}, Name: "Logo", Image: base64.StdEncoding.EncodeToString([]byte("png"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := IOSBundleFiles([]*protocol.OpIOSAddBundleResource{iosBundleOp("other", "Logo.png", []byte("other"))})
	if err != nil {
		t.Fatal(err)
	}
	_, err = WriteIOSBundleResources(dir, append(images, files...))
	if err == nil || !strings.Contains(err.Error(), "splash") || !strings.Contains(err.Error(), "other") {
		t.Fatalf("expected duplicate-name error naming both plugins, got %v", err)
	}
}

func TestImageSetBundleFilesLoosePNG(t *testing.T) {
	files, err := ImageSetBundleFiles([]*protocol.OpIOSAssetsAddImageSet{
		{Base: protocol.Base{Pkg: "splash"}, Name: "DriftSplash", Image: base64.StdEncoding.EncodeToString([]byte("png-bytes"))},
	})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(files) != 1 || files[0].Name != "DriftSplash.png" || string(files[0].Content) != "png-bytes" {
		t.Errorf("unexpected conversion: %+v", files)
	}
}

func TestWriteAndroidAppModuleFiles(t *testing.T) {
	dir := t.TempDir()
	op := &protocol.OpAndroidAddAppModuleFile{
		Base:    protocol.Base{Pkg: "fb"},
		Name:    "google-services.json",
		Content: base64.StdEncoding.EncodeToString([]byte(`{"project_info":{}}`)),
	}
	changed, err := WriteAndroidAppModuleFiles(dir, []*protocol.OpAndroidAddAppModuleFile{op})
	if err != nil || len(changed) != 1 {
		t.Fatalf("write: %v, %v", changed, err)
	}
	changed, err = WriteAndroidAppModuleFiles(dir, []*protocol.OpAndroidAddAppModuleFile{op})
	if err != nil || len(changed) != 0 {
		t.Errorf("expected idempotent rerun, got %v, %v", changed, err)
	}
}

// Binary content (zero bytes, all-ASCII, random) must survive the base64
// round trip the JSON bridge enforces.
func TestWriteIOSBundleResourceBinaryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]byte{
		"empty.bin":  {},
		"ascii.bin":  []byte("hello world"),
		"random.bin": randomBytes(t, 1024),
		"nulls.bin":  {0x00, 0x01, 0x00, 0x02, 0xFF, 0xFE},
	}
	var ops []*protocol.OpIOSAddBundleResource
	for name, content := range cases {
		ops = append(ops, iosBundleOp("p", name, content))
	}
	if _, err := writeBundleOps(t, dir, ops...); err != nil {
		t.Fatalf("write: %v", err)
	}
	for name, want := range cases {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: base64 round-trip changed bytes\ngot:  %v\nwant: %v", name, got, want)
		}
	}
}

func TestWriteAndroidAssetVerbatim(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`{"project_info":{"project_id":"test"}}`)
	changed, err := WriteAndroidAssets(dir, []*protocol.OpAndroidAddAsset{
		androidAssetOp("github.com/foo/data", "data/config.json", content),
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("expected 1 file changed, got %v", changed)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "data", "config.json"))
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: %q vs %q", got, content)
	}
}

func TestWriteAndroidAssetIdempotent(t *testing.T) {
	dir := t.TempDir()
	op := androidAssetOp("p", "fonts/roboto.ttf", []byte("font"))
	if _, err := WriteAndroidAssets(dir, []*protocol.OpAndroidAddAsset{op}); err != nil {
		t.Fatalf("first: %v", err)
	}
	changed, err := WriteAndroidAssets(dir, []*protocol.OpAndroidAddAsset{op})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("expected zero changes on rerun, got %v", changed)
	}
}

// Soft warning fires for resources at or above the threshold; nothing for
// smaller payloads. captures via warnSink override.
func TestBundleResourceSoftWarning(t *testing.T) {
	t.Run("large payload triggers warning", func(t *testing.T) {
		var buf bytes.Buffer
		orig := warnSink
		warnSink = &buf
		t.Cleanup(func() { warnSink = orig })

		dir := t.TempDir()
		big := make([]byte, bundleResourceWarnBytes+1)
		op := iosBundleOp("github.com/foo/big-plugin", "model.bin", big)
		if _, err := writeBundleOps(t, dir, op); err != nil {
			t.Fatalf("write: %v", err)
		}
		if !strings.Contains(buf.String(), "github.com/foo/big-plugin") {
			t.Errorf("expected plugin attribution in warning, got %q", buf.String())
		}
		if !strings.Contains(buf.String(), "model.bin") {
			t.Errorf("expected resource path in warning, got %q", buf.String())
		}
	})

	t.Run("small payload silent", func(t *testing.T) {
		var buf bytes.Buffer
		orig := warnSink
		warnSink = &buf
		t.Cleanup(func() { warnSink = orig })

		dir := t.TempDir()
		small := make([]byte, 1024)
		op := iosBundleOp("p", "icon.png", small)
		if _, err := writeBundleOps(t, dir, op); err != nil {
			t.Fatalf("write: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected silent for small payload, got %q", buf.String())
		}
	})
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return b
}
