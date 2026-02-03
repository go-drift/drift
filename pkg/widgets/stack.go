package widgets

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// StackFit determines how children are sized within a Stack.
type StackFit int

const (
	// StackFitLoose allows children to size themselves.
	StackFitLoose StackFit = iota
	// StackFitExpand forces children to fill the stack.
	StackFitExpand
)

// String returns a human-readable representation of the stack fit.
func (f StackFit) String() string {
	switch f {
	case StackFitLoose:
		return "loose"
	case StackFitExpand:
		return "expand"
	default:
		return fmt.Sprintf("StackFit(%d)", int(f))
	}
}

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
//	    Children: []core.Widget{
//	        // Background fills the stack
//	        Container{Color: bgColor},
//	        // Badge in top-right corner
//	        Positioned{Top: Ptr(8), Right: Ptr(8), Child: badge},
//	    },
//	}
type Stack struct {
	// Children are the widgets to overlay. First child is at the bottom,
	// last child is on top.
	Children []core.Widget
	// Alignment positions non-Positioned children within the stack.
	// Defaults to top-left (AlignmentTopLeft).
	Alignment layout.Alignment
	// Fit controls how children are sized.
	Fit StackFit
}

// StackOf creates a stack with the given children.
// This is a convenience helper for the common case of creating a Stack with children.
// Children are layered with the first child at the bottom and last child on top.
func StackOf(children ...core.Widget) Stack {
	return Stack{Children: children}
}

// CreateElement returns a RenderObjectElement for this Stack.
func (s Stack) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

// Key returns nil (no key).
func (s Stack) Key() any {
	return nil
}

// ChildrenWidgets returns the child widgets.
func (s Stack) ChildrenWidgets() []core.Widget {
	return s.Children
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
	// Clear parent on old children
	for _, child := range r.children {
		setParentOnChild(child, nil)
	}
	r.children = make([]layout.RenderBox, 0, len(children))
	for _, child := range children {
		if box, ok := child.(layout.RenderBox); ok {
			r.children = append(r.children, box)
			setParentOnChild(box, r)
		}
	}
}

// VisitChildren calls the visitor for each child.
func (r *renderStack) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		visitor(child)
	}
}

// PerformLayout computes the size of the stack and positions children.
func (r *renderStack) PerformLayout() {
	constraints := r.Constraints()
	size := layoutStackChildren(r.children, r.fit, r.alignment, constraints)
	r.SetSize(size)
}

// Paint paints all children in order.
func (r *renderStack) Paint(ctx *layout.PaintContext) {
	for _, child := range r.children {
		ctx.PaintChildWithLayer(child, getChildOffset(child))
	}
}

