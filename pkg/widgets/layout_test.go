package widgets

import (
	"image"
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// TestFlex_UnboundedConstraints verifies that Flex with Expanded children
// in unbounded constraints panics with a helpful error message.
func TestFlex_UnboundedConstraints(t *testing.T) {
	// Create a Row with an Expanded child
	flex := &renderFlex{
		direction: AxisHorizontal,
		axisSize:  MainAxisSizeMax,
	}
	flex.SetSelf(flex)

	// Create a mock expanded child
	expandedChild := &mockFlexChild{flex: 1}
	expandedChild.SetSelf(expandedChild)

	// Create a fixed-size child
	fixedChild := &mockFixedChild{width: 50, height: 30}
	fixedChild.SetSelf(fixedChild)

	flex.SetChildren([]layout.RenderObject{fixedChild, expandedChild})

	// Layout with unbounded horizontal constraint (simulating inside horizontal ScrollView)
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: 100,
	}

	// Should panic with helpful error message
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Expanded in unbounded Row")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic message, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Expanded/Flexible used in Row with unbounded width") {
			t.Errorf("panic message should mention Row and unbounded width, got: %s", msg)
		}
	}()

	flex.Layout(unboundedConstraints, false)
}

// TestFlex_UnboundedVertical tests Column with Expanded in unbounded vertical constraints.
func TestFlex_UnboundedVertical(t *testing.T) {
	flex := &renderFlex{
		direction: AxisVertical,
		axisSize:  MainAxisSizeMax,
	}
	flex.SetSelf(flex)

	expandedChild := &mockFlexChild{flex: 1}
	expandedChild.SetSelf(expandedChild)

	flex.SetChildren([]layout.RenderObject{expandedChild})

	// Unbounded vertical (simulating inside vertical ScrollView)
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}

	// Should panic with helpful error message
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Expanded in unbounded Column")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic message, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Expanded/Flexible used in Column with unbounded height") {
			t.Errorf("panic message should mention Column and unbounded height, got: %s", msg)
		}
	}()

	flex.Layout(unboundedConstraints, false)
}

// TestFlex_CrossAxisStretch_UnboundedHeight tests Row with CrossAxisStretch in unbounded height.
func TestFlex_CrossAxisStretch_UnboundedHeight(t *testing.T) {
	flex := &renderFlex{
		direction:      AxisHorizontal,
		crossAlignment: CrossAxisAlignmentStretch,
	}
	flex.SetSelf(flex)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)

	flex.SetChildren([]layout.RenderObject{child})

	// Unbounded height (simulating inside vertical ScrollView)
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}

	// Should panic with helpful error message
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for CrossAxisStretch in Row with unbounded height")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic message, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "CrossAxisAlignmentStretch used in Row with unbounded height") {
			t.Errorf("panic message should mention Row and unbounded height, got: %s", msg)
		}
	}()

	flex.Layout(unboundedConstraints, false)
}

// TestFlex_CrossAxisStretch_UnboundedWidth tests Column with CrossAxisStretch in unbounded width.
func TestFlex_CrossAxisStretch_UnboundedWidth(t *testing.T) {
	flex := &renderFlex{
		direction:      AxisVertical,
		crossAlignment: CrossAxisAlignmentStretch,
	}
	flex.SetSelf(flex)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)

	flex.SetChildren([]layout.RenderObject{child})

	// Unbounded width
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: 200,
	}

	// Should panic with helpful error message
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for CrossAxisStretch in Column with unbounded width")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic message, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "CrossAxisAlignmentStretch used in Column with unbounded width") {
			t.Errorf("panic message should mention Column and unbounded width, got: %s", msg)
		}
	}()

	flex.Layout(unboundedConstraints, false)
}

// TestFlex_BoundedConstraints_NoPanic verifies no panic when constraints are bounded.
func TestFlex_BoundedConstraints_NoPanic(t *testing.T) {
	flex := &renderFlex{
		direction: AxisHorizontal,
		axisSize:  MainAxisSizeMax,
	}
	flex.SetSelf(flex)

	expandedChild := &mockFlexChild{flex: 1}
	expandedChild.SetSelf(expandedChild)

	flex.SetChildren([]layout.RenderObject{expandedChild})

	// Bounded constraints
	boundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  400,
		MinHeight: 0,
		MaxHeight: 100,
	}

	// Should NOT panic with bounded constraints
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic with bounded constraints: %v", r)
		}
	}()

	flex.Layout(boundedConstraints, false)

	// Verify reasonable size
	size := flex.Size()
	if size.Width <= 0 || size.Height <= 0 {
		t.Errorf("expected positive size, got %v x %v", size.Width, size.Height)
	}
}

