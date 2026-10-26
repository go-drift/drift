package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// LockFile is where an ejected platform project records the plugin output
// Drift applied, relative to the project (platform/<platform>/). Managed
// builds regenerate the whole project instead and need no record. Commit it
// with the project: it is how Drift knows what a plugin left behind once
// it is removed from drift.yaml.
const LockFile = ".drift/plugins.lock.json"

// Lock is the plugin output an ejected project recorded at its last build.
// A nil *Lock is empty: a managed build, which regenerates its project, or
// an ejected project's first build with plugins.
type Lock struct {
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
	// Line is the exact line a versioned Gradle op wrote (a dependency, a
	// plugin declaration), so a later version replaces it in place.
	Line string `json:"line,omitempty"`
}

type lockedFile struct {
	Path   string `json:"path"` // slash-separated, relative to the project
	SHA256 string `json:"sha256"`
}

// SyncEjectedLock plans into cs the reconciliation of an ejected project
// with the ops just planned for it, using prev, the lock from the previous
// build (ReadLock), then plans ops' record as the new lock. It is the
// build's last planning step, so the commit writes the lock after the
// output it records. What the project holds
// decides, not which op made it, so an op renamed or retargeted by a CLI
// upgrade is neither a removal nor an addition:
//   - a file prev owned that no current op owns is deleted, unless it
//     changed since Drift wrote it, in which case it stays and a warning
//     names it;
//   - an edit prev made inside a shared file (Info.plist, the manifest,
//     build.gradle) that no current op makes, under the same key or as the
//     same edit, cannot be undone safely, so it is returned in dropped,
//     sorted, as "<edit> (from <plugin>)", for the caller to warn about
//     once cs is committed. It is not an error: a build's outcome must not
//     depend on whether the previous build's lock was committed.
//
// Returns the paths the commit deletes and the dropped edits. err is a
// failure to plan, after which cs must not be committed.
func SyncEjectedLock(cs *mutate.Changeset, projectDir, platform string, ops []protocol.Op, prev *Lock) (deleted, dropped []string, err error) {
	cur, err := lockFor(ops, projectDir, platform)
	if err != nil {
		return nil, nil, err
	}

	curKeys := make(map[string]bool, len(cur.Ops))
	curEdits := make(map[string]bool)
	curFiles := make(map[string]bool)
	for _, o := range cur.Ops {
		curKeys[o.Key] = true
		curEdits[o.Edit] = true
		for _, f := range o.Files {
			curFiles[f.Path] = true
		}
	}
	for _, o := range prev.ops() {
		for _, f := range o.Files {
			if curFiles[f.Path] {
				continue
			}
			path := filepath.Join(projectDir, filepath.FromSlash(f.Path))
			gone, err := deleteIfUnchanged(cs, path, f.SHA256)
			if err != nil {
				return nil, nil, err
			}
			if gone {
				deleted = append(deleted, path)
				removeEmptyParents(cs, filepath.Dir(path), projectDir)
			} else if _, statErr := os.Stat(path); statErr == nil {
				fmt.Fprintf(os.Stderr, "drift: kept %s from plugin %s: it changed since Drift wrote it\n", f.Path, o.Plugin)
			}
		}
		if o.Edit != "" && !curKeys[o.Key] && !curEdits[o.Edit] {
			dropped = append(dropped, fmt.Sprintf("%s (from %s)", o.Edit, o.Plugin))
		}
	}

	data, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	if _, err := cs.WriteFile(lockPath(projectDir), append(data, '\n')); err != nil {
		return nil, nil, err
	}

	sort.Strings(dropped)
	return deleted, dropped, nil
}

func lockPath(projectDir string) string {
	return filepath.Join(projectDir, filepath.FromSlash(LockFile))
}

// ReadLock reads the lock an ejected project at projectDir recorded at its
// last build; a project without one has an empty lock. Every path in it
// must stay inside the project, since Drift deletes files it names.
func ReadLock(projectDir string) (*Lock, error) {
	path := lockPath(projectDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Lock{Version: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse %s: %w\nResolve any merge conflict in it, or delete it: Drift then treats plugin files as new", path, err)
	}
	if l.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported version %d", path, l.Version)
	}
	for _, o := range l.Ops {
		for _, f := range o.Files {
			if !filepath.IsLocal(filepath.FromSlash(f.Path)) || pathpkg.Clean(f.Path) != f.Path {
				return nil, fmt.Errorf("%s: path %q is not inside the project", path, f.Path)
			}
		}
	}
	return &l, nil
}

// ops returns the recorded ops; none for a nil lock.
func (l *Lock) ops() []lockedOp {
	if l == nil {
		return nil
	}
	return l.Ops
}

