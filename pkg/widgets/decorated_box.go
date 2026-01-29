package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// DecoratedBox paints a background, border, and shadow behind its child.
//
// DecoratedBox applies decorations in this order:
//  1. Shadow (drawn behind, naturally overflows bounds)
//  2. Background color or gradient (overflow controlled by Overflow field)
//  3. Border stroke (drawn on top of background, supports dashing)
//  4. Child widget
//
// Use BorderRadius for rounded corners. When combined with gradients, the
// Overflow field controls whether the gradient extends beyond bounds:
//   - [OverflowClip] (default): gradient confined to bounds (or rounded shape)
//   - [OverflowVisible]: gradient can overflow, useful for glows
//
// For simpler use cases without borders or rounded corners, see [Container].
type DecoratedBox struct {
	ChildWidget core.Widget // Child widget to display inside the decoration

	// Background
	Color    graphics.Color    // Background fill color
	Gradient *graphics.Gradient // Background gradient; overrides Color if set

	// Border
	BorderColor  graphics.Color            // Border stroke color; transparent = no border
	BorderWidth  float64                    // Border stroke width in pixels; 0 = no border
	BorderRadius float64                    // Corner radius for rounded rectangles; 0 = sharp corners
	BorderDash   *graphics.DashPattern // Dash pattern for border; nil = solid line

	// Effects
	Shadow *graphics.BoxShadow // Drop shadow drawn behind the box; nil = no shadow

	// Overflow controls whether gradients extend beyond widget bounds.
	// Defaults to OverflowClip, confining gradients strictly within bounds
	// (clipped to rounded shape if BorderRadius > 0). Set to OverflowVisible
	// for glow effects where the gradient should extend beyond the widget.
	// Only affects gradients; shadows overflow naturally and solid colors
	// never overflow.
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
		color:        color,
		gradient:     d.Gradient,
		borderColor:  d.BorderColor,
		borderWidth:  d.BorderWidth,
		borderRadius: d.BorderRadius,
		borderDash:   d.BorderDash,
		shadow:       d.Shadow,
		overflow:     d.Overflow,
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
		box.color = color
		box.gradient = d.Gradient
		box.borderColor = d.BorderColor
		box.borderWidth = d.BorderWidth
		box.borderRadius = d.BorderRadius
		box.borderDash = d.BorderDash
		box.shadow = d.Shadow
		box.overflow = d.Overflow
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderDecoratedBox struct {
	layout.RenderBoxBase
	child        layout.RenderBox
	color        graphics.Color
	gradient     *graphics.Gradient
	borderColor  graphics.Color
	borderWidth  float64
	borderRadius float64
	borderDash   *graphics.DashPattern
	shadow       *graphics.BoxShadow
	overflow     Overflow
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
	if r.shadow != nil {
		r.drawShadow(ctx, rect, *r.shadow)
	}
	if r.color != graphics.ColorTransparent || r.gradient != nil {
		paint := graphics.DefaultPaint()
		paint.Color = r.color
		paint.Gradient = r.gradient

		if r.overflow == OverflowClip {
			ctx.Canvas.Save()
			if r.borderRadius > 0 {
				rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(r.borderRadius))
				ctx.Canvas.ClipRRect(rrect)
			} else {
				ctx.Canvas.ClipRect(rect)
			}
			r.drawShape(ctx, rect, paint)
			ctx.Canvas.Restore()
		} else if r.gradient != nil {
			// OverflowVisible with gradient: draw expanded rect for overflow,
			// then draw the normal shape on top for rounded corners in-bounds.
			// Note: This doubles gradient paint work when borderRadius > 0.
			// Use OverflowClip if performance is critical for large gradients.
			// To reduce the double-draw cost, the next step would be a shader/Skia
			// change to let a gradient draw beyond bounds without re-drawing the
			// in-bounds area.
			drawRect := r.gradient.Bounds(rect)
			ctx.Canvas.DrawRect(drawRect, paint)
			if r.borderRadius > 0 {
				rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(r.borderRadius))
				ctx.Canvas.DrawRRect(rrect, paint)
			}
		} else {
			// No gradient: use normal shape (respects border radius)
			r.drawShape(ctx, rect, paint)
		}
	}
	if r.borderWidth > 0 && r.borderColor != graphics.ColorTransparent {
		borderPaint := graphics.DefaultPaint()
		borderPaint.Color = r.borderColor
		borderPaint.Style = graphics.PaintStyleStroke
		borderPaint.StrokeWidth = r.borderWidth
		borderPaint.Dash = r.borderDash
		r.drawShape(ctx, rect, borderPaint)
	}
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
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

func (r *renderDecoratedBox) drawShape(ctx *layout.PaintContext, rect graphics.Rect, paint graphics.Paint) {
	if r.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRect(rrect, paint)
		return
	}
	ctx.Canvas.DrawRect(rect, paint)
}

func (r *renderDecoratedBox) drawShadow(ctx *layout.PaintContext, rect graphics.Rect, shadow graphics.BoxShadow) {
	if r.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRectShadow(rrect, shadow)
		return
	}
	ctx.Canvas.DrawRectShadow(rect, shadow)
}
