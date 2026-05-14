// Package mutate contains the file mutators driven by plugin ops. Each
// mutator reads the current file, computes the would-be-new file, and only
// writes if the bytes actually changed; this preserves comments,
// non-plugin-managed content, and avoids spurious touches on ejected builds.
package mutate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"howett.net/plist"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// ApplyInfoPlist applies Info.plist ops in-place. The file must exist already
// (scaffold writes the template Info.plist before plugin ops run). Returns
// changed=true iff the file was rewritten.
func ApplyInfoPlist(
	path string,
	setStrings []*driftplugin.OpInfoPlistSetString,
	setBools []*driftplugin.OpInfoPlistSetBool,
	setArrays []*driftplugin.OpInfoPlistSetStringArray,
	appendItems []*driftplugin.OpInfoPlistAppendArrayItem,
	setDicts []*driftplugin.OpInfoPlistSetDict,
) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read Info.plist: %w", err)
	}

	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse Info.plist: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}

	for _, op := range setStrings {
		root[op.Key] = op.Value
	}
	for _, op := range setBools {
		root[op.Key] = op.Value
	}
	for _, op := range setArrays {
		arr := make([]any, len(op.Values))
		for i, v := range op.Values {
			arr[i] = v
		}
		root[op.Key] = arr
	}
	for _, op := range setDicts {
		root[op.Key] = op.Value
	}
	// Append items after set arrays so the user's static array is the base
	// and appended items extend it. De-dup on append.
	for _, op := range appendItems {
		existing, _ := root[op.Key].([]any)
		seen := make(map[string]bool, len(existing))
		for _, e := range existing {
			if s, ok := e.(string); ok {
				seen[s] = true
			}
		}
		if !seen[op.Value] {
			existing = append(existing, op.Value)
			root[op.Key] = existing
		}
	}

	out, err := plist.MarshalIndent(root, plist.XMLFormat, "\t")
	if err != nil {
		return false, fmt.Errorf("marshal Info.plist: %w", err)
	}
	if bytes.Equal(out, data) {
		return false, nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write Info.plist: %w", err)
	}
	return true, nil
}

// EnsureDir is a small helper used by other mutators when writing into
// directories that may not exist (asset catalogs, etc.).
func EnsureDir(p string) error {
	return os.MkdirAll(filepath.Dir(p), 0o755)
}