// wrote reports whether the lock records content as what Drift wrote at
// path (slash-separated, relative to the project).
func (l *Lock) wrote(path string, content []byte) bool {
	sum := sha256Hex(content)
	for _, o := range l.ops() {
		for _, f := range o.Files {
			if f.Path == path && f.SHA256 == sum {
				return true
			}
		}
	}
	return false
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func lockFor(ops []protocol.Op, projectDir, platform string) (Lock, error) {
	l := Lock{Version: 1, Ops: []lockedOp{}}
	for _, op := range ops {
		if !opAppliesTo(op, platform) {
			continue
		}
		files, err := OwnedFiles(op, projectDir, platform)
		if err != nil {
			return l, err
		}
		entry := lockedOp{Key: lockKey(op), Plugin: op.PluginPackage(), Edit: describeEdit(op, platform), Line: gradleLine(op)}
		for _, f := range files {
			rel, err := filepath.Rel(projectDir, f.Path)
			if err != nil {
				return l, err
			}
			entry.Files = append(entry.Files, lockedFile{Path: filepath.ToSlash(rel), SHA256: sha256Hex(f.Content)})
		}
		l.Ops = append(l.Ops, entry)
	}
	return l, nil
}

func lockKey(op protocol.Op) string {
	if dep, ok := op.(*protocol.OpAndroidGradleAddDependency); ok {
		// Its target member is the full coordinate, so that plugins
		// pinning different versions both keep theirs; across builds the
		// version is content, or an upgrade would read as a removal.
		return dep.Type() + " gradle-dep:" + dep.Configuration + " [" + dep.Artifact() + "]"
	}
	parts := []string{op.Type()}
	for _, t := range op.Targets() {
		parts = append(parts, t.String())
	}
	return strings.Join(parts, " ")
}

// gradleLine is the line a versioned Gradle op writes, or "".
func gradleLine(op protocol.Op) string {
	switch v := op.(type) {
	case *protocol.OpAndroidGradleAddDependency:
		return mutate.GradleDependencyLine(v)
	case *protocol.OpAndroidGradleApplyPlugin:
		if v.Version != "" {
			return mutate.GradlePluginDeclaration(v)
		}
	}
	return ""
}

// gradleReplacements maps the line each versioned Gradle op in ops writes
// to the different line the lock records for the same op (same plugin and
// key): the line Drift wrote at the previous version, to update in place.
// Two plugins pinning one artifact share a key but each keep their line.
func (l *Lock) gradleReplacements(ops []protocol.Op) map[string]string {
	type id struct{ plugin, key string }
	prev := make(map[id]string)
	for _, o := range l.ops() {
		if o.Line != "" {
			prev[id{o.Plugin, o.Key}] = o.Line
		}
	}
	replace := make(map[string]string)
	for _, op := range ops {
		line := gradleLine(op)
		if old, ok := prev[id{op.PluginPackage(), lockKey(op)}]; ok && line != "" && old != line {
			replace[line] = old
		}
	}
	return replace
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
		if v.MaxSDK > 0 {
			return fmt.Sprintf("%s: <uses-permission android:name=%q android:maxSdkVersion=\"%d\">", manifest, v.Name, v.MaxSDK)
		}
		return fmt.Sprintf("%s: <uses-permission android:name=%q>", manifest, v.Name)
	case *protocol.OpAndroidManifestAddIntentFilter:
		return fmt.Sprintf("%s: %s on activity %s", manifest, strings.Join(strings.Fields(v.XML), " "), v.Activity)
	case *protocol.OpAndroidManifestSetActivityAttr:
		return fmt.Sprintf("%s: %s=%q on activity %s", manifest, v.Attr, v.Value, v.Activity)
	case *protocol.OpAndroidManifestAddService:
		return fmt.Sprintf("%s: <service android:name=%q>", manifest, v.ServiceName())
	case *protocol.OpAndroidManifestAddReceiver:
		return fmt.Sprintf("%s: <receiver android:name=%q>", manifest, v.ReceiverName())
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

// deleteIfUnchanged plans the removal of path if its content still hashes
// to sum. Reports whether it planned one.
func deleteIfUnchanged(cs *mutate.Changeset, path, sum string) (bool, error) {
	data, err := cs.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if sha256Hex(data) != sum {
		return false, nil
	}
	return cs.Remove(path)
}

// removeEmptyParents plans the removal of dir and its ancestors while they
// are empty, stopping at root.
func removeEmptyParents(cs *mutate.Changeset, dir, root string) {
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		cs.RemoveIfEmpty(dir)
		dir = filepath.Dir(dir)
	}
}
