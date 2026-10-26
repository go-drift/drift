package mutate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicKeepsOldContentWhenInterrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.gradle")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	crash := errors.New("crash before rename")
	rename = func(string, string) error { return crash }
	t.Cleanup(func() { rename = os.Rename })

	if err := WriteFileAtomic(path, []byte("new")); !errors.Is(err, crash) {
		t.Fatalf("WriteFileAtomic error = %v, want the rename failure", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("content after interrupted write = %q, want old", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}

func TestWriteFileAtomicReplacesAndKeepsMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gradlew")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" || info.Mode().Perm() != 0o755 {
		t.Errorf("content %q mode %v, want new 0755", got, info.Mode().Perm())
	}

	created := filepath.Join(dir, "sub", "new.xml")
	if err := WriteFileAtomic(created, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(created); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("new file: %v, %v; want mode 0644", info, err)
	}
}

func TestWriteFileAtomicWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.plist")
	link := filepath.Join(dir, "Info.plist")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := WriteFileAtomic(link, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink replaced by a regular file")
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Errorf("target = %q, want new", got)
	}
}
