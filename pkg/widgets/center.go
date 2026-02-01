package widgets

import (
	"math"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Center positions its child at the center of the available space.
//
// Center expands to fill available space (like [Expanded]), then centers
// the child within that space. The child is given loose constraints,
// allowing it to size itself.
//
// Example:
//
//	Center{ChildWidget: Text{Content: "Hello, World!"}}
//
// For more control over alignment, use [Container] with an Alignment field,
// or wrap the child in an [Align] widget.
type Center struct {
	ChildWidget core.Widget
}

func (c Center) CreateElement() core.Element {
	return core.NewRenderObjectElement(c, nil)
}

func (c Center) Key() any {
	return nil
}

func (c Center) Child() core.Widget {
	return c.ChildWidget
}

func (c Center) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	center := &renderCenter{}
	center.SetSelf(center)
	return center
}

func (c Center) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {}

type renderCenter struct {
	layout.RenderBoxBase
	child layout.RenderBox
}

func (r *renderCenter) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderCenter) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderCenter) PerformLayout() {
	constraints := r.Constraints()

	// Handle unbounded constraints by measuring child first
	targetWidth := constraints.MaxWidth
	targetHeight := constraints.MaxHeight
	childAlreadyLaidOut := false

	if r.child != nil && (targetWidth == math.MaxFloat64 || targetHeight == math.MaxFloat64) {
		// Measure child with loose constraints to get intrinsic size
		r.child.Layout(layout.Loose(graphics.Size{Width: targetWidth, Height: targetHeight}), true)
		childSize := r.child.Size()
		if targetWidth == math.MaxFloat64 {
			targetWidth = childSize.Width
		}
		if targetHeight == math.MaxFloat64 {
			targetHeight = childSize.Height
		}
		// If both dimensions were unbounded, child is already laid out with correct constraints
		if constraints.MaxWidth == math.MaxFloat64 && constraints.MaxHeight == math.MaxFloat64 {
			childAlreadyLaidOut = true
		}
	}

	size := constraints.Constrain(graphics.Size{Width: targetWidth, Height: targetHeight})
	r.SetSize(size)

	if r.child != nil {
		// Only re-layout if constraints changed from initial measurement
		if !childAlreadyLaidOut {
			r.child.Layout(layout.Loose(size), true)
		}
		childSize := r.child.Size()
		offset := layout.AlignmentCenter.WithinRect(
			graphics.RectFromLTWH(0, 0, size.Width, size.Height),
			childSize,
		)
		r.child.SetParentData(&layout.BoxParentData{Offset: offset})
	}
}

func (r *renderCenter) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
	}
}

func (r *renderCenter) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
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
