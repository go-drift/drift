package plugin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/cmd/drift/internal/scaffold"
	"github.com/go-drift/drift/cmd/drift/internal/templates"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// buildEjected runs the plugin steps of an ejected build in dir: the lock
// from the previous build, Apply, then SyncEjectedLock, committed together.
func buildEjected(t *testing.T, dir, platform string, ops []protocol.Op) (dropped []string, err error) {
	t.Helper()
	prev, err := ReadLock(dir)
	if err != nil {
		return nil, err
	}
	cs := mutate.NewChangeset()
	if _, err := Apply(cs, ops, dir, platform, prev); err != nil {
		return nil, err
	}
	if _, dropped, err = SyncEjectedLock(cs, dir, platform, ops, prev); err != nil {
		return nil, err
	}
	return dropped, cs.Commit()
}

// captureStderr returns what fn writes to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	saved := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = saved }()
	fn()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func launchScreenOp(content string) *protocol.OpIOSReplaceLaunchScreen {
	return &protocol.OpIOSReplaceLaunchScreen{Base: protocol.Base{Pkg: "github.com/acme/push"}, Content: content}
}

// seedIOSProject writes the files an ejected iOS project's plugin build
// reads, with the template's launch screen at launch.
func seedIOSProject(t *testing.T, dir, launch string) {
	t.Helper()
	def, err := templates.DefaultLaunchScreen()
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{launch: string(def), "xtool.yml": "bundleID: com.example.app\n"})
}

var launchScreenPaths = map[string]string{
	"ios":   "Runner/LaunchScreen.storyboard",
	"xtool": "Sources/Runner/Resources/LaunchScreen.storyboard",
}

// A plugin replaces the launch screen of an ejected project while it is
// the template's, and restores the template's once removed.
func TestEjectedLaunchScreenReplacedAndRestored(t *testing.T) {
	for platform, launch := range launchScreenPaths {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			seedIOSProject(t, dir, launch)
			def := readFile(t, filepath.Join(dir, launch))

			v1 := []protocol.Op{launchScreenOp("<splash v1/>")}
			if _, err := buildEjected(t, dir, platform, v1); err != nil {
				t.Fatalf("first build: %v", err)
			}
			if got := readFile(t, filepath.Join(dir, launch)); got != "<splash v1/>" {
				t.Fatalf("launch screen = %q, want the plugin's", got)
			}
			lock, err := ReadLock(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !lock.wrote(launch, []byte("<splash v1/>")) {
				t.Errorf("lock does not record the launch screen as Drift's: %+v", lock.Ops)
			}

			// A plugin upgrade replaces what Drift wrote.
			v2 := []protocol.Op{launchScreenOp("<splash v2/>")}
			if _, err := buildEjected(t, dir, platform, v2); err != nil {
				t.Fatalf("upgrade: %v", err)
			}

			// Removing the plugin restores the template's, which
			// Info.plist's UILaunchStoryboardName names, without listing it
			// as a dropped edit.
			dropped, err := buildEjected(t, dir, platform, nil)
			if err != nil || len(dropped) != 0 {
				t.Fatalf("removal: dropped=%v err=%v", dropped, err)
			}
			if got := readFile(t, filepath.Join(dir, launch)); got != def {
				t.Errorf("launch screen not restored to the template's:\n%s", got)
			}
		})
	}
}

// The user's own launch screen is never replaced.
func TestEjectedLaunchScreenRefusesAUserStoryboard(t *testing.T) {
	for platform, launch := range launchScreenPaths {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			seedIOSProject(t, dir, launch)
			writeTree(t, dir, map[string]string{launch: "<mine/>"})
			_, err := buildEjected(t, dir, platform, []protocol.Op{launchScreenOp("<splash/>")})
			wantForeign(t, err, launch)
			if got := readFile(t, filepath.Join(dir, launch)); got != "<mine/>" {
				t.Errorf("user launch screen overwritten: %q", got)
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(LockFile))); !os.IsNotExist(err) {
				t.Errorf("a refused build wrote the lock: %v", err)
			}

			// Edited after Drift wrote it: refused too.
			seedIOSProject(t, dir, launch)
			if _, err := buildEjected(t, dir, platform, []protocol.Op{launchScreenOp("<splash/>")}); err != nil {
				t.Fatal(err)
			}
			writeTree(t, dir, map[string]string{launch: "<splash, edited/>"})
			_, err = buildEjected(t, dir, platform, []protocol.Op{launchScreenOp("<splash v2/>")})
			wantForeign(t, err, launch)
			if got := readFile(t, filepath.Join(dir, launch)); got != "<splash, edited/>" {
				t.Errorf("edited launch screen overwritten: %q", got)
			}
		})
	}
}

// A launch screen the user edited since the plugin wrote it stays when the
// plugin is removed, with a warning.
func TestEjectedLaunchScreenKeptAfterUserEdit(t *testing.T) {
	for platform, launch := range launchScreenPaths {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			seedIOSProject(t, dir, launch)
			if _, err := buildEjected(t, dir, platform, []protocol.Op{launchScreenOp("<splash/>")}); err != nil {
				t.Fatal(err)
			}
			writeTree(t, dir, map[string]string{launch: "<splash, edited/>"})
			var err error
			stderr := captureStderr(t, func() { _, err = buildEjected(t, dir, platform, nil) })
			if err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, filepath.Join(dir, launch)); got != "<splash, edited/>" {
				t.Errorf("edited launch screen replaced: %q", got)
			}
			if want := fmt.Sprintf("kept %s from plugin github.com/acme/push", launch); !strings.Contains(stderr, want) {
				t.Errorf("stderr = %q, want a warning containing %q", stderr, want)
			}
		})
	}
}

// A lock from before Drift owned the launch screen records it only as an
// edit, with no hash; the file then holds the plugin's content, which the
// next build accepts and records.
func TestEjectedLaunchScreenUpgradesAnOldLock(t *testing.T) {
	for platform, launch := range launchScreenPaths {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			seedIOSProject(t, dir, launch)
			op := launchScreenOp("<splash/>")
			old := fmt.Sprintf(`{"version":1,"ops":[{"key":%q,"plugin":"github.com/acme/push","edit":"%s: replaced; restore your own launch screen"}]}`, lockKey(op), launch)
			writeTree(t, dir, map[string]string{launch: "<splash/>", LockFile: old})

			dropped, err := buildEjected(t, dir, platform, []protocol.Op{op})
			if err != nil || len(dropped) != 0 {
				t.Fatalf("dropped=%v err=%v", dropped, err)
			}
			lock, err := ReadLock(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !lock.wrote(launch, []byte("<splash/>")) {
				t.Errorf("lock does not record the launch screen: %+v", lock.Ops)
			}
		})
	}
}

// A managed build starts from the template's launch screen, which the
// plugin replaces.
func TestManagedLaunchScreenReplacesTheTemplates(t *testing.T) {
	settings := scaffold.Settings{AppName: "App", AppID: "com.example.app", Bundle: "com.example.app", Orientation: "portrait", ProjectRoot: t.TempDir()}
	write := map[string]func(root string, s scaffold.Settings) error{"ios": scaffold.WriteIOS, "xtool": scaffold.WriteXtool}
	for platform, launch := range launchScreenPaths {
		t.Run(platform, func(t *testing.T) {
			root := t.TempDir()
			if err := write[platform](root, settings); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, platform)
			if _, err := apply(t, []protocol.Op{launchScreenOp("<splash/>")}, dir, platform, nil); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, filepath.Join(dir, launch)); got != "<splash/>" {
				t.Errorf("launch screen = %q, want the plugin's", got)
			}
		})
	}
}
