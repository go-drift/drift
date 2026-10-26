package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

func parseMod(t *testing.T, dir, src string) repoModule {
	t.Helper()
	f, err := modfile.Parse(dir+"/go.mod", []byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	return repoModule{Dir: dir, Path: f.Module.Mod.Path, Released: releasedDir(dir), GoMod: f, Data: []byte(src)}
}

func testModules(t *testing.T, drift string) []repoModule {
	return []repoModule{
		parseMod(t, ".", "module github.com/go-drift/drift\n\ngo 1.24.0\n\nrequire golang.org/x/mod v0.30.0\n"),
		parseMod(t, "examples/demo", "module github.com/go-drift/drift/examples/demo\n\ngo 1.24.0\n\nrequire (\n\tgithub.com/go-drift/drift "+drift+"\n\tgithub.com/go-drift/drift/plugins/splash "+drift+"\n\tgolang.org/x/image v0.34.0\n)\n"),
		parseMod(t, "plugins/splash", "module github.com/go-drift/drift/plugins/splash\n\ngo 1.24.0\n\nrequire github.com/go-drift/drift "+drift+"\n\nreplace github.com/go-drift/drift => ../..\n"),
	}
}

func TestReleasedDir(t *testing.T) {
	for dir, want := range map[string]bool{
		".": true, "plugins/splash": true,
		"examples/splash-demo": false, "examples/plugins/hello": false, "showcase": false, "plugins/splash/sub": false,
	} {
		if got := releasedDir(dir); got != want {
			t.Errorf("releasedDir(%q) = %v, want %v", dir, got, want)
		}
	}
}

func TestTags(t *testing.T) {
	got := tags(testModules(t, "v0.26.1"), "v0.27.0")
	want := []string{"v0.27.0", "plugins/splash/v0.27.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags = %v, want %v", got, want)
	}
}

func TestMismatchesAndRequireReleased(t *testing.T) {
	mods := testModules(t, "v0.26.1")
	if bad := mismatches(mods, "v0.26.1"); len(bad) != 0 {
		t.Errorf("mismatches at the required version: %v", bad)
	}
	bad := mismatches(mods, "v0.27.0")
	want := []string{
		"examples/demo/go.mod: github.com/go-drift/drift v0.26.1",
		"examples/demo/go.mod: github.com/go-drift/drift/plugins/splash v0.26.1",
		"plugins/splash/go.mod: github.com/go-drift/drift v0.26.1",
	}
	if !reflect.DeepEqual(bad, want) {
		t.Errorf("mismatches = %v, want %v", bad, want)
	}

	released := releasedPaths(mods)
	for _, m := range mods {
		changed, err := requireReleased(m.GoMod, released, "v0.27.0")
		if err != nil {
			t.Fatal(err)
		}
		if changed != (m.Dir != ".") {
			t.Errorf("%s: changed = %v", m.Dir, changed)
		}
	}
	if bad := mismatches(mods, "v0.27.0"); len(bad) != 0 {
		t.Errorf("mismatches after requireReleased: %v", bad)
	}
	// Other requirements and the replace are untouched.
	data, err := mods[1].GoMod.Format()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "golang.org/x/image v0.34.0") {
		t.Errorf("unrelated requirement changed:\n%s", data)
	}
	if mods[2].GoMod.Replace == nil || len(mods[2].GoMod.Replace) != 1 {
		t.Errorf("plugin replace lost")
	}
}

func TestParseReleaseVersion(t *testing.T) {
	for v, ok := range map[string]bool{
		"v0.27.0": true, "v1.0.0-rc.1": true,
		"0.27.0": false, "v0.27": false, "v0.27.0+meta": false,
		"v0.27.1-0.20261026120000-abcdef123456": false,
	} {
		if _, err := parseReleaseVersion(v); (err == nil) != ok {
			t.Errorf("parseReleaseVersion(%q) err = %v, want ok=%v", v, err, ok)
		}
	}
}

// TestWriteProxy builds a proxy from a scratch repository and checks each
// released module's zip: valid, and without the nested module's files.
func TestWriteProxy(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	files := map[string]string{
		"go.mod":              "module example.com/root\n\ngo 1.24.0\n",
		"root.go":             "package root\n",
		"plugins/p/go.mod":    "module example.com/root/plugins/p\n\ngo 1.24.0\n\nrequire example.com/root v0.0.0\n",
		"plugins/p/p.go":      "package p\n",
		"examples/app/go.mod": "module example.com/root/examples/app\n\ngo 1.24.0\n",
	}
	for name, content := range files {
		path := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "go.mod", "root.go", "plugins", "examples/app/go.mod"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "init"},
	} {
		if _, err := gitOutput(repo, args...); err != nil {
			t.Fatal(err)
		}
	}

	mods, err := listModules(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	proxy := t.TempDir()
	for _, v := range []string{"v0.2.0", "v0.1.0"} {
		if err := writeProxy(proxy, repo, "HEAD", mods, v); err != nil {
			t.Fatal(err)
		}
	}

	for path, want := range map[string][]string{
		"example.com/root":           {"example.com/root@v0.1.0/go.mod", "example.com/root@v0.1.0/root.go"},
		"example.com/root/plugins/p": {"example.com/root/plugins/p@v0.1.0/go.mod", "example.com/root/plugins/p@v0.1.0/p.go"},
	} {
		vdir := filepath.Join(proxy, filepath.FromSlash(path), "@v")
		list, err := os.ReadFile(filepath.Join(vdir, "list"))
		if err != nil || string(list) != "v0.1.0\nv0.2.0\n" {
			t.Errorf("%s list = %q, %v", path, list, err)
		}
		cz, err := zip.CheckZip(module.Version{Path: path, Version: "v0.1.0"}, filepath.Join(vdir, "v0.1.0.zip"))
		if err != nil {
			t.Fatalf("%s zip: %v", path, err)
		}
		if !reflect.DeepEqual(cz.Valid, want) {
			t.Errorf("%s zip files = %v, want %v", path, cz.Valid, want)
		}
		mod, err := os.ReadFile(filepath.Join(vdir, "v0.1.0.mod"))
		if err != nil || !strings.HasPrefix(string(mod), "module "+path+"\n") {
			t.Errorf("%s .mod = %q, %v", path, mod, err)
		}
	}
	if _, err := os.Stat(filepath.Join(proxy, "example.com/root/examples")); !os.IsNotExist(err) {
		t.Errorf("unreleased example module in the proxy: %v", err)
	}
}
