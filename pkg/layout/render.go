package layout

import "github.com/go-drift/drift/pkg/rendering"

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

// RenderBox is a RenderObject with box layout.
type RenderBox interface {
	RenderObject
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
