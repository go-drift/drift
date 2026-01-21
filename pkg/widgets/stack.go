package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// StackFit determines how children are sized within a Stack.
type StackFit int

const (
	// StackFitLoose allows children to size themselves.
	StackFitLoose StackFit = iota
	// StackFitExpand forces children to fill the stack.
	StackFitExpand
)

// Stack overlays children on top of each other.
//
// Children are painted in order, with the first child at the bottom and
// the last child on top. Hit testing proceeds in reverse (topmost first).
//
// # Sizing Behavior
//
// With StackFitLoose (default), the Stack sizes itself to fit the largest child.
// With StackFitExpand, children are forced to fill the available space.
//
// # Positioning Children
//
// Non-positioned children use the Alignment to determine their position.
// For absolute positioning, wrap children in [Positioned]:
//
//	Stack{
//	    ChildrenWidgets: []core.Widget{
//	        // Background fills the stack
//	        Container{Color: bgColor},
//	        // Badge in top-right corner
//	        Positioned{Top: Ptr(8), Right: Ptr(8), ChildWidget: badge},
//	    },
//	}
type Stack struct {
	// ChildrenWidgets are the widgets to overlay. First child is at the bottom,
	// last child is on top.
	ChildrenWidgets []core.Widget
	// Alignment positions non-Positioned children within the stack.
	// Defaults to top-left (AlignmentTopLeft).
	Alignment layout.Alignment
	// Fit controls how children are sized.
	Fit StackFit
}

// CreateElement returns a RenderObjectElement for this Stack.
func (s Stack) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

// Key returns nil (no key).
func (s Stack) Key() any {
	return nil
}

// Children returns the child widgets.
func (s Stack) Children() []core.Widget {
	return s.ChildrenWidgets
}

// CreateRenderObject creates the RenderStack.
func (s Stack) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	stack := &renderStack{
		alignment: s.Alignment,
		fit:       s.Fit,
	}
	stack.SetSelf(stack)
	return stack
}

// UpdateRenderObject updates the RenderStack.
func (s Stack) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if stack, ok := renderObject.(*renderStack); ok {
		stack.alignment = s.Alignment
		stack.fit = s.Fit
		stack.MarkNeedsLayout()
	}
}

type renderStack struct {
	layout.RenderBoxBase
	children  []layout.RenderBox
	alignment layout.Alignment
	fit       StackFit
}

// SetChildren sets the child render objects.
func (r *renderStack) SetChildren(children []layout.RenderObject) {
	r.children = make([]layout.RenderBox, 0, len(children))
	for _, child := range children {
		if box, ok := child.(layout.RenderBox); ok {
			r.children = append(r.children, box)
		}
	}
}

// VisitChildren calls the visitor for each child.
func (r *renderStack) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		visitor(child)
	}
}

// Layout computes the size of the stack and positions children.
func (r *renderStack) Layout(constraints layout.Constraints) {
	size := layoutStackChildren(r.children, r.fit, r.alignment, constraints)
	r.SetSize(size)
}

// Paint paints all children in order.
func (r *renderStack) Paint(ctx *layout.PaintContext) {
	for _, child := range r.children {
		ctx.PaintChild(child, getChildOffset(child))
	}
}

// HitTest tests children in reverse order (topmost first).
func (r *renderStack) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if hitTestChildrenReverse(r.children, position, result) {
		return true
	}
	result.Add(r)
	return true
}

