package layout

import (
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// RenderObject handles layout, painting, and hit testing.
type RenderObject interface {
	Layout(constraints Constraints)
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
	size       rendering.Size
	parentData any
	owner      *PipelineOwner
	self       RenderObject
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

// MarkNeedsLayout schedules this render box for layout.
func (r *RenderBoxBase) MarkNeedsLayout() {
	if r.owner != nil && r.self != nil {
		r.owner.ScheduleLayout(r.self)
	}
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
