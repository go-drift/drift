package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Container is a convenience widget that combines common painting, positioning,
// and sizing operations into a single widget.
//
// Container applies decorations in this order:
//  1. Shadow (drawn behind the container)
//  2. Background color or gradient
//  3. Child widget (positioned according to Alignment within the padded area)
//
// # Sizing Behavior
//
// Without explicit Width/Height, Container sizes to fit its child plus padding.
// With Width and/or Height set, Container uses those dimensions (constrained by
// parent constraints) and positions the child using Alignment.
//
// # Common Patterns
//
//	// Colored box with padding
//	Container{
//	    Color:   rendering.ColorBlue,
//	    Padding: layout.EdgeInsetsAll(16),
//	    ChildWidget: Text{Content: "Hello"},
//	}
//
//	// Fixed-size centered child
//	Container{
//	    Width:     200,
//	    Height:    100,
//	    Alignment: layout.AlignmentCenter,
//	    ChildWidget: icon,
//	}
//
//	// Gradient background with shadow
//	Container{
//	    Gradient: &rendering.Gradient{...},
//	    Shadow:   &rendering.BoxShadow{BlurRadius: 8, Color: shadowColor},
//	    ChildWidget: content,
//	}
//
// For more complex decorations (borders, border radius, clipping), use [DecoratedBox].
type Container struct {
	ChildWidget core.Widget
	Padding     layout.EdgeInsets
	Width       float64
	Height      float64
	Color       rendering.Color
	Gradient    *rendering.Gradient
	Alignment   layout.Alignment
	Shadow      *rendering.BoxShadow
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
		shadow:    c.Shadow,
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
		box.shadow = c.Shadow
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
	shadow    *rendering.BoxShadow
}

func (r *renderContainer) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderContainer) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderContainer) PerformLayout() {
	constraints := r.Constraints()
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
		r.child.Layout(childConstraints, true) // true: we read child.Size()
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
	rect := rendering.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height)
	if r.shadow != nil {
		ctx.Canvas.DrawRectShadow(rect, *r.shadow)
	}
	if r.color != rendering.ColorTransparent || r.gradient != nil {
		paint := rendering.DefaultPaint()
		paint.Color = r.color
		paint.Gradient = r.gradient
		ctx.Canvas.DrawRect(rect, paint)
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
