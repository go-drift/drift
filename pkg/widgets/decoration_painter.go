package widgets

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// decorationPainter provides shared painting logic for Container and DecoratedBox.
type decorationPainter struct {
	color        graphics.Color
	gradient     *graphics.Gradient
	borderColor  graphics.Color
	borderWidth  float64
	borderRadius float64
	borderDash   *graphics.DashPattern
	shadow       *graphics.BoxShadow
	overflow     Overflow
}

// paint draws the decoration (shadow, background, border) within the given rect.
func (p *decorationPainter) paint(ctx *layout.PaintContext, rect graphics.Rect) {
	// Draw shadow
	if p.shadow != nil {
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

	// Draw border
	if p.borderWidth > 0 && p.borderColor != graphics.ColorTransparent {
		borderPaint := graphics.DefaultPaint()
		borderPaint.Color = p.borderColor
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
