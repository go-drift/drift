package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// ClipRRect clips its child using rounded corners.
type ClipRRect struct {
	ChildWidget core.Widget
	Radius      float64
}

func (c ClipRRect) CreateElement() core.Element {
	return core.NewRenderObjectElement(c, nil)
}

func (c ClipRRect) Key() any {
	return nil
}

func (c ClipRRect) Child() core.Widget {
	return c.ChildWidget
}

func (c ClipRRect) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderClipRRect{radius: c.Radius}
	box.SetSelf(box)
	return box
}

func (c ClipRRect) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderClipRRect); ok {
		box.radius = c.Radius
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderClipRRect struct {
	layout.RenderBoxBase
	child  layout.RenderBox
	radius float64
}

func (r *renderClipRRect) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderClipRRect) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderClipRRect) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(constraints.Constrain(rendering.Size{}))
		return
	}
	r.child.Layout(constraints, true) // true: we read child.Size()
	size := constraints.Constrain(r.child.Size())
	r.SetSize(size)
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderClipRRect) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	size := r.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	radius := r.radius
	if radius < 0 {
		radius = 0
	}
	rect := rendering.RectFromLTWH(0, 0, size.Width, size.Height)
	rrect := rendering.RRectFromRectAndRadius(rect, rendering.CircularRadius(radius))
	ctx.Canvas.Save()
	ctx.Canvas.ClipRRect(rrect)
	ctx.PaintChild(r.child, getChildOffset(r.child))
	ctx.Canvas.Restore()
}

func (r *renderClipRRect) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil && r.child.HitTest(position, result) {
		return true
	}
	result.Add(r)
	return true
}
