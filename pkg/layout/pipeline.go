package layout

import "slices"

// PipelineOwner tracks render objects that need layout or paint.
type PipelineOwner struct {
	dirtyLayout []RenderObject
	dirtyPaint  []RenderObject
	needsLayout bool
	needsPaint  bool
}

// ScheduleLayout marks a render object as needing layout.
func (p *PipelineOwner) ScheduleLayout(object RenderObject) {
	if slices.Contains(p.dirtyLayout, object) {
		return
	}
	p.dirtyLayout = append(p.dirtyLayout, object)
	p.needsLayout = true
	p.needsPaint = true
}

// SchedulePaint marks a render object as needing paint.
func (p *PipelineOwner) SchedulePaint(object RenderObject) {
	if slices.Contains(p.dirtyPaint, object) {
		return
	}
	p.dirtyPaint = append(p.dirtyPaint, object)
	p.needsPaint = true
}

// NeedsLayout reports if any render objects need layout.
func (p *PipelineOwner) NeedsLayout() bool {
	return p.needsLayout
}

// NeedsPaint reports if any render objects need paint.
func (p *PipelineOwner) NeedsPaint() bool {
	return p.needsPaint
}

// FlushLayoutForRoot runs layout from the root when any object is dirty.
func (p *PipelineOwner) FlushLayoutForRoot(root RenderObject, constraints Constraints) {
	if !p.needsLayout || root == nil {
		return
	}
	root.Layout(constraints)
	p.dirtyLayout = nil
	p.needsLayout = false
}

// FlushPaint clears the dirty paint list.
func (p *PipelineOwner) FlushPaint() {
	p.dirtyPaint = nil
	p.needsPaint = false
}

// FlushLayout clears the dirty layout list without performing layout.
func (p *PipelineOwner) FlushLayout() {
	p.dirtyLayout = nil
	p.needsLayout = false
}
