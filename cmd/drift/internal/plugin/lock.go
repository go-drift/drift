package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// LockFile is where an ejected platform project records the plugin output
// Drift applied, relative to the project (platform/<platform>/). Managed
// builds regenerate the whole project instead and need no record. Commit it
// with the project: it is how Drift knows what a plugin left behind once
// it is removed from drift.yaml.
const LockFile = ".drift/plugins.lock.json"

type pluginLock struct {
	Version int        `json:"version"`
	Ops     []lockedOp `json:"ops"`
}

// lockedOp is one applied op: the files it owns, and the edit it made
// inside a shared file, if any.
type lockedOp struct {
	// Key identifies the op across builds: its type and targets, without
	// content, so a plugin changing what it writes is an update, not a
	// removal.
	Key    string       `json:"key"`
	Plugin string       `json:"plugin"`
	Files  []lockedFile `json:"files,omitempty"`
	Edit   string       `json:"edit,omitempty"`
}

type lockedFile struct {
	Path   string `json:"path"` // slash-separated, relative to the project
	SHA256 string `json:"sha256"`
}

// SyncEjectedLock reconciles an ejected project with the ops just applied
// to it, using the lock from the previous build, then records ops as the
// new lock. For each op that is gone:
//   - files it owned are deleted, unless they changed since Drift wrote
//     them, in which case they stay and a warning names them;
//   - an edit it made inside a shared file (Info.plist, the manifest,
//     build.gradle) cannot be undone safely, so it is listed in the
//     returned error. The new lock is written first, so the list is shown
//     once and the next build proceeds.
//
// Returns the deleted paths.
func SyncEjectedLock(projectDir, platform string, ops []protocol.Op) ([]string, error) {
	lockPath := filepath.Join(projectDir, filepath.FromSlash(LockFile))
	prev, err := readLock(lockPath)
	if err != nil {
		return nil, err
	}
	cur, err := lockFor(ops, projectDir, platform)
	if err != nil {
		return nil, err
	}

	current := make(map[string]bool, len(cur.Ops))
	for _, o := range cur.Ops {
		current[o.Key] = true
	}
	var deleted, edits []string
	for _, o := range prev.Ops {
		if current[o.Key] {
			continue
		}
		for _, f := range o.Files {
			path := filepath.Join(projectDir, filepath.FromSlash(f.Path))
			gone, err := deleteIfUnchanged(path, f.SHA256)
			if err != nil {
				return deleted, err
			}
			if gone {
				deleted = append(deleted, path)
				removeEmptyParents(filepath.Dir(path), projectDir)
			} else if _, statErr := os.Stat(path); statErr == nil {
				fmt.Fprintf(os.Stderr, "drift: kept %s from removed plugin %s: it changed since Drift wrote it\n", f.Path, o.Plugin)
			}
		}
		if o.Edit != "" {
			edits = append(edits, fmt.Sprintf("%s (from %s)", o.Edit, o.Plugin))
		}
	}

	data, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return deleted, err
	}
	if _, err := writeIfDifferentSingle(lockPath, append(data, '\n')); err != nil {
		return deleted, err
	}

	if len(edits) > 0 {
		sort.Strings(edits)
		return deleted, fmt.Errorf("plugins removed from drift.yaml left these edits in the ejected project at %s; "+
			"remove the ones you no longer need, then build again (this list is shown once):\n  - %s",
			projectDir, strings.Join(edits, "\n  - "))
	}
	return deleted, nil
}

func readLock(path string) (pluginLock, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return pluginLock{Version: 1}, nil
	}
	if err != nil {
		return pluginLock{}, err
	}
	var l pluginLock
	if err := json.Unmarshal(data, &l); err != nil {
		return pluginLock{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if l.Version != 1 {
		return pluginLock{}, fmt.Errorf("%s: unsupported version %d", path, l.Version)
	}
	return l, nil
}

func lockFor(ops []protocol.Op, projectDir, platform string) (pluginLock, error) {
	l := pluginLock{Version: 1, Ops: []lockedOp{}}
	for _, op := range ops {
		if !opAppliesTo(op, platform) {
			continue
		}
		files, err := OwnedFiles(op, projectDir, platform)
		if err != nil {
			return l, err
		}
		entry := lockedOp{Key: lockKey(op), Plugin: op.PluginPackage(), Edit: describeEdit(op, platform)}
		for _, f := range files {
			rel, err := filepath.Rel(projectDir, f.Path)
			if err != nil {
				return l, err
			}
			sum := sha256.Sum256(f.Content)
			entry.Files = append(entry.Files, lockedFile{Path: filepath.ToSlash(rel), SHA256: hex.EncodeToString(sum[:])})
		}
		l.Ops = append(l.Ops, entry)
	}
	return l, nil
}

func lockKey(op protocol.Op) string {
	parts := []string{op.Type()}
	for _, t := range op.Targets() {
		parts = append(parts, t.String())
	}
	return strings.Join(parts, " ")
}

// describeEdit names the edit op makes inside a shared file, or "" if it
// makes none (it owns whole files, or writes output Drift regenerates).
func describeEdit(op protocol.Op, platform string) string {
	launch := "Runner/LaunchScreen.storyboard"
	if platform == "xtool" {
		launch = "Sources/Runner/Resources/LaunchScreen.storyboard"
	}
	const manifest = "app/src/main/AndroidManifest.xml"
	switch v := op.(type) {
	case *protocol.OpPlistAppendArrayItem:
		return fmt.Sprintf("%s: %q in array %s", iosPlistRelPath(platform, v.File), v.Value, v.Key)
	case protocol.PlistOp:
		return fmt.Sprintf("%s: key %s", iosPlistRelPath(platform, v.PlistFile()), v.PlistKey())
	case *protocol.OpIOSReplaceLaunchScreen:
		return fmt.Sprintf("%s: replaced; restore your own launch screen", launch)
	case *protocol.OpAndroidManifestAddPermission:
		return fmt.Sprintf("%s: <uses-permission android:name=%q>", manifest, v.Name)
	case *protocol.OpAndroidManifestAddIntentFilter:
		return fmt.Sprintf("%s: an <intent-filter> on activity %s", manifest, v.Activity)
	case *protocol.OpAndroidManifestSetActivityAttr:
		return fmt.Sprintf("%s: %s=%q on activity %s", manifest, v.Attr, v.Value, v.Activity)
	case *protocol.OpAndroidManifestAddService:
		return fmt.Sprintf("%s: <service android:name=%q>", manifest, v.ServiceName())
	case *protocol.OpAndroidManifestAddMetaData:
		return fmt.Sprintf("%s: <meta-data android:name=%q> under %s", manifest, v.Name, v.Parent)
	case *protocol.OpAndroidGradleAddDependency:
		return fmt.Sprintf("app/build.gradle: %s %q", v.Configuration, v.Coord)
	case *protocol.OpAndroidGradleApplyPlugin:
		if v.Version != "" {
			return fmt.Sprintf("app/build.gradle and build.gradle: Gradle plugin %s", v.ID)
		}
		return fmt.Sprintf("app/build.gradle: Gradle plugin %s", v.ID)
	}
	return ""
}

// deleteIfUnchanged removes path if its content still hashes to sum.
// Reports whether it removed the file.
func deleteIfUnchanged(path, sum string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != sum {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

// removeEmptyParents removes dir and its ancestors while they are empty,
// stopping at root.
func removeEmptyParents(dir, root string) {
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
