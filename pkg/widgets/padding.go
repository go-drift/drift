package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// Padding adds empty space around its child widget.
//
// The child is constrained to the remaining space after padding is applied.
// If no child is provided, Padding creates an empty box of the padding size.
//
// Use [layout.EdgeInsets] helpers to create padding values:
//
//	Padding{Padding: layout.EdgeInsetsAll(16), ChildWidget: child}
//	Padding{Padding: layout.EdgeInsetsSymmetric(24, 12), ChildWidget: child}
//	Padding{Padding: layout.EdgeInsetsOnly(Left: 8, Right: 8), ChildWidget: child}
//
// For padding combined with background color, consider [Container] instead.
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
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderPadding) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderPadding) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}
	childConstraints := constraints.Deflate(r.padding)
	r.child.Layout(childConstraints, true) // true: we read child.Size()
	childSize := r.child.Size()
	size := constraints.Constrain(graphics.Size{
		Width:  childSize.Width + r.padding.Horizontal(),
		Height: childSize.Height + r.padding.Vertical(),
	})
	r.SetSize(size)
	r.child.SetParentData(&layout.BoxParentData{
		Offset: graphics.Offset{X: r.padding.Left, Y: r.padding.Top},
	})
}

func (r *renderPadding) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
}

func (r *renderPadding) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	offset := getChildOffset(r.child)
	local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	if r.child != nil && r.child.HitTest(local, result) {
		return true
	}
	result.Add(r)
	return true
}
