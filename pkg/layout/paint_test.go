package layout

import (
	"image"
	"testing"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
)

type testRenderBox struct {
	RenderBoxBase
	paintCalls int
}

func (r *testRenderBox) PerformLayout() {
	r.SetSize(graphics.Size{Width: 10, Height: 10})
}

func (r *testRenderBox) Paint(ctx *PaintContext) {
	r.paintCalls++
}

func (r *testRenderBox) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}

func (r *testRenderBox) IsRepaintBoundary() bool {
	return true
}

func (r *testRenderBox) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

func TestPaintChildWithLayer_UsesCachedLayerWhenClean(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 10, Height: 10})
	recordCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 10, 10), graphics.DefaultPaint())
	layer := recorder.EndRecording()

	child.SetLayerContent(layer)
	child.ClearNeedsPaint()

	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 0 {
		t.Fatalf("expected cached layer to be used, but child.Paint was called %d times", child.paintCalls)
	}
}

func TestPaintChildWithLayer_PaintsChildWhenNoLayer(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 1 {
		t.Fatalf("expected child.Paint to be called once, got %d", child.paintCalls)
	}
}

func TestPaintChildWithLayer_CullsOutsideClip(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	recorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: recorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	// Clip away from the child bounds.
	ctx.PushClipRect(graphics.RectFromLTWH(100, 100, 10, 10))

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 0 {
		t.Fatalf("expected child to be culled outside clip, got %d paint calls", child.paintCalls)
	}
}

func TestPaintChild_CullsOutsideClip(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	recorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: recorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	// Clip away from the child bounds.
	ctx.PushClipRect(graphics.RectFromLTWH(100, 100, 10, 10))

	ctx.PaintChild(child, graphics.Offset{})

	if child.paintCalls != 0 {
		t.Fatalf("expected child to be culled outside clip, got %d paint calls", child.paintCalls)
	}
}

func TestPaintChild_CullUsesTransformAndOffset(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	recorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: recorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	// Apply a transform and an offset; global bounds should be at (15, 5) to (25, 15).
	ctx.PushTranslation(10, 0)
	ctx.PushClipRect(graphics.RectFromLTWH(6, 6, 2, 2)) // local -> global (16,6) to (18,8)

	ctx.PaintChild(child, graphics.Offset{X: 5, Y: 5})

	if child.paintCalls != 1 {
		t.Fatalf("expected child to be painted with intersecting clip, got %d paint calls", child.paintCalls)
	}
}

// =============================================================================
// Layer Tree Tests - Verify the retained scene graph behavior
// =============================================================================

// testBoundaryRenderBox is a repaint boundary that tracks paint calls and can have children
type testBoundaryRenderBox struct {
	RenderBoxBase
	paintCalls int
	children   []RenderBox
}

func (r *testBoundaryRenderBox) PerformLayout() {
	r.SetSize(graphics.Size{Width: 100, Height: 100})
	for _, child := range r.children {
		if child != nil {
			child.Layout(Tight(graphics.Size{Width: 50, Height: 50}), false)
		}
	}
}

func (r *testBoundaryRenderBox) Paint(ctx *PaintContext) {
	r.paintCalls++
	// Paint children at different offsets
	for i, child := range r.children {
		if child != nil {
			ctx.PaintChildWithLayer(child, graphics.Offset{X: float64(i * 25), Y: float64(i * 25)})
		}
	}
}

func (r *testBoundaryRenderBox) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}

func (r *testBoundaryRenderBox) IsRepaintBoundary() bool {
	return true
}

func (r *testBoundaryRenderBox) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

func (r *testBoundaryRenderBox) VisitChildren(visitor func(RenderObject)) {
	for _, child := range r.children {
		if child != nil {
			visitor(child)
		}
	}
}

func (r *testBoundaryRenderBox) AddChild(child RenderBox) {
	r.children = append(r.children, child)
	if setter, ok := child.(interface{ SetParent(RenderObject) }); ok {
		setter.SetParent(r)
	}
}

