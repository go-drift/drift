package engine

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// mockBoundaryRenderBox is a repaint boundary that tracks recording order
type mockBoundaryRenderBox struct {
	layout.RenderBoxBase
	name           string
	recordingOrder *[]string
	children       []layout.RenderBox
}

func newMockBoundary(name string, recordingOrder *[]string) *mockBoundaryRenderBox {
	m := &mockBoundaryRenderBox{
		name:           name,
		recordingOrder: recordingOrder,
	}
	m.SetSelf(m)
	m.SetSize(graphics.Size{Width: 100, Height: 100})
	return m
}

func (r *mockBoundaryRenderBox) PerformLayout() {
	r.SetSize(graphics.Size{Width: 100, Height: 100})
	for i, child := range r.children {
		if child != nil {
			child.Layout(layout.Tight(graphics.Size{Width: 50, Height: 50}), false)
			if setter, ok := child.(interface {
				SetParentData(any)
			}); ok {
				setter.SetParentData(&layout.BoxParentData{
					Offset: graphics.Offset{X: float64(i * 10), Y: float64(i * 10)},
				})
			}
		}
	}
}

func (r *mockBoundaryRenderBox) Paint(ctx *layout.PaintContext) {
	// Record that this boundary was painted/recorded
	if r.recordingOrder != nil {
		*r.recordingOrder = append(*r.recordingOrder, r.name)
	}
	for i, child := range r.children {
		if child != nil {
			ctx.PaintChildWithLayer(child, graphics.Offset{X: float64(i * 10), Y: float64(i * 10)})
		}
	}
}

func (r *mockBoundaryRenderBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

func (r *mockBoundaryRenderBox) IsRepaintBoundary() bool {
	return true
}

func (r *mockBoundaryRenderBox) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

func (r *mockBoundaryRenderBox) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		if child != nil {
			visitor(child)
		}
	}
}

func (r *mockBoundaryRenderBox) AddChild(child layout.RenderBox) {
	r.children = append(r.children, child)
	if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
		setter.SetParent(r)
	}
}

// TestDFSRecordingOrder verifies that children are recorded before parents
// in the layer tree recording phase when using recordDirtyLayers
func TestDFSRecordingOrder(t *testing.T) {
	// Build tree:
	//       root
	//      /    \
	//   child1  child2
	//     |
	//  grandchild

	var recordingOrder []string

	root := newMockBoundary("root", &recordingOrder)
	child1 := newMockBoundary("child1", &recordingOrder)
	child2 := newMockBoundary("child2", &recordingOrder)
	grandchild := newMockBoundary("grandchild", &recordingOrder)

	root.AddChild(child1)
	root.AddChild(child2)
	child1.AddChild(grandchild)

	// Mark all as needing paint
	root.MarkNeedsPaint()
	child1.MarkNeedsPaint()
	child2.MarkNeedsPaint()
	grandchild.MarkNeedsPaint()

	// Use recordDirtyLayers which processes all dirty boundaries
	// Simulating FlushPaint output: sorted by depth (parents first)
	dirtyBoundaries := []layout.RenderObject{root, child1, child2, grandchild}
	recordDirtyLayers(dirtyBoundaries, false, 1.0)

	// Verify children recorded before parents (reverse depth order).
	// The recording order is determined by:
	// 1. recordDirtyLayers iterates dirtyBoundaries in REVERSE (deepest first)
	// 2. For each boundary, DFS stops at child boundaries (they're processed separately)
	//
	// With input [root, child1, child2, grandchild] (depth order from FlushPaint):
	// - Process grandchild (idx 3): records "grandchild"
	// - Process child2 (idx 2): records "child2" (no children that are boundaries)
	// - Process child1 (idx 1): DFS stops at grandchild boundary, records "child1"
	// - Process root (idx 0): DFS stops at child1/child2 boundaries, records "root"
	expected := []string{"grandchild", "child2", "child1", "root"}

	if len(recordingOrder) != len(expected) {
		t.Fatalf("expected %d recordings, got %d: %v", len(expected), len(recordingOrder), recordingOrder)
	}

	for i, name := range expected {
		if recordingOrder[i] != name {
			t.Errorf("recording order[%d]: expected %q, got %q (full order: %v)",
				i, name, recordingOrder[i], recordingOrder)
		}
	}
}

