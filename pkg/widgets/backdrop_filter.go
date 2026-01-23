package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// BackdropFilter applies a blur effect to content behind this widget.
// The blur is applied within the widget's bounds and affects any content
// drawn before this widget in the compositing order.
type BackdropFilter struct {
	ChildWidget core.Widget
	SigmaX      float64
	SigmaY      float64
}

// NewBackdropFilter creates a BackdropFilter with uniform blur in both directions.
func NewBackdropFilter(sigma float64, child core.Widget) BackdropFilter {
	return BackdropFilter{
		ChildWidget: child,
		SigmaX:      sigma,
		SigmaY:      sigma,
	}
}

func (b BackdropFilter) CreateElement() core.Element {
	return core.NewRenderObjectElement(b, nil)
}

func (b BackdropFilter) Key() any {
	return nil
}

func (b BackdropFilter) Child() core.Widget {
	return b.ChildWidget
}

func (b BackdropFilter) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderBackdropFilter{
		sigmaX: b.SigmaX,
		sigmaY: b.SigmaY,
	}
	box.SetSelf(box)
	return box
}

func (b BackdropFilter) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderBackdropFilter); ok {
		box.sigmaX = b.SigmaX
		box.sigmaY = b.SigmaY
		box.MarkNeedsPaint()
	}
}

type renderBackdropFilter struct {
	layout.RenderBoxBase
	child  layout.RenderBox
	sigmaX float64
	sigmaY float64
}

func (r *renderBackdropFilter) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderBackdropFilter) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderBackdropFilter) PerformLayout() {
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

func (r *renderBackdropFilter) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	bounds := rendering.RectFromLTWH(0, 0, size.Width, size.Height)
	ctx.Canvas.Save()
	ctx.Canvas.ClipRect(bounds)
	ctx.Canvas.SaveLayerBlur(bounds, r.sigmaX, r.sigmaY)
	ctx.Canvas.Restore() // apply blur to backdrop
	// Paint child on top (unblurred)
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
	ctx.Canvas.Restore() // clip
}

func (r *renderBackdropFilter) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil && r.child.HitTest(position, result) {
		return true
	}
	result.Add(r)
	return true
}
