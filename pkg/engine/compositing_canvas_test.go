package engine

import (
	"image"
	"testing"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
)

// mockSink records platform view geometry updates for testing.
type mockSink struct {
	updates []sinkUpdate
}

type sinkUpdate struct {
	viewID     int64
	offset     graphics.Offset
	size       graphics.Size
	clipBounds *graphics.Rect
}

func (s *mockSink) UpdateViewGeometry(viewID int64, offset graphics.Offset, size graphics.Size, clipBounds *graphics.Rect) error {
	s.updates = append(s.updates, sinkUpdate{
		viewID:     viewID,
		offset:     offset,
		size:       size,
		clipBounds: clipBounds,
	})
	return nil
}

// nullCanvas is a minimal Canvas implementation for testing.
type nullCanvas struct {
	size graphics.Size
}

func (c *nullCanvas) Save()                                                                   {}
func (c *nullCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64)                      {}
func (c *nullCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint)                   {}
func (c *nullCanvas) Restore()                                                                {}
func (c *nullCanvas) Translate(dx, dy float64)                                                {}
func (c *nullCanvas) Scale(sx, sy float64)                                                    {}
func (c *nullCanvas) Rotate(radians float64)                                                  {}
func (c *nullCanvas) ClipRect(rect graphics.Rect)                                             {}
func (c *nullCanvas) ClipRRect(rrect graphics.RRect)                                          {}
func (c *nullCanvas) ClipPath(path *graphics.Path, op graphics.ClipOp, aa bool)               {}
func (c *nullCanvas) Clear(color graphics.Color)                                              {}
func (c *nullCanvas) DrawRect(rect graphics.Rect, paint graphics.Paint)                       {}
func (c *nullCanvas) DrawRRect(rrect graphics.RRect, paint graphics.Paint)                    {}
func (c *nullCanvas) DrawCircle(center graphics.Offset, radius float64, paint graphics.Paint) {}
func (c *nullCanvas) DrawLine(start, end graphics.Offset, paint graphics.Paint)               {}
func (c *nullCanvas) DrawText(layout *graphics.TextLayout, position graphics.Offset)          {}
func (c *nullCanvas) DrawImage(img image.Image, position graphics.Offset)                     {}
func (c *nullCanvas) DrawImageRect(img image.Image, srcRect, dstRect graphics.Rect, quality graphics.FilterQuality, cacheKey uintptr) {
}
func (c *nullCanvas) DrawPath(path *graphics.Path, paint graphics.Paint)              {}
func (c *nullCanvas) DrawRectShadow(rect graphics.Rect, shadow graphics.BoxShadow)    {}
func (c *nullCanvas) DrawRRectShadow(rrect graphics.RRect, shadow graphics.BoxShadow) {}
func (c *nullCanvas) SaveLayerBlur(bounds graphics.Rect, sigmaX, sigmaY float64)      {}
func (c *nullCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds graphics.Rect)             {}
func (c *nullCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds graphics.Rect, tintColor graphics.Color) {
}
func (c *nullCanvas) EmbedPlatformView(viewID int64, size graphics.Size) {}
func (c *nullCanvas) Size() graphics.Size                                { return c.size }

func TestCompositingCanvas_PlatformViewWithTranslation(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Save()
	cc.Translate(50, 100)
	cc.EmbedPlatformView(1, graphics.Size{Width: 200, Height: 150})
	cc.Restore()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	if u.viewID != 1 {
		t.Errorf("viewID = %d, want 1", u.viewID)
	}
	if u.offset.X != 50 || u.offset.Y != 100 {
		t.Errorf("offset = (%v, %v), want (50, 100)", u.offset.X, u.offset.Y)
	}
	if u.size.Width != 200 || u.size.Height != 150 {
		t.Errorf("size = (%v, %v), want (200, 150)", u.size.Width, u.size.Height)
	}
	if u.clipBounds != nil {
		t.Errorf("clipBounds = %v, want nil", u.clipBounds)
	}
}

func TestCompositingCanvas_PlatformViewWithClipBounds(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Save()
	cc.Translate(10, 20)
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 100, 80))
	cc.EmbedPlatformView(2, graphics.Size{Width: 50, Height: 40})
	cc.Restore()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	if u.clipBounds == nil {
		t.Fatal("expected non-nil clipBounds")
	}
	// ClipRect is transformed to global: (10, 20, 110, 100)
	if u.clipBounds.Left != 10 || u.clipBounds.Top != 20 {
		t.Errorf("clip left/top = (%v, %v), want (10, 20)", u.clipBounds.Left, u.clipBounds.Top)
	}
	if u.clipBounds.Right != 110 || u.clipBounds.Bottom != 100 {
		t.Errorf("clip right/bottom = (%v, %v), want (110, 100)", u.clipBounds.Right, u.clipBounds.Bottom)
	}
}

