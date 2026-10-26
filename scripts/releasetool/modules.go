package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// releasedDir reports whether the module in dir (slash-separated, relative
// to the repository root, "." for the root) is released: the root module
// and plugins/<name>. Examples and the showcase are not.
func releasedDir(dir string) bool {
	return dir == "." || path.Dir(dir) == "plugins"
}

// repoModule is a module in the repository.
type repoModule struct {
	Dir      string // slash-separated, relative to the repository root
	Path     string // module path
	Released bool
	GoMod    *modfile.File
	Data     []byte // go.mod as read
}

// listModules returns the modules whose go.mod git tracks at rev, or in the
// working tree when rev is empty, sorted by directory.
func listModules(root, rev string) ([]repoModule, error) {
	var out []byte
	var err error
	if rev == "" {
		out, err = gitOutput(root, "ls-files", "--", "go.mod", "*/go.mod")
	} else {
		out, err = gitOutput(root, "ls-tree", "-r", "--name-only", rev, "--", ".")
	}
	if err != nil {
		return nil, err
	}
	var mods []repoModule
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f != "go.mod" && !strings.HasSuffix(f, "/go.mod") {
			continue
		}
		var data []byte
		if rev == "" {
			data, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		} else {
			data, err = gitOutput(root, "show", rev+":"+f)
		}
		if err != nil {
			return nil, err
		}
		gm, err := modfile.Parse(f, data, nil)
		if err != nil {
			return nil, err
		}
		if gm.Module == nil {
			return nil, fmt.Errorf("%s: no module directive", f)
		}
		dir := path.Dir(f)
		mods = append(mods, repoModule{Dir: dir, Path: gm.Module.Mod.Path, Released: releasedDir(dir), GoMod: gm, Data: data})
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Dir < mods[j].Dir })
	return mods, nil
}

// releasedPaths returns the set of released module paths.
func releasedPaths(mods []repoModule) map[string]bool {
	set := map[string]bool{}
	for _, m := range mods {
		if m.Released {
			set[m.Path] = true
		}
	}
	return set
}

// mismatches lists every requirement on a released module that is not at
// version, as "<dir>/go.mod: <path> <version>".
func mismatches(mods []repoModule, version string) []string {
	released := releasedPaths(mods)
	var bad []string
	for _, m := range mods {
		for _, r := range m.GoMod.Require {
			if released[r.Mod.Path] && r.Mod.Version != version {
				bad = append(bad, fmt.Sprintf("%s: %s %s", path.Join(m.Dir, "go.mod"), r.Mod.Path, r.Mod.Version))
			}
		}
	}
	return bad
}

// bump requires every released module at version in every module, then
// runs go mod tidy in each module it changed.
func bump(root string, mods []repoModule, version string) error {
	released := releasedPaths(mods)
	for _, m := range mods {
		changed, err := requireReleased(m.GoMod, released, version)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		data, err := m.GoMod.Format()
		if err != nil {
			return err
		}
		dir := filepath.Join(root, filepath.FromSlash(m.Dir))
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), data, 0o644); err != nil {
			return err
		}
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = dir
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: go mod tidy: %w", m.Dir, err)
		}
		fmt.Printf("%s/go.mod: released modules at %s\n", m.Dir, version)
	}
	return nil
}

// requireReleased sets f's requirements on released modules to version and
// reports whether any changed.
func requireReleased(f *modfile.File, released map[string]bool, version string) (bool, error) {
	changed := false
	for _, r := range f.Require {
		if released[r.Mod.Path] && r.Mod.Version != version {
			if err := f.AddRequire(r.Mod.Path, version); err != nil {
				return false, err
			}
			changed = true
		}
	}
	return changed, nil
}

// tags returns the tags a release of version creates: version for the root
// module and <dir>/<version> for each other released module.
func tags(mods []repoModule, version string) []string {
	var out []string
	for _, m := range mods {
		if !m.Released {
			continue
		}
		if m.Dir == "." {
			out = append(out, version)
		} else {
			out = append(out, m.Dir+"/"+version)
		}
	}
	return out
}

func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
