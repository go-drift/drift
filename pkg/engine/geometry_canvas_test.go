package engine

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

// occludeRect is a test helper that builds a rect path and calls OccludePlatformViews.
func occludeRect(gc *GeometryCanvas, left, top, width, height float64) {
	mask := graphics.NewPath()
	mask.AddRect(graphics.RectFromLTWH(left, top, width, height))
	gc.OccludePlatformViews(mask)
}

func TestFlushDirectWhenNoOcclusion(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	gc.Save()
	gc.Translate(10, 20)
	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 80})
	gc.Restore()

	gc.FlushToSink()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	u := sink.updates[0]
	if u.viewID != 1 {
		t.Errorf("viewID = %d, want 1", u.viewID)
	}
	if u.offset.X != 10 || u.offset.Y != 20 {
		t.Errorf("offset = (%v, %v), want (10, 20)", u.offset.X, u.offset.Y)
	}
	if u.size.Width != 100 || u.size.Height != 80 {
		t.Errorf("size = %v, want (100, 80)", u.size)
	}
	// No occlusion, so clipBounds should be nil (fast path sends raw geometry).
	if u.clipBounds != nil {
		t.Errorf("expected nil clipBounds on fast path, got %v", u.clipBounds)
	}
	// visibleRect should equal full view bounds.
	wantVisible := graphics.RectFromLTWH(10, 20, 100, 80)
	if u.visibleRect != wantVisible {
		t.Errorf("visibleRect = %v, want %v", u.visibleRect, wantVisible)
	}
	// No occlusion paths.
	if len(u.occlusionPaths) != 0 {
		t.Errorf("expected 0 occlusionPaths, got %d", len(u.occlusionPaths))
	}
}

func TestFullOcclusion(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	// Embed a platform view at (10, 10) size 100x80.
	gc.Save()
	gc.Translate(10, 10)
	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 80})
	gc.Restore()

	// Full-screen occluder covers the view entirely.
	occludeRect(gc, 0, 0, 800, 600)

	gc.FlushToSink()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	u := sink.updates[0]
	if u.clipBounds == nil {
		t.Fatal("expected non-nil clipBounds for occluded view")
	}
	if !u.clipBounds.IsEmpty() {
		t.Errorf("expected empty clip (hidden), got %v", *u.clipBounds)
	}
	// visibleRect should be the full view bounds (no parent clip).
	wantVisible := graphics.RectFromLTWH(10, 10, 100, 80)
	if u.visibleRect != wantVisible {
		t.Errorf("visibleRect = %v, want %v", u.visibleRect, wantVisible)
	}
	// occlusionPaths should have one entry covering the view.
	if len(u.occlusionPaths) != 1 {
		t.Fatalf("expected 1 occlusionPath, got %d", len(u.occlusionPaths))
	}
	occBounds := u.occlusionPaths[0].Bounds()
	// The occlusion path is the full-screen rect, but clipped to visibleRect
	// during the intersection pre-filter. The path itself is the full-screen
	// rect (bounds check passes since it intersects the view).
	if occBounds.Left != 0 || occBounds.Top != 0 || occBounds.Right != 800 || occBounds.Bottom != 600 {
		t.Errorf("occlusionPaths[0].Bounds() = %v, want full-screen", occBounds)
	}
}

func TestPartialOcclusionEdge(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	// Embed view at (0, 0) size 100x100.
	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 100})

	// Left-half occluder: covers x=[0,50], full height.
	occludeRect(gc, 0, 0, 50, 100)

	gc.FlushToSink()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	u := sink.updates[0]
	if u.clipBounds == nil {
		t.Fatal("expected non-nil clipBounds for partially occluded view")
	}
	// Remaining strip should be the right half: (50, 0, 100, 100).
	clip := *u.clipBounds
	if clip.Left != 50 || clip.Top != 0 || clip.Right != 100 || clip.Bottom != 100 {
		t.Errorf("clip = %v, want {50 0 100 100}", clip)
	}
	// visibleRect = full view bounds (no parent clip).
	wantVisible := graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 100}
	if u.visibleRect != wantVisible {
		t.Errorf("visibleRect = %v, want %v", u.visibleRect, wantVisible)
	}
	// One occlusion path.
	if len(u.occlusionPaths) != 1 {
		t.Fatalf("expected 1 occlusionPath, got %d", len(u.occlusionPaths))
	}
	wantOcc := graphics.Rect{Left: 0, Top: 0, Right: 50, Bottom: 100}
	if u.occlusionPaths[0].Bounds() != wantOcc {
		t.Errorf("occlusionPaths[0].Bounds() = %v, want %v", u.occlusionPaths[0].Bounds(), wantOcc)
	}
}