func TestCompositingCanvas_NestedDisplayLists(t *testing.T) {
	// Build a child layer with an EmbedPlatformView op inside
	childRec := &graphics.PictureRecorder{}
	childCanvas := childRec.BeginRecording(graphics.Size{Width: 200, Height: 100})
	childCanvas.Translate(5, 5)
	childCanvas.EmbedPlatformView(42, graphics.Size{Width: 50, Height: 30})
	childDL := childRec.EndRecording()

	childLayer := &graphics.Layer{Size: graphics.Size{Width: 200, Height: 100}}
	childLayer.SetContent(childDL)

	// Build a parent layer with DrawChildLayer op
	parentRec := &graphics.PictureRecorder{}
	parentCanvas := parentRec.BeginRecording(graphics.Size{Width: 800, Height: 600})
	parentCanvas.Save()
	parentCanvas.Translate(100, 50)
	parentRec.DrawChildLayer(childLayer)
	parentCanvas.Restore()
	parentDL := parentRec.EndRecording()

	// Composite onto CompositingCanvas
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	parentDL.Paint(cc)

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	if u.viewID != 42 {
		t.Errorf("viewID = %d, want 42", u.viewID)
	}
	// Parent translates by (100, 50), child translates by (5, 5)
	// Global offset = (105, 55)
	if u.offset.X != 105 || u.offset.Y != 55 {
		t.Errorf("offset = (%v, %v), want (105, 55)", u.offset.X, u.offset.Y)
	}
}

func TestCompositingCanvas_ScrolledContent(t *testing.T) {
	// Simulate a scroll viewport: clip + scroll offset
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 400, Height: 300}}
	cc := NewCompositingCanvas(inner, sink)

	// Viewport at (0, 0) with size 400x300
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 400, 300))

	// Scroll offset: content scrolled up by 50px
	cc.Save()
	cc.Translate(0, -50)
	cc.EmbedPlatformView(10, graphics.Size{Width: 200, Height: 100})
	cc.Restore()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	if u.offset.Y != -50 {
		t.Errorf("offset.Y = %v, want -50", u.offset.Y)
	}
	if u.clipBounds == nil {
		t.Fatal("expected non-nil clipBounds for scrolled content")
	}
	if u.clipBounds.Top != 0 || u.clipBounds.Bottom != 300 {
		t.Errorf("clip top/bottom = (%v, %v), want (0, 300)", u.clipBounds.Top, u.clipBounds.Bottom)
	}
}

func TestCompositingCanvas_FullyClippedView(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	// Two non-overlapping clips → intersection is empty
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 100, 100))
	cc.ClipRect(graphics.RectFromLTWH(200, 200, 100, 100))

	cc.EmbedPlatformView(99, graphics.Size{Width: 50, Height: 50})

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	if u.clipBounds == nil {
		t.Fatal("expected non-nil clipBounds for fully clipped view")
	}
	// The clip intersection should be empty
	if !u.clipBounds.IsEmpty() {
		t.Errorf("expected empty clip bounds, got %+v", u.clipBounds)
	}
}

func TestCompositingCanvas_SaveRestoreState(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	// Apply transform and clip
	cc.Save()
	cc.Translate(100, 200)
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 50, 50))

	// Restore should undo both
	cc.Restore()

	cc.EmbedPlatformView(5, graphics.Size{Width: 20, Height: 20})

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	u := sink.updates[0]
	// Transform should be restored to (0, 0)
	if u.offset.X != 0 || u.offset.Y != 0 {
		t.Errorf("offset = (%v, %v), want (0, 0) after restore", u.offset.X, u.offset.Y)
	}
	// Clip should be restored (no clips active)
	if u.clipBounds != nil {
		t.Errorf("clipBounds = %v, want nil after restore", u.clipBounds)
	}
}

func TestCompositingCanvas_NoClipNilClipBounds(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	// No clip applied
	cc.EmbedPlatformView(7, graphics.Size{Width: 100, Height: 50})

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}

	if sink.updates[0].clipBounds != nil {
		t.Errorf("expected nil clipBounds when no clip is active, got %+v", sink.updates[0].clipBounds)
	}
}

func TestCompositingCanvas_NilSinkSafe(t *testing.T) {
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, nil)

	// Should not panic with nil sink
	cc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 50})
}

