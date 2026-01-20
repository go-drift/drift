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
// For absolute positioning, wrap children in Positioned:
//
//	Stack{
//	    ChildrenWidgets: []core.Widget{
//	        // Background fills the stack
//	        Container{Color: bgColor},
//	        // Badge in top-right corner
//	        Positioned{Top: ptr(8), Right: ptr(8), ChildWidget: badge},
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
func layoutStackChildren(children []layout.RenderBox, fit StackFit, alignment layout.Alignment, constraints layout.Constraints) rendering.Size {
	var stackWidth, stackHeight float64
	if fit == StackFitExpand {
		stackWidth = constraints.MaxWidth
		stackHeight = constraints.MaxHeight
	}

	for _, child := range children {
		childConstraints := stackConstraints(fit, stackWidth, stackHeight, constraints)
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

	for _, child := range children {
		offset := alignment.WithinRect(
			rendering.RectFromLTWH(0, 0, size.Width, size.Height),
			child.Size(),
		)
		child.SetParentData(&layout.BoxParentData{Offset: offset})
	}

	return size
}

// stackConstraints returns the constraints for a child based on the fit mode.
func stackConstraints(fit StackFit, stackWidth, stackHeight float64, constraints layout.Constraints) layout.Constraints {
	if fit == StackFitExpand {
		return layout.Tight(rendering.Size{Width: stackWidth, Height: stackHeight})
	}
	return layout.Loose(rendering.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight})
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
// Use pointer fields (nil = unset) to control positioning:
//
//	// Pin to top-left corner with 8pt margins
//	Positioned{Left: ptr(8), Top: ptr(8), ChildWidget: icon}
//
//	// Pin to bottom-right corner
//	Positioned{Right: ptr(16), Bottom: ptr(16), ChildWidget: fab}
//
//	// Stretch horizontally with fixed vertical position
//	Positioned{Left: ptr(0), Right: ptr(0), Top: ptr(100), ChildWidget: divider}
//
//	// Fixed size at specific position
//	Positioned{Left: ptr(20), Top: ptr(20), Width: ptr(100), Height: ptr(50), ChildWidget: box}
//
// When both Left and Right are set (or Top and Bottom), the child stretches
// to fill that dimension. Width/Height override the stretching behavior.
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

func (r *renderPositioned) Layout(constraints layout.Constraints) {
	if r.child == nil {
		r.SetSize(rendering.Size{})
		return
	}
	// For now, just pass through
	r.child.Layout(constraints)
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