func TestNoOcclusionBeforeView(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	// Occluder first (seq=0), then view (seq=1).
	occludeRect(gc, 0, 0, 800, 600)
	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 100})

	gc.FlushToSink()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	u := sink.updates[0]
	// Occluder has seqIndex=0, view has seqIndex=1.
	// Occluder should NOT affect the view (only higher seq occludes lower seq).
	clip := *u.clipBounds
	if clip.IsEmpty() {
		t.Error("view should NOT be occluded by region with lower seqIndex")
	}
	// The view rect should match its full bounds.
	if clip.Left != 0 || clip.Top != 0 || clip.Right != 100 || clip.Bottom != 100 {
		t.Errorf("clip = %v, want {0 0 100 100}", clip)
	}
	// No occlusion paths (occluder has lower seqIndex).
	if len(u.occlusionPaths) != 0 {
		t.Errorf("expected 0 occlusionPaths, got %d", len(u.occlusionPaths))
	}
}

func TestMultipleViewsSelectiveOcclusion(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	// Embed A (seq=0).
	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 100})
	// Occlude (seq=1) - full screen.
	occludeRect(gc, 0, 0, 800, 600)
	// Embed B (seq=2) at offset (200, 200).
	gc.Save()
	gc.Translate(200, 200)
	gc.EmbedPlatformView(2, graphics.Size{Width: 50, Height: 50})
	gc.Restore()

	gc.FlushToSink()

	if len(sink.updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(sink.updates))
	}

	// View A (seq=0) should be occluded by the full-screen occluder (seq=1).
	uA := sink.updates[0]
	if uA.viewID != 1 {
		t.Errorf("first update viewID = %d, want 1", uA.viewID)
	}
	if uA.clipBounds == nil || !uA.clipBounds.IsEmpty() {
		t.Errorf("view A should be hidden, clipBounds = %v", uA.clipBounds)
	}
	// View A should have occlusion paths.
	if len(uA.occlusionPaths) != 1 {
		t.Fatalf("view A: expected 1 occlusionPath, got %d", len(uA.occlusionPaths))
	}

	// View B (seq=2) should NOT be occluded (occluder seq=1 < view seq=2).
	uB := sink.updates[1]
	if uB.viewID != 2 {
		t.Errorf("second update viewID = %d, want 2", uB.viewID)
	}
	if uB.clipBounds == nil {
		t.Fatal("expected non-nil clipBounds for view B")
	}
	clipB := *uB.clipBounds
	if clipB.IsEmpty() {
		t.Error("view B should NOT be occluded")
	}
	if len(uB.occlusionPaths) != 0 {
		t.Errorf("view B: expected 0 occlusionPaths, got %d", len(uB.occlusionPaths))
	}
}

func TestCenterHoleHidesView(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	// View at (0, 0) size 100x100.
	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 100})
	// Center hole occluder: inside the view, not touching any edge.
	occludeRect(gc, 20, 20, 60, 60)

	gc.FlushToSink()

	if len(sink.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(sink.updates))
	}
	u := sink.updates[0]
	// clipBounds = empty (Android hides).
	if u.clipBounds == nil || !u.clipBounds.IsEmpty() {
		t.Errorf("center hole should hide view (safety-first), clipBounds = %v", u.clipBounds)
	}
	// visibleRect = full view rect (iOS masks precisely).
	wantVisible := graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 100}
	if u.visibleRect != wantVisible {
		t.Errorf("visibleRect = %v, want %v", u.visibleRect, wantVisible)
	}
	// occlusionPaths has the center rect path.
	if len(u.occlusionPaths) != 1 {
		t.Fatalf("expected 1 occlusionPath, got %d", len(u.occlusionPaths))
	}
	wantOcc := graphics.Rect{Left: 20, Top: 20, Right: 80, Bottom: 80}
	if u.occlusionPaths[0].Bounds() != wantOcc {
		t.Errorf("occlusionPaths[0].Bounds() = %v, want %v", u.occlusionPaths[0].Bounds(), wantOcc)
	}
}