func TestCompositingCanvas_NestedSaveRestore(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Save()
	cc.Translate(10, 20)
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 500, 500))

	cc.Save()
	cc.Translate(30, 40)
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 100, 100))

	// Inner state: transform (40, 60), clip intersected
	cc.EmbedPlatformView(1, graphics.Size{Width: 50, Height: 50})

	cc.Restore() // Back to first save

	cc.EmbedPlatformView(2, graphics.Size{Width: 50, Height: 50})

	cc.Restore() // Back to initial

	cc.EmbedPlatformView(3, graphics.Size{Width: 50, Height: 50})

	if len(sink.updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(sink.updates))
	}

	// View 1: transform (40, 60)
	if sink.updates[0].offset.X != 40 || sink.updates[0].offset.Y != 60 {
		t.Errorf("view 1 offset = (%v, %v), want (40, 60)", sink.updates[0].offset.X, sink.updates[0].offset.Y)
	}
	if sink.updates[0].clipBounds == nil {
		t.Error("view 1 should have clip bounds")
	}

	// View 2: transform (10, 20), only outer clip
	if sink.updates[1].offset.X != 10 || sink.updates[1].offset.Y != 20 {
		t.Errorf("view 2 offset = (%v, %v), want (10, 20)", sink.updates[1].offset.X, sink.updates[1].offset.Y)
	}
	if sink.updates[1].clipBounds == nil {
		t.Error("view 2 should have clip bounds")
	}

	// View 3: transform (0, 0), no clip
	if sink.updates[2].offset.X != 0 || sink.updates[2].offset.Y != 0 {
		t.Errorf("view 3 offset = (%v, %v), want (0, 0)", sink.updates[2].offset.X, sink.updates[2].offset.Y)
	}
	if sink.updates[2].clipBounds != nil {
		t.Errorf("view 3 should have nil clip bounds, got %+v", sink.updates[2].clipBounds)
	}
}

func TestCompositingCanvas_SaveLayerAlphaPreservesState(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Translate(10, 20)
	cc.ClipRect(graphics.RectFromLTWH(0, 0, 100, 100))

	cc.SaveLayerAlpha(graphics.RectFromLTWH(0, 0, 100, 100), 0.5)
	cc.Translate(30, 40)
	cc.Restore() // Should restore transform and clip depth

	cc.EmbedPlatformView(1, graphics.Size{Width: 50, Height: 50})

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	// Transform should be restored to (10, 20) not (40, 60)
	if sink.updates[0].offset.X != 10 || sink.updates[0].offset.Y != 20 {
		t.Errorf("offset = (%v, %v), want (10, 20) after SaveLayerAlpha/Restore",
			sink.updates[0].offset.X, sink.updates[0].offset.Y)
	}
}

func TestCompositingCanvas_SaveLayerPreservesState(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Translate(5, 10)
	cc.SaveLayer(graphics.RectFromLTWH(0, 0, 100, 100), nil)
	cc.Translate(15, 25)
	cc.Restore()

	cc.EmbedPlatformView(1, graphics.Size{Width: 50, Height: 50})

	if sink.updates[0].offset.X != 5 || sink.updates[0].offset.Y != 10 {
		t.Errorf("offset = (%v, %v), want (5, 10) after SaveLayer/Restore",
			sink.updates[0].offset.X, sink.updates[0].offset.Y)
	}
}

func TestCompositingCanvas_ScaleRotateForwardedButNotTracked(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Translate(10, 20)
	cc.Scale(2, 2)
	cc.Rotate(1.5)
	cc.EmbedPlatformView(1, graphics.Size{Width: 50, Height: 50})

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	// Scale/Rotate should NOT affect platform view transform tracking
	if sink.updates[0].offset.X != 10 || sink.updates[0].offset.Y != 20 {
		t.Errorf("offset = (%v, %v), want (10, 20) — Scale/Rotate should not affect tracking",
			sink.updates[0].offset.X, sink.updates[0].offset.Y)
	}
}

func TestCompositingCanvas_ClipPathDoesNotAffectTracking(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	// ClipPath is forwarded but not tracked (too complex for rect approximation)
	cc.ClipPath(nil, graphics.ClipOpIntersect, false)
	cc.EmbedPlatformView(1, graphics.Size{Width: 50, Height: 50})

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	// ClipPath should NOT add to clips stack
	if sink.updates[0].clipBounds != nil {
		t.Error("ClipPath should not affect platform view clip tracking")
	}
}

func TestCompositingCanvas_ClipRRectTracked(t *testing.T) {
	sink := &mockSink{}
	inner := &nullCanvas{size: graphics.Size{Width: 800, Height: 600}}
	cc := NewCompositingCanvas(inner, sink)

	cc.Save()
	cc.Translate(10, 10)
	cc.ClipRRect(graphics.RRect{
		Rect: graphics.RectFromLTWH(0, 0, 200, 200),
	})
	cc.EmbedPlatformView(1, graphics.Size{Width: 50, Height: 50})
	cc.Restore()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	if sink.updates[0].clipBounds == nil {
		t.Fatal("expected clip bounds from ClipRRect")
	}
	// Global rect: (10, 10, 210, 210)
	if sink.updates[0].clipBounds.Left != 10 || sink.updates[0].clipBounds.Top != 10 {
		t.Errorf("clip = %+v, want left=10, top=10", sink.updates[0].clipBounds)
	}
}
