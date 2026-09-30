package plugin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
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

// Validate normalises an op list per the conflict policy and returns the
// deduped, merged result. Errors are *ConflictError on collision.
func Validate(ops []protocol.Op) ([]protocol.Op, error) {
	type bucket struct {
		op        protocol.Op
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
		case protocol.ClassIdempotent:
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
		case protocol.ClassAdditive:
			// Same identity covers full payload; collapse silently.
			if !b.pluginSet[pkg] {
				b.plugins = append(b.plugins, pkg)
				b.pluginSet[pkg] = true
			}
		case protocol.ClassExclusive:
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

	out := make([]protocol.Op, 0, len(order))
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
