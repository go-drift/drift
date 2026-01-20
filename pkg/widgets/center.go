package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Center centers its child within the available space.
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
	r.child = setChildFromRenderObject(child)
}

func (r *renderCenter) Layout(constraints layout.Constraints) {
	size := constraints.Constrain(rendering.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight})
	r.SetSize(size)
	if r.child != nil {
		r.child.Layout(layout.Loose(size))
		childSize := r.child.Size()
		offset := layout.AlignmentCenter.WithinRect(
			rendering.RectFromLTWH(0, 0, size.Width, size.Height),
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

func (r *renderCenter) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	offset := getChildOffset(r.child)
	local := rendering.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	if r.child != nil && r.child.HitTest(local, result) {
		return true
	}
	result.Add(r)
	return true
}
