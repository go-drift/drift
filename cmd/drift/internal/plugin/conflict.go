package plugin

import (
	"fmt"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ConflictError reports two ops that write the same target incompatibly.
type ConflictError struct {
	// Target is the contested location, e.g. "plist:NSCameraUsageDescription".
	Target string
	// First and Second are the earlier and later op on the target.
	First, Second protocol.Op
	// Mixed is true when one op replaces the whole location while the
	// other adds an element to it; otherwise they write different values.
	Mixed bool
}

func (e *ConflictError) Error() string {
	how := "write different values"
	if e.Mixed {
		how = "disagree: one replaces the whole value, the other adds to it"
	}
	return fmt.Sprintf("plugin conflict on %s: %s (%s) and %s (%s) %s",
		e.Target, e.First.PluginPackage(), e.First.Type(), e.Second.PluginPackage(), e.Second.Type(), how)
}

// Validate checks every op's targets against those of the ops before it
// (see protocol.Target for the rule) and returns the ops in their original
// order minus exact duplicates: an op is dropped only when each of its
// targets was already written identically by an earlier op. Errors are
// *ConflictError.
func Validate(ops []protocol.Op) ([]protocol.Op, error) {
	type claim struct {
		op      protocol.Op
		content string
	}
	type keyState struct {
		owner       *claim
		firstMember *claim
		members     map[string]*claim
	}
	keys := make(map[string]*keyState)

	out := make([]protocol.Op, 0, len(ops))
	for _, op := range ops {
		fresh := false
		for _, t := range op.Targets() {
			ks := keys[t.Key]
			if ks == nil {
				ks = &keyState{members: map[string]*claim{}}
				keys[t.Key] = ks
			}
			var prev *claim
			if t.Member == "" {
				if ks.firstMember != nil {
					return nil, &ConflictError{Target: t.String(), First: ks.firstMember.op, Second: op, Mixed: true}
				}
				prev = ks.owner
				if prev == nil {
					ks.owner = &claim{op: op, content: t.Content}
				}
			} else {
				if ks.owner != nil {
					return nil, &ConflictError{Target: t.String(), First: ks.owner.op, Second: op, Mixed: true}
				}
				prev = ks.members[t.Member]
				if prev == nil {
					c := &claim{op: op, content: t.Content}
					ks.members[t.Member] = c
					if ks.firstMember == nil {
						ks.firstMember = c
					}
				}
			}
			if prev == nil {
				fresh = true
				continue
			}
			if prev.content != t.Content {
				return nil, &ConflictError{Target: t.String(), First: prev.op, Second: op}
			}
		}
		if fresh {
			out = append(out, op)
		}
	}
	return out, nil
}
