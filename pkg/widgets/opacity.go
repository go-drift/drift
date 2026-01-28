package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// Opacity applies transparency to its child widget.
//
// The Opacity value should be between 0.0 (fully transparent) and 1.0 (fully opaque).
// When Opacity is 0.0, the child is not painted at all.
// When Opacity is 1.0, the child is painted normally without any performance overhead.
// Intermediate values use SaveLayerAlpha for proper alpha compositing.
//
// Note: The layer bounds are based on this widget's size. Children that paint
// outside their bounds (e.g., via transforms or overflow) may be clipped.
type Opacity struct {
	// Opacity is the transparency value (0.0 to 1.0).
	Opacity float64
	// ChildWidget is the widget to which opacity is applied.
	ChildWidget core.Widget
}

// OpacityOf creates an opacity widget with the given opacity value and child.
// The opacity value should be between 0.0 (fully transparent) and 1.0 (fully opaque).
// This is a convenience helper equivalent to:
//
//	Opacity{Opacity: opacity, ChildWidget: child}
func OpacityOf(opacity float64, child core.Widget) Opacity {
	return Opacity{Opacity: opacity, ChildWidget: child}
}

func (o Opacity) CreateElement() core.Element {
	return core.NewRenderObjectElement(o, nil)
}

func (o Opacity) Key() any {
	return nil
}

func (o Opacity) Child() core.Widget {
	return o.ChildWidget
}

func (o Opacity) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderOpacity{
		opacity: o.Opacity,
	}
	box.SetSelf(box)
	return box
}

func (o Opacity) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderOpacity); ok {
		box.opacity = o.Opacity
		box.MarkNeedsPaint()
	}
}

type renderOpacity struct {
	layout.RenderBoxBase
	child   layout.RenderBox
	opacity float64
}

// IsRepaintBoundary returns true when opacity uses SaveLayerAlpha.
func (r *renderOpacity) IsRepaintBoundary() bool {
	return r.opacity > 0 && r.opacity < 1
}

func (r *renderOpacity) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderOpacity) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderOpacity) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderOpacity) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	// Skip painting entirely when fully transparent
	if r.opacity <= 0 {
		return
	}
	// Paint normally when fully opaque (no layer overhead)
	if r.opacity >= 1 {
		ctx.PaintChild(r.child, getChildOffset(r.child))
		return
	}
	// Use SaveLayerAlpha for intermediate opacity values
	size := r.Size()
	bounds := graphics.RectFromLTWH(0, 0, size.Width, size.Height)
	ctx.Canvas.SaveLayerAlpha(bounds, r.opacity)
	ctx.PaintChild(r.child, getChildOffset(r.child))
	ctx.Canvas.Restore()
}

func (r *renderOpacity) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	// Don't respond to hit tests when fully transparent
	if r.opacity <= 0 {
		return false
	}
	if r.child != nil {
		offset := getChildOffset(r.child)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if r.child.HitTest(local, result) {
			return true
		}
	}
	return false
}
