package testbed

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// LayoutBox is a fixed-size colored box for layout testing.
type LayoutBox struct {
	Width  float64
	Height float64
	Color  graphics.Color
}

func (b LayoutBox) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (b LayoutBox) Key() any { return nil }

func (b LayoutBox) CreateRenderObject(_ core.BuildContext) layout.RenderObject {
	ro := &renderLayoutBox{
		width:  b.Width,
		height: b.Height,
		color:  b.Color,
	}
	ro.SetSelf(ro)
	return ro
}

func (b LayoutBox) UpdateRenderObject(_ core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderLayoutBox); ok {
		box.width = b.Width
		box.height = b.Height
		box.color = b.Color
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderLayoutBox struct {
	layout.RenderBoxBase
	width  float64
	height float64
	color  graphics.Color
}

func (r *renderLayoutBox) PerformLayout() {
	constraints := r.Constraints()
	r.SetSize(constraints.Constrain(graphics.Size{Width: r.width, Height: r.height}))
}

func (r *renderLayoutBox) Paint(ctx *layout.PaintContext) {
	if r.color != 0 {
		ctx.Canvas.DrawRect(
			graphics.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height),
			graphics.Paint{Color: r.color},
		)
	}
}

func (r *renderLayoutBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	size := r.Size()
	if position.X >= 0 && position.Y >= 0 && position.X <= size.Width && position.Y <= size.Height {
		result.Add(r)
		return true
	}
	return false
}