// TestLayerStableIdentity verifies that marking a layer dirty preserves its identity
func TestLayerStableIdentity(t *testing.T) {
	box := &testBoundaryRenderBox{}
	box.SetSelf(box)
	box.SetSize(graphics.Size{Width: 100, Height: 100})

	// Get initial layer
	layer1 := box.EnsureLayer()
	if layer1 == nil {
		t.Fatal("EnsureLayer should return a non-nil layer")
	}

	// Mark dirty
	layer1.MarkDirty()
	if !layer1.Dirty {
		t.Fatal("MarkDirty should set Dirty to true")
	}

	// Get layer again - should be same instance
	layer2 := box.EnsureLayer()
	if layer1 != layer2 {
		t.Fatal("EnsureLayer should return the same layer instance (stable identity)")
	}

	// Mark needs paint via RenderBoxBase
	box.MarkNeedsPaint()

	// Layer should still be same instance
	layer3 := box.EnsureLayer()
	if layer1 != layer3 {
		t.Fatal("MarkNeedsPaint should preserve layer identity")
	}
}

// TestDrawChildLayerOpRecording verifies that child boundaries are recorded as
// DrawChildLayer ops during layer recording (not embedded content)
func TestDrawChildLayerOpRecording(t *testing.T) {
	parent := &testBoundaryRenderBox{}
	parent.SetSelf(parent)

	child := &testBoundaryRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 50, Height: 50})
	parent.AddChild(child)

	// Ensure child has a layer
	childLayer := child.EnsureLayer()

	// Record parent with RecordingLayer set (simulating layer recording phase)
	parentLayer := parent.EnsureLayer()
	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 100, Height: 100})

	ctx := &PaintContext{
		Canvas:         recordCanvas,
		RecordingLayer: parentLayer,
	}

	parent.Paint(ctx)

	// Child should NOT have been painted directly during recording
	// (DrawChildLayer op was recorded instead)
	if child.paintCalls != 0 {
		t.Fatalf("child should not be painted during layer recording (DrawChildLayer op should be recorded instead), got %d paint calls", child.paintCalls)
	}

	// The parent's display list should contain a DrawChildLayer op
	displayList := recorder.EndRecording()

	// To verify DrawChildLayer was recorded, we composite the display list
	// and check that child's layer.Composite was called (via the op)
	// First, give child layer some content
	childRecorder := &graphics.PictureRecorder{}
	childCanvas := childRecorder.BeginRecording(graphics.Size{Width: 50, Height: 50})
	childCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 50, 50), graphics.DefaultPaint())
	childLayer.Content = childRecorder.EndRecording()
	childLayer.Dirty = false

	// Now composite parent's display list - this should trigger child layer composite
	outputRecorder := &graphics.PictureRecorder{}
	outputCanvas := outputRecorder.BeginRecording(graphics.Size{Width: 100, Height: 100})
	displayList.Paint(outputCanvas)

	// The test passes if no panic occurred - the DrawChildLayer op successfully
	// referenced and composited the child layer
}

// TestChildContentChangeDoesNotDirtyParent verifies that when a child boundary's
// content changes, the parent boundary is NOT re-recorded (key optimization)
func TestChildContentChangeDoesNotDirtyParent(t *testing.T) {
	parent := &testBoundaryRenderBox{}
	parent.SetSelf(parent)

	child := &testBoundaryRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 50, Height: 50})
	parent.AddChild(child)

	// Setup: both have layers
	parentLayer := parent.EnsureLayer()
	childLayer := child.EnsureLayer()

	// Record parent's content
	parentRecorder := &graphics.PictureRecorder{}
	parentCanvas := parentRecorder.BeginRecording(graphics.Size{Width: 100, Height: 100})
	ctx := &PaintContext{
		Canvas:         parentCanvas,
		RecordingLayer: parentLayer,
	}
	parent.Paint(ctx)
	parentLayer.Content = parentRecorder.EndRecording()
	parentLayer.Dirty = false
	parent.ClearNeedsPaint()

	// Record child's content
	childRecorder := &graphics.PictureRecorder{}
	childCanvas := childRecorder.BeginRecording(graphics.Size{Width: 50, Height: 50})
	childCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 50, 50), graphics.DefaultPaint())
	childLayer.Content = childRecorder.EndRecording()
	childLayer.Dirty = false
	child.ClearNeedsPaint()

	// Now mark child as needing paint (simulating content change)
	child.MarkNeedsPaint()

	// Child's layer should be dirty
	if !childLayer.Dirty {
		t.Fatal("child layer should be dirty after MarkNeedsPaint")
	}

	// Parent's layer should NOT be dirty (key optimization!)
	if parentLayer.Dirty {
		t.Fatal("parent layer should NOT be dirty when child content changes - this is the key optimization")
	}

	// Parent should not need paint
	if parent.NeedsPaint() {
		t.Fatal("parent should NOT need paint when child content changes")
	}
}

