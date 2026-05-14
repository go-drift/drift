package plugin

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// ConflictError is returned when two ops with the same identity can't be
// merged. The plugin packages that contributed each op are listed.
type ConflictError struct {
	Identity string
	OpType   string
	Plugins  []string
}

func (e *ConflictError) Error() string {
	pluginList := strings.Join(e.Plugins, " and ")
	return fmt.Sprintf("plugin conflict: %s (op %s, key %s)", pluginList, e.OpType, e.Identity)
}

// DecodeOps converts a slice of raw JSON ops into typed Ops, mirroring the
// boundary-parser convention from pkg/platform/stream.go.
func DecodeOps(raws []json.RawMessage) ([]driftplugin.Op, error) {
	out := make([]driftplugin.Op, 0, len(raws))
	for i, raw := range raws {
		op, err := driftplugin.UnmarshalOp(raw)
		if err != nil {
			return nil, fmt.Errorf("op %d: %w", i, err)
		}
		out = append(out, op)
	}
	return out, nil
}

// Validate normalises an op list per the conflict policy and returns the
// deduped, merged result. Errors are *ConflictError on collision.
func Validate(ops []driftplugin.Op) ([]driftplugin.Op, error) {
	type bucket struct {
		op        driftplugin.Op
		hash      string
		plugins   []string // ordered
		pluginSet map[string]bool
	}

	buckets := make(map[string]*bucket)
	order := make([]string, 0, len(ops)) // identity insertion order

	for _, op := range ops {
		id := op.Identity()
		hash := op.ContentHash()
		pkg := op.PluginPackage()

		b, exists := buckets[id]
		if !exists {
			b = &bucket{op: op, hash: hash, pluginSet: map[string]bool{pkg: true}, plugins: []string{pkg}}
			buckets[id] = b
			order = append(order, id)
			continue
		}

		switch op.MergeClass() {
		case driftplugin.ClassIdempotent:
			if b.hash != hash {
				return nil, &ConflictError{
					Identity: id,
					OpType:   op.Type(),
					Plugins:  uniquePlugins(append(append([]string{}, b.plugins...), pkg)),
				}
			}
			// Same payload, collapse.
			if !b.pluginSet[pkg] {
				b.plugins = append(b.plugins, pkg)
				b.pluginSet[pkg] = true
			}
		case driftplugin.ClassAdditive:
			// Same identity covers full payload; collapse silently.
			if !b.pluginSet[pkg] {
				b.plugins = append(b.plugins, pkg)
				b.pluginSet[pkg] = true
			}
		case driftplugin.ClassExclusive:
			if b.hash != hash {
				return nil, &ConflictError{
					Identity: id,
					OpType:   op.Type(),
					Plugins:  uniquePlugins(append(append([]string{}, b.plugins...), pkg)),
				}
			}
			if !b.pluginSet[pkg] {
				b.plugins = append(b.plugins, pkg)
				b.pluginSet[pkg] = true
			}
		}
	}

	out := make([]driftplugin.Op, 0, len(order))
	for _, id := range order {
		out = append(out, buckets[id].op)
	}
	return out, nil
}

func uniquePlugins(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