// TestExpanded_ClampsChildSize ensures Expanded clamps a misbehaving child to its constraints.
func TestExpanded_ClampsChildSize(t *testing.T) {
	expanded := &renderExpanded{flex: 1}
	expanded.SetSelf(expanded)

	child := &mockOversizeChild{width: 200, height: 80}
	child.SetSelf(child)

	expanded.SetChild(child)

	constraints := layout.Constraints{
		MinWidth:  100,
		MaxWidth:  100,
		MinHeight: 0,
		MaxHeight: 50,
	}

	expanded.Layout(constraints, false)

	size := expanded.Size()
	if size.Width != 100 || size.Height != 50 {
		t.Errorf("expected Expanded size 100x50, got %vx%v", size.Width, size.Height)
	}
}

// TestCenter_UnboundedConstraints verifies Center shrink-wraps to child size
// when given unbounded constraints.
func TestCenter_UnboundedConstraints(t *testing.T) {
	center := &renderCenter{}
	center.SetSelf(center)

	child := &mockFixedChild{width: 100, height: 50}
	child.SetSelf(child)
	center.SetChild(child)

	// Fully unbounded constraints
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	center.Layout(unboundedConstraints, false)

	size := center.Size()

	// Center should shrink-wrap to child size
	if size.Width != 100 {
		t.Errorf("expected width 100, got %v", size.Width)
	}
	if size.Height != 50 {
		t.Errorf("expected height 50, got %v", size.Height)
	}

	// Verify no Inf/NaN
	if math.IsInf(size.Width, 0) || math.IsNaN(size.Width) ||
		math.IsInf(size.Height, 0) || math.IsNaN(size.Height) {
		t.Errorf("size should be finite, got %v x %v", size.Width, size.Height)
	}

	// Verify child offset is zero (centered in same-sized parent)
	childOffset := getChildOffset(child)
	if childOffset.X != 0 || childOffset.Y != 0 {
		t.Errorf("expected child offset (0, 0), got (%v, %v)", childOffset.X, childOffset.Y)
	}
}

// TestCenter_PartiallyUnbounded tests Center with only one dimension unbounded.
func TestCenter_PartiallyUnbounded(t *testing.T) {
	center := &renderCenter{}
	center.SetSelf(center)

	child := &mockFixedChild{width: 80, height: 40}
	child.SetSelf(child)
	center.SetChild(child)

	// Only width unbounded
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: 200,
	}
	center.Layout(constraints, false)

	size := center.Size()

	// Width should shrink to child, height should expand to max
	if size.Width != 80 {
		t.Errorf("expected width 80 (shrink-wrap), got %v", size.Width)
	}
	if size.Height != 200 {
		t.Errorf("expected height 200 (max constraint), got %v", size.Height)
	}

	// Child should be centered vertically
	childOffset := getChildOffset(child)
	expectedY := (200 - 40) / 2.0
	if childOffset.Y != expectedY {
		t.Errorf("expected child Y offset %v, got %v", expectedY, childOffset.Y)
	}
}

// TestAlign_UnboundedConstraints verifies Align shrink-wraps to child size
// when given unbounded constraints.
func TestAlign_UnboundedConstraints(t *testing.T) {
	align := &renderAlign{
		alignment: layout.AlignmentBottomRight,
	}
	align.SetSelf(align)

	child := &mockFixedChild{width: 60, height: 30}
	child.SetSelf(child)
	align.SetChild(child)

	// Fully unbounded constraints
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	align.Layout(unboundedConstraints, false)

	size := align.Size()

	// Align should shrink-wrap to child size
	if size.Width != 60 {
		t.Errorf("expected width 60, got %v", size.Width)
	}
	if size.Height != 30 {
		t.Errorf("expected height 30, got %v", size.Height)
	}

	// Child offset should be (0, 0) since parent == child size
	childOffset := getChildOffset(child)
	if childOffset.X != 0 || childOffset.Y != 0 {
		t.Errorf("expected child offset (0, 0), got (%v, %v)", childOffset.X, childOffset.Y)
	}
}

// TestAlign_PartiallyUnbounded tests Align with only height unbounded.
func TestAlign_PartiallyUnbounded(t *testing.T) {
	align := &renderAlign{
		alignment: layout.AlignmentBottomRight,
	}
	align.SetSelf(align)

	child := &mockFixedChild{width: 50, height: 25}
	child.SetSelf(child)
	align.SetChild(child)

	// Only height unbounded
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  300,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	align.Layout(constraints, false)

	size := align.Size()

	// Width should expand, height should shrink-wrap
	if size.Width != 300 {
		t.Errorf("expected width 300 (max constraint), got %v", size.Width)
	}
	if size.Height != 25 {
		t.Errorf("expected height 25 (shrink-wrap), got %v", size.Height)
	}

	// Child should be at bottom-right
	childOffset := getChildOffset(child)
	expectedX := 300.0 - 50.0
	if childOffset.X != expectedX {
		t.Errorf("expected child X offset %v, got %v", expectedX, childOffset.X)
	}
	// Y should be 0 since parent height == child height
	if childOffset.Y != 0 {
		t.Errorf("expected child Y offset 0, got %v", childOffset.Y)
	}
}