func TestResetFrame(t *testing.T) {
	sink := &mockSink{}
	gc := NewGeometryCanvas(graphics.Size{Width: 800, Height: 600}, sink)

	gc.EmbedPlatformView(1, graphics.Size{Width: 100, Height: 100})
	occludeRect(gc, 0, 0, 800, 600)

	gc.ResetFrame()

	if len(gc.views) != 0 {
		t.Errorf("expected 0 views after reset, got %d", len(gc.views))
	}
	if len(gc.occlusions) != 0 {
		t.Errorf("expected 0 occlusions after reset, got %d", len(gc.occlusions))
	}
	if gc.seqCounter != 0 {
		t.Errorf("expected seqCounter=0 after reset, got %d", gc.seqCounter)
	}

	// Flush after reset should produce no updates.
	gc.FlushToSink()
	if len(sink.updates) != 0 {
		t.Errorf("expected 0 updates after reset+flush, got %d", len(sink.updates))
	}
}

// --- capOcclusionPaths tests ---

func TestMergeOverlappingPaths_NoOverlap(t *testing.T) {
	// Two non-overlapping button paths should be preserved as-is.
	p1 := graphics.NewPath()
	p1.AddRect(graphics.RectFromLTWH(0, 0, 50, 30))
	p2 := graphics.NewPath()
	p2.AddRect(graphics.RectFromLTWH(200, 0, 50, 30))

	result := mergeOverlappingPaths([]*graphics.Path{p1, p2})
	if len(result) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(result))
	}
}

func TestMergeOverlappingPaths_FullOverlap(t *testing.T) {
	// Two full-screen paths (barrier + dialog) should merge into one.
	p1 := graphics.NewPath()
	p1.AddRect(graphics.RectFromLTWH(0, 0, 800, 600))
	p2 := graphics.NewPath()
	p2.AddRect(graphics.RectFromLTWH(0, 0, 800, 600))

	result := mergeOverlappingPaths([]*graphics.Path{p1, p2})
	if len(result) != 1 {
		t.Fatalf("expected 1 merged path, got %d", len(result))
	}
	want := graphics.Rect{Left: 0, Top: 0, Right: 800, Bottom: 600}
	if result[0].Bounds() != want {
		t.Errorf("merged bounds = %v, want %v", result[0].Bounds(), want)
	}
}

func TestMergeOverlappingPaths_ChainMerge(t *testing.T) {
	// Three overlapping paths: full-screen + two buttons inside it.
	// All should merge into one since buttons overlap with full-screen.
	fullScreen := graphics.NewPath()
	fullScreen.AddRect(graphics.RectFromLTWH(0, 0, 800, 600))
	button1 := graphics.NewPath()
	button1.AddRect(graphics.RectFromLTWH(10, 10, 60, 30))
	button2 := graphics.NewPath()
	button2.AddRect(graphics.RectFromLTWH(700, 10, 60, 30))

	result := mergeOverlappingPaths([]*graphics.Path{button1, fullScreen, button2})
	if len(result) != 1 {
		t.Fatalf("expected 1 merged path, got %d", len(result))
	}
}

func TestMergeOverlappingPaths_Empty(t *testing.T) {
	result := mergeOverlappingPaths(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
	result = mergeOverlappingPaths([]*graphics.Path{})
	if len(result) != 0 {
		t.Errorf("expected empty for empty input, got %d", len(result))
	}
}

func TestCapOcclusionPaths_Empty(t *testing.T) {
	result := capOcclusionPaths(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestCapOcclusionPaths_UnderCap(t *testing.T) {
	paths := make([]*graphics.Path, 3)
	for i := range paths {
		p := graphics.NewPath()
		p.AddRect(graphics.RectFromLTWH(float64(i*20), 0, 10, 10))
		paths[i] = p
	}
	result := capOcclusionPaths(paths)
	if len(result) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(result))
	}
}

func TestCapOcclusionPaths_OverCap(t *testing.T) {
	// Create 10 non-overlapping paths.
	var paths []*graphics.Path
	for i := 0; i < 10; i++ {
		p := graphics.NewPath()
		x := float64(i * 20)
		p.AddRect(graphics.RectFromLTWH(x, 0, 10, 10))
		paths = append(paths, p)
	}
	result := capOcclusionPaths(paths)
	if len(result) != 1 {
		t.Fatalf("expected 1 collapsed path (>8 cap), got %d", len(result))
	}
	// Should be bounding rect of all inputs.
	want := graphics.Rect{Left: 0, Top: 0, Right: 190, Bottom: 10}
	if result[0].Bounds() != want {
		t.Errorf("collapsed path bounds = %v, want %v", result[0].Bounds(), want)
	}
}
