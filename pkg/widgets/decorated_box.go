package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// DecoratedBox paints a background and border behind its child.
type DecoratedBox struct {
	ChildWidget  core.Widget
	Color        rendering.Color
	Gradient     *rendering.Gradient
	BorderColor  rendering.Color
	BorderWidth  float64
	BorderRadius float64
	Shadow       *rendering.BoxShadow
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
	if d.Gradient != nil && color == rendering.ColorTransparent {
		color = rendering.ColorWhite
	}
	box := &renderDecoratedBox{
		color:        color,
		gradient:     d.Gradient,
		borderColor:  d.BorderColor,
		borderWidth:  d.BorderWidth,
		borderRadius: d.BorderRadius,
		shadow:       d.Shadow,
	}
	box.SetSelf(box)
	return box
}

func (d DecoratedBox) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderDecoratedBox); ok {
		color := d.Color
		if d.Gradient != nil && color == rendering.ColorTransparent {
			color = rendering.ColorWhite
		}
		box.color = color
		box.gradient = d.Gradient
		box.borderColor = d.BorderColor
		box.borderWidth = d.BorderWidth
		box.borderRadius = d.BorderRadius
		box.shadow = d.Shadow
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderDecoratedBox struct {
	layout.RenderBoxBase
	child        layout.RenderBox
	color        rendering.Color
	gradient     *rendering.Gradient
	borderColor  rendering.Color
	borderWidth  float64
	borderRadius float64
	shadow       *rendering.BoxShadow
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
		r.SetSize(constraints.Constrain(rendering.Size{}))
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
	rect := rendering.RectFromLTWH(0, 0, size.Width, size.Height)
	if r.shadow != nil {
		r.drawShadow(ctx, rect, *r.shadow)
	}
	if r.color != rendering.ColorTransparent || r.gradient != nil {
		paint := rendering.DefaultPaint()
		paint.Color = r.color
		paint.Gradient = r.gradient
		r.drawShape(ctx, rect, paint)
	}
	if r.borderWidth > 0 && r.borderColor != rendering.ColorTransparent {
		borderPaint := rendering.DefaultPaint()
		borderPaint.Color = r.borderColor
		borderPaint.Style = rendering.PaintStyleStroke
		borderPaint.StrokeWidth = r.borderWidth
		r.drawShape(ctx, rect, borderPaint)
	}
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
}

func (r *renderDecoratedBox) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil && r.child.HitTest(position, result) {
		return true
	}
	result.Add(r)
	return true
}

func (r *renderDecoratedBox) drawShape(ctx *layout.PaintContext, rect rendering.Rect, paint rendering.Paint) {
	if r.borderRadius > 0 {
		rrect := rendering.RRectFromRectAndRadius(rect, rendering.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRect(rrect, paint)
		return
	}
	ctx.Canvas.DrawRect(rect, paint)
}

func (r *renderDecoratedBox) drawShadow(ctx *layout.PaintContext, rect rendering.Rect, shadow rendering.BoxShadow) {
	if r.borderRadius > 0 {
		rrect := rendering.RRectFromRectAndRadius(rect, rendering.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRectShadow(rrect, shadow)
		return
	}
	ctx.Canvas.DrawRectShadow(rect, shadow)
}
