package mutate

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// committed runs plan against a fresh changeset and commits it when plan
// succeeds, as a build does, so tests can check the result on disk.
func committed[R any](t *testing.T, plan func(cs *Changeset) (R, error)) (R, error) {
	t.Helper()
	cs := NewChangeset()
	r, err := plan(cs)
	if err == nil {
		if cerr := cs.Commit(); cerr != nil {
			t.Fatalf("Commit: %v", cerr)
		}
	}
	return r, err
}

// committed3 is committed for mutators that also return their path.
func committed3(t *testing.T, plan func(cs *Changeset) (string, bool, error)) (string, bool, error) {
	t.Helper()
	var path string
	ch, err := committed(t, func(cs *Changeset) (bool, error) {
		var (
			ch  bool
			err error
		)
		path, ch, err = plan(cs)
		return ch, err
	})
	return path, ch, err
}

func applyPlist(t *testing.T, path string, ops []protocol.PlistOp) (bool, error) {
	return committed(t, func(cs *Changeset) (bool, error) { return ApplyPlist(cs, path, ops) })
}

func applyAndroidManifest(t *testing.T, path string, ops ManifestOps) (bool, error) {
	return committed(t, func(cs *Changeset) (bool, error) { return ApplyAndroidManifest(cs, path, ops) })
}

func setXtoolResources(t *testing.T, path string, resources []string) (bool, error) {
	return committed(t, func(cs *Changeset) (bool, error) { return SetXtoolResources(cs, path, resources) })
}

func writePluginPackage(t *testing.T, root string, pkg PluginPackage) ([]string, error) {
	return committed(t, func(cs *Changeset) ([]string, error) { return WritePluginPackage(cs, root, pkg) })
}

func writeIOSBundleResources(t *testing.T, dir string, files []BundleFile) ([]string, error) {
	return committed(t, func(cs *Changeset) ([]string, error) { return WriteIOSBundleResources(cs, dir, files) })
}

func writeOwned(t *testing.T, files ...OwnedFile) ([]string, error) {
	return committed(t, func(cs *Changeset) ([]string, error) { return WriteOwned(cs, files...) })
}

func writeAndroidColors(t *testing.T, path string, ops []*protocol.OpAndroidColorSet) (string, bool, error) {
	return committed3(t, func(cs *Changeset) (string, bool, error) { return WriteAndroidColors(cs, path, ops) })
}

func writeAndroidStrings(t *testing.T, path string, ops []*protocol.OpAndroidStringSet) (string, bool, error) {
	return committed3(t, func(cs *Changeset) (string, bool, error) { return WriteAndroidStrings(cs, path, ops) })
}

func writeAndroidStyles(t *testing.T, path string, ops []*protocol.OpAndroidStyleSet) (string, bool, error) {
	return committed3(t, func(cs *Changeset) (string, bool, error) { return WriteAndroidStyles(cs, path, ops) })
}

func applyGradleAddDependencies(t *testing.T, path string, ops []*protocol.OpAndroidGradleAddDependency, replace map[string]string) (string, bool, error) {
	return committed3(t, func(cs *Changeset) (string, bool, error) {
		return ApplyGradleAddDependencies(cs, path, ops, replace)
	})
}

func applyGradleApplyPlugins(t *testing.T, path string, ops []*protocol.OpAndroidGradleApplyPlugin) (string, bool, error) {
	return committed3(t, func(cs *Changeset) (string, bool, error) { return ApplyGradleApplyPlugins(cs, path, ops) })
}

func applyGradleProjectPlugins(t *testing.T, path string, ops []*protocol.OpAndroidGradleApplyPlugin, replace map[string]string) (string, bool, error) {
	return committed3(t, func(cs *Changeset) (string, bool, error) {
		return ApplyGradleProjectPlugins(cs, path, ops, replace)
	})
}

