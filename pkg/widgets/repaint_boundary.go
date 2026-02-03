package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// RepaintBoundary isolates its subtree into a separate paint layer.
// This allows the subtree to be cached and reused when it doesn't change,
// which can significantly improve performance for static content next to
// frequently animating content.
type RepaintBoundary struct {
	Child core.Widget
}

func (r RepaintBoundary) CreateElement() core.Element {
	return core.NewRenderObjectElement(r, nil)
}

func (r RepaintBoundary) Key() any {
	return nil
}

func (r RepaintBoundary) ChildWidget() core.Widget {
	return r.Child
}

func (r RepaintBoundary) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderRepaintBoundary{}
	box.SetSelf(box)
	return box
}

func (r RepaintBoundary) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	// No properties to update
}

type renderRepaintBoundary struct {
	layout.RenderBoxBase
	child layout.RenderBox
}

// IsRepaintBoundary returns true - this IS a repaint boundary.
func (r *renderRepaintBoundary) IsRepaintBoundary() bool {
	return true
}

func (r *renderRepaintBoundary) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderRepaintBoundary) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderRepaintBoundary) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true)
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderRepaintBoundary) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
	}
}

func (r *renderRepaintBoundary) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil {
		offset := getChildOffset(r.child)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if r.child.HitTest(local, result) {
			return true
		}
	}
	return false
}
