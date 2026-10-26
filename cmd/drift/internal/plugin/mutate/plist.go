// Package mutate contains the file mutators driven by plugin ops. Each
// mutator reads the current file through a Changeset, computes the
// would-be-new file, and plans it there; nothing reaches disk until the
// build commits the changeset, and then only files whose bytes changed.
// This preserves comments and non-plugin-managed content, avoids spurious
// touches on ejected builds, and lets a failing op abort a build before
// it writes anything.
package mutate

import (
	"fmt"
	"path/filepath"
	"slices"

	"howett.net/plist"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ApplyPlist plans plist ops' edits to the plist at path into cs. The file must
// exist already (scaffold writes Info.plist and Runner.entitlements before
// plugin ops run); every op must target that one file. Setters run before
// appends, so an array a plugin sets is the base others append to; appends
// de-duplicate. Returns changed=true iff the file's bytes change.
func ApplyPlist(cs *Changeset, path string, ops []protocol.PlistOp) (bool, error) {
	name := filepath.Base(path)
	data, err := cs.ReadFile(path)
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
		case *protocol.OpPlistSet:
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
	return cs.WriteFile(path, out)
}
