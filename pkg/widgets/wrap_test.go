package widgets

import (
	"math"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/layout"
)

func TestWrap_BasicHorizontalWrapping(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
	}
	wrap.SetSelf(wrap)

	// Create 4 children of 50px width each
	children := make([]layout.RenderObject, 4)
	for i := range children {
		child := &mockFixedChild{width: 50, height: 30}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	// Layout with 120px width - should fit 2 children per run (50+50=100 < 120)
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  120,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	// Should have 2 runs of height 30 each = 60 total height
	if size.Height != 60 {
		t.Errorf("expected height 60, got %v", size.Height)
	}
	if size.Width != 120 {
		t.Errorf("expected width 120, got %v", size.Width)
	}

	// Verify children are positioned in 2 runs
	// Run 1: children 0 and 1 at y=0
	// Run 2: children 2 and 3 at y=30
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		expectedY := float64((i / 2) * 30)
		if offset.Y != expectedY {
			t.Errorf("child %d: expected Y=%v, got Y=%v", i, expectedY, offset.Y)
		}
	}
}

func TestWrap_VerticalDirection(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisVertical,
	}
	wrap.SetSelf(wrap)

	// Create 4 children of 50px height each
	children := make([]layout.RenderObject, 4)
	for i := range children {
		child := &mockFixedChild{width: 30, height: 50}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	// Layout with 120px height - should fit 2 children per run (50+50=100 < 120)
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: 120,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	// Should have 2 runs of width 30 each = 60 total width
	if size.Width != 60 {
		t.Errorf("expected width 60, got %v", size.Width)
	}
	if size.Height != 120 {
		t.Errorf("expected height 120, got %v", size.Height)
	}

	// Verify children are positioned in 2 vertical runs
	// Run 1: children 0 and 1 at x=0
	// Run 2: children 2 and 3 at x=30
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		expectedX := float64((i / 2) * 30)
		if offset.X != expectedX {
			t.Errorf("child %d: expected X=%v, got X=%v", i, expectedX, offset.X)
		}
	}
}

func TestWrap_SpacingBetweenItems(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		spacing:   10,
	}
	wrap.SetSelf(wrap)

	// Create 3 children of 30px width each
	children := make([]layout.RenderObject, 3)
	for i := range children {
		child := &mockFixedChild{width: 30, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	// Layout with 200px width - all 3 should fit: 30+10+30+10+30 = 110 < 200
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// All children should be in one run
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		expectedX := float64(i) * 40 // 30 width + 10 spacing
		if offset.X != expectedX {
			t.Errorf("child %d: expected X=%v, got X=%v", i, expectedX, offset.X)
		}
		if offset.Y != 0 {
			t.Errorf("child %d: expected Y=0, got Y=%v", i, offset.Y)
		}
	}
}

func TestWrap_RunSpacing(t *testing.T) {
	wrap := &renderWrap{
		direction:  AxisHorizontal,
		runSpacing: 15,
	}
	wrap.SetSelf(wrap)

	// Create 4 children of 60px width each
	children := make([]layout.RenderObject, 4)
	for i := range children {
		child := &mockFixedChild{width: 60, height: 25}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	// Layout with 100px width - 1 child per run
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	// 4 runs of height 25, with 3 gaps of 15 = 25*4 + 15*3 = 100 + 45 = 145
	expectedHeight := 25.0*4 + 15.0*3
	if size.Height != expectedHeight {
		t.Errorf("expected height %v, got %v", expectedHeight, size.Height)
	}

	// Check run positions
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		expectedY := float64(i) * (25 + 15)
		if offset.Y != expectedY {
			t.Errorf("child %d: expected Y=%v, got Y=%v", i, expectedY, offset.Y)
		}
	}
}

func TestWrap_AlignmentCenter(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		alignment: WrapAlignmentCenter,
	}
	wrap.SetSelf(wrap)

	// Single child of 50px width
	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	// Layout with 200px width
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// Child should be centered: (200 - 50) / 2 = 75
	offset := getChildOffset(wrap.children[0])
	expectedX := (200.0 - 50.0) / 2
	if offset.X != expectedX {
		t.Errorf("expected X=%v, got X=%v", expectedX, offset.X)
	}
}

