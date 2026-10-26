package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/zip"
)

// writeProxy adds the module at the root of the git repository root, at
// rev, to the file GOPROXY in dir as version. Files in nested modules are
// left out, as in a real module zip.
func writeProxy(dir, root, rev, version string) error {
	timeOut, err := gitOutput(root, "show", "-s", "--format=%cI", rev)
	if err != nil {
		return err
	}
	info, err := json.Marshal(struct{ Version, Time string }{version, strings.TrimSpace(string(timeOut))})
	if err != nil {
		return err
	}
	gomod, err := gitOutput(root, "show", rev+":go.mod")
	if err != nil {
		return err
	}
	path := modfile.ModulePath(gomod)
	if path == "" {
		return fmt.Errorf("%s:go.mod at %s has no module directive", root, rev)
	}
	mv := module.Version{Path: path, Version: version}
	escPath, err := module.EscapePath(path)
	if err != nil {
		return err
	}
	escVersion, err := module.EscapeVersion(version)
	if err != nil {
		return err
	}
	vdir := filepath.Join(dir, filepath.FromSlash(escPath), "@v")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		return err
	}
	var zbuf bytes.Buffer
	if err := zip.CreateFromVCS(&zbuf, mv, root, rev, ""); err != nil {
		return err
	}
	for ext, data := range map[string][]byte{".info": info, ".mod": gomod, ".zip": zbuf.Bytes()} {
		if err := os.WriteFile(filepath.Join(vdir, escVersion+ext), data, 0o644); err != nil {
			return err
		}
	}
	return addToList(filepath.Join(vdir, "list"), version)
}

// addToList adds version to a proxy's version list file.
func addToList(file, version string) error {
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	versions := []string{version}
	for _, v := range strings.Fields(string(data)) {
		if v != version {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return semver.Compare(versions[i], versions[j]) < 0 })
	return os.WriteFile(file, []byte(strings.Join(versions, "\n")+"\n"), 0o644)
}
