package layout

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
)

// RenderObject handles layout, painting, and hit testing.
type RenderObject interface {
	Layout(constraints Constraints, parentUsesSize bool)
	Size() graphics.Size
	Paint(ctx *PaintContext)
	HitTest(position graphics.Offset, result *HitTestResult) bool
	ParentData() any
	SetParentData(data any)
	MarkNeedsLayout()
	MarkNeedsPaint()
	MarkNeedsSemanticsUpdate()
	SetOwner(owner *PipelineOwner)
	IsRepaintBoundary() bool
}

// SemanticsDescriber is implemented by render objects that provide semantic information.
type SemanticsDescriber interface {
	// DescribeSemanticsConfiguration populates the semantic configuration for this render object.
	// Returns true if this render object contributes semantic information.
	DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool
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

// RepaintBoundaryNode is implemented by render objects that are repaint boundaries.
// This interface groups the methods needed for layer tree recording.
type RepaintBoundaryNode interface {
	IsRepaintBoundary() bool
	NeedsPaint() bool
}

// LayerTreeNode combines the interfaces needed for layer tree traversal.
// Using a combined interface avoids multiple type assertions per node in the
// recording hot path - a single assertion extracts all needed capabilities.
type LayerTreeNode interface {
	RepaintBoundaryNode
	ChildVisitor
}

// ScrollOffsetProvider is implemented by scrollable render objects.
// The accessibility system uses this to adjust child positions for scroll offset.
type ScrollOffsetProvider interface {
	// SemanticScrollOffset returns the scroll offset to subtract from child positions.
	// A positive Y value means content has scrolled up (showing lower content).
	SemanticScrollOffset() graphics.Offset
}

// BoxParentData stores the offset for a child in a box layout.
type BoxParentData struct {
	Offset graphics.Offset
}

// RenderBoxBase provides base behavior for render boxes.
type RenderBoxBase struct {
	size                 graphics.Size
	parentData           any
	owner                *PipelineOwner
	self                 RenderObject
	parent               RenderObject    // parent reference for tree walking
	depth                int             // tree depth (root = 0)
	relayoutBoundary     RenderObject    // cached nearest relayout boundary
	needsLayout          bool            // local dirty flag
	constraints          Constraints     // last received constraints
	repaintBoundary      RenderObject    // cached nearest repaint boundary
	needsPaint           bool            // local dirty flag for paint
	layer                *graphics.Layer // stable layer for boundaries (never nil after creation)
	semanticsBoundary    RenderObject    // cached nearest semantics boundary
	needsSemanticsUpdate bool            // local dirty flag for semantics
}

// Size returns the current size of the render box.
func (r *RenderBoxBase) Size() graphics.Size {
	return r.size
}

// SetSize updates the render box size.
// If the size changes, marks paint as dirty since the render object's content
// needs to be re-recorded at the new size.
func (r *RenderBoxBase) SetSize(size graphics.Size) {
	if r.size == size {
		return
	}
	r.size = size
	// Size changed during layout - mark paint dirty.
	// For repaint boundaries, this ensures the layer is re-recorded with the new size.
	// For non-boundaries, this propagates up to the parent boundary.
	r.MarkNeedsPaint()
}

// ParentData returns the parent-assigned data for this render box.
func (r *RenderBoxBase) ParentData() any {
	return r.parentData
}

// SetParentData assigns parent-controlled data to this render box.
// If the offset in BoxParentData changes, marks the parent for repaint since the
// parent's DrawChildLayer op embeds the child offset and becomes stale.
func (r *RenderBoxBase) SetParentData(data any) {
	// Check if offset changed - parent needs repaint if so
	if newData, ok := data.(*BoxParentData); ok {
		oldOffset := r.parentDataOffset()
		if oldOffset != newData.Offset && r.parent != nil {
			// Parent's layer records DrawChildLayer with our offset, so it's stale
			r.parent.MarkNeedsPaint()
		}
	}
	r.parentData = data
}

// parentDataOffset extracts the offset from current parentData, or zero if not set.
func (r *RenderBoxBase) parentDataOffset() graphics.Offset {
	if data, ok := r.parentData.(*BoxParentData); ok {
		return data.Offset
	}
	return graphics.Offset{}
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

// MarkNeedsPaint marks this render box as needing paint.
//
// This follows Flutter's repaint boundary pattern: when a node needs paint,
// we walk up the tree until we reach a repaint boundary. The boundary then
// gets scheduled for paint.
//
// Key optimization: When we hit a repaint boundary, we STOP walking up.
// Parent boundaries reference child boundaries via DrawChildLayer ops, not
// embedded content. This means changing a child's content doesn't require
// re-recording the parent - the parent's DrawChildLayer op will composite
// the child's updated content automatically.
//
// Note: Unlike MarkNeedsLayout, we don't early-return when needsPaint is true.
// This is because SetSelf() pre-sets needsPaint=true without scheduling, and
// SchedulePaint() already handles deduplication internally.
func (r *RenderBoxBase) MarkNeedsPaint() {
	if r.owner == nil || r.self == nil {
		r.needsPaint = true
		// Mark layer dirty if it exists (can't ensure layer without owner)
		if r.layer != nil {
			r.layer.MarkDirty()
		}
		return
	}

	// If we are a repaint boundary, ensure layer exists and mark dirty, then schedule.
	// Parent boundaries reference us via DrawChildLayer, so they don't need
	// to re-record when our content changes - we STOP here.
	if r.repaintBoundary == r.self {
		r.needsPaint = true
		// Ensure layer exists so it can be marked dirty (keeps needsPaint and layer.Dirty in sync)
		r.EnsureLayer().MarkDirty()
		r.owner.SchedulePaint(r.self)
		return
	}

	// Not a boundary - mark layer dirty if it exists, then walk up
	if r.layer != nil {
		r.layer.MarkDirty()
	}

	// Walk up to parent. This continues until hitting a boundary.
	if r.parent != nil {
		r.needsPaint = true
		r.parent.MarkNeedsPaint()
		return
	}

	// No parent and not a boundary - schedule self
	r.needsPaint = true
	r.owner.SchedulePaint(r.self)
}

// SetOwner assigns the pipeline owner for scheduling layout and paint.
func (r *RenderBoxBase) SetOwner(owner *PipelineOwner) {
	r.owner = owner
}

// SetSelf registers the concrete render object for scheduling.
func (r *RenderBoxBase) SetSelf(self RenderObject) {
	r.self = self
	r.needsLayout = true          // New render objects always need initial layout
	r.needsPaint = true           // New render objects always need initial paint
	r.needsSemanticsUpdate = true // New render objects always need initial semantics
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
	r.repaintBoundary = nil
	r.needsPaint = true
	// Mark layer dirty if it exists (preserves stable identity when reparenting)
	if r.layer != nil {
		r.layer.MarkDirty()
	}
	r.semanticsBoundary = nil
	r.needsSemanticsUpdate = true
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

// IsRepaintBoundary returns whether this render object repaints separately.
// Override this in render objects that should isolate their paint.
func (r *RenderBoxBase) IsRepaintBoundary() bool {
	return false
}

// RepaintBoundary returns the cached nearest repaint boundary.
func (r *RenderBoxBase) RepaintBoundary() RenderObject {
	return r.repaintBoundary
}

// NeedsPaint returns true if this render box needs painting.
func (r *RenderBoxBase) NeedsPaint() bool {
	return r.needsPaint
}

// Layer returns the cached layer for repaint boundaries.
func (r *RenderBoxBase) Layer() *graphics.Layer {
	return r.layer
}

// EnsureLayer returns the existing layer or creates one if needed.
// The layer has stable identity - never replace it, only mark dirty.
func (r *RenderBoxBase) EnsureLayer() *graphics.Layer {
	if r.layer == nil {
		r.layer = &graphics.Layer{Dirty: true, Size: r.size}
	}
	return r.layer
}

// SetLayerContent updates the layer's content (called after recording).
// Disposes old content before setting new content.
func (r *RenderBoxBase) SetLayerContent(content *graphics.DisplayList) {
	if r.layer == nil {
		r.layer = &graphics.Layer{}
	}
	r.layer.SetContent(content)
	r.layer.Size = r.size
}

// ClearNeedsPaint marks this render object as painted.
func (r *RenderBoxBase) ClearNeedsPaint() {
	r.needsPaint = false
}

// SemanticsBoundary returns the cached nearest semantics boundary.
func (r *RenderBoxBase) SemanticsBoundary() RenderObject {
	return r.semanticsBoundary
}

// NeedsSemanticsUpdate returns true if this render box needs semantics update.
func (r *RenderBoxBase) NeedsSemanticsUpdate() bool {
	return r.needsSemanticsUpdate
}

// ClearNeedsSemanticsUpdate marks this render object's semantics as updated.
func (r *RenderBoxBase) ClearNeedsSemanticsUpdate() {
	r.needsSemanticsUpdate = false
}

// Dispose releases resources held by this render box.
// Call this when the render object is permanently removed from the tree.
func (r *RenderBoxBase) Dispose() {
	if r.layer != nil {
		r.layer.Dispose()
		r.layer = nil
	}
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

	// Determine repaint boundary (inherited unless explicit)
	if r.self != nil && r.self.IsRepaintBoundary() {
		r.repaintBoundary = r.self
		// Schedule paint if this boundary needs it. This ensures boundaries are scheduled
		// on their first layout, since SetSelf() sets needsPaint=true but can't schedule
		// (no owner yet). Without this, outer boundaries could be skipped if a child
		// boundary schedules itself first (MarkNeedsPaint stops at the first boundary).
		if r.needsPaint && r.owner != nil {
			r.EnsureLayer().MarkDirty()
			r.owner.SchedulePaint(r.self)
		}
	} else if r.parent != nil {
		if getter, ok := r.parent.(interface{ RepaintBoundary() RenderObject }); ok {
			r.repaintBoundary = getter.RepaintBoundary()
		}
	}

	// Determine semantics boundary
	// A node is a semantics boundary if it creates a discrete semantic node
	// (has IsSemanticBoundary or IsMergingSemanticsOfDescendants set, or contributes non-empty semantics)
	//
	// NOTE: semanticsBoundary is computed only during Layout(). If semantic properties
	// change without triggering layout (e.g., label becomes empty), MarkNeedsSemanticsUpdate()
	// will use stale boundary info. This is safe with the current full-rebuild approach
	// in FlushSemantics, but would need revisiting for true incremental updates.
	if r.self != nil {
		isBoundary := false
		if describer, ok := r.self.(SemanticsDescriber); ok {
			var config semantics.SemanticsConfiguration
			contributes := describer.DescribeSemanticsConfiguration(&config)
			isBoundary = config.IsSemanticBoundary || config.IsMergingSemanticsOfDescendants ||
				(contributes && !config.IsEmpty())
		}
		if isBoundary {
			r.semanticsBoundary = r.self
		} else if r.parent != nil {
			if getter, ok := r.parent.(interface{ SemanticsBoundary() RenderObject }); ok {
				r.semanticsBoundary = getter.SemanticsBoundary()
			}
		}
	}

	// Skip layout if we're clean and constraints haven't changed.
	// This is the key optimization - unchanged subtrees don't re-layout.
	if !r.needsLayout && r.constraints == constraints {
		return
	}

	// Layout is happening - mark semantics dirty since positions may change.
	// This ensures semantic rects stay in sync with visual positions after layout.
	// ScheduleSemantics handles deduplication, so this is safe to call frequently.
	r.MarkNeedsSemanticsUpdate()

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

// MarkNeedsSemanticsUpdate marks this render box as needing semantics update.
//
// This follows Flutter's semantics boundary pattern: when a node needs semantics
// update, we walk up the tree until we reach a semantics boundary.
// The boundary then gets scheduled for semantics update.
//
// Note: Unlike MarkNeedsLayout, we don't early-return when needsSemanticsUpdate is true.
// This is because SetSelf() and SetParent() pre-set needsSemanticsUpdate=true without
// scheduling, and ScheduleSemantics() already handles deduplication internally.
//
// NOTE: This method marks needsSemanticsUpdate=true on all nodes along the path to
// the boundary, but only the boundary is added to dirtySemantics. When FlushSemantics
// clears flags, only boundary flags are cleared - intermediate nodes remain dirty.
// This is harmless with the current full-rebuild approach but would need addressing
// for true incremental updates (either clear all affected nodes, or only mark boundaries).
func (r *RenderBoxBase) MarkNeedsSemanticsUpdate() {
	if r.owner == nil || r.self == nil {
		r.needsSemanticsUpdate = true
		return
	}

	// If we are a semantics boundary, schedule ourselves
	if r.semanticsBoundary == r.self {
		r.needsSemanticsUpdate = true
		r.owner.ScheduleSemantics(r.self)
		return
	}

	// Walk up to parent
	if r.parent != nil {
		r.needsSemanticsUpdate = true
		r.parent.MarkNeedsSemanticsUpdate()
		return
	}

	// Root - schedule self
	r.needsSemanticsUpdate = true
	r.owner.ScheduleSemantics(r.self)
}

// DescribeSemanticsConfiguration is the default implementation that reports no semantic content.
// Override this method in render objects that provide semantic information.
func (r *RenderBoxBase) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	return false
}
