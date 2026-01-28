//go:build !android && !darwin && !ios

// Package semantics provides accessibility semantics support for Drift.
// This stub implementation provides type definitions for non-supported platforms.
package semantics

import "github.com/go-drift/drift/pkg/graphics"

// SemanticsConfiguration describes semantic properties and actions for a render object.
type SemanticsConfiguration struct {
	// IsSemanticBoundary indicates this node creates a separate semantic node
	// rather than merging with its ancestors.
	IsSemanticBoundary bool

	// IsMergingSemanticsOfDescendants indicates this node merges the semantics
	// of its descendants into itself.
	IsMergingSemanticsOfDescendants bool

	// ExplicitChildNodes indicates whether child nodes should be explicitly
	// added rather than inferred from the render tree.
	ExplicitChildNodes bool

	// IsBlockingUserActions indicates the node blocks user actions (e.g., modal overlay).
	IsBlockingUserActions bool

	// Properties contains semantic property values.
	Properties SemanticsProperties

	// Actions contains action handlers.
	Actions *SemanticsActions
}

// IsEmpty reports whether the configuration contains any semantic information.
func (c SemanticsConfiguration) IsEmpty() bool {
	return !c.IsSemanticBoundary &&
		!c.IsMergingSemanticsOfDescendants &&
		!c.ExplicitChildNodes &&
		!c.IsBlockingUserActions &&
		c.Properties.IsEmpty() &&
		(c.Actions == nil || c.Actions.IsEmpty())
}

// EnsureFocusable marks the configuration as focusable when it has meaningful content.
func (c *SemanticsConfiguration) EnsureFocusable() {
	if c == nil {
		return
	}
	if c.Properties.Flags.Has(SemanticsIsHidden) {
		return
	}
	if c.Properties.Flags.Has(SemanticsIsFocusable) {
		return
	}
	if !c.Properties.IsEmpty() || (c.Actions != nil && !c.Actions.IsEmpty()) {
		c.Properties.Flags = c.Properties.Flags.Set(SemanticsIsFocusable)
	}
}

// Merge combines another configuration into this one.
func (c *SemanticsConfiguration) Merge(other SemanticsConfiguration) {
	c.IsSemanticBoundary = c.IsSemanticBoundary || other.IsSemanticBoundary
	c.IsMergingSemanticsOfDescendants = c.IsMergingSemanticsOfDescendants || other.IsMergingSemanticsOfDescendants
	c.ExplicitChildNodes = c.ExplicitChildNodes || other.ExplicitChildNodes
	c.IsBlockingUserActions = c.IsBlockingUserActions || other.IsBlockingUserActions
	c.Properties = c.Properties.Merge(other.Properties)
	if other.Actions != nil {
		if c.Actions == nil {
			c.Actions = NewSemanticsActions()
		}
		c.Actions.Merge(other.Actions)
	}
}

// SemanticsNode represents a node in the semantics tree.
type SemanticsNode struct {
	// ID uniquely identifies this node.
	ID int64

	// Rect is the bounding rectangle in global coordinates.
	Rect graphics.Rect

	// Config contains the semantic configuration.
	Config SemanticsConfiguration

	// Parent is the parent node, or nil for root.
	Parent *SemanticsNode

	// Children are the child nodes.
	Children []*SemanticsNode

	// dirty indicates the node needs to be sent to the platform.
	dirty bool
}

// NewSemanticsNode creates a new semantics node with a unique ID.
func NewSemanticsNode() *SemanticsNode {
	return &SemanticsNode{
		ID:    0,
		dirty: true,
	}
}

// NewSemanticsNodeWithID creates a new semantics node with a specific ID.
func NewSemanticsNodeWithID(id int64) *SemanticsNode {
	return &SemanticsNode{
		ID:    id,
		dirty: true,
	}
}

// SemanticsOwner manages the semantics tree and tracks dirty nodes.
type SemanticsOwner struct {
	Root       *SemanticsNode
	dirtyNodes map[*SemanticsNode]struct{}
	nodesByID  map[int64]*SemanticsNode
}

// NewSemanticsOwner creates a new semantics owner.
func NewSemanticsOwner() *SemanticsOwner {
	return &SemanticsOwner{
		dirtyNodes: make(map[*SemanticsNode]struct{}),
		nodesByID:  make(map[int64]*SemanticsNode),
	}
}

// GetDirtyNodes returns and clears all dirty nodes.
func (o *SemanticsOwner) GetDirtyNodes() []*SemanticsNode {
	return nil
}

// BuildSemanticsTree creates a semantics tree node with children.
func BuildSemanticsTree(config SemanticsConfiguration, rect graphics.Rect, children ...*SemanticsNode) *SemanticsNode {
	return &SemanticsNode{
		Config:   config,
		Rect:     rect,
		Children: children,
	}
}

// SemanticsUpdate represents an update to send to the platform.
type SemanticsUpdate struct {
	Nodes []*SemanticsNode
}

// IsEmpty returns true if the update has no changes.
func (u SemanticsUpdate) IsEmpty() bool {
	return len(u.Nodes) == 0
}

// SemanticsBinding connects the semantics system to platform accessibility services.
type SemanticsBinding struct {
	owner   *SemanticsOwner
	enabled bool
}

var binding = &SemanticsBinding{}

// GetSemanticsBinding returns the global semantics binding.
func GetSemanticsBinding() *SemanticsBinding {
	return binding
}

// SetOwner sets the semantics owner for this binding.
func (b *SemanticsBinding) SetOwner(owner *SemanticsOwner) {
	b.owner = owner
}

// Owner returns the current semantics owner.
func (b *SemanticsBinding) Owner() *SemanticsOwner {
	return b.owner
}

// SetEnabled enables or disables accessibility.
func (b *SemanticsBinding) SetEnabled(enabled bool) {
	b.enabled = enabled
}

// IsEnabled reports whether accessibility is enabled.
func (b *SemanticsBinding) IsEnabled() bool {
	return b.enabled
}

// SetSendFunction sets the function used to send updates to the platform.
func (b *SemanticsBinding) SetSendFunction(fn func(SemanticsUpdate) error) {}

// SetActionCallback sets the callback for handling actions from the platform.
func (b *SemanticsBinding) SetActionCallback(fn func(nodeID int64, action SemanticsAction, args any) bool) {
}

// HandleAction handles an action request from the platform.
func (b *SemanticsBinding) HandleAction(nodeID int64, action SemanticsAction, args any) bool {
	return false
}

// RequestFullUpdate requests a full semantics tree update.
func (b *SemanticsBinding) RequestFullUpdate() {}

// FlushSemantics sends any pending semantics updates.
func (b *SemanticsBinding) FlushSemantics() {}

// MarkNodeDirty marks a specific node as needing update.
func (b *SemanticsBinding) MarkNodeDirty(node *SemanticsNode) {}

// FindNodeByID finds a semantics node by its ID.
func (b *SemanticsBinding) FindNodeByID(id int64) *SemanticsNode {
	return nil
}