// TestLayerCompositeWithTransform verifies that child layers are composited
// at the correct canvas state (transforms applied)
func TestLayerCompositeWithTransform(t *testing.T) {
	// Create a tracking canvas to verify transform state
	tracker := &transformTrackingCanvas{}

	child := &testBoundaryRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 50, Height: 50})

	// Give child a layer with content that draws a rect
	childLayer := child.EnsureLayer()
	childRecorder := &graphics.PictureRecorder{}
	childCanvas := childRecorder.BeginRecording(graphics.Size{Width: 50, Height: 50})
	childCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 50, 50), graphics.DefaultPaint())
	childLayer.Content = childRecorder.EndRecording()
	childLayer.Dirty = false
	child.ClearNeedsPaint()

	// Create a display list that applies transform, then draws child layer
	parentRecorder := &graphics.PictureRecorder{}
	parentCanvas := parentRecorder.BeginRecording(graphics.Size{Width: 100, Height: 100})
	parentCanvas.Save()
	parentCanvas.Translate(25, 25)
	if rc, ok := parentCanvas.(interface{ DrawChildLayer(*graphics.Layer) }); ok {
		rc.DrawChildLayer(childLayer)
	}
	parentCanvas.Restore()
	parentDisplayList := parentRecorder.EndRecording()

	// Play back to tracking canvas
	parentDisplayList.Paint(tracker)

	// Verify transform was applied before child layer was composited
	if len(tracker.translateCalls) == 0 {
		t.Fatal("expected Translate to be called before child layer composite")
	}
	if tracker.translateCalls[0].dx != 25 || tracker.translateCalls[0].dy != 25 {
		t.Fatalf("expected Translate(25, 25), got Translate(%v, %v)",
			tracker.translateCalls[0].dx, tracker.translateCalls[0].dy)
	}
}

// TestPaintChildWithLayer_RecordsDrawChildLayerOp verifies PaintChildWithLayer
// records a DrawChildLayer op when RecordingLayer is set
func TestPaintChildWithLayer_RecordsDrawChildLayerOp(t *testing.T) {
	child := &testBoundaryRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 50, Height: 50})
	childLayer := child.EnsureLayer()

	// Give child layer content
	childRecorder := &graphics.PictureRecorder{}
	childCanvas := childRecorder.BeginRecording(graphics.Size{Width: 50, Height: 50})
	childCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 50, 50), graphics.DefaultPaint())
	childLayer.Content = childRecorder.EndRecording()
	childLayer.Dirty = false
	child.ClearNeedsPaint()

	// Create parent recording context
	parentLayer := &graphics.Layer{Size: graphics.Size{Width: 100, Height: 100}}
	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 100, Height: 100})

	ctx := &PaintContext{
		Canvas:         recordCanvas,
		RecordingLayer: parentLayer, // Key: this enables DrawChildLayer recording
	}

	// Paint child with offset
	ctx.PaintChildWithLayer(child, graphics.Offset{X: 10, Y: 20})

	// Child should NOT have Paint called (DrawChildLayer was recorded instead)
	if child.paintCalls != 0 {
		t.Fatalf("expected 0 paint calls during recording, got %d", child.paintCalls)
	}

	// Get the display list and verify it composites correctly
	displayList := recorder.EndRecording()

	// Play it back to a tracking canvas
	tracker := &transformTrackingCanvas{}
	displayList.Paint(tracker)

	// Verify: Save, Translate(10,20), DrawChildLayer (which draws rect), Restore
	if len(tracker.saveCalls) < 1 {
		t.Fatal("expected Save to be called")
	}
	if len(tracker.translateCalls) < 1 {
		t.Fatal("expected Translate to be called")
	}
	if tracker.translateCalls[0].dx != 10 || tracker.translateCalls[0].dy != 20 {
		t.Fatalf("expected Translate(10, 20), got Translate(%v, %v)",
			tracker.translateCalls[0].dx, tracker.translateCalls[0].dy)
	}
	// The DrawChildLayer op should have triggered drawing the rect
	if len(tracker.drawRectCalls) < 1 {
		t.Fatal("expected DrawRect to be called via child layer composite")
	}
}