func TestWrap_AlignmentEnd(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		alignment: WrapAlignmentEnd,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// Child should be at end: 200 - 50 = 150
	offset := getChildOffset(wrap.children[0])
	expectedX := 200.0 - 50.0
	if offset.X != expectedX {
		t.Errorf("expected X=%v, got X=%v", expectedX, offset.X)
	}
}

func TestWrap_AlignmentSpaceBetween(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		alignment: WrapAlignmentSpaceBetween,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 3)
	for i := range children {
		child := &mockFixedChild{width: 20, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// Free space: 100 - 60 = 40, divided between 2 gaps = 20 each
	// Positions: 0, 20+20=40, 40+20+20=80
	expectedX := []float64{0, 40, 80}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.X != expectedX[i] {
			t.Errorf("child %d: expected X=%v, got X=%v", i, expectedX[i], offset.X)
		}
	}
}

func TestWrap_AlignmentSpaceAround(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		alignment: WrapAlignmentSpaceAround,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 2)
	for i := range children {
		child := &mockFixedChild{width: 20, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// Free space: 100 - 40 = 60, spacing = 60/2 = 30
	// Half space at edges: 15
	// Positions: 15, 15+20+30=65
	expectedX := []float64{15, 65}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.X != expectedX[i] {
			t.Errorf("child %d: expected X=%v, got X=%v", i, expectedX[i], offset.X)
		}
	}
}

func TestWrap_AlignmentSpaceEvenly(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		alignment: WrapAlignmentSpaceEvenly,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 2)
	for i := range children {
		child := &mockFixedChild{width: 20, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// Free space: 100 - 40 = 60, spacing = 60/3 = 20
	// Positions: 20, 20+20+20=60
	expectedX := []float64{20, 60}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.X != expectedX[i] {
			t.Errorf("child %d: expected X=%v, got X=%v", i, expectedX[i], offset.X)
		}
	}
}

func TestWrap_CrossAxisAlignmentCenter(t *testing.T) {
	wrap := &renderWrap{
		direction:          AxisHorizontal,
		crossAxisAlignment: WrapCrossAlignmentCenter,
	}
	wrap.SetSelf(wrap)

	// Two children with different heights in one run
	child1 := &mockFixedChild{width: 30, height: 20}
	child1.SetSelf(child1)
	child2 := &mockFixedChild{width: 30, height: 40}
	child2.SetSelf(child2)
	wrap.SetChildren([]layout.RenderObject{child1, child2})

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// Run height is 40 (max of children)
	// child1 (height 20) should be centered: (40-20)/2 = 10
	// child2 (height 40) should be at 0
	offset1 := getChildOffset(wrap.children[0])
	offset2 := getChildOffset(wrap.children[1])

	if offset1.Y != 10 {
		t.Errorf("child1: expected Y=10, got Y=%v", offset1.Y)
	}
	if offset2.Y != 0 {
		t.Errorf("child2: expected Y=0, got Y=%v", offset2.Y)
	}
}

func TestWrap_CrossAxisAlignmentEnd(t *testing.T) {
	wrap := &renderWrap{
		direction:          AxisHorizontal,
		crossAxisAlignment: WrapCrossAlignmentEnd,
	}
	wrap.SetSelf(wrap)

	child1 := &mockFixedChild{width: 30, height: 20}
	child1.SetSelf(child1)
	child2 := &mockFixedChild{width: 30, height: 40}
	child2.SetSelf(child2)
	wrap.SetChildren([]layout.RenderObject{child1, child2})

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	// child1 (height 20) should be at end: 40-20 = 20
	// child2 (height 40) should be at 0
	offset1 := getChildOffset(wrap.children[0])
	offset2 := getChildOffset(wrap.children[1])

	if offset1.Y != 20 {
		t.Errorf("child1: expected Y=20, got Y=%v", offset1.Y)
	}
	if offset2.Y != 0 {
		t.Errorf("child2: expected Y=0, got Y=%v", offset2.Y)
	}
}

