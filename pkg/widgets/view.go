package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// View is the root widget that hosts the render tree.
type View struct {
	ChildWidget core.Widget
}

func (v View) CreateElement() core.Element {
	return core.NewRenderObjectElement(v, nil)
}

func (v View) Key() any {
	return nil
}

// Child returns the single child for render object wiring.
func (v View) Child() core.Widget {
	return v.ChildWidget
}

// CreateRenderObject builds the root render view.
func (v View) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	view := &renderView{}
	view.SetSelf(view)
	return view
}

// UpdateRenderObject updates the root render view.
func (v View) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {}

type renderView struct {
	layout.RenderBoxBase
	child layout.RenderBox
}

func (r *renderView) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderView) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderView) PerformLayout() {
	constraints := r.Constraints()
	width := constraints.MaxWidth
	if width <= 0 {
		width = constraints.MinWidth
	}
	height := constraints.MaxHeight
	if height <= 0 {
		height = constraints.MinHeight
	}
	size := graphics.Size{Width: width, Height: height}
	r.SetSize(size)
	if r.child != nil {
		r.child.Layout(layout.Tight(size), false) // false: tight constraints, child is boundary
	}
}

func (r *renderView) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
	}
}

func (r *renderView) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}
