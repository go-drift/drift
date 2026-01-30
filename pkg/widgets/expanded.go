package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// Expanded makes its child fill all remaining space along the main axis of a
// [Row] or [Column].
//
// After non-flexible children are laid out, remaining space is distributed among
// Expanded children proportionally based on their Flex factor. The default Flex
// is 1; set higher values to allocate more space to specific children.
//
// Important: The parent Row or Column must have MainAxisSizeMax for Expanded to
// receive any space. With MainAxisSizeMin, there is no remaining space to fill.
//
// Example:
//
//	Row{
//	    MainAxisSize: MainAxisSizeMax,
//	    ChildrenWidgets: []core.Widget{
//	        Icon{...},                                    // Fixed size
//	        Expanded{ChildWidget: Text{Content: "..."}},  // Fills remaining space
//	        Button{...},                                   // Fixed size
//	    },
//	}
//
// Example with different flex factors:
//
//	Row{
//	    MainAxisSize: MainAxisSizeMax,
//	    ChildrenWidgets: []core.Widget{
//	        Expanded{Flex: 1, ChildWidget: panelA}, // Gets 1/3 of space
//	        Expanded{Flex: 2, ChildWidget: panelB}, // Gets 2/3 of space
//	    },
//	}
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

// PerformLayout lays out the child with the constraints received from the parent Flex.
// The parent Flex (Row/Column) already provides correctly-configured constraints:
// - Main axis: tight to the allocated flex space
// - Cross axis: loose or tight depending on CrossAxisAlignment
// Expanded passes these through and sizes itself to match its child.
func (r *renderExpanded) PerformLayout() {
	constraints := r.Constraints()

	if r.child != nil {
		// Pass through constraints from parent Flex - they're already set up correctly
		r.child.Layout(constraints, true)
		// Clamp to constraints in case a child misbehaves.
		r.SetSize(constraints.Constrain(r.child.Size()))
		r.child.SetParentData(&layout.BoxParentData{})
	} else {
		// No child: take minimum size that satisfies constraints
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderExpanded) FlexFactor() int {
	return r.flex
}

// Paint paints the child.
func (r *renderExpanded) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, graphics.Offset{})
	}
}

// HitTest tests if the position hits this widget.
func (r *renderExpanded) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
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
