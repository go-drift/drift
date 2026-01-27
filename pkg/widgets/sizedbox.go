package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// SizedBox constrains its child to a specific width and/or height.
//
// When both Width and Height are set, SizedBox forces those exact dimensions
// (constrained by parent). When only one dimension is set, the other uses
// the child's intrinsic size.
//
// Common uses:
//
//	// Fixed-size box
//	SizedBox{Width: 100, Height: 50, ChildWidget: child}
//
//	// Horizontal spacer in a Row
//	SizedBox{Width: 16}
//
//	// Vertical spacer in a Column
//	SizedBox{Height: 24}
//
//	// Force child to specific width only
//	SizedBox{Width: 200, ChildWidget: textField}
//
// For convenience, use [HSpace] and [VSpace] helper functions for spacers.
type SizedBox struct {
	Width       float64
	Height      float64
	ChildWidget core.Widget
}

func (s SizedBox) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s SizedBox) Key() any {
	return nil
}

func (s SizedBox) Child() core.Widget {
	return s.ChildWidget
}

func (s SizedBox) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderSizedBox{width: s.Width, height: s.Height}
	box.SetSelf(box)
	return box
}

func (s SizedBox) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderSizedBox); ok {
		box.width = s.Width
		box.height = s.Height
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderSizedBox struct {
	layout.RenderBoxBase
	child  layout.RenderBox
	width  float64
	height float64
}

func (r *renderSizedBox) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderSizedBox) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderSizedBox) PerformLayout() {
	constraints := r.Constraints()
	// Build desired size from explicit dimensions
	desired := rendering.Size{Width: r.width, Height: r.height}

	if r.child == nil {
		r.SetSize(constraints.Constrain(desired))
		return
	}

	// Constrain to parent bounds
	constrained := constraints.Constrain(desired)

	// Build child constraints, tightening only explicit dimensions
	childConstraints := constraints
	if r.width > 0 {
		childConstraints.MinWidth = constrained.Width
		childConstraints.MaxWidth = constrained.Width
	}
	if r.height > 0 {
		childConstraints.MinHeight = constrained.Height
		childConstraints.MaxHeight = constrained.Height
	}

	r.child.Layout(childConstraints, true) // true: we read child.Size()
	r.child.SetParentData(&layout.BoxParentData{})

	// Final size uses explicit dimensions where specified, child size otherwise
	finalSize := r.child.Size()
	if r.width > 0 {
		finalSize.Width = constrained.Width
	}
	if r.height > 0 {
		finalSize.Height = constrained.Height
	}
	r.SetSize(constraints.Constrain(finalSize))
}

func (r *renderSizedBox) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, rendering.Offset{})
	}
}

func (r *renderSizedBox) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil && r.child.HitTest(position, result) {
		return true
	}
	result.Add(r)
	return true
}