// TestPositioned_AlignmentMode_IgnoresEdgeConstraints verifies that when Alignment is set,
// Left/Right/Top/Bottom don't reduce the child's available constraints.
func TestPositioned_AlignmentMode_IgnoresEdgeConstraints(t *testing.T) {
	pos := &renderPositioned{
		alignment: &graphics.AlignCenter,
		left:      ptrFloat(50),  // Should be ignored for constraints in alignment mode
		right:     ptrFloat(50),  // Should be ignored for constraints in alignment mode
		top:       ptrFloat(100), // Should be ignored for constraints in alignment mode
	}
	pos.SetSelf(pos)

	child := &mockFixedChild{width: 200, height: 100}
	child.SetSelf(child)
	pos.SetChild(child)

	// Stack constraints: 400x300
	stackConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  400,
		MinHeight: 0,
		MaxHeight: 300,
	}

	// Get first-pass constraints for the positioned child
	childConstraints := positionedFirstPassConstraints(pos, stackConstraints)

	// In alignment mode, edges should NOT reduce max constraints
	// Child should get full 400x300, not (400-50-50)x(300-100) = 300x200
	if childConstraints.MaxWidth != 400 {
		t.Errorf("expected MaxWidth 400 (alignment mode ignores Left/Right), got %v", childConstraints.MaxWidth)
	}
	if childConstraints.MaxHeight != 300 {
		t.Errorf("expected MaxHeight 300 (alignment mode ignores Top/Bottom), got %v", childConstraints.MaxHeight)
	}
}

// TestPositioned_AbsoluteMode_AppliesEdgeConstraints verifies that without Alignment,
// Left/Right/Top/Bottom reduce the child's available constraints.
func TestPositioned_AbsoluteMode_AppliesEdgeConstraints(t *testing.T) {
	pos := &renderPositioned{
		alignment: nil, // Absolute positioning mode
		left:      ptrFloat(50),
		right:     ptrFloat(50),
	}
	pos.SetSelf(pos)

	child := &mockFixedChild{width: 200, height: 100}
	child.SetSelf(child)
	pos.SetChild(child)

	// Stack constraints: 400x300
	stackConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  400,
		MinHeight: 0,
		MaxHeight: 300,
	}

	// Get first-pass constraints for the positioned child
	childConstraints := positionedFirstPassConstraints(pos, stackConstraints)

	// In absolute mode with both left+right, it's stretching mode - use loose constraints
	// (actual stretching happens in second pass)
	if childConstraints.MaxWidth != 400 {
		t.Errorf("expected MaxWidth 400 (stretching mode uses loose), got %v", childConstraints.MaxWidth)
	}
}

// TestPositioned_AbsoluteMode_SingleEdgeReducesConstraint verifies that a single edge
// reduces available space in absolute mode.
func TestPositioned_AbsoluteMode_SingleEdgeReducesConstraint(t *testing.T) {
	pos := &renderPositioned{
		alignment: nil, // Absolute positioning mode
		left:      ptrFloat(100),
		// right is nil - single edge
	}
	pos.SetSelf(pos)

	child := &mockFixedChild{width: 200, height: 100}
	child.SetSelf(child)
	pos.SetChild(child)

	// Stack constraints: 400x300
	stackConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  400,
		MinHeight: 0,
		MaxHeight: 300,
	}

	// Get first-pass constraints for the positioned child
	childConstraints := positionedFirstPassConstraints(pos, stackConstraints)

	// In absolute mode with single left edge, max width should be reduced
	expectedMaxWidth := 400.0 - 100.0
	if childConstraints.MaxWidth != expectedMaxWidth {
		t.Errorf("expected MaxWidth %v (reduced by left), got %v", expectedMaxWidth, childConstraints.MaxWidth)
	}
}

