package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Container is a convenience widget that combines common painting, positioning,
// and sizing operations into a single widget.
//
// Container applies decorations in this order:
//  1. Shadow (drawn behind the container, naturally overflows bounds)
//  2. Background color or gradient (clipped to BorderRadius when Overflow is OverflowClip)
//  3. Border stroke (if BorderWidth > 0)
//  4. Child widget (clipped to bounds when Overflow is OverflowClip)
//
// # Sizing Behavior
//
// Without explicit Width/Height, Container sizes to fit its child plus padding.
// With Width and/or Height set, those dimensions become the container's preferred
// size (subject to parent constraints) and set maximum child size on those axes.
// Children can be smaller than the container, and Alignment controls their
// placement within the available content area (after padding).
//
// Note: Parent constraints take precedence. If a parent imposes tight constraints
// larger than Width/Height, the container will expand to meet those constraints.
//
// # Common Patterns
//
//	// Rounded card with padding
//	Container{
//	    Color:        colors.Surface,
//	    BorderRadius: 12,
//	    Padding:      layout.EdgeInsetsAll(16),
//	    ChildWidget:  content,
//	}
//
//	// Bordered box
//	Container{
//	    BorderColor:  colors.Outline,
//	    BorderWidth:  1,
//	    BorderRadius: 8,
//	    Padding:      layout.EdgeInsetsAll(12),
//	    ChildWidget:  Text{Content: "Hello"},
//	}
//
//	// Fixed-size centered child
//	Container{
//	    Width:       200,
//	    Height:      100,
//	    Alignment:   layout.AlignmentCenter,
//	    ChildWidget: icon,
//	}
//
// Container supports all [DecoratedBox] features. For decoration without layout
// behavior (no padding, sizing, or alignment), use DecoratedBox directly.
type Container struct {
	ChildWidget core.Widget
	Padding     layout.EdgeInsets
	Width       float64
	Height      float64
	Color       graphics.Color
	Gradient    *graphics.Gradient
	Alignment   layout.Alignment
	Shadow      *graphics.BoxShadow

	// Border
	BorderColor  graphics.Color        // Border stroke color; transparent = no border
	BorderWidth  float64               // Border stroke width in pixels; 0 = no border
	BorderRadius float64               // Corner radius for rounded rectangles; 0 = sharp corners
	BorderDash   *graphics.DashPattern // Dash pattern for border; nil = solid line
	// BorderGradient applies a gradient to the border stroke. When set, overrides
	// BorderColor. Requires BorderWidth > 0 to be visible. Works with BorderDash
	// for dashed gradient borders.
	BorderGradient *graphics.Gradient

	// Overflow controls clipping behavior for gradients and children.
	// Defaults to OverflowClip, which confines gradients and children strictly
	// within bounds (clipped to rounded shape when BorderRadius > 0).
	// Set to OverflowVisible for glow effects where the gradient should extend
	// beyond the container; children will not be clipped.
	// Shadows always overflow naturally. Solid background colors never overflow.
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

// WithBorderRadius returns a copy of the container with the specified corner radius.
func (c Container) WithBorderRadius(radius float64) Container {
	c.BorderRadius = radius
	return c
}

// WithBorder returns a copy of the container with the specified border color and width.
func (c Container) WithBorder(color graphics.Color, width float64) Container {
	c.BorderColor = color
	c.BorderWidth = width
	return c
}

// WithBorderGradient returns a copy with the specified border gradient.
// The gradient overrides BorderColor when both are set.
func (c Container) WithBorderGradient(gradient *graphics.Gradient) Container {
	c.BorderGradient = gradient
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
		alignment: c.Alignment,
		painter: decorationPainter{
			color:          color,
			gradient:       c.Gradient,
			borderColor:    c.BorderColor,
			borderWidth:    c.BorderWidth,
			borderRadius:   c.BorderRadius,
			borderDash:     c.BorderDash,
			borderGradient: c.BorderGradient,
			shadow:         c.Shadow,
			overflow:       c.Overflow,
		},
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
		box.alignment = c.Alignment
		box.painter = decorationPainter{
			color:          color,
			gradient:       c.Gradient,
			borderColor:    c.BorderColor,
			borderWidth:    c.BorderWidth,
			borderRadius:   c.BorderRadius,
			borderDash:     c.BorderDash,
			borderGradient: c.BorderGradient,
			shadow:         c.Shadow,
			overflow:       c.Overflow,
		}
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
	alignment layout.Alignment
	painter   decorationPainter
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
		childConstraints.MinWidth = 0 // Loose: child can be smaller
		childConstraints.MaxWidth = available
	}
	if hasHeight {
		constrained := constraints.Constrain(graphics.Size{Height: r.height}).Height
		available := max(constrained-r.padding.Vertical(), 0)
		childConstraints.MinHeight = 0 // Loose: child can be smaller
		childConstraints.MaxHeight = available
	}
	var childSize graphics.Size
	if r.child != nil {
		r.child.Layout(childConstraints, true)
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
	size := r.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	rect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)
	r.painter.paint(ctx, rect)

	if r.child == nil {
		return
	}

	if r.painter.shouldClipChildren() {
		r.painter.applyChildClip(ctx, rect)
		ctx.PaintChild(r.child, getChildOffset(r.child))
		ctx.PopClipRect()
		ctx.Canvas.Restore()
	} else {
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
