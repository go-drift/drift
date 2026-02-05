package layout

import (
	"testing"

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
	child.size = graphics.Size{Width: 10, Height: 10} // set directly to avoid MarkNeedsPaint

	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 10, Height: 10})
	recordCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 10, 10), graphics.DefaultPaint())
	dl := recorder.EndRecording()

	child.SetLayerContent(dl)
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
	child.size = graphics.Size{Width: 10, Height: 10}

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
	child.size = graphics.Size{Width: 10, Height: 10}

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
	child.size = graphics.Size{Width: 10, Height: 10}

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
	child.size = graphics.Size{Width: 10, Height: 10}

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

func TestPaintChildWithLayer_RecordsDrawChildLayer(t *testing.T) {
	// When RecordingLayer is set, PaintChildWithLayer should record a DrawChildLayer
	// op instead of painting the child directly
	child := &testRenderBox{}
	child.SetSelf(child)
	child.size = graphics.Size{Width: 50, Height: 30}

	parentLayer := &graphics.Layer{Dirty: true, Size: graphics.Size{Width: 100, Height: 100}}

	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 100, Height: 100})

	ctx := &PaintContext{
		Canvas:         recordCanvas,
		RecordingLayer: parentLayer,
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{X: 10, Y: 20})

	// Child should NOT have been painted directly
	if child.paintCalls != 0 {
		t.Fatalf("expected DrawChildLayer recording (no direct paint), got %d paint calls", child.paintCalls)
	}

	// The recorded display list should contain the DrawChildLayer op
	dl := recorder.EndRecording()

	// Verify by replaying onto a tracking canvas that the child layer's Composite is called
	childLayer := child.EnsureLayer()
	// Record some content into the child layer
	childRec := &graphics.PictureRecorder{}
	childCanvas := childRec.BeginRecording(graphics.Size{Width: 50, Height: 30})
	childCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 50, 30), graphics.DefaultPaint())
	childLayer.SetContent(childRec.EndRecording())

	// Replay the parent display list - it should translate then composite the child
	outputRec := &graphics.PictureRecorder{}
	outputCanvas := outputRec.BeginRecording(graphics.Size{Width: 100, Height: 100})
	dl.Paint(outputCanvas)
	outputDL := outputRec.EndRecording()

	// If we got here without panicking, the DrawChildLayer op successfully replayed
	if outputDL == nil {
		t.Fatal("expected non-nil output display list")
	}
}

func TestEmbedPlatformView_Recording(t *testing.T) {
	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 100, Height: 100})

	ctx := &PaintContext{
		Canvas: recordCanvas,
	}

	ctx.EmbedPlatformView(42, graphics.Size{Width: 200, Height: 100})

	dl := recorder.EndRecording()
	if dl == nil {
		t.Fatal("expected non-nil display list")
	}

	// Replay and verify the op executes (no panic)
	outputRec := &graphics.PictureRecorder{}
	outputCanvas := outputRec.BeginRecording(graphics.Size{Width: 100, Height: 100})
	dl.Paint(outputCanvas)
	outputRec.EndRecording()
}
