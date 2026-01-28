//go:build android || darwin || ios

package semantics

import (
	"sync"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/graphics"
)

// nextNodeID is a global counter for generating unique node IDs.
var nextNodeID atomic.Int64

// generateNodeID returns a new unique node ID.
func generateNodeID() int64 {
	return nextNodeID.Add(1)
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
		ID:    generateNodeID(),
		dirty: true,
	}
}

// NewSemanticsNodeWithID creates a new semantics node with a specific ID.
// Use this when you need stable IDs across frames.
func NewSemanticsNodeWithID(id int64) *SemanticsNode {
	return &SemanticsNode{
		ID:    id,
		dirty: true,
	}
}

// AddChild adds a child node.
func (n *SemanticsNode) AddChild(child *SemanticsNode) {
	child.Parent = n
	n.Children = append(n.Children, child)
}

// RemoveChild removes a child node.
func (n *SemanticsNode) RemoveChild(child *SemanticsNode) {
	for i, c := range n.Children {
		if c == child {
			child.Parent = nil
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			return
		}
	}
}

// ClearChildren removes all children.
func (n *SemanticsNode) ClearChildren() {
	for _, child := range n.Children {
		child.Parent = nil
	}
	n.Children = nil
}

// MarkDirty marks this node as needing update.
func (n *SemanticsNode) MarkDirty() {
	n.dirty = true
}

// IsDirty reports whether the node needs update.
func (n *SemanticsNode) IsDirty() bool {
	return n.dirty
}

// ClearDirty marks the node as clean.
func (n *SemanticsNode) ClearDirty() {
	n.dirty = false
}

// HasFlag checks if the node has a specific flag set.
func (n *SemanticsNode) HasFlag(flag SemanticsFlag) bool {
	return n.Config.Properties.Flags.Has(flag)
}

// PerformAction performs an action on this node.
func (n *SemanticsNode) PerformAction(action SemanticsAction, args any) bool {
	if n.Config.Actions == nil {
		return false
	}
	return n.Config.Actions.PerformAction(action, args)
}

// FindNodeByID searches this subtree for a node with the given ID.
func (n *SemanticsNode) FindNodeByID(id int64) *SemanticsNode {
	if n.ID == id {
		return n
	}
	for _, child := range n.Children {
		if found := child.FindNodeByID(id); found != nil {
			return found
		}
	}
	return nil
}

// Visit traverses the subtree depth-first, calling fn for each node.
// Returns false if traversal was stopped early.
func (n *SemanticsNode) Visit(fn func(*SemanticsNode) bool) bool {
	if !fn(n) {
		return false
	}
	for _, child := range n.Children {
		if !child.Visit(fn) {
			return false
		}
	}
	return true
}

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
// This keeps platform accessibility focus consistent across widgets.
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

// SemanticsOwner manages the semantics tree and tracks dirty nodes.
type SemanticsOwner struct {
	// Root is the root of the semantics tree.
	Root *SemanticsNode

	// dirtyNodes tracks nodes that need to be updated.
	dirtyNodes map[*SemanticsNode]struct{}

	// nodesByID provides fast lookup by node ID.
	nodesByID map[int64]*SemanticsNode

	// stableIDs maps render objects to stable node IDs across frames.
	// This ensures actions from the platform find the correct node.
	stableIDs map[any]int64

	// nextStableID is the next ID to assign (starts at 1, 0 is reserved for synthetic root).
	nextStableID int64

	// onSemanticsUpdate is called when the semantics tree is updated.
	onSemanticsUpdate func(update SemanticsUpdate)

	mu sync.RWMutex
}

// NewSemanticsOwner creates a new semantics owner.
func NewSemanticsOwner() *SemanticsOwner {
	return &SemanticsOwner{
		dirtyNodes:   make(map[*SemanticsNode]struct{}),
		nodesByID:    make(map[int64]*SemanticsNode),
		stableIDs:    make(map[any]int64),
		nextStableID: 1, // 0 is reserved for synthetic root
	}
}

// GetStableID returns a stable node ID for the given key (typically a RenderObject).
// If the key has been seen before, returns the same ID. Otherwise assigns a new ID.
func (o *SemanticsOwner) GetStableID(key any) int64 {
	o.mu.Lock()
	defer o.mu.Unlock()

	if id, exists := o.stableIDs[key]; exists {
		return id
	}
	id := o.nextStableID
	o.nextStableID++
	o.stableIDs[key] = id
	return id
}

// SetUpdateCallback sets the callback for semantics updates.
func (o *SemanticsOwner) SetUpdateCallback(fn func(SemanticsUpdate)) {
	o.mu.Lock()
	o.onSemanticsUpdate = fn
	o.mu.Unlock()
}

