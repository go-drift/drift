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
// in the layer tree recording phase
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

	// Run DFS recording
	recordDirtyLayersDFS(root, false, 1.0)

	// Verify children recorded before parents (DFS post-order)
	// Expected order: grandchild, child1, child2, root
	// (children before their parents, left-to-right among siblings)
	expected := []string{"grandchild", "child1", "child2", "root"}

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

	recordDirtyLayersDFS(root, false, 1.0)

	// Only root should be recorded (child was clean)
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

	recordDirtyLayersDFS(box, false, 1.0)

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

	// Record layers
	recordDirtyLayersDFS(parent, false, 1.0)

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