func TestWrap_RunAlignmentCenter(t *testing.T) {
	wrap := &renderWrap{
		direction:    AxisHorizontal,
		runAlignment: RunAlignmentCenter,
	}
	wrap.SetSelf(wrap)

	// Create 2 children that will form 2 runs
	children := make([]layout.RenderObject, 2)
	for i := range children {
		child := &mockFixedChild{width: 80, height: 30}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	// Layout with bounded cross axis (MinHeight forces expansion)
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 200,
		MaxHeight: 200,
	}
	wrap.Layout(constraints, false)

	// 2 runs of height 30 = 60 total
	// Free space: 200 - 60 = 140
	// Center offset: 70
	// Positions: 70, 70+30=100
	offset1 := getChildOffset(wrap.children[0])
	offset2 := getChildOffset(wrap.children[1])

	if offset1.Y != 70 {
		t.Errorf("child1: expected Y=70, got Y=%v", offset1.Y)
	}
	if offset2.Y != 100 {
		t.Errorf("child2: expected Y=100, got Y=%v", offset2.Y)
	}
}

func TestWrap_RunAlignmentEnd(t *testing.T) {
	wrap := &renderWrap{
		direction:    AxisHorizontal,
		runAlignment: RunAlignmentEnd,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 2)
	for i := range children {
		child := &mockFixedChild{width: 80, height: 30}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 200,
		MaxHeight: 200,
	}
	wrap.Layout(constraints, false)

	// Free space: 200 - 60 = 140
	// End offset: 140
	// Positions: 140, 170
	offset1 := getChildOffset(wrap.children[0])
	offset2 := getChildOffset(wrap.children[1])

	if offset1.Y != 140 {
		t.Errorf("child1: expected Y=140, got Y=%v", offset1.Y)
	}
	if offset2.Y != 170 {
		t.Errorf("child2: expected Y=170, got Y=%v", offset2.Y)
	}
}

func TestWrap_RunAlignmentSpaceBetween(t *testing.T) {
	wrap := &renderWrap{
		direction:    AxisHorizontal,
		runAlignment: RunAlignmentSpaceBetween,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 3)
	for i := range children {
		child := &mockFixedChild{width: 80, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 200,
		MaxHeight: 200,
	}
	wrap.Layout(constraints, false)

	// 3 runs of height 20 = 60 total
	// Free space: 200 - 60 = 140
	// Space between: 140 / 2 = 70
	// Positions: 0, 20+70=90, 90+20+70=180
	expectedY := []float64{0, 90, 180}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.Y != expectedY[i] {
			t.Errorf("child %d: expected Y=%v, got Y=%v", i, expectedY[i], offset.Y)
		}
	}
}

func TestWrap_RunAlignmentSpaceAround(t *testing.T) {
	wrap := &renderWrap{
		direction:    AxisHorizontal,
		runAlignment: RunAlignmentSpaceAround,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 2)
	for i := range children {
		child := &mockFixedChild{width: 80, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 100,
		MaxHeight: 100,
	}
	wrap.Layout(constraints, false)

	// 2 runs of height 20 = 40 total
	// Free space: 100 - 40 = 60
	// Spacing = 60 / 2 = 30, half at edges = 15
	// Positions: 15, 15+20+30=65
	expectedY := []float64{15, 65}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.Y != expectedY[i] {
			t.Errorf("child %d: expected Y=%v, got Y=%v", i, expectedY[i], offset.Y)
		}
	}
}

func TestWrap_RunAlignmentSpaceEvenly(t *testing.T) {
	wrap := &renderWrap{
		direction:    AxisHorizontal,
		runAlignment: RunAlignmentSpaceEvenly,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 2)
	for i := range children {
		child := &mockFixedChild{width: 80, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 100,
		MaxHeight: 100,
	}
	wrap.Layout(constraints, false)

	// 2 runs of height 20 = 40 total
	// Free space: 100 - 40 = 60
	// Spacing = 60 / 3 = 20
	// Positions: 20, 20+20+20=60
	expectedY := []float64{20, 60}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.Y != expectedY[i] {
			t.Errorf("child %d: expected Y=%v, got Y=%v", i, expectedY[i], offset.Y)
		}
	}
}

