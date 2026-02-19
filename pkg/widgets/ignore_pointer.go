package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// IgnorePointer lays out and paints its child normally but optionally blocks
// all hit testing. When Ignoring is true, pointer events cannot reach the child
// or any of its descendants. This is useful for disabling interaction during
// animations without affecting visual output.
type IgnorePointer struct {
	// Ignoring controls whether pointer events are blocked.
	Ignoring bool
	// Child is the widget to render.
	Child core.Widget
}

func (ip IgnorePointer) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (ip IgnorePointer) Key() any {
	return nil
}

func (ip IgnorePointer) ChildWidget() core.Widget {
	return ip.Child
}

func (ip IgnorePointer) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderIgnorePointer{ignoring: ip.Ignoring}
	box.SetSelf(box)
	return box
}

func (ip IgnorePointer) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderIgnorePointer); ok {
		box.ignoring = ip.Ignoring
	}
}

type renderIgnorePointer struct {
	layout.RenderBoxBase
	child    layout.RenderBox
	ignoring bool
}

func (r *renderIgnorePointer) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderIgnorePointer) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderIgnorePointer) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true)
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderIgnorePointer) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
	}
}

func (r *renderIgnorePointer) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil || r.ignoring || !withinBounds(position, r.Size()) {
		return false
	}
	offset := getChildOffset(r.child)
	local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	return r.child.HitTest(local, result)
}