// layoutStackChildren performs the common layout logic for stack-based widgets.
// It lays out children according to the fit mode and positions them using alignment.
// Positioned children contribute to stack sizing and use alignment for unset axes.
func layoutStackChildren(children []layout.RenderBox, fit StackFit, alignment layout.Alignment, constraints layout.Constraints) rendering.Size {
	var stackWidth, stackHeight float64
	if fit == StackFitExpand {
		stackWidth = constraints.MaxWidth
		stackHeight = constraints.MaxHeight
	}

	// First pass: layout all children to determine stack size.
	// For positioned children, apply explicit Width/Height and single-edge constraints
	// so their sizes contribute correctly to stack sizing.
	// Edge-based stretching (both edges set) will be resolved in the second pass.
	for _, child := range children {
		var childConstraints layout.Constraints
		if fit == StackFitExpand {
			childConstraints = layout.Tight(rendering.Size{Width: stackWidth, Height: stackHeight})
		} else if pos, ok := child.(*renderPositioned); ok {
			childConstraints = positionedFirstPassConstraints(pos, constraints)
		} else {
			childConstraints = layout.Loose(rendering.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight})
		}
		child.Layout(childConstraints)
		childSize := child.Size()
		if childSize.Width > stackWidth {
			stackWidth = childSize.Width
		}
		if childSize.Height > stackHeight {
			stackHeight = childSize.Height
		}
	}

	size := constraints.Constrain(rendering.Size{Width: stackWidth, Height: stackHeight})

	// Second pass: finalize positioned children.
	// Re-layout those that stretch (both edges set), and calculate offsets for all.
	for _, child := range children {
		if pos, ok := child.(*renderPositioned); ok {
			layoutPositionedChild(pos, size, alignment)
		}
	}

	// Third pass: position non-positioned children using alignment
	for _, child := range children {
		if _, ok := child.(*renderPositioned); ok {
			continue // Already positioned
		}
		offset := alignment.WithinRect(
			rendering.RectFromLTWH(0, 0, size.Width, size.Height),
			child.Size(),
		)
		child.SetParentData(&layout.BoxParentData{Offset: offset})
	}

	return size
}

// positionedFirstPassConstraints calculates constraints for a positioned child
// during the first layout pass. Explicit Width/Height are applied as tight constraints.
// Single-edge offsets reduce the max constraint. Edge-based stretching (both edges set)
// uses loose constraints since it depends on final stack size.
func positionedFirstPassConstraints(pos *renderPositioned, constraints layout.Constraints) layout.Constraints {
	var minWidth, maxWidth, minHeight, maxHeight float64

	// Width constraints
	if pos.width != nil {
		// Explicit width - tight constraint
		minWidth = *pos.width
		maxWidth = *pos.width
	} else if pos.left != nil && pos.right != nil {
		// Both edges set - stretching, use loose (will be re-laid out in second pass)
		maxWidth = constraints.MaxWidth
	} else {
		// Loose, reduced by any single edge
		maxWidth = constraints.MaxWidth
		if pos.left != nil {
			maxWidth -= *pos.left
		}
		if pos.right != nil {
			maxWidth -= *pos.right
		}
		if maxWidth < 0 {
			maxWidth = 0
		}
	}

	// Height constraints
	if pos.height != nil {
		// Explicit height - tight constraint
		minHeight = *pos.height
		maxHeight = *pos.height
	} else if pos.top != nil && pos.bottom != nil {
		// Both edges set - stretching, use loose (will be re-laid out in second pass)
		maxHeight = constraints.MaxHeight
	} else {
		// Loose, reduced by any single edge
		maxHeight = constraints.MaxHeight
		if pos.top != nil {
			maxHeight -= *pos.top
		}
		if pos.bottom != nil {
			maxHeight -= *pos.bottom
		}
		if maxHeight < 0 {
			maxHeight = 0
		}
	}

	return layout.Constraints{
		MinWidth:  minWidth,
		MaxWidth:  maxWidth,
		MinHeight: minHeight,
		MaxHeight: maxHeight,
	}
}