// TestDFSSkipsCleanLayers verifies that clean layers are not re-recorded
func TestDFSSkipsCleanLayers(t *testing.T) {
	var recordingOrder []string

	root := newMockBoundary("root", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	root.AddChild(child)

	// Mark only root as needing paint (child is clean)
	root.MarkNeedsPaint()
	// Child's layer should exist but be clean
	childLayer := child.EnsureLayer()
	childLayer.Dirty = false
	child.ClearNeedsPaint()

	// With optimization, recordDirtyLayersDFS stops at child boundaries anyway,
	// but the key test is that clean boundaries don't get re-recorded
	recordDirtyLayersDFS(root, false, 1.0, true)

	// Only root should be recorded (child is a boundary so DFS stops there)
	if len(recordingOrder) != 1 {
		t.Fatalf("expected 1 recording, got %d: %v", len(recordingOrder), recordingOrder)
	}
	if recordingOrder[0] != "root" {
		t.Errorf("expected 'root' to be recorded, got %q", recordingOrder[0])
	}
}

// TestLayerContentPreservedAfterRecording verifies that layer content is set
// after recording and dirty flag is cleared
func TestLayerContentPreservedAfterRecording(t *testing.T) {
	var recordingOrder []string
	box := newMockBoundary("box", &recordingOrder)
	box.MarkNeedsPaint()

	layer := box.EnsureLayer()
	if !layer.Dirty {
		t.Fatal("layer should be dirty before recording")
	}
	if layer.Content != nil {
		t.Fatal("layer content should be nil before recording")
	}

	recordDirtyLayersDFS(box, false, 1.0, true)

	if layer.Dirty {
		t.Fatal("layer should be clean after recording")
	}
	if layer.Content == nil {
		t.Fatal("layer content should be set after recording")
	}
}

// TestNestedBoundariesDrawChildLayerOps verifies that nested boundaries
// record DrawChildLayer ops instead of embedding content
func TestNestedBoundariesDrawChildLayerOps(t *testing.T) {
	var recordingOrder []string

	parent := newMockBoundary("parent", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	parent.AddChild(child)

	// Mark both as needing paint
	parent.MarkNeedsPaint()
	child.MarkNeedsPaint()

	// Record layers using recordDirtyLayers (processes both boundaries)
	dirtyBoundaries := []layout.RenderObject{parent, child}
	recordDirtyLayers(dirtyBoundaries, false, 1.0)

	// Both should have content
	parentLayer := parent.EnsureLayer()
	childLayer := child.EnsureLayer()

	if parentLayer.Content == nil {
		t.Fatal("parent layer should have content")
	}
	if childLayer.Content == nil {
		t.Fatal("child layer should have content")
	}

	// Now modify child's content and verify parent doesn't need re-recording
	child.MarkNeedsPaint()

	// Parent layer should still be clean
	if parentLayer.Dirty {
		t.Fatal("parent layer should remain clean when child content changes")
	}
}

// TestRecordDirtyLayersOptimization verifies that recordDirtyLayers processes
// only the provided dirty boundaries (not the full tree)
func TestRecordDirtyLayersOptimization(t *testing.T) {
	var recordingOrder []string

	// Build tree:
	//       root
	//      /    \
	//   child1  child2
	root := newMockBoundary("root", &recordingOrder)
	child1 := newMockBoundary("child1", &recordingOrder)
	child2 := newMockBoundary("child2", &recordingOrder)

	root.AddChild(child1)
	root.AddChild(child2)

	// Only mark child1 as needing paint
	child1.MarkNeedsPaint()

	// Simulate what FlushPaint would return - only child1 is dirty
	dirtyBoundaries := []layout.RenderObject{child1}

	// Record using the optimized function
	recordDirtyLayers(dirtyBoundaries, false, 1.0)

	// Only child1 should have been recorded
	if len(recordingOrder) != 1 {
		t.Fatalf("expected 1 recording, got %d: %v", len(recordingOrder), recordingOrder)
	}
	if recordingOrder[0] != "child1" {
		t.Errorf("expected 'child1' to be recorded, got %q", recordingOrder[0])
	}

	// child1's layer should now have content
	child1Layer := child1.EnsureLayer()
	if child1Layer.Content == nil {
		t.Fatal("child1 layer should have content after recording")
	}
}

// TestRecordDirtyLayersReverseDepthOrder verifies that recordDirtyLayers
// processes boundaries in reverse depth order (children before parents)
func TestRecordDirtyLayersReverseDepthOrder(t *testing.T) {
	var recordingOrder []string

	// Build tree:
	//       root
	//         \
	//        child
	root := newMockBoundary("root", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	root.AddChild(child)

	// Mark both as needing paint
	root.MarkNeedsPaint()
	child.MarkNeedsPaint()

	// Simulate FlushPaint return (sorted by depth - parents first)
	// root has depth 0, child has depth 1 (but depth isn't set without owner)
	// The function should process in reverse, so child is recorded before root
	dirtyBoundaries := []layout.RenderObject{root, child}

	recordDirtyLayers(dirtyBoundaries, false, 1.0)

	// Child should be recorded before root (reverse order processing).
	// With input [root, child] (depth order from FlushPaint):
	// - Process child (idx 1, reverse iteration): records "child"
	// - Process root (idx 0, reverse iteration): DFS stops at child boundary,
	//   then records "root"
	// This ensures child layer has content when root records its DrawChildLayer op.
	expected := []string{"child", "root"}

	if len(recordingOrder) != len(expected) {
		t.Fatalf("expected %d recordings, got %d: %v", len(expected), len(recordingOrder), recordingOrder)
	}

	for i, name := range expected {
		if recordingOrder[i] != name {
			t.Errorf("recording order[%d]: expected %q, got %q (full order: %v)",
				i, name, recordingOrder[i], recordingOrder)
		}
	}
}

// TestDFSStopsAtChildBoundaries verifies that recordDirtyLayersDFS stops at
// child boundaries and doesn't traverse their subtrees
func TestDFSStopsAtChildBoundaries(t *testing.T) {
	var recordingOrder []string

	// Build tree:
	//       root
	//         \
	//        child (boundary)
	//           \
	//          grandchild (boundary)
	root := newMockBoundary("root", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	grandchild := newMockBoundary("grandchild", &recordingOrder)

	root.AddChild(child)
	child.AddChild(grandchild)

	// Mark all as needing paint
	root.MarkNeedsPaint()
	child.MarkNeedsPaint()
	grandchild.MarkNeedsPaint()

	// Call DFS on root only - should only record root, not descend into child/grandchild
	recordDirtyLayersDFS(root, false, 1.0, true)

	// Only root should be recorded (DFS stops at child boundary)
	if len(recordingOrder) != 1 {
		t.Fatalf("expected 1 recording (DFS stops at boundaries), got %d: %v", len(recordingOrder), recordingOrder)
	}
	if recordingOrder[0] != "root" {
		t.Errorf("expected 'root' to be recorded, got %q", recordingOrder[0])
	}
}

// TestLayerDisposedOnBoundaryRemoval verifies that when a repaint boundary
// render object is disposed, its layer and display list content are released.
func TestLayerDisposedOnBoundaryRemoval(t *testing.T) {
	var recordingOrder []string

	// Create a boundary render object
	box := newMockBoundary("box", &recordingOrder)
	box.MarkNeedsPaint()

	// Record to create layer with content
	recordDirtyLayersDFS(box, false, 1.0, true)

	// Verify layer exists with content
	layer := box.Layer()
	if layer == nil {
		t.Fatal("layer should exist after recording")
	}
	if layer.Content == nil {
		t.Fatal("layer content should exist after recording")
	}

	// Dispose the render object (simulates what happens during Unmount)
	box.Dispose()

	// Verify layer is nil after disposal
	if box.Layer() != nil {
		t.Error("layer should be nil after Dispose()")
	}

	// Verify the layer's content was disposed (Content set to nil by Dispose)
	if layer.Content != nil {
		t.Error("layer content should be nil after Dispose()")
	}
}

// TestChildOffsetChangeInvalidatesParentLayer verifies that when a repaint-boundary
// child's offset changes during layout, the parent boundary is marked dirty.
// This is critical because the parent's DrawChildLayer op embeds the child offset,
// so the parent layer must be re-recorded when the child moves.
func TestChildOffsetChangeInvalidatesParentLayer(t *testing.T) {
	var recordingOrder []string

	// Create parent and child boundaries
	parent := newMockBoundary("parent", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	parent.AddChild(child)

	// Initial recording - both boundaries recorded
	parent.MarkNeedsPaint()
	child.MarkNeedsPaint()
	dirtyBoundaries := []layout.RenderObject{parent, child}
	recordDirtyLayers(dirtyBoundaries, false, 1.0)

	// Verify both were recorded
	parentLayer := parent.EnsureLayer()
	childLayer := child.EnsureLayer()
	if parentLayer.Content == nil || childLayer.Content == nil {
		t.Fatal("both layers should have content after initial recording")
	}

	// Clear recording order for next check
	recordingOrder = nil

	// Both layers should be clean now
	if parentLayer.Dirty || childLayer.Dirty {
		t.Fatal("layers should be clean after recording")
	}

	// Change child's offset via SetParentData (simulates layout change)
	// This should mark the parent dirty because parent's DrawChildLayer op has the old offset
	child.SetParentData(&layout.BoxParentData{
		Offset: graphics.Offset{X: 50, Y: 50}, // Different from initial (0,0)
	})

	// Parent layer should now be dirty (child offset changed)
	if !parentLayer.Dirty {
		t.Error("parent layer should be dirty after child offset change")
	}

	// Child layer should still be clean (its content didn't change, only position)
	if childLayer.Dirty {
		t.Error("child layer should remain clean - only its position changed, not content")
	}

	// Verify parent needs paint flag was set
	if !parent.NeedsPaint() {
		t.Error("parent should need paint after child offset change")
	}

	// Record dirty layers - parent should be re-recorded
	recordDirtyLayers([]layout.RenderObject{parent}, false, 1.0)

	// Only parent should be recorded (child content unchanged)
	if len(recordingOrder) != 1 || recordingOrder[0] != "parent" {
		t.Errorf("expected only parent to be re-recorded, got: %v", recordingOrder)
	}
}

// TestChildSizeChangeInvalidatesLayer verifies that when a render object's
// size changes during layout, it is marked for repaint.
func TestChildSizeChangeInvalidatesLayer(t *testing.T) {
	var recordingOrder []string

	// Create a boundary
	box := newMockBoundary("box", &recordingOrder)
	box.MarkNeedsPaint()

	// Initial recording
	recordDirtyLayersDFS(box, false, 1.0, true)

	layer := box.EnsureLayer()
	if layer.Content == nil {
		t.Fatal("layer should have content after initial recording")
	}

	// Clear flags
	recordingOrder = nil
	if layer.Dirty || box.NeedsPaint() {
		t.Fatal("layer and box should be clean after recording")
	}

	// Change size - this should mark paint dirty
	box.SetSize(graphics.Size{Width: 200, Height: 200})

	// Layer should now be dirty
	if !layer.Dirty {
		t.Error("layer should be dirty after size change")
	}
	if !box.NeedsPaint() {
		t.Error("box should need paint after size change")
	}

	// Re-record
	recordDirtyLayersDFS(box, false, 1.0, true)

	// Box should be recorded
	if len(recordingOrder) != 1 || recordingOrder[0] != "box" {
		t.Errorf("expected box to be re-recorded, got: %v", recordingOrder)
	}
}

// TestSameSizeNoRepaint verifies that setting the same size doesn't trigger repaint
func TestSameSizeNoRepaint(t *testing.T) {
	var recordingOrder []string

	box := newMockBoundary("box", &recordingOrder)
	box.MarkNeedsPaint()

	// Initial recording
	recordDirtyLayersDFS(box, false, 1.0, true)

	layer := box.EnsureLayer()
	recordingOrder = nil

	// Set same size - should NOT trigger repaint
	box.SetSize(graphics.Size{Width: 100, Height: 100}) // Same as initial size

	if layer.Dirty {
		t.Error("layer should NOT be dirty after setting same size")
	}
	if box.NeedsPaint() {
		t.Error("box should NOT need paint after setting same size")
	}
}

// TestSameOffsetNoRepaint verifies that setting the same offset doesn't trigger parent repaint
func TestSameOffsetNoRepaint(t *testing.T) {
	var recordingOrder []string

	parent := newMockBoundary("parent", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	parent.AddChild(child)

	// Set initial offset
	child.SetParentData(&layout.BoxParentData{
		Offset: graphics.Offset{X: 10, Y: 10},
	})

	// Initial recording
	parent.MarkNeedsPaint()
	child.MarkNeedsPaint()
	recordDirtyLayers([]layout.RenderObject{parent, child}, false, 1.0)

	parentLayer := parent.EnsureLayer()
	recordingOrder = nil

	// Set same offset - should NOT trigger parent repaint
	child.SetParentData(&layout.BoxParentData{
		Offset: graphics.Offset{X: 10, Y: 10}, // Same as before
	})

	if parentLayer.Dirty {
		t.Error("parent layer should NOT be dirty after setting same offset")
	}
	if parent.NeedsPaint() {
		t.Error("parent should NOT need paint after setting same offset")
	}
}

// TestFirstParentDataAssignmentInvalidatesParent verifies that the first SetParentData
// call marks the parent dirty, even when the offset is zero. This ensures newly added
// children appear in the parent's layer (parent needs to record a DrawChildLayer op).
func TestFirstParentDataAssignmentInvalidatesParent(t *testing.T) {
	var recordingOrder []string

	parent := newMockBoundary("parent", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	parent.AddChild(child)

	// Initial recording (child has no parent data yet)
	parent.MarkNeedsPaint()
	child.MarkNeedsPaint()
	recordDirtyLayers([]layout.RenderObject{parent, child}, false, 1.0)

	parentLayer := parent.EnsureLayer()
	recordingOrder = nil

	// Clear dirty flags
	parentLayer.Dirty = false
	parent.ClearNeedsPaint()

	// First SetParentData with zero offset - should still mark parent dirty
	// because parent's DrawChildLayer ops change (child is now positioned)
	child.SetParentData(&layout.BoxParentData{
		Offset: graphics.Offset{X: 0, Y: 0}, // Zero offset
	})

	if !parentLayer.Dirty {
		t.Error("parent layer should be dirty after first SetParentData (even with zero offset)")
	}
	if !parent.NeedsPaint() {
		t.Error("parent should need paint after first SetParentData")
	}
}

// mockBoundaryWithRemovableChild is a boundary that supports add/remove of children
type mockBoundaryWithRemovableChild struct {
	layout.RenderBoxBase
	name           string
	recordingOrder *[]string
	children       []layout.RenderBox
}

func newMockBoundaryRemovable(name string, recordingOrder *[]string) *mockBoundaryWithRemovableChild {
	m := &mockBoundaryWithRemovableChild{
		name:           name,
		recordingOrder: recordingOrder,
	}
	m.SetSelf(m)
	m.SetSize(graphics.Size{Width: 100, Height: 100})
	return m
}

func (r *mockBoundaryWithRemovableChild) PerformLayout() {
	r.SetSize(graphics.Size{Width: 100, Height: 100})
}

func (r *mockBoundaryWithRemovableChild) Paint(ctx *layout.PaintContext) {
	if r.recordingOrder != nil {
		*r.recordingOrder = append(*r.recordingOrder, r.name)
	}
	for _, child := range r.children {
		if child != nil {
			ctx.PaintChildWithLayer(child, graphics.Offset{}) // Zero offset
		}
	}
}

func (r *mockBoundaryWithRemovableChild) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

func (r *mockBoundaryWithRemovableChild) IsRepaintBoundary() bool {
	return true
}

func (r *mockBoundaryWithRemovableChild) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

func (r *mockBoundaryWithRemovableChild) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		if child != nil {
			visitor(child)
		}
	}
}

func (r *mockBoundaryWithRemovableChild) AddChild(child layout.RenderBox) {
	r.children = append(r.children, child)
	if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
		setter.SetParent(r)
	}
}

func (r *mockBoundaryWithRemovableChild) RemoveChild(child layout.RenderBox) {
	for i, c := range r.children {
		if c == child {
			r.children = append(r.children[:i], r.children[i+1:]...)
			if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
				setter.SetParent(nil)
			}
			return
		}
	}
}

// TestReparentingInvalidatesParentLayers verifies that when a child is reparented,
// both the old and new parent are marked dirty. This is critical for the layer tree:
// - Old parent's DrawChildLayer ops are stale (child no longer exists there)
// - New parent needs new DrawChildLayer ops for the added child
func TestReparentingInvalidatesParentLayers(t *testing.T) {
	var recordingOrder []string

	parent1 := newMockBoundaryRemovable("parent1", &recordingOrder)
	parent2 := newMockBoundaryRemovable("parent2", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)

	// Add child to parent1
	parent1.AddChild(child)

	// Initial recording
	parent1.MarkNeedsPaint()
	parent2.MarkNeedsPaint()
	child.MarkNeedsPaint()
	recordDirtyLayers([]layout.RenderObject{parent1, parent2, child}, false, 1.0)

	parent1Layer := parent1.EnsureLayer()
	parent2Layer := parent2.EnsureLayer()
	recordingOrder = nil

	// Clear dirty flags
	parent1Layer.Dirty = false
	parent2Layer.Dirty = false
	parent1.ClearNeedsPaint()
	parent2.ClearNeedsPaint()

	// Move child from parent1 to parent2
	parent1.RemoveChild(child)
	parent2.AddChild(child)

	// Both parents should be marked dirty
	if !parent1Layer.Dirty {
		t.Error("parent1 layer should be dirty after losing child")
	}
	if !parent1.NeedsPaint() {
		t.Error("parent1 should need paint after losing child")
	}
	if !parent2Layer.Dirty {
		t.Error("parent2 layer should be dirty after gaining child")
	}
	if !parent2.NeedsPaint() {
		t.Error("parent2 should need paint after gaining child")
	}
}

// TestAddChildWithZeroOffsetInvalidatesParent verifies that adding a new boundary
// child at zero offset still invalidates the parent. Regression test for the case
// where first SetParentData with {0,0} offset wasn't detected as a change.
func TestAddChildWithZeroOffsetInvalidatesParent(t *testing.T) {
	var recordingOrder []string

	parent := newMockBoundaryRemovable("parent", &recordingOrder)

	// Initial recording (no children)
	parent.MarkNeedsPaint()
	recordDirtyLayers([]layout.RenderObject{parent}, false, 1.0)

	parentLayer := parent.EnsureLayer()
	recordingOrder = nil

	// Clear dirty flags
	parentLayer.Dirty = false
	parent.ClearNeedsPaint()

	// Add a new child at zero offset
	child := newMockBoundary("child", &recordingOrder)
	parent.AddChild(child)

	// Parent should be marked dirty (gains DrawChildLayer op for new child)
	if !parentLayer.Dirty {
		t.Error("parent layer should be dirty after adding child")
	}
	if !parent.NeedsPaint() {
		t.Error("parent should need paint after adding child")
	}

	// Record and verify child appears
	recordDirtyLayers([]layout.RenderObject{parent, child}, false, 1.0)

	// Both should have been recorded
	if len(recordingOrder) < 2 {
		t.Errorf("expected both parent and child to be recorded, got: %v", recordingOrder)
	}
}

// TestRemoveChildInvalidatesParent verifies that removing a boundary child
// invalidates the parent layer.
func TestRemoveChildInvalidatesParent(t *testing.T) {
	var recordingOrder []string

	parent := newMockBoundaryRemovable("parent", &recordingOrder)
	child := newMockBoundary("child", &recordingOrder)
	parent.AddChild(child)

	// Initial recording
	parent.MarkNeedsPaint()
	child.MarkNeedsPaint()
	recordDirtyLayers([]layout.RenderObject{parent, child}, false, 1.0)

	parentLayer := parent.EnsureLayer()
	recordingOrder = nil

	// Clear dirty flags
	parentLayer.Dirty = false
	parent.ClearNeedsPaint()

	// Remove child
	parent.RemoveChild(child)

	// Parent should be marked dirty (loses DrawChildLayer op)
	if !parentLayer.Dirty {
		t.Error("parent layer should be dirty after removing child")
	}
	if !parent.NeedsPaint() {
		t.Error("parent should need paint after removing child")
	}
}