// HitTest tests children in reverse order (topmost first).
func (r *renderStack) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
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
func layoutStackChildren(children []layout.RenderBox, fit StackFit, alignment layout.Alignment, constraints layout.Constraints) graphics.Size {
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
			childConstraints = layout.Tight(graphics.Size{Width: stackWidth, Height: stackHeight})
		} else if pos, ok := child.(*renderPositioned); ok {
			childConstraints = positionedFirstPassConstraints(pos, constraints)
		} else {
			childConstraints = layout.Loose(graphics.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight})
		}
		child.Layout(childConstraints, true) // true: we read child.Size()
		childSize := child.Size()
		if childSize.Width > stackWidth {
			stackWidth = childSize.Width
		}
		if childSize.Height > stackHeight {
			stackHeight = childSize.Height
		}
	}

	size := constraints.Constrain(graphics.Size{Width: stackWidth, Height: stackHeight})

	// Second pass: finalize positioned children.
	// Re-layout those that stretch (both edges set), and calculate offsets for all.
	for _, child := range children {
		if pos, ok := child.(*renderPositioned); ok {
			layoutPositionedChild(pos, size, alignment, fit == StackFitExpand)
		}
	}

	// Third pass: position non-positioned children using alignment
	for _, child := range children {
		if _, ok := child.(*renderPositioned); ok {
			continue // Already positioned
		}
		offset := alignment.WithinRect(
			graphics.RectFromLTWH(0, 0, size.Width, size.Height),
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
//
// When alignment mode is used (pos.alignment != nil), Left/Right/Top/Bottom are
// positional offsets rather than edge distances, so they don't affect constraints.
func positionedFirstPassConstraints(pos *renderPositioned, constraints layout.Constraints) layout.Constraints {
	var minWidth, maxWidth, minHeight, maxHeight float64

	// In alignment mode, edges are offsets not constraints - use loose constraints
	// (only Width/Height apply as sizing constraints)
	if pos.alignment != nil {
		if pos.width != nil {
			minWidth = *pos.width
			maxWidth = *pos.width
		} else {
			maxWidth = constraints.MaxWidth
		}
		if pos.height != nil {
			minHeight = *pos.height
			maxHeight = *pos.height
		} else {
			maxHeight = constraints.MaxHeight
		}
		return layout.Constraints{
			MinWidth:  minWidth,
			MaxWidth:  maxWidth,
			MinHeight: minHeight,
			MaxHeight: maxHeight,
		}
	}

	// Absolute positioning mode: edges affect constraints

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
// This function re-layouts when:
// - Alignment mode is used and relayoutAlignment is true (child should size naturally, not fill stack)
// - Edge-based stretching is needed (both edges set without explicit dimension)
func layoutPositionedChild(pos *renderPositioned, stackSize graphics.Size, stackAlignment layout.Alignment, relayoutAlignment bool) {
	if pos.child == nil {
		pos.SetSize(graphics.Size{})
		return
	}

	// For alignment mode, re-layout with loose constraints so child sizes naturally.
	// The first pass may have applied tight constraints (when StackFitExpand),
	// but alignment mode expects the child to use its intrinsic size.
	if relayoutAlignment && pos.alignment != nil {
		var minWidth, maxWidth, minHeight, maxHeight float64
		if pos.width != nil {
			minWidth = *pos.width
			maxWidth = *pos.width
		} else {
			maxWidth = stackSize.Width
		}
		if pos.height != nil {
			minHeight = *pos.height
			maxHeight = *pos.height
		} else {
			maxHeight = stackSize.Height
		}
		childConstraints := layout.Constraints{
			MinWidth:  minWidth,
			MaxWidth:  maxWidth,
			MinHeight: minHeight,
			MaxHeight: maxHeight,
		}
		pos.child.Layout(childConstraints, true)
	}

	// Only re-layout if stretching on either axis (both edges set without explicit dimension)
	// Note: Stretching only applies in absolute positioning mode (no alignment)
	stretchesWidth := pos.alignment == nil && pos.width == nil && pos.left != nil && pos.right != nil
	stretchesHeight := pos.alignment == nil && pos.height == nil && pos.top != nil && pos.bottom != nil

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
		pos.child.Layout(childConstraints, true) // true: we read child.Size()
	}

	childSize := pos.child.Size()
	pos.SetSize(childSize)

	var x, y float64

	// If Alignment is set, use relative positioning
	if pos.alignment != nil {
		// Resolve alignment to get the anchor point within the stack
		stackBounds := graphics.RectFromLTWH(0, 0, stackSize.Width, stackSize.Height)
		anchorPoint := pos.alignment.Resolve(stackBounds)

		// Position child centered on anchor point, then apply offsets
		x = anchorPoint.X - childSize.Width/2
		y = anchorPoint.Y - childSize.Height/2

		// Apply offsets from the alignment position
		// Left/Top are positive offsets, Right/Bottom are negative offsets
		if pos.left != nil {
			x += *pos.left
		}
		if pos.right != nil {
			x -= *pos.right
		}
		if pos.top != nil {
			y += *pos.top
		}
		if pos.bottom != nil {
			y -= *pos.bottom
		}
	} else {
		// Traditional absolute positioning
		hasHorizontalPosition := pos.left != nil || pos.right != nil
		hasVerticalPosition := pos.top != nil || pos.bottom != nil

		// Compute stack alignment offset for unset axes
		var alignedOffset graphics.Offset
		if !hasHorizontalPosition || !hasVerticalPosition {
			alignedOffset = stackAlignment.WithinRect(
				graphics.RectFromLTWH(0, 0, stackSize.Width, stackSize.Height),
				childSize,
			)
		}

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
	}

	pos.child.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{}})
	pos.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{X: x, Y: y}})
}

