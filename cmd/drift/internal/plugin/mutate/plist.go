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
	"slices"

	"howett.net/plist"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ApplyPlist applies plist ops to the plist at path in-place. The file must
// exist already (scaffold writes Info.plist and Runner.entitlements before
// plugin ops run); every op must target that one file. Setters run before
// appends, so an array a plugin sets is the base others append to; appends
// de-duplicate. Returns changed=true iff the file was rewritten.
func ApplyPlist(path string, ops []protocol.PlistOp) (bool, error) {
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", name, err)
	}

	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}
	if root == nil {
		root = map[string]any{}
	}

	var appends []*protocol.OpPlistAppendArrayItem
	for _, op := range ops {
		switch v := op.(type) {
		case *protocol.OpPlistSetString:
			root[v.Key] = v.Value
		case *protocol.OpPlistSetBool:
			root[v.Key] = v.Value
		case *protocol.OpPlistSetStringArray:
			arr := make([]any, len(v.Values))
			for i, s := range v.Values {
				arr[i] = s
			}
			root[v.Key] = arr
		case *protocol.OpPlistSetDict:
			root[v.Key] = v.Value
		case *protocol.OpPlistAppendArrayItem:
			appends = append(appends, v)
		default:
			return false, fmt.Errorf("%s: unknown plist op %T", name, op)
		}
	}
	for _, op := range appends {
		existing, ok := root[op.Key].([]any)
		if !ok && root[op.Key] != nil {
			return false, fmt.Errorf("%s: plugin %s appends %q to %s, which is a %T, not an array",
				name, op.PluginPackage(), op.Value, op.Key, root[op.Key])
		}
		if !slices.Contains(existing, any(op.Value)) {
			root[op.Key] = append(existing, op.Value)
		}
	}

	out, err := plist.MarshalIndent(root, plist.XMLFormat, "\t")
	if err != nil {
		return false, fmt.Errorf("marshal %s: %w", name, err)
	}
	if bytes.Equal(out, data) {
		return false, nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", name, err)
	}
	return true, nil
}

// EnsureDir is a small helper used by other mutators when writing into
// directories that may not exist (asset catalogs, etc.).
func EnsureDir(p string) error {
	return os.MkdirAll(filepath.Dir(p), 0o755)
}
