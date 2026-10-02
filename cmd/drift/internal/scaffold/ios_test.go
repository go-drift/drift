package scaffold

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
)

// The project template signs with an entitlements file; every writer of an
// Xcode project (managed builds and drift eject) must ship it, or Xcode
// fails the build before compiling anything.
func TestWriteIOSProjectWritesSigningEntitlements(t *testing.T) {
	dir := t.TempDir()
	data := templates.NewTemplateData(templates.TemplateInput{
		AppName:        "Demo",
		AndroidPackage: "com.example.demo",
		IOSBundleID:    "com.example.demo",
	})
	if err := WriteIOSProject(dir, data, t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	pbxproj, err := os.ReadFile(filepath.Join(dir, "Runner.xcodeproj", "project.pbxproj"))
	if err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`CODE_SIGN_ENTITLEMENTS = ([^;]+);`).FindAllSubmatch(pbxproj, -1)
	if len(refs) == 0 {
		t.Fatal("project.pbxproj sets no CODE_SIGN_ENTITLEMENTS")
	}
	for _, ref := range refs {
		if _, err := os.Stat(filepath.Join(dir, string(ref[1]))); err != nil {
			t.Errorf("CODE_SIGN_ENTITLEMENTS names %s, which was not written: %v", ref[1], err)
		}
	}
}
