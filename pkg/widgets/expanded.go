package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Expanded expands to fill all available space in the parent constraints.
// When placed inside Row or Column, it consumes remaining space based on Flex.
type Expanded struct {
	ChildWidget core.Widget
	Flex        int
}

// CreateElement returns a RenderObjectElement for this Expanded.
func (e Expanded) CreateElement() core.Element {
	return core.NewRenderObjectElement(e, nil)
}

// Key returns nil (no key).
func (e Expanded) Key() any {
	return nil
}

// Child returns the child widget.
func (e Expanded) Child() core.Widget {
	return e.ChildWidget
}

// CreateRenderObject creates the renderExpanded.
func (e Expanded) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	expanded := &renderExpanded{flex: e.effectiveFlex()}
	expanded.SetSelf(expanded)
	return expanded
}

// UpdateRenderObject updates the renderExpanded.
func (e Expanded) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if expanded, ok := renderObject.(*renderExpanded); ok {
		expanded.flex = e.effectiveFlex()
		expanded.MarkNeedsLayout()
	}
}

// effectiveFlex returns the flex factor, defaulting to 1 if not set.
func (e Expanded) effectiveFlex() int {
	if e.Flex <= 0 {
		return 1
	}
	return e.Flex
}

type renderExpanded struct {
	layout.RenderBoxBase
	child layout.RenderBox
	flex  int
}

// SetChild sets the child render object.
func (r *renderExpanded) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
		setParentOnChild(r.child, r)
	}
}

// VisitChildren calls the visitor for each child.
func (r *renderExpanded) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

// PerformLayout expands to fill available space and constrains child to that size.
func (r *renderExpanded) PerformLayout() {
	constraints := r.Constraints()
	size := constraints.Constrain(rendering.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight})
	r.SetSize(size)

	if r.child != nil {
		r.child.Layout(layout.Tight(size), false) // false: tight constraints, child is boundary
		r.child.SetParentData(&layout.BoxParentData{})
	}
}

func (r *renderExpanded) FlexFactor() int {
	return r.flex
}

// Paint paints the child.
func (r *renderExpanded) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, rendering.Offset{})
	}
}

// HitTest tests if the position hits this widget.
func (r *renderExpanded) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	size := r.Size()
	if position.X < 0 || position.Y < 0 || position.X > size.Width || position.Y > size.Height {
		return false
	}
	if r.child != nil {
		if r.child.HitTest(position, result) {
			return true
		}
	}
	result.Add(r)
	return true
}
