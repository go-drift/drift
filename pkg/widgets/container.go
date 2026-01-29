package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
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
//	    Color:   graphics.ColorBlue,
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
//	    Gradient: &graphics.Gradient{...},
//	    Shadow:   &graphics.BoxShadow{BlurRadius: 8, Color: shadowColor},
//	    ChildWidget: content,
//	}
//
// For more complex decorations (borders, border radius, clipping), use [DecoratedBox].
type Container struct {
	ChildWidget core.Widget
	Padding     layout.EdgeInsets
	Width       float64
	Height      float64
	Color       graphics.Color
	Gradient    *graphics.Gradient
	Alignment   layout.Alignment
	Shadow *graphics.BoxShadow
	// Overflow controls whether gradients extend beyond container bounds.
	// Defaults to OverflowClip, confining gradients strictly within bounds.
	// Set to OverflowVisible for glow effects where the gradient should
	// extend beyond the widget. Only affects gradients; shadows overflow
	// naturally and solid colors never overflow.
	Overflow Overflow
}

// WithColor returns a copy of the container with the specified background color.
func (c Container) WithColor(color graphics.Color) Container {
	c.Color = color
	return c
}

// WithPadding returns a copy of the container with the specified padding.
func (c Container) WithPadding(padding layout.EdgeInsets) Container {
	c.Padding = padding
	return c
}

// WithSize returns a copy of the container with the specified width and height.
func (c Container) WithSize(width, height float64) Container {
	c.Width = width
	c.Height = height
	return c
}

// WithAlignment returns a copy of the container with the specified child alignment.
func (c Container) WithAlignment(alignment layout.Alignment) Container {
	c.Alignment = alignment
	return c
}

// WithGradient returns a copy of the container with the specified background gradient.
func (c Container) WithGradient(gradient *graphics.Gradient) Container {
	c.Gradient = gradient
	return c
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
	color := c.Color
	if c.Gradient != nil && color == graphics.ColorTransparent {
		color = graphics.ColorWhite
	}
	box := &renderContainer{
		padding:   c.Padding,
		width:     c.Width,
		height:    c.Height,
		color:     color,
		gradient:  c.Gradient,
		alignment: c.Alignment,
		shadow:    c.Shadow,
		overflow:  c.Overflow,
	}
	box.SetSelf(box)
	return box
}

func (c Container) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderContainer); ok {
		color := c.Color
		if c.Gradient != nil && color == graphics.ColorTransparent {
			color = graphics.ColorWhite
		}
		box.padding = c.Padding
		box.width = c.Width
		box.height = c.Height
		box.color = color
		box.gradient = c.Gradient
		box.alignment = c.Alignment
		box.shadow = c.Shadow
		box.overflow = c.Overflow
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
	color     graphics.Color
	gradient  *graphics.Gradient
	alignment layout.Alignment
	shadow    *graphics.BoxShadow
	overflow  Overflow
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
		constrained := constraints.Constrain(graphics.Size{Width: r.width}).Width
		available := max(constrained-r.padding.Horizontal(), 0)
		childConstraints.MinWidth = available
		childConstraints.MaxWidth = available
	}
	if hasHeight {
		constrained := constraints.Constrain(graphics.Size{Height: r.height}).Height
		available := max(constrained-r.padding.Vertical(), 0)
		childConstraints.MinHeight = available
		childConstraints.MaxHeight = available
	}
	var childSize graphics.Size
	if r.child != nil {
		r.child.Layout(childConstraints, true) // true: we read child.Size()
		childSize = r.child.Size()
	}

	size := graphics.Size{
		Width:  childSize.Width + r.padding.Horizontal(),
		Height: childSize.Height + r.padding.Vertical(),
	}
	if hasWidth {
		size.Width = constraints.Constrain(graphics.Size{Width: r.width}).Width
	}
	if hasHeight {
		size.Height = constraints.Constrain(graphics.Size{Height: r.height}).Height
	}
	size = constraints.Constrain(size)
	r.SetSize(size)

	if r.child != nil {
		contentRect := graphics.RectFromLTWH(
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
	widgetRect := graphics.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height)
	if r.shadow != nil {
		ctx.Canvas.DrawRectShadow(widgetRect, *r.shadow)
	}
	if r.color != graphics.ColorTransparent || r.gradient != nil {
		paint := graphics.DefaultPaint()
		paint.Color = r.color
		paint.Gradient = r.gradient

		if r.overflow == OverflowClip {
			ctx.Canvas.Save()
			ctx.Canvas.ClipRect(widgetRect)
			ctx.Canvas.DrawRect(widgetRect, paint)
			ctx.Canvas.Restore()
		} else {
			drawRect := widgetRect
			if r.gradient != nil {
				drawRect = r.gradient.Bounds(widgetRect)
			}
			ctx.Canvas.DrawRect(drawRect, paint)
		}
	}
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
}

func (r *renderContainer) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
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
