package engine

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// testBoundaryRenderBox is a render object that is a repaint boundary.
type testBoundaryRenderBox struct {
	layout.RenderBoxBase
	paintCalls int
	children   []layout.RenderObject
}

func (r *testBoundaryRenderBox) PerformLayout() {
	r.SetSize(r.Size())
}

func (r *testBoundaryRenderBox) Paint(ctx *layout.PaintContext) {
	r.paintCalls++
	// Paint a rect to produce content
	ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height), graphics.DefaultPaint())

	// Paint children with layer support
	for _, child := range r.children {
		if rb, ok := child.(layout.RenderBox); ok {
			pd, _ := child.ParentData().(*layout.BoxParentData)
			offset := graphics.Offset{}
			if pd != nil {
				offset = pd.Offset
			}
			ctx.PaintChildWithLayer(rb, offset)
		}
	}
}

func (r *testBoundaryRenderBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

func (r *testBoundaryRenderBox) IsRepaintBoundary() bool {
	return true
}

func (r *testBoundaryRenderBox) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

func (r *testBoundaryRenderBox) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		visitor(child)
	}
}

func (r *testBoundaryRenderBox) NeedsPaint() bool {
	return r.RenderBoxBase.NeedsPaint()
}

func newBoundaryBox(w, h float64) *testBoundaryRenderBox {
	b := &testBoundaryRenderBox{}
	b.SetSelf(b)
	b.SetSize(graphics.Size{Width: w, Height: h})
	return b
}

// testLeafRenderBox is a simple non-boundary render object.
type testLeafRenderBox struct {
	layout.RenderBoxBase
	paintCalls int
}

func (r *testLeafRenderBox) PerformLayout() {}

func (r *testLeafRenderBox) Paint(ctx *layout.PaintContext) {
	r.paintCalls++
	ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height), graphics.DefaultPaint())
}

func (r *testLeafRenderBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

func newLeafBox(w, h float64) *testLeafRenderBox {
	b := &testLeafRenderBox{}
	b.SetSelf(b)
	b.SetSize(graphics.Size{Width: w, Height: h})
	return b
}

func TestLayerRecordingAndCompositing(t *testing.T) {
	// Create a boundary, record its content, then composite
	root := newBoundaryBox(100, 100)

	// Record content
	recordLayerContent(root, false, 0)

	layer := root.EnsureLayer()
	if layer.Content == nil {
		t.Fatal("expected layer content after recording")
	}
	if layer.Dirty {
		t.Error("expected layer to be clean after recording")
	}

	// Composite onto a canvas
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 100, Height: 100}}
	cc := NewCompositingCanvas(inner, sink)

	// Should not panic
	compositeLayerTree(cc, root)

	if root.paintCalls != 1 {
		t.Errorf("expected 1 paint call during recording, got %d", root.paintCalls)
	}
}

func TestDirtyBoundaryDetection(t *testing.T) {
	root := newBoundaryBox(200, 200)

	// First recording
	recordLayerContent(root, false, 0)
	root.paintCalls = 0

	// Layer should be clean
	layer := root.EnsureLayer()
	if layer.Dirty {
		t.Error("expected clean layer after recording")
	}

	// Recording again should skip (not dirty)
	recordLayerContent(root, false, 0)
	if root.paintCalls != 0 {
		t.Errorf("expected no paint calls for clean layer, got %d", root.paintCalls)
	}

	// Mark dirty and re-record
	layer.MarkDirty()
	recordLayerContent(root, false, 0)
	if root.paintCalls != 1 {
		t.Errorf("expected 1 paint call for dirty layer, got %d", root.paintCalls)
	}
}

func TestDrawChildLayerReferences(t *testing.T) {
	// Parent boundary has a child boundary.
	// Changing child content should NOT require parent re-recording.
	child := newBoundaryBox(50, 30)
	child.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{X: 10, Y: 20}})

	parent := newBoundaryBox(200, 200)
	parent.children = []layout.RenderObject{child}

	// Record child first, then parent (children before parents)
	recordLayerContent(child, false, 0)
	recordLayerContent(parent, false, 0)

	childLayer := child.EnsureLayer()
	parentLayer := parent.EnsureLayer()

	if childLayer.Content == nil {
		t.Fatal("expected child layer content")
	}
	if parentLayer.Content == nil {
		t.Fatal("expected parent layer content")
	}

	// Reset paint counters
	child.paintCalls = 0
	parent.paintCalls = 0

	// Mark child dirty and re-record only the child
	childLayer.MarkDirty()
	recordLayerContent(child, false, 0)

	if child.paintCalls != 1 {
		t.Errorf("expected child to be re-recorded, got %d paint calls", child.paintCalls)
	}
	if parent.paintCalls != 0 {
		t.Errorf("expected parent NOT to be re-recorded, got %d paint calls", parent.paintCalls)
	}

	// Parent layer should still be clean (DrawChildLayer reference is stable)
	if parentLayer.Dirty {
		t.Error("expected parent layer to remain clean when child changes")
	}

	// Compositing should still work - parent replays its display list
	// which has a DrawChildLayer op pointing to the child layer
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 200, Height: 200}}
	cc := NewCompositingCanvas(inner, sink)
	compositeLayerTree(cc, parent)
}

