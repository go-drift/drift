package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Container is a convenience widget for common decorations.
type Container struct {
	ChildWidget core.Widget
	Padding     layout.EdgeInsets
	Width       float64
	Height      float64
	Color       rendering.Color
	Gradient    *rendering.Gradient
	Alignment   layout.Alignment
}

func (c Container) CreateElement() core.Element {
	return core.NewRenderObjectElement(c, nil)
}

func (c Container) Key() any {
	return nil
}

func (c Container) Child() core.Widget {
	return c.ChildWidget
}

func (c Container) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderContainer{
		padding:   c.Padding,
		width:     c.Width,
		height:    c.Height,
		color:     c.Color,
		gradient:  c.Gradient,
		alignment: c.Alignment,
	}
	box.SetSelf(box)
	return box
}

func (c Container) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderContainer); ok {
		box.padding = c.Padding
		box.width = c.Width
		box.height = c.Height
		box.color = c.Color
		box.gradient = c.Gradient
		box.alignment = c.Alignment
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderContainer struct {
	layout.RenderBoxBase
	child     layout.RenderBox
	padding   layout.EdgeInsets
	width     float64
	height    float64
	color     rendering.Color
	gradient  *rendering.Gradient
	alignment layout.Alignment
}

func (r *renderContainer) SetChild(child layout.RenderObject) {
	r.child = setChildFromRenderObject(child)
}

func (r *renderContainer) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderContainer) Layout(constraints layout.Constraints) {
	childConstraints := constraints.Deflate(r.padding)
	hasWidth := r.width > 0
	hasHeight := r.height > 0
	if hasWidth {
		constrained := constraints.Constrain(rendering.Size{Width: r.width}).Width
		available := max(constrained-r.padding.Horizontal(), 0)
		childConstraints.MinWidth = available
		childConstraints.MaxWidth = available
	}
	if hasHeight {
		constrained := constraints.Constrain(rendering.Size{Height: r.height}).Height
		available := max(constrained-r.padding.Vertical(), 0)
		childConstraints.MinHeight = available
		childConstraints.MaxHeight = available
	}
	var childSize rendering.Size
	if r.child != nil {
		r.child.Layout(childConstraints)
		childSize = r.child.Size()
	}

	size := rendering.Size{
		Width:  childSize.Width + r.padding.Horizontal(),
		Height: childSize.Height + r.padding.Vertical(),
	}
	if hasWidth {
		size.Width = constraints.Constrain(rendering.Size{Width: r.width}).Width
	}
	if hasHeight {
		size.Height = constraints.Constrain(rendering.Size{Height: r.height}).Height
	}
	size = constraints.Constrain(size)
	r.SetSize(size)

	if r.child != nil {
		contentRect := rendering.RectFromLTWH(
			r.padding.Left,
			r.padding.Top,
			size.Width-r.padding.Horizontal(),
			size.Height-r.padding.Vertical(),
		)
		offset := r.alignment.WithinRect(contentRect, r.child.Size())
		r.child.SetParentData(&layout.BoxParentData{Offset: offset})
	}
}

func (r *renderContainer) Paint(ctx *layout.PaintContext) {
	if r.color != rendering.ColorTransparent || r.gradient != nil {
		paint := rendering.DefaultPaint()
		paint.Color = r.color
		paint.Gradient = r.gradient
		ctx.Canvas.DrawRect(
			rendering.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height),
			paint,
		)
	}
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
}

func (r *renderContainer) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
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