// hitTestChildrenReverse tests children in reverse order and returns true if any child was hit.
func hitTestChildrenReverse(children []layout.RenderBox, position graphics.Offset, result *layout.HitTestResult) bool {
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		offset := getChildOffset(child)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if child.HitTest(local, result) {
			return true
		}
	}
	return false
}

// IndexedStack is a [Stack] that only displays one child at a time.
//
// By default, all children are laid out (maintaining their state), but only
// the child at Index is painted and receives hit tests. This is useful for tab
// views or wizards where you want to preserve the state of off-screen pages.
//
// With Fit == StackFitExpand, only the active child is laid out because the
// stack size is constraint-driven (inactive children cannot affect it).
//
// Example:
//
//	IndexedStack{
//	    Index: currentTab,
//	    Children: []core.Widget{
//	        HomeTab{},
//	        SearchTab{},
//	        ProfileTab{},
//	    },
//	}
//
// If Index is out of bounds, nothing is painted.
type IndexedStack struct {
	Children  []core.Widget
	Alignment layout.Alignment
	Fit       StackFit
	Index     int
}

func (s IndexedStack) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s IndexedStack) Key() any {
	return nil
}

func (s IndexedStack) ChildrenWidgets() []core.Widget {
	return s.Children
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
	// Clear parent on old children
	for _, child := range r.children {
		setParentOnChild(child, nil)
	}
	r.children = make([]layout.RenderBox, 0, len(children))
	for _, child := range children {
		if box, ok := child.(layout.RenderBox); ok {
			r.children = append(r.children, box)
			setParentOnChild(box, r)
		}
	}
}

func (r *renderIndexedStack) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		visitor(child)
	}
}

func (r *renderIndexedStack) PerformLayout() {
	constraints := r.Constraints()
	if r.fit == StackFitExpand {
		size := graphics.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight}
		if size.Width <= 0 {
			size.Width = constraints.MinWidth
		}
		if size.Height <= 0 {
			size.Height = constraints.MinHeight
		}
		size = constraints.Constrain(size)
		r.SetSize(size)

		if child := r.activeChild(); child != nil {
			if pos, ok := child.(*renderPositioned); ok {
				// Mirror Stack behavior for positioned children.
				pos.Layout(layout.Tight(size), true) // true: we read child.Size()
				layoutPositionedChild(pos, size, r.alignment, true)
			} else {
				child.Layout(layout.Tight(size), true) // true: we read child.Size()
				offset := r.alignment.WithinRect(
					graphics.RectFromLTWH(0, 0, size.Width, size.Height),
					child.Size(),
				)
				child.SetParentData(&layout.BoxParentData{Offset: offset})
			}
		}
		return
	}
	size := layoutStackChildren(r.children, r.fit, r.alignment, constraints)
	r.SetSize(size)
}

func (r *renderIndexedStack) Paint(ctx *layout.PaintContext) {
	if child := r.activeChild(); child != nil {
		ctx.PaintChildWithLayer(child, getChildOffset(child))
	}
}

// activeChild returns the currently visible child, or nil if index is out of bounds.
func (r *renderIndexedStack) activeChild() layout.RenderBox {
	if r.index < 0 || r.index >= len(r.children) {
		return nil
	}
	return r.children[r.index]
}

func (r *renderIndexedStack) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	child := r.activeChild()
	if child == nil {
		return false
	}
	offset := getChildOffset(child)
	local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	if child.HitTest(local, result) {
		return true
	}
	result.Add(r)
	return true
}

