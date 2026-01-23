package layout

import (
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// RenderObject handles layout, painting, and hit testing.
type RenderObject interface {
	Layout(constraints Constraints, parentUsesSize bool)
	Size() rendering.Size
	Paint(ctx *PaintContext)
	HitTest(position rendering.Offset, result *HitTestResult) bool
	ParentData() any
	SetParentData(data any)
	MarkNeedsLayout()
	MarkNeedsPaint()
	SetOwner(owner *PipelineOwner)
}

// SemanticsDescriber is implemented by render objects that provide semantic information.
type SemanticsDescriber interface {
	// DescribeSemanticsConfiguration populates the semantic configuration for this render object.
	// Returns true if this render object contributes semantic information.
	DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool

	// MarkNeedsSemanticsUpdate is called when semantic properties change.
	// Note: The semantics tree is rebuilt each frame, so this is currently a no-op.
	// It's kept for API compatibility and potential future incremental updates.
	MarkNeedsSemanticsUpdate()
}

// RenderBox is a RenderObject with box layout.
type RenderBox interface {
	RenderObject
}

// ChildVisitor is implemented by render objects that have children.
type ChildVisitor interface {
	// VisitChildren calls the visitor function for each child.
	VisitChildren(visitor func(RenderObject))
}

// ScrollOffsetProvider is implemented by scrollable render objects.
// The accessibility system uses this to adjust child positions for scroll offset.
type ScrollOffsetProvider interface {
	// SemanticScrollOffset returns the scroll offset to subtract from child positions.
	// A positive Y value means content has scrolled up (showing lower content).
	SemanticScrollOffset() rendering.Offset
}

// BoxParentData stores the offset for a child in a box layout.
type BoxParentData struct {
	Offset rendering.Offset
}

// RenderBoxBase provides base behavior for render boxes.
type RenderBoxBase struct {
	size             rendering.Size
	parentData       any
	owner            *PipelineOwner
	self             RenderObject
	parent           RenderObject // parent reference for tree walking
	depth            int          // tree depth (root = 0)
	relayoutBoundary RenderObject // cached nearest relayout boundary
	needsLayout      bool         // local dirty flag
	constraints      Constraints  // last received constraints
}

// Size returns the current size of the render box.
func (r *RenderBoxBase) Size() rendering.Size {
	return r.size
}

// SetSize updates the render box size.
func (r *RenderBoxBase) SetSize(size rendering.Size) {
	r.size = size
}

// ParentData returns the parent-assigned data for this render box.
func (r *RenderBoxBase) ParentData() any {
	return r.parentData
}

// SetParentData assigns parent-controlled data to this render box.
func (r *RenderBoxBase) SetParentData(data any) {
	r.parentData = data
}

// MarkNeedsLayout marks this render box as needing layout.
//
// This follows Flutter's relayout boundary pattern: when a node needs layout,
// we walk up the tree marking each node until we reach a relayout boundary.
// The boundary then gets scheduled for layout. During layout, all marked nodes
// will run their PerformLayout because their needsLayout flag is true.
//
// This ensures that when a deep descendant changes, layout properly propagates
// from the boundary down through all intermediate nodes to reach the changed node.
func (r *RenderBoxBase) MarkNeedsLayout() {
	if r.needsLayout {
		return
	}
	r.needsLayout = true

	if r.owner == nil || r.self == nil {
		return
	}

	// If we are our own relayout boundary, schedule ourselves
	if r.relayoutBoundary == r.self {
		r.owner.ScheduleLayout(r.self)
		return
	}

	// If we have a parent, mark it as needing layout too.
	// This walks up the tree until we hit a boundary (which schedules itself).
	// Each node along the path gets needsLayout=true, ensuring the layout
	// chain doesn't break at intermediate nodes.
	if r.parent != nil {
		r.parent.MarkNeedsLayout()
		return
	}

	// No parent and not a boundary - this is likely during initial setup
	// before the tree is fully connected. Schedule self to ensure we get laid out.
	r.owner.ScheduleLayout(r.self)
}

// MarkNeedsPaint schedules this render box for painting.
func (r *RenderBoxBase) MarkNeedsPaint() {
	if r.owner != nil && r.self != nil {
		r.owner.SchedulePaint(r.self)
	}
}

// SetOwner assigns the pipeline owner for scheduling layout and paint.
func (r *RenderBoxBase) SetOwner(owner *PipelineOwner) {
	r.owner = owner
}

// SetSelf registers the concrete render object for scheduling.
func (r *RenderBoxBase) SetSelf(self RenderObject) {
	r.self = self
	r.needsLayout = true // New render objects always need initial layout
}

// Parent returns the parent render object.
func (r *RenderBoxBase) Parent() RenderObject {
	return r.parent
}

// SetParent sets the parent render object and computes depth.
// Clears relayoutBoundary and constraints to prevent stale references
// when the object is reparented to a different subtree.
func (r *RenderBoxBase) SetParent(parent RenderObject) {
	if r.parent == parent {
		return
	}
	r.parent = parent
	if parent == nil {
		r.depth = 0
	} else if getter, ok := parent.(interface{ Depth() int }); ok {
		r.depth = getter.Depth() + 1
	} else {
		r.depth = 1
	}
	// Clear stale state from old parent tree
	r.relayoutBoundary = nil
	r.constraints = Constraints{}
	r.needsLayout = true
}

// Depth returns the tree depth (root = 0).
func (r *RenderBoxBase) Depth() int {
	return r.depth
}

// RelayoutBoundary returns the cached nearest relayout boundary.
func (r *RenderBoxBase) RelayoutBoundary() RenderObject {
	return r.relayoutBoundary
}

// NeedsLayout returns true if this render box needs layout.
func (r *RenderBoxBase) NeedsLayout() bool {
	return r.needsLayout
}

// Constraints returns the last received constraints.
func (r *RenderBoxBase) Constraints() Constraints {
	return r.constraints
}

// Layout handles boundary determination and delegates to PerformLayout.
//
// This implements Flutter's relayout boundary optimization. A node becomes a
// relayout boundary when:
//   - It receives tight constraints (parent dictates exact size)
//   - It is the root (no parent)
//   - Parent doesn't use our size (parentUsesSize=false)
//
// Boundaries contain layout changes - when a descendant needs layout, the walk
// up stops at the boundary, preventing unnecessary relayout of ancestors.
//
// Widgets should implement PerformLayout() for their specific layout logic.
// The base Layout() handles:
//   - Updating the relayout boundary reference
//   - Skipping layout when clean and constraints unchanged
//   - Clearing the needsLayout flag
//   - Calling PerformLayout()
func (r *RenderBoxBase) Layout(constraints Constraints, parentUsesSize bool) {
	// Determine if this should be a relayout boundary
	shouldBeBoundary := constraints.IsTight() || r.parent == nil || !parentUsesSize

	if shouldBeBoundary {
		r.relayoutBoundary = r.self
	} else if r.parent != nil {
		// Inherit boundary from parent
		if getter, ok := r.parent.(interface{ RelayoutBoundary() RenderObject }); ok {
			r.relayoutBoundary = getter.RelayoutBoundary()
		}
	}

	// Skip layout if we're clean and constraints haven't changed.
	// This is the key optimization - unchanged subtrees don't re-layout.
	if !r.needsLayout && r.constraints == constraints {
		return
	}

	// Store constraints and clear dirty flag before performing layout.
	// This order matters: if PerformLayout causes a child to mark us dirty
	// again (shouldn't happen but defensive), we'll catch it next frame.
	r.constraints = constraints
	r.needsLayout = false

	// Call the concrete implementation's PerformLayout
	if performer, ok := r.self.(interface{ PerformLayout() }); ok {
		performer.PerformLayout()
	}
}

// MarkNeedsSemanticsUpdate is called when semantic properties change.
// Note: The semantics tree is rebuilt each frame, so this is currently a no-op.
// It's kept for API compatibility and potential future incremental updates.
func (r *RenderBoxBase) MarkNeedsSemanticsUpdate() {
	// No-op: semantics are rebuilt every frame
}

// DescribeSemanticsConfiguration is the default implementation that reports no semantic content.
// Override this method in render objects that provide semantic information.
func (r *RenderBoxBase) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	return false
}