// TestPositioned_AlignmentMode_WidthHeightStillApply verifies that explicit Width/Height
// still apply as tight constraints even in alignment mode.
func TestPositioned_AlignmentMode_WidthHeightStillApply(t *testing.T) {
	pos := &renderPositioned{
		alignment: &graphics.AlignCenter,
		width:     ptrFloat(150),
		height:    ptrFloat(75),
		left:      ptrFloat(999), // Should be ignored for constraints
	}
	pos.SetSelf(pos)

	child := &mockFixedChild{width: 200, height: 100}
	child.SetSelf(child)
	pos.SetChild(child)

	stackConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  400,
		MinHeight: 0,
		MaxHeight: 300,
	}

	childConstraints := positionedFirstPassConstraints(pos, stackConstraints)

	// Width/Height should apply as tight constraints
	if childConstraints.MinWidth != 150 || childConstraints.MaxWidth != 150 {
		t.Errorf("expected tight width 150, got min=%v max=%v", childConstraints.MinWidth, childConstraints.MaxWidth)
	}
	if childConstraints.MinHeight != 75 || childConstraints.MaxHeight != 75 {
		t.Errorf("expected tight height 75, got min=%v max=%v", childConstraints.MinHeight, childConstraints.MaxHeight)
	}
}

func ptrFloat(v float64) *float64 {
	return &v
}

// mockFlexChild is a render box that reports a flex factor.
type mockFlexChild struct {
	layout.RenderBoxBase
	flex int
}

func (m *mockFlexChild) FlexFactor() int {
	return m.flex
}

func (m *mockFlexChild) PerformLayout() {
	constraints := m.Constraints()
	// Take the max of constraints or a minimum size
	w := constraints.MaxWidth
	if w == math.MaxFloat64 {
		w = 50
	}
	h := constraints.MaxHeight
	if h == math.MaxFloat64 {
		h = 30
	}
	m.SetSize(constraints.Constrain(graphics.Size{Width: w, Height: h}))
}

func (m *mockFlexChild) Paint(ctx *layout.PaintContext) {}

func (m *mockFlexChild) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

// mockFixedChild is a render box with a fixed intrinsic size.
type mockFixedChild struct {
	layout.RenderBoxBase
	width, height float64
}

func (m *mockFixedChild) PerformLayout() {
	constraints := m.Constraints()
	m.SetSize(constraints.Constrain(graphics.Size{Width: m.width, Height: m.height}))
}

func (m *mockFixedChild) Paint(ctx *layout.PaintContext) {}

func (m *mockFixedChild) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

// mockOversizeChild ignores constraints and reports a fixed size.
type mockOversizeChild struct {
	layout.RenderBoxBase
	width, height float64
}

func (m *mockOversizeChild) PerformLayout() {
	m.SetSize(graphics.Size{Width: m.width, Height: m.height})
}

func (m *mockOversizeChild) Paint(ctx *layout.PaintContext) {}

func (m *mockOversizeChild) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

// mockCanvas implements graphics.Canvas for testing paint calls.
type mockCanvas struct{}

func (c *mockCanvas) Save()                                                     {}
func (c *mockCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64)        {}
func (c *mockCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint)     {}
func (c *mockCanvas) SaveLayerBlur(bounds graphics.Rect, sigmaX, sigmaY float64) {}
func (c *mockCanvas) Restore()                                                  {}
func (c *mockCanvas) Translate(dx, dy float64)                                  {}
func (c *mockCanvas) Scale(sx, sy float64)                                      {}
func (c *mockCanvas) Rotate(radians float64)                                    {}
func (c *mockCanvas) ClipRect(rect graphics.Rect)                               {}
func (c *mockCanvas) ClipRRect(rect graphics.RRect)                             {}
func (c *mockCanvas) ClipPath(path *graphics.Path, op graphics.ClipOp, aa bool) {}
func (c *mockCanvas) Clear(color graphics.Color)                                {}
func (c *mockCanvas) DrawRect(rect graphics.Rect, paint graphics.Paint)         {}
func (c *mockCanvas) DrawRRect(rect graphics.RRect, paint graphics.Paint)       {}
func (c *mockCanvas) DrawCircle(center graphics.Offset, radius float64, paint graphics.Paint) {
}
func (c *mockCanvas) DrawLine(p1, p2 graphics.Offset, paint graphics.Paint)          {}
func (c *mockCanvas) DrawPath(path *graphics.Path, paint graphics.Paint)             {}
func (c *mockCanvas) DrawText(layout *graphics.TextLayout, position graphics.Offset) {}
func (c *mockCanvas) DrawImage(img image.Image, position graphics.Offset)            {}
func (c *mockCanvas) DrawImageRect(img image.Image, src, dst graphics.Rect, q graphics.FilterQuality, key uintptr) {
}
func (c *mockCanvas) DrawRectShadow(rect graphics.Rect, shadow graphics.BoxShadow)  {}
func (c *mockCanvas) DrawRRectShadow(rect graphics.RRect, shadow graphics.BoxShadow) {}
func (c *mockCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds graphics.Rect)           {}
func (c *mockCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds graphics.Rect, tint graphics.Color) {
}
func (c *mockCanvas) Size() graphics.Size { return graphics.Size{Width: 800, Height: 600} }