func TestWrap_RunAlignmentWithRunSpacing(t *testing.T) {
	wrap := &renderWrap{
		direction:    AxisHorizontal,
		runAlignment: RunAlignmentSpaceBetween,
		runSpacing:   10, // RunSpacing should be added in addition to alignment spacing
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 3)
	for i := range children {
		child := &mockFixedChild{width: 80, height: 20}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 200,
		MaxHeight: 200,
	}
	wrap.Layout(constraints, false)

	// 3 runs of height 20 with runSpacing 10 between = 20*3 + 10*2 = 80 total
	// Free space: 200 - 80 = 120
	// SpaceBetween: 120 / 2 = 60 between runs (added to runSpacing)
	// Run 0: Y = 0
	// Run 1: Y = 0 + 20 + 60 + 10 = 90
	// Run 2: Y = 90 + 20 + 60 + 10 = 180
	expectedY := []float64{0, 90, 180}
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.Y != expectedY[i] {
			t.Errorf("child %d: expected Y=%v, got Y=%v", i, expectedY[i], offset.Y)
		}
	}
}

func TestWrap_VerticalCrossAxisSizing(t *testing.T) {
	// Regression test: vertical wrap should use MinWidth for cross-axis, not MinHeight
	wrap := &renderWrap{
		direction: AxisVertical,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 30, height: 50}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	// MinHeight is large but MinWidth is small - width should NOT be affected by MinHeight
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 150,
		MaxHeight: 200,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	// Cross axis (width) should be child width (30), not affected by MinHeight (150)
	if size.Width != 30 {
		t.Errorf("expected width 30 (child width), got %v", size.Width)
	}
	// Main axis (height) should be max constraint
	if size.Height != 200 {
		t.Errorf("expected height 200, got %v", size.Height)
	}
}

func TestWrap_VerticalCrossAxisMinWidth(t *testing.T) {
	// Vertical wrap should respect MinWidth for cross-axis sizing
	wrap := &renderWrap{
		direction: AxisVertical,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 30, height: 50}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	constraints := layout.Constraints{
		MinWidth:  100, // Cross axis minimum
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: 200,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	// Cross axis (width) should respect MinWidth
	if size.Width != 100 {
		t.Errorf("expected width 100 (MinWidth), got %v", size.Width)
	}
}

func TestWrap_EmptyChildren(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
	}
	wrap.SetSelf(wrap)
	wrap.SetChildren([]layout.RenderObject{})

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: 100,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	if size.Width != 0 || size.Height != 0 {
		t.Errorf("expected size 0x0, got %vx%v", size.Width, size.Height)
	}
}

func TestWrap_SingleChild(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	if size.Width != 200 {
		t.Errorf("expected width 200, got %v", size.Width)
	}
	if size.Height != 30 {
		t.Errorf("expected height 30, got %v", size.Height)
	}

	offset := getChildOffset(wrap.children[0])
	if offset.X != 0 || offset.Y != 0 {
		t.Errorf("expected offset (0,0), got (%v,%v)", offset.X, offset.Y)
	}
}

func TestWrap_SingleRowFitsAll(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
		spacing:   10,
	}
	wrap.SetSelf(wrap)

	children := make([]layout.RenderObject, 3)
	for i := range children {
		child := &mockFixedChild{width: 30, height: 25}
		child.SetSelf(child)
		children[i] = child
	}
	wrap.SetChildren(children)

	// All children fit: 30*3 + 10*2 = 110 < 200
	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}
	wrap.Layout(constraints, false)

	size := wrap.Size()
	// Single run, height should be 25
	if size.Height != 25 {
		t.Errorf("expected height 25, got %v", size.Height)
	}

	// All children should be on same row
	for i, child := range wrap.children {
		offset := getChildOffset(child)
		if offset.Y != 0 {
			t.Errorf("child %d: expected Y=0, got Y=%v", i, offset.Y)
		}
	}
}

func TestWrap_UnboundedMainAxisPanics(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	// Unbounded width
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  math.MaxFloat64,
		MinHeight: 0,
		MaxHeight: 100,
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for Wrap with unbounded main axis")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic message, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Wrap (horizontal) used with unbounded width") {
			t.Errorf("panic message should mention horizontal Wrap and unbounded width, got: %s", msg)
		}
	}()

	wrap.Layout(unboundedConstraints, false)
}

