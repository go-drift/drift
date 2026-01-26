//go:build android || darwin || ios

package semantics

import "github.com/go-drift/drift/pkg/rendering"

// SemanticsUpdate represents changes to the semantics tree that need to be sent to the platform.
type SemanticsUpdate struct {
	// Updates contains nodes that were added or modified.
	Updates []SemanticsNodeUpdate

	// Removals contains IDs of nodes that were removed.
	Removals []int64
}

// IsEmpty reports whether the update contains no changes.
func (u SemanticsUpdate) IsEmpty() bool {
	return len(u.Updates) == 0 && len(u.Removals) == 0
}

// SemanticsNodeUpdate represents a single node's state for platform communication.
type SemanticsNodeUpdate struct {
	// ID uniquely identifies this node.
	ID int64

	// Rect is the bounding rectangle in global coordinates.
	Rect rendering.Rect

	// Label is the primary text description.
	Label string

	// Value is the current value.
	Value string

	// Hint provides action guidance.
	Hint string

	// Role defines the semantic role.
	Role SemanticsRole

	// Flags contains boolean state flags.
	Flags SemanticsFlag

	// Actions contains supported actions bitmask.
	Actions SemanticsAction

	// ChildIDs are the IDs of child nodes.
	ChildIDs []int64

	// CurrentValue for slider-type controls.
	CurrentValue *float64

	// MinValue for slider-type controls.
	MinValue *float64

	// MaxValue for slider-type controls.
	MaxValue *float64

	// ScrollPosition for scrollable containers.
	ScrollPosition *float64

	// ScrollExtentMin is the minimum scroll extent.
	ScrollExtentMin *float64

	// ScrollExtentMax is the maximum scroll extent.
	ScrollExtentMax *float64

	// HeadingLevel indicates heading level (1-6, 0 for none).
	HeadingLevel int

	// CustomActions is a list of custom accessibility actions.
	CustomActions []CustomSemanticsAction
}

// ToMap converts the update to a map for JSON serialization.
func (u SemanticsNodeUpdate) ToMap() map[string]any {
	m := map[string]any{
		"id":      u.ID,
		"left":    u.Rect.Left,
		"top":     u.Rect.Top,
		"right":   u.Rect.Right,
		"bottom":  u.Rect.Bottom,
		"role":    u.Role.String(),
		"flags":   uint64(u.Flags),
		"actions": uint64(u.Actions),
	}

	if u.Label != "" {
		m["label"] = u.Label
	}
	if u.Value != "" {
		m["value"] = u.Value
	}
	if u.Hint != "" {
		m["hint"] = u.Hint
	}
	if len(u.ChildIDs) > 0 {
		m["childIds"] = u.ChildIDs
	}
	if u.CurrentValue != nil {
		m["currentValue"] = *u.CurrentValue
	}
	if u.MinValue != nil {
		m["minValue"] = *u.MinValue
	}
	if u.MaxValue != nil {
		m["maxValue"] = *u.MaxValue
	}
	if u.ScrollPosition != nil {
		m["scrollPosition"] = *u.ScrollPosition
	}
	if u.ScrollExtentMin != nil {
		m["scrollExtentMin"] = *u.ScrollExtentMin
	}
	if u.ScrollExtentMax != nil {
		m["scrollExtentMax"] = *u.ScrollExtentMax
	}
	if u.HeadingLevel > 0 {
		m["headingLevel"] = u.HeadingLevel
	}
	if len(u.CustomActions) > 0 {
		actions := make([]map[string]any, len(u.CustomActions))
		for i, a := range u.CustomActions {
			actions[i] = map[string]any{
				"id":    a.ID,
				"label": a.Label,
			}
		}
		m["customActions"] = actions
	}

	return m
}

// ComputeDiff computes the difference between old and new semantics trees.
// Returns an update containing modified nodes and removed node IDs.
func ComputeDiff(oldRoot, newRoot *SemanticsNode) SemanticsUpdate {
	update := SemanticsUpdate{}

	// Build maps of old and new nodes by ID
	oldNodes := make(map[int64]*SemanticsNode)
	newNodes := make(map[int64]*SemanticsNode)

	if oldRoot != nil {
		oldRoot.Visit(func(node *SemanticsNode) bool {
			oldNodes[node.ID] = node
			return true
		})
	}

	if newRoot != nil {
		newRoot.Visit(func(node *SemanticsNode) bool {
			newNodes[node.ID] = node
			return true
		})
	}

	// Find removed nodes (in old but not in new)
	for id := range oldNodes {
		if _, exists := newNodes[id]; !exists {
			update.Removals = append(update.Removals, id)
		}
	}

	// Find added or modified nodes
	for id, newNode := range newNodes {
		oldNode, existed := oldNodes[id]
		if !existed || nodesChanged(oldNode, newNode) {
			update.Updates = append(update.Updates, nodeToUpdate(newNode))
		}
	}

	return update
}

// nodesChanged reports whether two nodes have different content.
func nodesChanged(old, new *SemanticsNode) bool {
	if old == nil || new == nil {
		return old != new
	}

	// Check rect
	if old.Rect != new.Rect {
		return true
	}

	// Check properties
	op := old.Config.Properties
	np := new.Config.Properties
	if op.Label != np.Label ||
		op.Value != np.Value ||
		op.Hint != np.Hint ||
		op.Role != np.Role ||
		op.Flags != np.Flags ||
		op.HeadingLevel != np.HeadingLevel {
		return true
	}

	// Check optional float values
	if !floatPtrEqual(op.CurrentValue, np.CurrentValue) ||
		!floatPtrEqual(op.MinValue, np.MinValue) ||
		!floatPtrEqual(op.MaxValue, np.MaxValue) ||
		!floatPtrEqual(op.ScrollPosition, np.ScrollPosition) ||
		!floatPtrEqual(op.ScrollExtentMin, np.ScrollExtentMin) ||
		!floatPtrEqual(op.ScrollExtentMax, np.ScrollExtentMax) {
		return true
	}

	// Check actions
	var oldActions, newActions SemanticsAction
	if old.Config.Actions != nil {
		oldActions = old.Config.Actions.SupportedActions()
	}
	if new.Config.Actions != nil {
		newActions = new.Config.Actions.SupportedActions()
	}
	if oldActions != newActions {
		return true
	}

	// Check children count and IDs
	if len(old.Children) != len(new.Children) {
		return true
	}
	for i, oldChild := range old.Children {
		if oldChild.ID != new.Children[i].ID {
			return true
		}
	}

	return false
}

// floatPtrEqual compares two optional float64 pointers for equality.
func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// BuildSemanticsTree constructs a semantics tree from a root configuration.
// This is a helper for testing and debugging.
func BuildSemanticsTree(config SemanticsConfiguration, rect rendering.Rect, children ...*SemanticsNode) *SemanticsNode {
	node := NewSemanticsNode()
	node.Rect = rect
	node.Config = config
	for _, child := range children {
		node.AddChild(child)
	}
	return node
}

// FlattenTree returns all nodes in the tree as a flat slice.
func FlattenTree(root *SemanticsNode) []*SemanticsNode {
	if root == nil {
		return nil
	}
	var nodes []*SemanticsNode
	root.Visit(func(node *SemanticsNode) bool {
		nodes = append(nodes, node)
		return true
	})
	return nodes
}
