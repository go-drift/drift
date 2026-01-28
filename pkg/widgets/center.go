package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
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
	size := constraints.Constrain(graphics.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight})
	r.SetSize(size)
	if r.child != nil {
		r.child.Layout(layout.Loose(size), true) // true: we read child.Size()
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
		ctx.PaintChild(r.child, getChildOffset(r.child))
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
