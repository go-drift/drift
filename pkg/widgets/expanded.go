package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Expanded makes its child fill all remaining space along the main axis of a
// [Row] or [Column].
//
// After non-flexible children are laid out, remaining space is distributed among
// Expanded children proportionally based on their Flex factor. The default Flex
// is 1; set higher values to allocate more space to specific children.
//
// Expanded is equivalent to [Flexible] with Fit set to [FlexFitTight]. Use
// [Flexible] instead when the child should be allowed to be smaller than its
// allocated space.
//
// Note: With [MainAxisSizeMin], there is no remaining space to fill, so
// Expanded children receive zero space. Using Expanded inside a [ScrollView]
// (unbounded main axis) will panic, since there is no finite space to divide.
//
// # Example
//
// Fill remaining space between fixed-size widgets:
//
//	Row{
//	    Children: []core.Widget{
//	        Icon{...},                                // Fixed size
//	        Expanded{Child: Text{Content: "..."}},   // Fills remaining space
//	        Button{...},                              // Fixed size
//	    },
//	}
//
// # Example with Flex Factors
//
// Distribute space proportionally among multiple Expanded children:
//
//	Row{
//	    Children: []core.Widget{
//	        Expanded{Flex: 1, Child: panelA}, // Gets 1/3 of space
//	        Expanded{Flex: 2, Child: panelB}, // Gets 2/3 of space
//	    },
//	}
type Expanded struct {
	// Child is the widget to expand into the available space.
	Child core.Widget

	// Flex determines the ratio of space allocated to this child relative to
	// other flexible children. Defaults to 1 if not set or <= 0.
	//
	// For example, in a Row with two Expanded children where one has Flex: 1
	// and the other has Flex: 2, the remaining space is split 1:2.
	Flex int
}

// CreateElement returns a RenderObjectElement for this Expanded.
func (e Expanded) CreateElement() core.Element {
	return core.NewRenderObjectElement(e, nil)
}

// Key returns nil (no key).
func (e Expanded) Key() any {
	return nil
}

// ChildWidget returns the child widget.
func (e Expanded) ChildWidget() core.Widget {
	return e.Child
}

// CreateRenderObject creates the renderFlexChild.
func (e Expanded) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderFlexChild{
		flex: e.effectiveFlex(),
		fit:  FlexFitTight, // Expanded is always tight
	}
	r.SetSelf(r)
	return r
}

// UpdateRenderObject updates the renderFlexChild.
func (e Expanded) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderFlexChild); ok {
		r.flex = e.effectiveFlex()
		r.fit = FlexFitTight // Expanded is always tight
		r.MarkNeedsLayout()
	}
}

// effectiveFlex returns the flex factor, defaulting to 1 if not set.
func (e Expanded) effectiveFlex() int {
	if e.Flex <= 0 {
		return 1
	}
	return e.Flex
}

// renderFlexChild is the shared render object for Expanded and Flexible widgets.
// Expanded always uses FlexFitTight; Flexible defaults to FlexFitLoose.
type renderFlexChild struct {
	layout.RenderBoxBase
	child layout.RenderBox
	flex  int
	fit   FlexFit
}

// SetChild sets the child render object.
func (r *renderFlexChild) SetChild(child layout.RenderObject) {
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
func (r *renderFlexChild) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

// PerformLayout lays out the child with the constraints received from the parent Flex.
// The parent Flex (Row/Column) already provides correctly-configured constraints:
// - Main axis: tight or loose depending on fit
// - Cross axis: loose or tight depending on CrossAxisAlignment
// The render object passes these through and sizes itself to match its child.
func (r *renderFlexChild) PerformLayout() {
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

// FlexFactor returns the flex factor for this child, implementing [FlexFactor].
func (r *renderFlexChild) FlexFactor() int {
	return r.flex
}

// FlexFit returns the fit mode for this child, implementing [FlexFitProvider].
func (r *renderFlexChild) FlexFit() FlexFit {
	return r.fit
}

// Paint paints the child.
func (r *renderFlexChild) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
	}
}

// HitTest tests if the position hits this widget.
func (r *renderFlexChild) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
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
