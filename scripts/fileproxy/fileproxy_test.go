package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

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

// TestWriteProxy builds a proxy from a scratch repository and checks the
// root module's zip: valid, and without the nested module's files.
func TestWriteProxy(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	files := map[string]string{
		"go.mod":                "module example.com/root\n\ngo 1.24.0\n",
		"root.go":               "package root\n",
		"plugins/p/p.go":        "package p\n",
		"examples/app/go.mod":   "module example.com/root/examples/app\n\ngo 1.24.0\n",
		"examples/app/main.go":  "package main\n",
		"uncommitted/ignore.go": "package ignore\n",
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
		{"add", "go.mod", "root.go", "plugins", "examples"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "init"},
	} {
		if _, err := gitOutput(repo, args...); err != nil {
			t.Fatal(err)
		}
	}

	proxy := t.TempDir()
	for _, v := range []string{"v0.2.0", "v0.1.0"} {
		if err := writeProxy(proxy, repo, "HEAD", v); err != nil {
			t.Fatal(err)
		}
	}

	const path = "example.com/root"
	vdir := filepath.Join(proxy, filepath.FromSlash(path), "@v")
	list, err := os.ReadFile(filepath.Join(vdir, "list"))
	if err != nil || string(list) != "v0.1.0\nv0.2.0\n" {
		t.Errorf("list = %q, %v", list, err)
	}
	cz, err := zip.CheckZip(module.Version{Path: path, Version: "v0.1.0"}, filepath.Join(vdir, "v0.1.0.zip"))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	want := []string{"example.com/root@v0.1.0/go.mod", "example.com/root@v0.1.0/plugins/p/p.go", "example.com/root@v0.1.0/root.go"}
	if !reflect.DeepEqual(cz.Valid, want) {
		t.Errorf("zip files = %v, want %v", cz.Valid, want)
	}
	mod, err := os.ReadFile(filepath.Join(vdir, "v0.1.0.mod"))
	if err != nil || !strings.HasPrefix(string(mod), "module "+path+"\n") {
		t.Errorf(".mod = %q, %v", mod, err)
	}
	if _, err := os.Stat(filepath.Join(proxy, "example.com/root/examples")); !os.IsNotExist(err) {
		t.Errorf("nested module in the proxy: %v", err)
	}
}
