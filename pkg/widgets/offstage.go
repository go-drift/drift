package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Offstage lays out its child but optionally skips painting and hit testing.
//
// This keeps element/render object state alive without contributing to visual output.
// Use this to keep routes or tabs alive while preventing offscreen paint cost.
type Offstage struct {
	// Offstage controls whether the child is hidden.
	Offstage bool
	// ChildWidget is the widget to lay out and optionally hide.
	ChildWidget core.Widget
}

func (o Offstage) CreateElement() core.Element {
	return core.NewRenderObjectElement(o, nil)
}

func (o Offstage) Key() any {
	return nil
}

func (o Offstage) Child() core.Widget {
	return o.ChildWidget
}

func (o Offstage) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderOffstage{offstage: o.Offstage}
	box.SetSelf(box)
	return box
}

func (o Offstage) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderOffstage); ok {
		wasOffstage := box.offstage
		box.offstage = o.Offstage
		if wasOffstage && !box.offstage {
			// Ensure layout refresh when becoming visible again.
			box.MarkNeedsLayout()
		}
		box.MarkNeedsPaint()
	}
}

type renderOffstage struct {
	layout.RenderBoxBase
	child                layout.RenderBox
	offstage             bool
	lastChildSize        graphics.Size
	lastChildConstraints layout.Constraints
	hasChildSize         bool
}

func (r *renderOffstage) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
	// Clear cached size/constraints when the child changes.
	r.lastChildSize = graphics.Size{}
	r.lastChildConstraints = layout.Constraints{}
	r.hasChildSize = false
}

func (r *renderOffstage) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderOffstage) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		if r.offstage && r.hasChildSize && constraints == r.lastChildConstraints {
			r.SetSize(r.lastChildSize)
			return
		}
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.lastChildSize = r.child.Size()
		r.lastChildConstraints = constraints
		r.hasChildSize = true
		r.SetSize(r.lastChildSize)
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderOffstage) Paint(ctx *layout.PaintContext) {
	if r.child == nil || r.offstage {
		return
	}
	ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
}

func (r *renderOffstage) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil || r.offstage || !withinBounds(position, r.Size()) {
		return false
	}
	offset := getChildOffset(r.child)
	local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	return r.child.HitTest(local, result)
}