// Positioned positions a child within a Stack using absolute or relative positioning.
//
// # Coordinate System
//
// The coordinate system has its origin at the top-left of the Stack:
//   - Left/Right: Distance from the left/right edge of the Stack
//   - Top/Bottom: Distance from the top/bottom edge of the Stack
//
// # Relative Positioning with Alignment
//
// When Alignment is set, the child is positioned relative to that point
// within the Stack bounds. Left/Top/Right/Bottom become pixel offsets from
// the alignment position:
//
//	// Center of stack
//	Positioned{
//	    Alignment: &graphics.AlignCenter,
//	    Child: dialog,
//	}
//
//	// Bottom-right corner, 16px inset
//	Positioned{
//	    Alignment: &graphics.AlignBottomRight,
//	    Right: Ptr(16),
//	    Bottom: Ptr(16),
//	    Child: fab,
//	}
//
// # Absolute Positioning
//
// Use pointer fields (nil = unset) to control positioning. The [Ptr] helper
// creates float64 pointers conveniently:
//
//	// Pin to top-left corner with 8pt margins
//	Positioned{Left: Ptr(8), Top: Ptr(8), Child: icon}
//
//	// Pin to bottom-right corner
//	Positioned{Right: Ptr(16), Bottom: Ptr(16), Child: fab}
//
//	// Stretch horizontally with fixed vertical position
//	Positioned{Left: Ptr(0), Right: Ptr(0), Top: Ptr(100), Child: divider}
//
//	// Fixed size at specific position
//	Positioned{Left: Ptr(20), Top: Ptr(20), Width: Ptr(100), Height: Ptr(50), Child: box}
//
//	// Position only vertically - horizontal uses Stack.Alignment
//	Positioned{Top: Ptr(20), Child: header}
//
// When both Left and Right are set (or Top and Bottom), the child stretches
// to fill that dimension. Width/Height override the stretching behavior.
//
// For axes where no position is set (neither Left nor Right, or neither Top
// nor Bottom), the child uses the Stack's Alignment for that axis.
type Positioned struct {
	// Child is the widget to position.
	Child core.Widget

	// Alignment positions the child relative to the Stack bounds using the
	// graphics.Alignment coordinate system where (-1, -1) is top-left,
	// (0, 0) is center, and (1, 1) is bottom-right.
	//
	// When set, Left/Top/Right/Bottom become offsets from this position
	// rather than absolute pixel coordinates.
	//
	// If nil, Left/Top/Right/Bottom are absolute pixel coordinates from
	// the Stack edges (traditional absolute positioning).
	Alignment *graphics.Alignment

	// Left is the distance from the left edge of the Stack (nil = unset).
	// When Alignment is set, this is an offset from the alignment point.
	Left *float64
	// Top is the distance from the top edge of the Stack (nil = unset).
	// When Alignment is set, this is an offset from the alignment point.
	Top *float64
	// Right is the distance from the right edge of the Stack (nil = unset).
	// When Alignment is set, this is an offset from the alignment point.
	Right *float64
	// Bottom is the distance from the bottom edge of the Stack (nil = unset).
	// When Alignment is set, this is an offset from the alignment point.
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

// ChildWidget returns the child widget.
func (p Positioned) ChildWidget() core.Widget {
	return p.Child
}

// CreateRenderObject creates the RenderPositioned.
func (p Positioned) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	pos := &renderPositioned{
		alignment: p.Alignment,
		left:      p.Left,
		top:       p.Top,
		right:     p.Right,
		bottom:    p.Bottom,
		width:     p.Width,
		height:    p.Height,
	}
	pos.SetSelf(pos)
	return pos
}

// UpdateRenderObject updates the RenderPositioned.
func (p Positioned) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if pos, ok := renderObject.(*renderPositioned); ok {
		pos.alignment = p.Alignment
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
	child     layout.RenderBox
	alignment *graphics.Alignment
	left      *float64
	top       *float64
	right     *float64
	bottom    *float64
	width     *float64
	height    *float64
}

func (r *renderPositioned) SetChild(child layout.RenderObject) {
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

func (r *renderPositioned) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderPositioned) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(graphics.Size{})
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
	r.child.Layout(childConstraints, true) // true: we read child.Size()
	r.SetSize(r.child.Size())
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderPositioned) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
	}
}

func (r *renderPositioned) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	offset := getChildOffset(r.child)
	local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
	return r.child.HitTest(local, result)
}