// layoutPositionedChild lays out a positioned child within the given stack size.
// It calculates constraints from the position parameters, determines the child's offset,
// and uses alignment for axes where no position is specified.
//
// Note: The first pass already applied explicit Width/Height and single-edge constraints.
// This function only re-layouts when edge-based stretching is needed (both edges set
// without explicit dimension), since stretching depends on final stack size.
func layoutPositionedChild(pos *renderPositioned, stackSize rendering.Size, alignment layout.Alignment) {
	if pos.child == nil {
		pos.SetSize(rendering.Size{})
		return
	}

	// Only re-layout if stretching on either axis (both edges set without explicit dimension)
	stretchesWidth := pos.width == nil && pos.left != nil && pos.right != nil
	stretchesHeight := pos.height == nil && pos.top != nil && pos.bottom != nil

	if stretchesWidth || stretchesHeight {
		childSize := pos.child.Size()
		var minWidth, maxWidth, minHeight, maxHeight float64

		// Width constraints
		if stretchesWidth {
			w := stackSize.Width - *pos.left - *pos.right
			if w < 0 {
				w = 0
			}
			minWidth = w
			maxWidth = w
		} else {
			// Keep current width from first pass
			maxWidth = childSize.Width
		}

		// Height constraints
		if stretchesHeight {
			h := stackSize.Height - *pos.top - *pos.bottom
			if h < 0 {
				h = 0
			}
			minHeight = h
			maxHeight = h
		} else {
			// Keep current height from first pass
			maxHeight = childSize.Height
		}

		childConstraints := layout.Constraints{
			MinWidth:  minWidth,
			MaxWidth:  maxWidth,
			MinHeight: minHeight,
			MaxHeight: maxHeight,
		}
		pos.child.Layout(childConstraints)
	}

	childSize := pos.child.Size()
	pos.SetSize(childSize)

	// Calculate offset based on position parameters.
	// For axes with no position set, use alignment.
	hasHorizontalPosition := pos.left != nil || pos.right != nil
	hasVerticalPosition := pos.top != nil || pos.bottom != nil

	// Compute alignment offset once for unset axes
	var alignedOffset rendering.Offset
	if !hasHorizontalPosition || !hasVerticalPosition {
		alignedOffset = alignment.WithinRect(
			rendering.RectFromLTWH(0, 0, stackSize.Width, stackSize.Height),
			childSize,
		)
	}

	var x, y float64
	if pos.left != nil {
		x = *pos.left
	} else if pos.right != nil {
		x = stackSize.Width - *pos.right - childSize.Width
	} else {
		x = alignedOffset.X
	}

	if pos.top != nil {
		y = *pos.top
	} else if pos.bottom != nil {
		y = stackSize.Height - *pos.bottom - childSize.Height
	} else {
		y = alignedOffset.Y
	}

	pos.child.SetParentData(&layout.BoxParentData{Offset: rendering.Offset{}})
	pos.SetParentData(&layout.BoxParentData{Offset: rendering.Offset{X: x, Y: y}})
}

// hitTestChildrenReverse tests children in reverse order and returns true if any child was hit.
func hitTestChildrenReverse(children []layout.RenderBox, position rendering.Offset, result *layout.HitTestResult) bool {
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		offset := getChildOffset(child)
		local := rendering.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if child.HitTest(local, result) {
			return true
		}
	}
	return false
}

// IndexedStack lays out all children but only paints the active index.
type IndexedStack struct {
	ChildrenWidgets []core.Widget
	Alignment       layout.Alignment
	Fit             StackFit
	Index           int
}

func (s IndexedStack) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s IndexedStack) Key() any {
	return nil
}

func (s IndexedStack) Children() []core.Widget {
	return s.ChildrenWidgets
}

func (s IndexedStack) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	stack := &renderIndexedStack{
		alignment: s.Alignment,
		fit:       s.Fit,
		index:     s.Index,
	}
	stack.SetSelf(stack)
	return stack
}

func (s IndexedStack) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if stack, ok := renderObject.(*renderIndexedStack); ok {
		stack.alignment = s.Alignment
		stack.fit = s.Fit
		stack.index = s.Index
		stack.MarkNeedsLayout()
		stack.MarkNeedsPaint()
	}
}

type renderIndexedStack struct {
	layout.RenderBoxBase
	children  []layout.RenderBox
	alignment layout.Alignment
	fit       StackFit
	index     int
}

func (r *renderIndexedStack) SetChildren(children []layout.RenderObject) {
	r.children = make([]layout.RenderBox, 0, len(children))
	for _, child := range children {
		if box, ok := child.(layout.RenderBox); ok {
			r.children = append(r.children, box)
		}
	}
}

func (r *renderIndexedStack) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		visitor(child)
	}
}

func (r *renderIndexedStack) Layout(constraints layout.Constraints) {
	size := layoutStackChildren(r.children, r.fit, r.alignment, constraints)
	r.SetSize(size)
}

func (r *renderIndexedStack) Paint(ctx *layout.PaintContext) {
	if child := r.activeChild(); child != nil {
		ctx.PaintChild(child, getChildOffset(child))
	}
}

// activeChild returns the currently visible child, or nil if index is out of bounds.
func (r *renderIndexedStack) activeChild() layout.RenderBox {
	if r.index < 0 || r.index >= len(r.children) {
		return nil
	}
	return r.children[r.index]
}

func (r *renderIndexedStack) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	child := r.activeChild()
	if child == nil {
		return false
	}
	offset := getChildOffset(child)
	local := rendering.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	if child.HitTest(local, result) {
		return true
	}
	result.Add(r)
	return true
}

