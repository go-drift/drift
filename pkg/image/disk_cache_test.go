package image

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "cache")
}

func TestDiskCache_PutAndGet(t *testing.T) {
	dc, err := NewDiskCache(tempDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("fake png data")
	if err := dc.Put("https://example.com/img.png", data); err != nil {
		t.Fatal(err)
	}

	rc, ok := dc.Get("https://example.com/img.png")
	if !ok {
		t.Fatal("expected hit")
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("expected %q, got %q", data, got)
	}
}

func TestDiskCache_Miss(t *testing.T) {
	dc, err := NewDiskCache(tempDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, ok := dc.Get("nonexistent")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestDiskCache_Remove(t *testing.T) {
	dc, err := NewDiskCache(tempDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	dc.Put("key", []byte("data"))
	dc.Remove("key")
	_, ok := dc.Get("key")
	if ok {
		t.Fatal("expected miss after remove")
	}
}

func TestDiskCache_Clear(t *testing.T) {
	dc, err := NewDiskCache(tempDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	dc.Put("a", []byte("data-a"))
	dc.Put("b", []byte("data-b"))
	if err := dc.Clear(); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dc.dir)
	if len(entries) != 0 {
		t.Fatalf("expected empty dir, got %d files", len(entries))
	}
}

func TestDiskCache_EvictsBySize(t *testing.T) {
	dc, err := NewDiskCache(tempDir(t), 50) // 50 bytes max
	if err != nil {
		t.Fatal(err)
	}

	// Write 3 files of 20 bytes each (60 bytes total, exceeds 50).
	dc.Put("a", make([]byte, 20))
	dc.Put("b", make([]byte, 20))
	dc.Put("c", make([]byte, 20))

	entries, _ := os.ReadDir(dc.dir)
	var totalSize int64
	for _, e := range entries {
		info, _ := e.Info()
		totalSize += info.Size()
	}
	if totalSize > 50 {
		t.Fatalf("expected total size <= 50, got %d", totalSize)
	}
}

func TestDiskCache_DifferentKeysProduceDifferentFiles(t *testing.T) {
	dc, err := NewDiskCache(tempDir(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	dc.Put("key1", []byte("data1"))
	dc.Put("key2", []byte("data2"))

	entries, _ := os.ReadDir(dc.dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 files, got %d", len(entries))
	}
}