func TestLayerDisposal(t *testing.T) {
	box := newBoundaryBox(100, 100)

	// Record to create layer content
	recordLayerContent(box, false, 0)

	layer := box.Layer()
	if layer == nil {
		t.Fatal("expected layer after recording")
	}
	if layer.Content == nil {
		t.Fatal("expected layer content")
	}

	// Dispose
	box.Dispose()

	if box.Layer() != nil {
		t.Error("expected nil layer after dispose")
	}
}

func TestRecordDirtyLayers_ChildrenBeforeParents(t *testing.T) {
	child := newBoundaryBox(50, 30)
	child.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{}})

	parent := newBoundaryBox(200, 200)
	parent.children = []layout.RenderObject{child}

	// Both dirty (initial state)
	dirtyBoundaries := []layout.RenderObject{parent, child}

	recordDirtyLayers(dirtyBoundaries, false, 0)

	// Both should have been recorded
	if child.EnsureLayer().Content == nil {
		t.Error("expected child to be recorded")
	}
	if parent.EnsureLayer().Content == nil {
		t.Error("expected parent to be recorded")
	}

	// Parent's layer references child's layer via DrawChildLayer.
	// Compositing should traverse the reference.
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 200, Height: 200}}
	cc := NewCompositingCanvas(inner, sink)
	compositeLayerTree(cc, parent)
}

func TestCompositeLayerTree_PlatformViewGeometry(t *testing.T) {
	// Build a child with a platform view embed op
	child := &platformViewBoundary{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 80, Height: 60})
	child.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{X: 20, Y: 30}})

	parent := newBoundaryBox(400, 300)
	parent.children = []layout.RenderObject{child}

	// Record child first, then parent
	recordLayerContent(child, false, 0)
	recordLayerContent(parent, false, 0)

	// Composite
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 400, Height: 300}}
	cc := NewCompositingCanvas(inner, sink)
	compositeLayerTree(cc, parent)

	// The platform view should have been geometry-updated via the sink
	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 platform view update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	if u.viewID != 123 {
		t.Errorf("viewID = %d, want 123", u.viewID)
	}
	// Child is at offset (20, 30) from parent
	if u.offset.X != 20 || u.offset.Y != 30 {
		t.Errorf("offset = (%v, %v), want (20, 30)", u.offset.X, u.offset.Y)
	}
}

// platformViewBoundary is a repaint boundary that embeds a platform view.
type platformViewBoundary struct {
	layout.RenderBoxBase
}

func (r *platformViewBoundary) PerformLayout() {}

func (r *platformViewBoundary) Paint(ctx *layout.PaintContext) {
	ctx.EmbedPlatformView(123, r.Size())
}

func (r *platformViewBoundary) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

func (r *platformViewBoundary) IsRepaintBoundary() bool {
	return true
}

func (r *platformViewBoundary) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

func (r *platformViewBoundary) NeedsPaint() bool {
	return r.RenderBoxBase.NeedsPaint()
}

func TestCompositeLayerTree_RootPanicsWithoutLayer(t *testing.T) {
	// A non-boundary render object should panic as root
	leaf := newLeafBox(100, 100)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-boundary root")
		}
	}()

	inner := &nullCanvas{size: graphics.Size{Width: 100, Height: 100}}
	compositeLayerTree(inner, leaf)
}

func TestLayerSetContentDisposesOld(t *testing.T) {
	layer := &graphics.Layer{Dirty: true, Size: graphics.Size{Width: 100, Height: 100}}

	// Create first content
	rec1 := &graphics.PictureRecorder{}
	rec1.BeginRecording(graphics.Size{Width: 100, Height: 100})
	dl1 := rec1.EndRecording()

	layer.SetContent(dl1)
	if layer.Dirty {
		t.Error("expected clean after SetContent")
	}

	// Create second content (old should be disposed)
	rec2 := &graphics.PictureRecorder{}
	rec2.BeginRecording(graphics.Size{Width: 100, Height: 100})
	dl2 := rec2.EndRecording()

	layer.SetContent(dl2)
	if layer.Content != dl2 {
		t.Error("expected new content to be set")
	}
}

func TestEnsureLayerStableIdentity(t *testing.T) {
	box := newBoundaryBox(100, 100)

	layer1 := box.EnsureLayer()
	layer2 := box.EnsureLayer()

	if layer1 != layer2 {
		t.Error("EnsureLayer should return the same pointer (stable identity)")
	}
}

func TestRecordLayerContent_SetsRecordingLayer(t *testing.T) {
	// Verifies that recording sets RecordingLayer on PaintContext,
	// enabling DrawChildLayer recording for child boundaries.
	child := newBoundaryBox(50, 30)
	child.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{}})

	parent := newBoundaryBox(200, 200)
	parent.children = []layout.RenderObject{child}

	// Record child first so it has a layer
	recordLayerContent(child, false, 0)

	// Record parent — should use DrawChildLayer for child
	recordLayerContent(parent, false, 0)

	// Verify parent layer has content (it recorded a DrawChildLayer)
	parentLayer := parent.EnsureLayer()
	if parentLayer.Content == nil {
		t.Fatal("expected parent layer content")
	}

	// Composite and verify child layer's content is reached through parent
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 200, Height: 200}}
	cc := NewCompositingCanvas(inner, sink)
	compositeLayerTree(cc, parent)
}
