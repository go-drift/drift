package platform

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

// --- Shared test helpers (used by other *_test.go files in this package) ---

// testBridge captures native method invocations for assertions.
type testBridge struct {
	mu    sync.Mutex
	calls []testBridgeCall
}

type testBridgeCall struct {
	channel string
	method  string
	args    any // JSON-decoded
}

func (b *testBridge) InvokeMethod(channel, method string, argsData []byte) ([]byte, error) {
	var args any
	if len(argsData) > 0 {
		json.Unmarshal(argsData, &args)
	}
	b.mu.Lock()
	b.calls = append(b.calls, testBridgeCall{channel: channel, method: method, args: args})
	b.mu.Unlock()
	return DefaultCodec.Encode(nil)
}

func (b *testBridge) StartEventStream(string) error { return nil }
func (b *testBridge) StopEventStream(string) error  { return nil }

func (b *testBridge) reset() {
	b.mu.Lock()
	b.calls = b.calls[:0]
	b.mu.Unlock()
}

func setupTestBridge(t *testing.T) *testBridge {
	bridge := &testBridge{}
	SetupTestBridge(t.Cleanup)
	SetNativeBridge(bridge)
	return bridge
}

// --- Geometry batch test helpers ---

// stubView is a minimal PlatformView for testing geometry batching.
type stubView struct{ id int64 }

func (v *stubView) ViewID() int64                      { return v.id }
func (v *stubView) ViewType() string                   { return "test" }
func (v *stubView) Create(params map[string]any) error { return nil }
func (v *stubView) Dispose()                           {}

func newTestRegistry(viewIDs ...int64) *PlatformViewRegistry {
	r := &PlatformViewRegistry{
		factories:          make(map[string]PlatformViewFactory),
		views:              make(map[int64]PlatformView),
		channel:            NewMethodChannel("test/platform_views"),
		geometryCache:      make(map[int64]CapturedViewGeometry),
		viewsSeenThisFrame: make(map[int64]struct{}),
	}
	for _, id := range viewIDs {
		r.views[id] = &stubView{id: id}
	}
	return r
}

// capturedForView finds the captured geometry for a given viewID, or nil.
func capturedForView(captured []CapturedViewGeometry, viewID int64) *CapturedViewGeometry {
	for i := range captured {
		if captured[i].ViewID == viewID {
			return &captured[i]
		}
	}
	return nil
}

// isEmptyClip checks if a captured geometry has zero clip bounds (hidden).
func isEmptyClip(cv *CapturedViewGeometry) bool {
	if cv.ClipBounds == nil {
		return false
	}
	return cv.ClipBounds.Left == 0 && cv.ClipBounds.Top == 0 &&
		cv.ClipBounds.Right == 0 && cv.ClipBounds.Bottom == 0
}

// --- Tests ---

func TestFlushGeometryBatch_HidesUnseenViews(t *testing.T) {
	reg := newTestRegistry(1, 2)

	reg.BeginGeometryBatch()
	// Only update view 1; view 2 is culled (off-screen).
	reg.UpdateViewGeometry(1,
		graphics.Offset{X: 10, Y: 20},
		graphics.Size{Width: 100, Height: 50},
		&graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
	)
	reg.FlushGeometryBatch()
	captured := reg.TakeCapturedSnapshot()

	if len(captured) != 2 {
		t.Fatalf("expected 2 geometry entries (1 visible + 1 hide), got %d", len(captured))
	}

	g1 := capturedForView(captured, 1)
	g2 := capturedForView(captured, 2)
	if g1 == nil {
		t.Fatal("missing geometry for view 1")
	}
	if g2 == nil {
		t.Fatal("missing geometry for view 2 (hide entry)")
	}

	if isEmptyClip(g1) {
		t.Error("view 1 should not have empty clip (it was visible)")
	}
	if !isEmptyClip(g2) {
		t.Error("view 2 should have empty clip (it was culled)")
	}
}

