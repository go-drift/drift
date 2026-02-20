package widgets

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// decorationPainter provides shared painting logic for Container and DecoratedBox.
type decorationPainter struct {
	color          graphics.Color
	gradient       *graphics.Gradient
	borderColor    graphics.Color
	borderWidth    float64
	borderRadius   float64
	borderDash     *graphics.DashPattern
	borderGradient *graphics.Gradient
	shadow         *graphics.BoxShadow
	overflow       Overflow
}

// paint draws the decoration (shadow, background, border) within the given rect.
func (p *decorationPainter) paint(ctx *layout.PaintContext, rect graphics.Rect) {
	// Draw outer shadow (before background so it appears behind)
	if p.shadow != nil && p.shadow.BlurStyle != graphics.BlurStyleInner {
		p.drawShadow(ctx, rect, *p.shadow)
	}

	// Draw background (color or gradient)
	if p.color != graphics.ColorTransparent || p.gradient != nil {
		paint := graphics.DefaultPaint()
		paint.Color = p.color
		paint.Gradient = p.gradient

		if p.overflow == OverflowClip {
			ctx.Canvas.Save()
			if p.borderRadius > 0 {
				rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(p.borderRadius))
				ctx.Canvas.ClipRRect(rrect)
			} else {
				ctx.Canvas.ClipRect(rect)
			}
			p.drawShape(ctx, rect, paint)
			ctx.Canvas.Restore()
		} else if p.gradient != nil {
			// OverflowVisible with gradient: draw expanded rect for overflow,
			// then draw the normal shape on top for rounded corners in-bounds.
			// Set GradientBounds so the gradient is resolved against the original
			// widget bounds, not the expanded drawing area.
			drawRect := p.gradient.Bounds(rect)
			paint.GradientBounds = &rect
			ctx.Canvas.DrawRect(drawRect, paint)
			if p.borderRadius > 0 {
				rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(p.borderRadius))
				ctx.Canvas.DrawRRect(rrect, paint)
			}
		} else {
			p.drawShape(ctx, rect, paint)
		}
	}

	// Draw inner shadow (after background so it appears on top)
	if p.shadow != nil && p.shadow.BlurStyle == graphics.BlurStyleInner {
		p.drawShadow(ctx, rect, *p.shadow)
	}

	// Draw border
	if p.borderWidth > 0 && (p.borderColor != graphics.ColorTransparent || p.borderGradient != nil) {
		borderPaint := graphics.DefaultPaint()
		borderColor := p.borderColor
		if p.borderGradient != nil && borderColor == graphics.ColorTransparent {
			borderColor = graphics.ColorWhite
		}
		borderPaint.Color = borderColor
		borderPaint.Gradient = p.borderGradient
		borderPaint.Style = graphics.PaintStyleStroke
		borderPaint.StrokeWidth = p.borderWidth
		borderPaint.Dash = p.borderDash
		p.drawShape(ctx, rect, borderPaint)
	}
}

func (p *decorationPainter) drawShape(ctx *layout.PaintContext, rect graphics.Rect, paint graphics.Paint) {
	if p.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(p.borderRadius))
		ctx.Canvas.DrawRRect(rrect, paint)
		return
	}
	ctx.Canvas.DrawRect(rect, paint)
}

func (p *decorationPainter) drawShadow(ctx *layout.PaintContext, rect graphics.Rect, shadow graphics.BoxShadow) {
	if p.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(p.borderRadius))
		ctx.Canvas.DrawRRectShadow(rrect, shadow)
		return
	}
	ctx.Canvas.DrawRectShadow(rect, shadow)
}

// occlusionPath returns a path representing the shape this decoration paints
// within the given rect. Used by parent widgets (Stack, Overlay) to emit
// shape-accurate occlusion masks for platform views.
func (p *decorationPainter) occlusionPath(rect graphics.Rect) *graphics.Path {
	mask := graphics.NewPath()
	if p.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(p.borderRadius))
		mask.AddRRect(rrect)
	} else {
		mask.AddRect(rect)
	}
	return mask
}

// shouldClipChildren reports whether children should be clipped to the
// decoration bounds. Returns true when overflow is OverflowClip.
func (p *decorationPainter) shouldClipChildren() bool {
	return p.overflow == OverflowClip
}

// applyChildClip applies the appropriate clip for children based on border radius.
// Uses a rounded rect clip when borderRadius > 0, otherwise a regular rect clip.
// The caller must call ctx.PopClipRect() and ctx.Canvas.Restore() after painting children.
//
// Note: Platform views (native text fields, etc.) are clipped to the rectangular
// bounds only, not the rounded shape. This is a platform limitation.
func (p *decorationPainter) applyChildClip(ctx *layout.PaintContext, rect graphics.Rect) {
	ctx.Canvas.Save()
	if p.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(p.borderRadius))
		ctx.Canvas.ClipRRect(rrect)
	} else {
		ctx.Canvas.ClipRect(rect)
	}
	ctx.PushClipRect(rect) // platform views clip to rect only (no rounded corners)
}