// SetRoot sets the root semantics node.
func (o *SemanticsOwner) SetRoot(root *SemanticsNode) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.Root = root
	o.rebuildIndex()
}

// rebuildIndex rebuilds the nodesByID index from the tree.
func (o *SemanticsOwner) rebuildIndex() {
	o.nodesByID = make(map[int64]*SemanticsNode)
	if o.Root == nil {
		return
	}
	o.Root.Visit(func(node *SemanticsNode) bool {
		o.nodesByID[node.ID] = node
		return true
	})
}

// FindNodeByID finds a node by its ID.
func (o *SemanticsOwner) FindNodeByID(id int64) *SemanticsNode {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.nodesByID[id]
}

// MarkDirty marks a node as needing update.
func (o *SemanticsOwner) MarkDirty(node *SemanticsNode) {
	o.mu.Lock()
	defer o.mu.Unlock()

	node.MarkDirty()
	o.dirtyNodes[node] = struct{}{}
}

// HasDirtyNodes reports whether there are nodes needing update.
func (o *SemanticsOwner) HasDirtyNodes() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.dirtyNodes) > 0
}

// ClearDirtyNodes clears the dirty node set.
func (o *SemanticsOwner) ClearDirtyNodes() {
	o.mu.Lock()
	defer o.mu.Unlock()

	for node := range o.dirtyNodes {
		node.ClearDirty()
	}
	o.dirtyNodes = make(map[*SemanticsNode]struct{})
}

// SendSemanticsUpdate computes and sends the semantics update.
func (o *SemanticsOwner) SendSemanticsUpdate() {
	o.mu.Lock()
	callback := o.onSemanticsUpdate
	if callback == nil || o.Root == nil {
		o.mu.Unlock()
		return
	}

	// Collect all dirty nodes and build update
	var updates []SemanticsNodeUpdate
	for node := range o.dirtyNodes {
		updates = append(updates, nodeToUpdate(node))
		node.ClearDirty()
	}
	o.dirtyNodes = make(map[*SemanticsNode]struct{})
	o.mu.Unlock()

	if len(updates) > 0 {
		callback(SemanticsUpdate{Updates: updates})
	}
}

// SendFullUpdate sends a complete semantics tree update.
func (o *SemanticsOwner) SendFullUpdate() {
	o.mu.Lock()
	callback := o.onSemanticsUpdate
	if callback == nil || o.Root == nil {
		o.mu.Unlock()
		return
	}

	var updates []SemanticsNodeUpdate
	o.Root.Visit(func(node *SemanticsNode) bool {
		updates = append(updates, nodeToUpdate(node))
		node.ClearDirty()
		return true
	})
	o.dirtyNodes = make(map[*SemanticsNode]struct{})
	o.mu.Unlock()

	if len(updates) > 0 {
		callback(SemanticsUpdate{Updates: updates})
	}
}

// PerformAction performs an action on a node by ID.
func (o *SemanticsOwner) PerformAction(nodeID int64, action SemanticsAction, args any) bool {
	node := o.FindNodeByID(nodeID)
	if node == nil {
		return false
	}
	return node.PerformAction(action, args)
}

// nodeToUpdate converts a SemanticsNode to a SemanticsNodeUpdate.
func nodeToUpdate(node *SemanticsNode) SemanticsNodeUpdate {
	var childIDs []int64
	for _, child := range node.Children {
		childIDs = append(childIDs, child.ID)
	}

	var supportedActions SemanticsAction
	if node.Config.Actions != nil {
		supportedActions = node.Config.Actions.SupportedActions()
	}

	return SemanticsNodeUpdate{
		ID:              node.ID,
		Rect:            node.Rect,
		Label:           node.Config.Properties.Label,
		Value:           node.Config.Properties.Value,
		Hint:            node.Config.Properties.Hint,
		Role:            node.Config.Properties.Role,
		Flags:           node.Config.Properties.Flags,
		Actions:         supportedActions,
		ChildIDs:        childIDs,
		CurrentValue:    node.Config.Properties.CurrentValue,
		MinValue:        node.Config.Properties.MinValue,
		MaxValue:        node.Config.Properties.MaxValue,
		ScrollPosition:  node.Config.Properties.ScrollPosition,
		ScrollExtentMin: node.Config.Properties.ScrollExtentMin,
		ScrollExtentMax: node.Config.Properties.ScrollExtentMax,
		HeadingLevel:    node.Config.Properties.HeadingLevel,
		CustomActions:   node.Config.Properties.CustomActions,
	}
}
