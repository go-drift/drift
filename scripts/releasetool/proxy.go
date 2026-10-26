package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/zip"
)

// writeProxy adds each released module at rev to the file GOPROXY in dir
// (the layout GOPROXY=file://<dir> reads), as version. Calls for further
// versions add to the same proxy.
func writeProxy(dir, root, rev string, mods []repoModule, version string) error {
	timeOut, err := gitOutput(root, "show", "-s", "--format=%cI", rev)
	if err != nil {
		return err
	}
	info, err := json.Marshal(struct{ Version, Time string }{version, strings.TrimSpace(string(timeOut))})
	if err != nil {
		return err
	}
	for _, m := range mods {
		if !m.Released {
			continue
		}
		mv := module.Version{Path: m.Path, Version: version}
		escPath, err := module.EscapePath(m.Path)
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
		subdir := m.Dir
		if subdir == "." {
			subdir = ""
		}
		var zbuf bytes.Buffer
		if err := zip.CreateFromVCS(&zbuf, mv, root, rev, subdir); err != nil {
			return err
		}
		for ext, data := range map[string][]byte{".info": info, ".mod": m.Data, ".zip": zbuf.Bytes()} {
			if err := os.WriteFile(filepath.Join(vdir, escVersion+ext), data, 0o644); err != nil {
				return err
			}
		}
		if err := addToList(filepath.Join(vdir, "list"), version); err != nil {
			return err
		}
	}
	return nil
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
