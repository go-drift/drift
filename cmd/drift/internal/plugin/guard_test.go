package plugin

import (
	b64 "encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/scaffold"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func appModuleFile(name, body string) *protocol.OpAndroidAddAppModuleFile {
	return &protocol.OpAndroidAddAppModuleFile{Base: protocol.Base{Pkg: "github.com/acme/push"}, Name: name, Content: b64.StdEncoding.EncodeToString([]byte(body))}
}

// wantForeign checks err is a *ForeignFileError naming exactly paths.
func wantForeign(t *testing.T, err error, paths ...string) {
	t.Helper()
	var foreign *ForeignFileError
	if !errors.As(err, &foreign) {
		t.Fatalf("err = %v, want *ForeignFileError", err)
	}
	if len(foreign.Files) != len(paths) {
		t.Fatalf("refused %v, want %v", foreign.Files, paths)
	}
	for i, p := range paths {
		if foreign.Files[i].Path != p || foreign.Files[i].Plugin != "github.com/acme/push" {
			t.Errorf("refused[%d] = %+v, want %s from github.com/acme/push", i, foreign.Files[i], p)
		}
	}
}

// A file the user put where a plugin writes is never overwritten, and the
// build changes nothing.
func TestApplyRefusesToOverwriteAUserFile(t *testing.T) {
	dir := t.TempDir()
	seedAndroidScaffold(t, dir)
	writeTree(t, dir, map[string]string{"app/services.json": "mine"})
	ops := []protocol.Op{
		appModuleFile("other.json", "plugin"),
		appModuleFile("services.json", "plugin"),
	}
	prev, err := ReadLock(dir) // ejected, no lock yet
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(ops, dir, "android", prev)
	wantForeign(t, err, "app/services.json")
	if got := readFile(t, filepath.Join(dir, "app/services.json")); got != "mine" {
		t.Errorf("user file overwritten: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "app/other.json")); !os.IsNotExist(err) {
		t.Errorf("a refused build wrote other files: %v", err)
	}
}

// A plugin file the user changed since Drift wrote it is kept on a plugin
// upgrade; one left as Drift wrote it is updated; identical content passes.
func TestApplyUpdatesOnlyUnchangedPluginFiles(t *testing.T) {
	dir := t.TempDir()
	seedAndroidScaffold(t, dir)
	v1 := []protocol.Op{appModuleFile("a.json", "a1"), appModuleFile("b.json", "b1")}
	if _, err := Apply(v1, dir, "android", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := syncLock(t, dir, v1); err != nil {
		t.Fatal(err)
	}
	prev, err := ReadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(v1, dir, "android", prev); err != nil {
		t.Errorf("reapplying identical content: %v", err)
	}

	writeTree(t, dir, map[string]string{"app/b.json": "b1, edited"})
	v2 := []protocol.Op{appModuleFile("a.json", "a2"), appModuleFile("b.json", "b2")}
	_, err = Apply(v2, dir, "android", prev)
	wantForeign(t, err, "app/b.json")

	// Undoing the edit lets the upgrade through.
	writeTree(t, dir, map[string]string{"app/b.json": "b1"})
	if _, err := Apply(v2, dir, "android", prev); err != nil {
		t.Fatal(err)
	}
	if a, b := readFile(t, filepath.Join(dir, "app/a.json")), readFile(t, filepath.Join(dir, "app/b.json")); a != "a2" || b != "b2" {
		t.Errorf("after upgrade a=%q b=%q, want a2 b2", a, b)
	}
}

// In a managed build the project is a fresh scaffold, so a plugin writing
// over a template file is refused rather than silently replacing it.
func TestApplyRefusesToReplaceTemplateFiles(t *testing.T) {
	root := t.TempDir()
	if err := scaffold.WriteAndroid(root, scaffold.Settings{AppName: "App", AppID: "com.example.app", Bundle: "com.example.app", Orientation: "portrait"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "android")
	op := &protocol.OpAndroidWriteResourceXML{Base: protocol.Base{Pkg: "github.com/acme/push"}, RelPath: "drawable/launch_background.xml", Content: "<layer-list/>"}
	_, err := Apply([]protocol.Op{op}, dir, "android", nil)
	wantForeign(t, err, "app/src/main/res/drawable/launch_background.xml")
}