// Positioned positions a child within a Stack using absolute positioning.
//
// # Coordinate System
//
// The coordinate system has its origin at the top-left of the Stack:
//   - Left/Right: Distance from the left/right edge of the Stack
//   - Top/Bottom: Distance from the top/bottom edge of the Stack
//
// # Constraint Combinations
//
// Use pointer fields (nil = unset) to control positioning. The [Ptr] helper
// creates float64 pointers conveniently:
//
//	// Pin to top-left corner with 8pt margins
//	Positioned{Left: Ptr(8), Top: Ptr(8), ChildWidget: icon}
//
//	// Pin to bottom-right corner
//	Positioned{Right: Ptr(16), Bottom: Ptr(16), ChildWidget: fab}
//
//	// Stretch horizontally with fixed vertical position
//	Positioned{Left: Ptr(0), Right: Ptr(0), Top: Ptr(100), ChildWidget: divider}
//
//	// Fixed size at specific position
//	Positioned{Left: Ptr(20), Top: Ptr(20), Width: Ptr(100), Height: Ptr(50), ChildWidget: box}
//
//	// Position only vertically - horizontal uses Stack.Alignment
//	Positioned{Top: Ptr(20), ChildWidget: header}
//
// When both Left and Right are set (or Top and Bottom), the child stretches
// to fill that dimension. Width/Height override the stretching behavior.
//
// For axes where no position is set (neither Left nor Right, or neither Top
// nor Bottom), the child uses the Stack's Alignment for that axis.
type Positioned struct {
	// ChildWidget is the widget to position.
	ChildWidget core.Widget
	// Left is the distance from the left edge of the Stack (nil = unset).
	Left *float64
	// Top is the distance from the top edge of the Stack (nil = unset).
	Top *float64
	// Right is the distance from the right edge of the Stack (nil = unset).
	Right *float64
	// Bottom is the distance from the bottom edge of the Stack (nil = unset).
	Bottom *float64
	// Width overrides the child's width (nil = use child's intrinsic width).
	Width *float64
	// Height overrides the child's height (nil = use child's intrinsic height).
	Height *float64
}

// CreateElement returns a RenderObjectElement for this Positioned.
func (p Positioned) CreateElement() core.Element {
	return core.NewRenderObjectElement(p, nil)
}

// Key returns nil (no key).
func (p Positioned) Key() any {
	return nil
}

// Child returns the child widget.
func (p Positioned) Child() core.Widget {
	return p.ChildWidget
}

// CreateRenderObject creates the RenderPositioned.
func (p Positioned) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	pos := &renderPositioned{
		left:   p.Left,
		top:    p.Top,
		right:  p.Right,
		bottom: p.Bottom,
		width:  p.Width,
		height: p.Height,
	}
	pos.SetSelf(pos)
	return pos
}

// UpdateRenderObject updates the RenderPositioned.
func (p Positioned) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if pos, ok := renderObject.(*renderPositioned); ok {
		pos.left = p.Left
		pos.top = p.Top
		pos.right = p.Right
		pos.bottom = p.Bottom
		pos.width = p.Width
		pos.height = p.Height
		pos.MarkNeedsLayout()
	}
}

type renderPositioned struct {
	layout.RenderBoxBase
	child  layout.RenderBox
	left   *float64
	top    *float64
	right  *float64
	bottom *float64
	width  *float64
	height *float64
}

func (r *renderPositioned) SetChild(child layout.RenderObject) {
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
	}
}

func (r *renderPositioned) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderPositioned) Layout(constraints layout.Constraints) {
	if r.child == nil {
		r.SetSize(rendering.Size{})
		return
	}
	// When used outside a Stack, apply width/height constraints if specified.
	// Position parameters (left/top/right/bottom) only work inside a Stack.
	childConstraints := constraints
	if r.width != nil {
		childConstraints.MinWidth = *r.width
		childConstraints.MaxWidth = *r.width
	}
	if r.height != nil {
		childConstraints.MinHeight = *r.height
		childConstraints.MaxHeight = *r.height
	}
	r.child.Layout(childConstraints)
	r.SetSize(r.child.Size())
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderPositioned) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, getChildOffset(r.child))
	}
}

func (r *renderPositioned) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	offset := getChildOffset(r.child)
	local := rendering.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	return r.child.HitTest(local, result)
}