func TestWrap_UnboundedMainAxisVerticalPanics(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisVertical,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	// Unbounded height
	unboundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  100,
		MinHeight: 0,
		MaxHeight: math.MaxFloat64,
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for vertical Wrap with unbounded main axis")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic message, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "Wrap (vertical) used with unbounded height") {
			t.Errorf("panic message should mention vertical Wrap and unbounded height, got: %s", msg)
		}
	}()

	wrap.Layout(unboundedConstraints, false)
}

func TestWrap_BoundedConstraintsNoPanic(t *testing.T) {
	wrap := &renderWrap{
		direction: AxisHorizontal,
	}
	wrap.SetSelf(wrap)

	child := &mockFixedChild{width: 50, height: 30}
	child.SetSelf(child)
	wrap.SetChildren([]layout.RenderObject{child})

	// Bounded constraints
	boundedConstraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  200,
		MinHeight: 0,
		MaxHeight: 100,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic with bounded constraints: %v", r)
		}
	}()

	wrap.Layout(boundedConstraints, false)

	size := wrap.Size()
	if size.Width <= 0 || size.Height <= 0 {
		t.Errorf("expected positive size, got %v x %v", size.Width, size.Height)
	}
}

func TestWrapAlignment_String(t *testing.T) {
	tests := []struct {
		alignment WrapAlignment
		expected  string
	}{
		{WrapAlignmentStart, "start"},
		{WrapAlignmentEnd, "end"},
		{WrapAlignmentCenter, "center"},
		{WrapAlignmentSpaceBetween, "space_between"},
		{WrapAlignmentSpaceAround, "space_around"},
		{WrapAlignmentSpaceEvenly, "space_evenly"},
		{WrapAlignment(99), "WrapAlignment(99)"},
	}

	for _, tc := range tests {
		if got := tc.alignment.String(); got != tc.expected {
			t.Errorf("WrapAlignment(%d).String() = %q, want %q", tc.alignment, got, tc.expected)
		}
	}
}

func TestWrapCrossAlignment_String(t *testing.T) {
	tests := []struct {
		alignment WrapCrossAlignment
		expected  string
	}{
		{WrapCrossAlignmentStart, "start"},
		{WrapCrossAlignmentEnd, "end"},
		{WrapCrossAlignmentCenter, "center"},
		{WrapCrossAlignment(99), "WrapCrossAlignment(99)"},
	}

	for _, tc := range tests {
		if got := tc.alignment.String(); got != tc.expected {
			t.Errorf("WrapCrossAlignment(%d).String() = %q, want %q", tc.alignment, got, tc.expected)
		}
	}
}

func TestRunAlignment_String(t *testing.T) {
	tests := []struct {
		alignment RunAlignment
		expected  string
	}{
		{RunAlignmentStart, "start"},
		{RunAlignmentEnd, "end"},
		{RunAlignmentCenter, "center"},
		{RunAlignmentSpaceBetween, "space_between"},
		{RunAlignmentSpaceAround, "space_around"},
		{RunAlignmentSpaceEvenly, "space_evenly"},
		{RunAlignment(99), "RunAlignment(99)"},
	}

	for _, tc := range tests {
		if got := tc.alignment.String(); got != tc.expected {
			t.Errorf("RunAlignment(%d).String() = %q, want %q", tc.alignment, got, tc.expected)
		}
	}
}

func TestWrapOf_Helper(t *testing.T) {
	wrap := WrapOf(8, 12)
	if wrap.Spacing != 8 {
		t.Errorf("expected Spacing=8, got %v", wrap.Spacing)
	}
	if wrap.RunSpacing != 12 {
		t.Errorf("expected RunSpacing=12, got %v", wrap.RunSpacing)
	}
	// WrapOf uses default Direction which is AxisHorizontal
	if wrap.Direction != AxisHorizontal {
		t.Errorf("expected Direction=AxisHorizontal, got %v", wrap.Direction)
	}
}
