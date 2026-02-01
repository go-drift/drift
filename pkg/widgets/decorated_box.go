package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// DecoratedBox paints a background, border, and shadow behind its child.
//
// DecoratedBox applies decorations in this order:
//  1. Shadow (drawn behind, naturally overflows bounds)
//  2. Background color or gradient (overflow controlled by Overflow field)
//  3. Border stroke (drawn on top of background, supports dashing)
//  4. Child widget (clipped to bounds when Overflow is OverflowClip)
//
// Use BorderRadius for rounded corners. The Overflow field controls clipping:
//   - [OverflowClip] (default): gradient and children clipped to bounds
//   - [OverflowVisible]: gradient can overflow, children not clipped
//
// With OverflowClip, children are clipped to the widget bounds (rounded rectangle
// when BorderRadius > 0). This ensures content like images or accent bars at the
// edges conform to the bounds without needing a separate [ClipRRect].
//
// Note: Platform views (native text fields, etc.) are clipped to rectangular
// bounds only, not rounded corners. This is a platform limitation.
//
// For combined layout and decoration (padding, sizing, alignment), use [Container]
// which composes DecoratedBox internally. Use DecoratedBox directly when you need
// decoration without any layout behavior.
type DecoratedBox struct {
	ChildWidget core.Widget // Child widget to display inside the decoration

	// Background
	Color    graphics.Color     // Background fill color
	Gradient *graphics.Gradient // Background gradient; overrides Color if set

	// Border
	BorderColor  graphics.Color        // Border stroke color; transparent = no border
	BorderWidth  float64               // Border stroke width in pixels; 0 = no border
	BorderRadius float64               // Corner radius for rounded rectangles; 0 = sharp corners
	BorderDash   *graphics.DashPattern // Dash pattern for border; nil = solid line
	// BorderGradient applies a gradient to the border stroke. When set, overrides
	// BorderColor. Requires BorderWidth > 0 to be visible. Works with BorderDash
	// for dashed gradient borders.
	BorderGradient *graphics.Gradient

	// Effects
	Shadow *graphics.BoxShadow // Drop shadow drawn behind the box; nil = no shadow

	// Overflow controls clipping behavior for gradients and children.
	// Defaults to OverflowClip, which confines gradients and children strictly
	// within bounds (clipped to rounded shape when BorderRadius > 0).
	// Set to OverflowVisible for glow effects where the gradient should extend
	// beyond the widget; children will not be clipped.
	// Shadows always overflow naturally. Solid background colors never overflow.
	Overflow Overflow
}

func (d DecoratedBox) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d DecoratedBox) Key() any {
	return nil
}

func (d DecoratedBox) Child() core.Widget {
	return d.ChildWidget
}

func (d DecoratedBox) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	color := d.Color
	if d.Gradient != nil && color == graphics.ColorTransparent {
		color = graphics.ColorWhite
	}
	box := &renderDecoratedBox{
		painter: decorationPainter{
			color:          color,
			gradient:       d.Gradient,
			borderColor:    d.BorderColor,
			borderWidth:    d.BorderWidth,
			borderRadius:   d.BorderRadius,
			borderDash:     d.BorderDash,
			borderGradient: d.BorderGradient,
			shadow:         d.Shadow,
			overflow:       d.Overflow,
		},
	}
	box.SetSelf(box)
	return box
}

func (d DecoratedBox) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderDecoratedBox); ok {
		color := d.Color
		if d.Gradient != nil && color == graphics.ColorTransparent {
			color = graphics.ColorWhite
		}
		box.painter = decorationPainter{
			color:          color,
			gradient:       d.Gradient,
			borderColor:    d.BorderColor,
			borderWidth:    d.BorderWidth,
			borderRadius:   d.BorderRadius,
			borderDash:     d.BorderDash,
			borderGradient: d.BorderGradient,
			shadow:         d.Shadow,
			overflow:       d.Overflow,
		}
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderDecoratedBox struct {
	layout.RenderBoxBase
	child   layout.RenderBox
	painter decorationPainter
}

func (r *renderDecoratedBox) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderDecoratedBox) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderDecoratedBox) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}
	r.child.Layout(constraints, true) // true: we read child.Size()
	size := constraints.Constrain(r.child.Size())
	r.SetSize(size)
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderDecoratedBox) Paint(ctx *layout.PaintContext) {
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
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
		ctx.PopClipRect()
		ctx.Canvas.Restore()
	} else {
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
	}
}

func (r *renderDecoratedBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil && r.child.HitTest(position, result) {
		return true
	}
	result.Add(r)
	return true
}