func TestFlushGeometryBatch_AllViewsSeen(t *testing.T) {
	reg := newTestRegistry(1, 2)

	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, graphics.Offset{X: 10, Y: 20}, graphics.Size{Width: 100, Height: 50}, nil)
	reg.UpdateViewGeometry(2, graphics.Offset{X: 10, Y: 80}, graphics.Size{Width: 100, Height: 50}, nil)
	reg.FlushGeometryBatch()
	captured := reg.TakeCapturedSnapshot()

	if len(captured) != 2 {
		t.Fatalf("expected 2 geometry entries, got %d", len(captured))
	}

	for _, cv := range captured {
		if isEmptyClip(&cv) {
			t.Errorf("view %d should not have empty clip (both were visible)", cv.ViewID)
		}
	}
}

func TestFlushGeometryBatch_HiddenViewRestoresOnNextFrame(t *testing.T) {
	reg := newTestRegistry(1)

	// Frame 1: view unseen, hidden.
	reg.BeginGeometryBatch()
	reg.FlushGeometryBatch()
	captured := reg.TakeCapturedSnapshot()

	if len(captured) != 1 {
		t.Fatalf("frame 1: expected 1 geometry (hide), got %d", len(captured))
	}
	if !isEmptyClip(&captured[0]) {
		t.Error("frame 1: view should be hidden with empty clip")
	}

	// Frame 2: view scrolls back into view.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1,
		graphics.Offset{X: 10, Y: 20},
		graphics.Size{Width: 100, Height: 50},
		&graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
	)
	reg.FlushGeometryBatch()
	captured = reg.TakeCapturedSnapshot()

	if len(captured) != 1 {
		t.Fatalf("frame 2: expected 1 geometry (restore), got %d", len(captured))
	}
	if isEmptyClip(&captured[0]) {
		t.Error("frame 2: view should be visible, not hidden")
	}
}

func TestFlushGeometryBatch_NoViewsNoCrash(t *testing.T) {
	reg := newTestRegistry() // no views

	reg.BeginGeometryBatch()
	reg.FlushGeometryBatch()
	captured := reg.TakeCapturedSnapshot()

	if len(captured) != 0 {
		t.Fatalf("expected no captured geometry with no views, got %d", len(captured))
	}
}

func TestFlushGeometryBatch_ViewSeenNotHidden(t *testing.T) {
	reg := newTestRegistry(1)

	pos := graphics.Offset{X: 10, Y: 20}
	size := graphics.Size{Width: 100, Height: 50}

	// Frame 1: initial geometry.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, nil)
	reg.FlushGeometryBatch()
	reg.TakeCapturedSnapshot()

	// Frame 2: same geometry, view still seen.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, nil)
	reg.FlushGeometryBatch()
	captured := reg.TakeCapturedSnapshot()

	// View was seen, so it should appear in the snapshot (not hidden).
	if len(captured) != 1 {
		t.Fatalf("expected 1 captured geometry entry, got %d", len(captured))
	}
	if isEmptyClip(&captured[0]) {
		t.Error("view should not have empty clip (it was seen)")
	}
}

func TestFlushGeometryBatch_ViewScrollsOutAndBack(t *testing.T) {
	reg := newTestRegistry(1)

	pos := graphics.Offset{X: 10, Y: 200}
	size := graphics.Size{Width: 200, Height: 40}
	clip := &graphics.Rect{Left: 0, Top: 0, Right: 200, Bottom: 600}

	// Frame 1: view visible.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, clip)
	reg.FlushGeometryBatch()
	reg.TakeCapturedSnapshot()

	// Frame 2: view scrolled off-screen (no UpdateViewGeometry call).
	reg.BeginGeometryBatch()
	reg.FlushGeometryBatch()
	captured := reg.TakeCapturedSnapshot()

	if len(captured) != 1 {
		t.Fatalf("frame 2: expected 1 geometry (hide), got %d", len(captured))
	}
	if !isEmptyClip(&captured[0]) {
		t.Error("frame 2: off-screen view should be hidden")
	}

	// Frame 3: view scrolls back with SAME position as frame 1.
	// The hide in frame 2 updated the geometry cache, so this should
	// appear as a real update (cache holds hidden state, real geometry differs).
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, clip)
	reg.FlushGeometryBatch()
	captured = reg.TakeCapturedSnapshot()

	if len(captured) != 1 {
		t.Fatalf("frame 3: expected 1 geometry (restore), got %d", len(captured))
	}
	if isEmptyClip(&captured[0]) {
		t.Error("frame 3: restored view should not have empty clip")
	}
}