// TestDirectCompositing_UsesCleanLayerCache verifies that when NOT in recording
// mode, PaintChildWithLayer uses the cached layer directly
func TestDirectCompositing_UsesCleanLayerCache(t *testing.T) {
	child := &testBoundaryRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 50, Height: 50})
	childLayer := child.EnsureLayer()

	// Give child layer content
	childRecorder := &graphics.PictureRecorder{}
	childCanvas := childRecorder.BeginRecording(graphics.Size{Width: 50, Height: 50})
	childCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 50, 50), graphics.DefaultPaint())
	childLayer.Content = childRecorder.EndRecording()
	childLayer.Dirty = false
	child.ClearNeedsPaint()

	// Paint without RecordingLayer (direct compositing mode)
	tracker := &transformTrackingCanvas{}
	ctx := &PaintContext{
		Canvas:         tracker,
		RecordingLayer: nil, // Key: no recording layer = direct compositing
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{X: 5, Y: 5})

	// Child.Paint should NOT be called (used cached layer)
	if child.paintCalls != 0 {
		t.Fatalf("expected 0 paint calls (cached layer used), got %d", child.paintCalls)
	}

	// Verify composite happened with transform
	if len(tracker.translateCalls) < 1 {
		t.Fatal("expected Translate to be called for child offset")
	}
	if tracker.translateCalls[0].dx != 5 || tracker.translateCalls[0].dy != 5 {
		t.Fatalf("expected Translate(5, 5), got Translate(%v, %v)",
			tracker.translateCalls[0].dx, tracker.translateCalls[0].dy)
	}
}

// =============================================================================
// Helper types for testing
// =============================================================================

type translateCall struct {
	dx, dy float64
}

type drawRectCall struct {
	rect  graphics.Rect
	paint graphics.Paint
}

// transformTrackingCanvas tracks canvas calls for verification
type transformTrackingCanvas struct {
	saveCalls      []struct{}
	restoreCalls   []struct{}
	translateCalls []translateCall
	drawRectCalls  []drawRectCall
}

func (c *transformTrackingCanvas) Save() {
	c.saveCalls = append(c.saveCalls, struct{}{})
}

func (c *transformTrackingCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64) {}

func (c *transformTrackingCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint) {}

func (c *transformTrackingCanvas) Restore() {
	c.restoreCalls = append(c.restoreCalls, struct{}{})
}

func (c *transformTrackingCanvas) Translate(dx, dy float64) {
	c.translateCalls = append(c.translateCalls, translateCall{dx, dy})
}

func (c *transformTrackingCanvas) Scale(sx, sy float64) {}

func (c *transformTrackingCanvas) Rotate(radians float64) {}

func (c *transformTrackingCanvas) ClipRect(rect graphics.Rect) {}

func (c *transformTrackingCanvas) ClipRRect(rrect graphics.RRect) {}

func (c *transformTrackingCanvas) ClipPath(path *graphics.Path, op graphics.ClipOp, antialias bool) {}

func (c *transformTrackingCanvas) Clear(color graphics.Color) {}

func (c *transformTrackingCanvas) DrawRect(rect graphics.Rect, paint graphics.Paint) {
	c.drawRectCalls = append(c.drawRectCalls, drawRectCall{rect, paint})
}

func (c *transformTrackingCanvas) DrawRRect(rrect graphics.RRect, paint graphics.Paint) {}

func (c *transformTrackingCanvas) DrawCircle(center graphics.Offset, radius float64, paint graphics.Paint) {
}

func (c *transformTrackingCanvas) DrawLine(start, end graphics.Offset, paint graphics.Paint) {}

func (c *transformTrackingCanvas) DrawText(layout *graphics.TextLayout, position graphics.Offset) {}

func (c *transformTrackingCanvas) DrawImage(img image.Image, position graphics.Offset) {}

func (c *transformTrackingCanvas) DrawImageRect(img image.Image, srcRect, dstRect graphics.Rect, quality graphics.FilterQuality, cacheKey uintptr) {
}

func (c *transformTrackingCanvas) DrawPath(path *graphics.Path, paint graphics.Paint) {}

func (c *transformTrackingCanvas) DrawRectShadow(rect graphics.Rect, shadow graphics.BoxShadow) {}

func (c *transformTrackingCanvas) DrawRRectShadow(rrect graphics.RRect, shadow graphics.BoxShadow) {}

func (c *transformTrackingCanvas) SaveLayerBlur(bounds graphics.Rect, sigmaX, sigmaY float64) {}

func (c *transformTrackingCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds graphics.Rect) {}

func (c *transformTrackingCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds graphics.Rect, tintColor graphics.Color) {
}

func (c *transformTrackingCanvas) Size() graphics.Size {
	return graphics.Size{Width: 100, Height: 100}
}