// A changeset touches nothing until Commit, and reads back what it planned.
func TestChangesetPlansWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	kept := filepath.Join(dir, "kept.txt")
	edited := filepath.Join(dir, "edited.txt")
	removed := filepath.Join(dir, "old", "removed.txt")
	added := filepath.Join(dir, "new", "added.txt")
	for p, content := range map[string]string{kept: "kept", edited: "v1", removed: "old"} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cs := NewChangeset()
	if ch, err := cs.WriteFile(kept, []byte("kept")); err != nil || ch {
		t.Fatalf("WriteFile(same bytes) = %v, %v; want unchanged", ch, err)
	}
	if ch, err := cs.WriteFile(edited, []byte("v2")); err != nil || !ch {
		t.Fatalf("WriteFile(edited) = %v, %v; want changed", ch, err)
	}
	// A second edit sees the first.
	if got, err := cs.ReadFile(edited); err != nil || string(got) != "v2" {
		t.Fatalf("ReadFile(edited) = %q, %v; want planned v2", got, err)
	}
	if ch, err := cs.WriteFile(edited, []byte("v2")); err != nil || ch {
		t.Fatalf("WriteFile(edited again) = %v, %v; want unchanged against the plan", ch, err)
	}
	if gone, err := cs.Remove(removed); err != nil || !gone {
		t.Fatalf("Remove = %v, %v; want existed", gone, err)
	}
	if _, err := cs.ReadFile(removed); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(removed) err = %v; want ErrNotExist", err)
	}
	cs.RemoveIfEmpty(filepath.Dir(removed))
	if ch, err := cs.WriteFile(added, []byte("new")); err != nil || !ch {
		t.Fatalf("WriteFile(added) = %v, %v; want changed", ch, err)
	}

	assertFile(t, edited, "v1")
	assertFile(t, removed, "old")
	if _, err := os.Stat(added); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("added exists before Commit: %v", err)
	}

	if err := cs.Commit(); err != nil {
		t.Fatal(err)
	}
	assertFile(t, kept, "kept")
	assertFile(t, edited, "v2")
	assertFile(t, added, "new")
	if _, err := os.Stat(filepath.Dir(removed)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("emptied dir survives Commit: %v", err)
	}
}

// Removing a path drops writes planned under it; writing after a removal
// lands after it, so a directory can be replaced by a file of its name.
func TestChangesetRemoveThenWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.png")
	if err := os.MkdirAll(filepath.Join(target, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}

	cs := NewChangeset()
	if _, err := cs.WriteFile(filepath.Join(target, "inner", "x"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	if gone, err := cs.Remove(target); err != nil || !gone {
		t.Fatalf("Remove = %v, %v", gone, err)
	}
	if _, err := cs.ReadFile(filepath.Join(target, "inner", "x")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("write under a removed dir still planned: %v", err)
	}
	if _, err := cs.WriteFile(target, []byte("png")); err != nil {
		t.Fatal(err)
	}
	if err := cs.Commit(); err != nil {
		t.Fatal(err)
	}
	assertFile(t, target, "png")
}

// Two spellings of one path are one entry: a read never falls back to disk
// past a planned write or removal.
func TestChangesetCleansPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "res", "values.xml")
	messy := dir + "/res/./sub/../values.xml"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("disk"), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := NewChangeset()
	if _, err := cs.WriteFile(messy, []byte("planned")); err != nil {
		t.Fatal(err)
	}
	if got, err := cs.ReadFile(path); err != nil || string(got) != "planned" {
		t.Fatalf("ReadFile(clean) = %q, %v; want the write planned under another spelling", got, err)
	}
	if ch, err := cs.WriteFile(path, []byte("planned")); err != nil || ch {
		t.Fatalf("WriteFile(clean, same bytes) = %v, %v; want unchanged", ch, err)
	}
	if gone, err := cs.Remove(dir + "/res/"); err != nil || !gone {
		t.Fatalf("Remove = %v, %v", gone, err)
	}
	if _, err := cs.ReadFile(messy); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(messy) after removing its dir: err = %v, want ErrNotExist", err)
	}
	if err := cs.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removed dir survives Commit: %v", err)
	}
}

// Commit skips an empty-dir removal that finds the dir gone or still in
// use, and fails on any other error.
func TestChangesetCommitReportsEmptyDirFailures(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	full := filepath.Join(dir, "full")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cs := NewChangeset()
	cs.RemoveIfEmpty(full)
	cs.RemoveIfEmpty(filepath.Join(dir, "missing"))
	if err := cs.Commit(); err != nil {
		t.Fatalf("Commit over a full and a missing dir: %v", err)
	}

	locked := filepath.Join(dir, "locked")
	empty := filepath.Join(locked, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	cs = NewChangeset()
	cs.RemoveIfEmpty(empty)
	if err := cs.Commit(); err == nil {
		t.Fatal("Commit swallowed a failure to remove an empty dir")
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", filepath.Base(path), got, want)
	}
}
