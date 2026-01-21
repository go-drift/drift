package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Padding adds padding around a child.
type Padding struct {
	Padding     layout.EdgeInsets
	ChildWidget core.Widget
}

func (p Padding) CreateElement() core.Element {
	return core.NewRenderObjectElement(p, nil)
}

func (p Padding) Key() any {
	return nil
}

func (p Padding) Child() core.Widget {
	return p.ChildWidget
}

func (p Padding) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	pad := &renderPadding{padding: p.Padding}
	pad.SetSelf(pad)
	return pad
}

func (p Padding) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if pad, ok := renderObject.(*renderPadding); ok {
		pad.padding = p.Padding
		pad.MarkNeedsLayout()
		pad.MarkNeedsPaint()
	}
}

type renderPadding struct {
	layout.RenderBoxBase
	child   layout.RenderBox
	padding layout.EdgeInsets
}

func (r *renderPadding) SetChild(child layout.RenderObject) {
	r.child = setChildFromRenderObject(child)
}

func (r *renderPadding) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderPadding) Layout(constraints layout.Constraints) {
	if r.child == nil {
		r.SetSize(constraints.Constrain(rendering.Size{}))
		return
	}
	childConstraints := constraints.Deflate(r.padding)
	r.child.Layout(childConstraints)
	childSize := r.child.Size()
	size := constraints.Constrain(rendering.Size{
		Width:  childSize.Width + r.padding.Horizontal(),
		Height: childSize.Height + r.padding.Vertical(),
	})
	r.SetSize(size)
	r.child.SetParentData(&layout.BoxParentData{
		Offset: rendering.Offset{X: r.padding.Left, Y: r.padding.Top},
	})
}

func (r *renderPadding) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
}

func (r *renderPadding) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	offset := getChildOffset(r.child)
	local := rendering.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	if r.child != nil && r.child.HitTest(local, result) {
		return true
	}
	result.Add(r)
	return true
}
