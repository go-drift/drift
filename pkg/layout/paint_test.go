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

func TestPaintChildWithLayer_FallsBackWhenCanvasLacksDrawChildLayer(t *testing.T) {
	// When RecordingLayer is set but the canvas does NOT support DrawChildLayer,
	// PaintChildWithLayer should fall back to painting the child directly.
	child := &testRenderBox{}
	child.SetSelf(child)
	child.size = graphics.Size{Width: 50, Height: 30}

	parentLayer := &graphics.Layer{Dirty: true, Size: graphics.Size{Width: 100, Height: 100}}

	// Use a canvas that does NOT implement DrawChildLayer (nullPaintCanvas)
	ctx := &PaintContext{
		Canvas:         &nullPaintCanvas{size: graphics.Size{Width: 100, Height: 100}},
		RecordingLayer: parentLayer,
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{X: 10, Y: 20})

	// Child SHOULD have been painted directly (fallback path)
	if child.paintCalls != 1 {
		t.Fatalf("expected fallback direct paint, got %d paint calls", child.paintCalls)
	}
}

func TestPaintChildWithLayer_DirtyBoundaryPaintedDirectly(t *testing.T) {
	// A boundary with a dirty layer and no RecordingLayer should paint directly
	child := &testRenderBox{}
	child.SetSelf(child)
	child.size = graphics.Size{Width: 50, Height: 30}

	// Create layer but mark dirty
	child.EnsureLayer().MarkDirty()

	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 100, Height: 100}),
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 1 {
		t.Fatalf("expected dirty boundary to be painted directly, got %d", child.paintCalls)
	}
}

func TestPaintChildWithLayer_NilContentFallsThroughToPaint(t *testing.T) {
	// Layer with nil Content (not yet recorded) should paint directly
	child := &testRenderBox{}
	child.SetSelf(child)
	child.size = graphics.Size{Width: 50, Height: 30}

	layer := child.EnsureLayer()
	layer.Dirty = false // clean but no content

	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 100, Height: 100}),
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 1 {
		t.Fatalf("expected paint when layer has nil content, got %d", child.paintCalls)
	}
}

func TestPaintChild_NilChildIsNoOp(t *testing.T) {
	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}
	// Should not panic
	ctx.PaintChild(nil, graphics.Offset{})
	ctx.PaintChildWithLayer(nil, graphics.Offset{})
}

// nullPaintCanvas is a canvas that does NOT implement DrawChildLayer.
// Used to test the fallback path in PaintChildWithLayer.
type nullPaintCanvas struct {
	size graphics.Size
}

func (c *nullPaintCanvas) Save()                                                                   {}
func (c *nullPaintCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64)                      {}
func (c *nullPaintCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint)                   {}
func (c *nullPaintCanvas) Restore()                                                                {}
func (c *nullPaintCanvas) Translate(dx, dy float64)                                                {}
func (c *nullPaintCanvas) Scale(sx, sy float64)                                                    {}
func (c *nullPaintCanvas) Rotate(radians float64)                                                  {}
func (c *nullPaintCanvas) ClipRect(rect graphics.Rect)                                             {}
func (c *nullPaintCanvas) ClipRRect(rrect graphics.RRect)                                          {}
func (c *nullPaintCanvas) ClipPath(path *graphics.Path, op graphics.ClipOp, aa bool)               {}
func (c *nullPaintCanvas) Clear(color graphics.Color)                                              {}
func (c *nullPaintCanvas) DrawRect(rect graphics.Rect, paint graphics.Paint)                       {}
func (c *nullPaintCanvas) DrawRRect(rrect graphics.RRect, paint graphics.Paint)                    {}
func (c *nullPaintCanvas) DrawCircle(center graphics.Offset, radius float64, paint graphics.Paint) {}
func (c *nullPaintCanvas) DrawLine(start, end graphics.Offset, paint graphics.Paint)               {}
func (c *nullPaintCanvas) DrawText(layout *graphics.TextLayout, position graphics.Offset)          {}
func (c *nullPaintCanvas) DrawImage(img image.Image, position graphics.Offset)                     {}
func (c *nullPaintCanvas) DrawImageRect(img image.Image, srcRect, dstRect graphics.Rect, quality graphics.FilterQuality, cacheKey uintptr) {
}
func (c *nullPaintCanvas) DrawPath(path *graphics.Path, paint graphics.Paint)              {}
func (c *nullPaintCanvas) DrawRectShadow(rect graphics.Rect, shadow graphics.BoxShadow)    {}
func (c *nullPaintCanvas) DrawRRectShadow(rrect graphics.RRect, shadow graphics.BoxShadow) {}
func (c *nullPaintCanvas) SaveLayerBlur(bounds graphics.Rect, sigmaX, sigmaY float64)      {}
func (c *nullPaintCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds graphics.Rect)             {}
func (c *nullPaintCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds graphics.Rect, tintColor graphics.Color) {
}
func (c *nullPaintCanvas) DrawLottie(animPtr unsafe.Pointer, bounds graphics.Rect, t float64) {}
func (c *nullPaintCanvas) EmbedPlatformView(viewID int64, size graphics.Size)                 {}
func (c *nullPaintCanvas) Size() graphics.Size                                                { return c.size }

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
