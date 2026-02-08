package platform

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

// --- Test helpers ---

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

// batchCalls returns only the batchSetGeometry calls.
func (b *testBridge) batchCalls() []testBridgeCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []testBridgeCall
	for _, c := range b.calls {
		if c.method == "batchSetGeometry" {
			result = append(result, c)
		}
	}
	return result
}

func (b *testBridge) reset() {
	b.mu.Lock()
	b.calls = b.calls[:0]
	b.mu.Unlock()
}

// stubView is a minimal PlatformView for testing geometry batching.
type stubView struct{ id int64 }

func (v *stubView) ViewID() int64                      { return v.id }
func (v *stubView) ViewType() string                   { return "test" }
func (v *stubView) Create(params map[string]any) error { return nil }
func (v *stubView) Dispose()                           {}
func (v *stubView) SetSize(graphics.Size)              {}
func (v *stubView) SetOffset(graphics.Offset)          {}
func (v *stubView) SetVisible(bool)                    {}

func setupTestBridge(t *testing.T) *testBridge {
	bridge := &testBridge{}
	SetupTestBridge(t.Cleanup)
	SetNativeBridge(bridge)
	return bridge
}

func newTestRegistry(viewIDs ...int64) *PlatformViewRegistry {
	r := &PlatformViewRegistry{
		factories:          make(map[string]PlatformViewFactory),
		views:              make(map[int64]PlatformView),
		channel:            NewMethodChannel("test/platform_views"),
		geometryCache:      make(map[int64]viewGeometryCache),
		viewsSeenThisFrame: make(map[int64]struct{}),
	}
	for _, id := range viewIDs {
		r.views[id] = &stubView{id: id}
	}
	return r
}

// extractGeometries returns the geometry entries from a batchSetGeometry call.
func extractGeometries(call testBridgeCall) []map[string]any {
	argsMap := call.args.(map[string]any)
	geos := argsMap["geometries"].([]any)
	result := make([]map[string]any, len(geos))
	for i, g := range geos {
		result[i] = g.(map[string]any)
	}
	return result
}

// viewIDFromGeo extracts the viewId from a geometry entry (JSON float64).
func viewIDFromGeo(geo map[string]any) int64 {
	return int64(geo["viewId"].(float64))
}

// hasEmptyClip checks if a geometry entry has zero clip bounds (hidden).
func hasEmptyClip(geo map[string]any) bool {
	cl, okL := geo["clipLeft"]
	ct, okT := geo["clipTop"]
	cr, okR := geo["clipRight"]
	cb, okB := geo["clipBottom"]
	if !okL || !okT || !okR || !okB {
		return false
	}
	return cl.(float64) == 0 && ct.(float64) == 0 &&
		cr.(float64) == 0 && cb.(float64) == 0
}

// geoForView finds the geometry entry for a given viewID, or nil.
func geoForView(geos []map[string]any, viewID int64) map[string]any {
	for _, g := range geos {
		if viewIDFromGeo(g) == viewID {
			return g
		}
	}
	return nil
}

// --- Tests ---

func TestFlushGeometryBatch_HidesUnseenViews(t *testing.T) {
	bridge := setupTestBridge(t)
	reg := newTestRegistry(1, 2)

	reg.BeginGeometryBatch()
	// Only update view 1 — view 2 is culled (off-screen).
	reg.UpdateViewGeometry(1,
		graphics.Offset{X: 10, Y: 20},
		graphics.Size{Width: 100, Height: 50},
		&graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
	)
	reg.FlushGeometryBatch()

	calls := bridge.batchCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 batch call, got %d", len(calls))
	}

	geos := extractGeometries(calls[0])
	if len(geos) != 2 {
		t.Fatalf("expected 2 geometry entries (1 visible + 1 hide), got %d", len(geos))
	}

	g1 := geoForView(geos, 1)
	g2 := geoForView(geos, 2)
	if g1 == nil {
		t.Fatal("missing geometry for view 1")
	}
	if g2 == nil {
		t.Fatal("missing geometry for view 2 (hide entry)")
	}

	if hasEmptyClip(g1) {
		t.Error("view 1 should not have empty clip (it was visible)")
	}
	if !hasEmptyClip(g2) {
		t.Error("view 2 should have empty clip (it was culled)")
	}
}

func TestFlushGeometryBatch_AllViewsSeen(t *testing.T) {
	bridge := setupTestBridge(t)
	reg := newTestRegistry(1, 2)

	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, graphics.Offset{X: 10, Y: 20}, graphics.Size{Width: 100, Height: 50}, nil)
	reg.UpdateViewGeometry(2, graphics.Offset{X: 10, Y: 80}, graphics.Size{Width: 100, Height: 50}, nil)
	reg.FlushGeometryBatch()

	calls := bridge.batchCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 batch call, got %d", len(calls))
	}

	geos := extractGeometries(calls[0])
	if len(geos) != 2 {
		t.Fatalf("expected 2 geometry entries, got %d", len(geos))
	}

	for _, g := range geos {
		if hasEmptyClip(g) {
			t.Errorf("view %d should not have empty clip (both were visible)", viewIDFromGeo(g))
		}
	}
}

func TestFlushGeometryBatch_HiddenViewRestoresOnNextFrame(t *testing.T) {
	bridge := setupTestBridge(t)
	reg := newTestRegistry(1)

	// Frame 1: view unseen → hidden.
	reg.BeginGeometryBatch()
	reg.FlushGeometryBatch()

	calls := bridge.batchCalls()
	if len(calls) != 1 {
		t.Fatalf("frame 1: expected 1 batch call, got %d", len(calls))
	}
	geos := extractGeometries(calls[0])
	if len(geos) != 1 {
		t.Fatalf("frame 1: expected 1 geometry (hide), got %d", len(geos))
	}
	if !hasEmptyClip(geos[0]) {
		t.Error("frame 1: view should be hidden with empty clip")
	}

	bridge.reset()

	// Frame 2: view scrolls back into view.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1,
		graphics.Offset{X: 10, Y: 20},
		graphics.Size{Width: 100, Height: 50},
		&graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
	)
	reg.FlushGeometryBatch()

	calls = bridge.batchCalls()
	if len(calls) != 1 {
		t.Fatalf("frame 2: expected 1 batch call (restore), got %d", len(calls))
	}
	geos = extractGeometries(calls[0])
	if len(geos) != 1 {
		t.Fatalf("frame 2: expected 1 geometry (restore), got %d", len(geos))
	}
	if hasEmptyClip(geos[0]) {
		t.Error("frame 2: view should be visible, not hidden")
	}
}

func TestFlushGeometryBatch_NoViewsNoCrash(t *testing.T) {
	bridge := setupTestBridge(t)
	reg := newTestRegistry() // no views

	reg.BeginGeometryBatch()
	reg.FlushGeometryBatch()

	calls := bridge.batchCalls()
	if len(calls) != 0 {
		t.Fatalf("expected no batch calls with no views, got %d", len(calls))
	}
}

func TestFlushGeometryBatch_DeduplicatedViewNotHidden(t *testing.T) {
	bridge := setupTestBridge(t)
	reg := newTestRegistry(1)

	pos := graphics.Offset{X: 10, Y: 20}
	size := graphics.Size{Width: 100, Height: 50}

	// Frame 1: initial geometry.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, nil)
	reg.FlushGeometryBatch()

	bridge.reset()

	// Frame 2: same geometry → deduped, but view is still seen.
	// Should produce no batch call (nothing changed, nothing to hide).
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, nil)
	reg.FlushGeometryBatch()

	calls := bridge.batchCalls()
	if len(calls) != 0 {
		t.Fatalf("expected no batch calls (deduped + still visible), got %d", len(calls))
	}
}

func TestFlushGeometryBatch_ViewScrollsOutAndBack(t *testing.T) {
	bridge := setupTestBridge(t)
	reg := newTestRegistry(1)

	pos := graphics.Offset{X: 10, Y: 200}
	size := graphics.Size{Width: 200, Height: 40}
	clip := &graphics.Rect{Left: 0, Top: 0, Right: 200, Bottom: 600}

	// Frame 1: view visible.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, clip)
	reg.FlushGeometryBatch()
	bridge.reset()

	// Frame 2: view scrolled off-screen (no UpdateViewGeometry call).
	reg.BeginGeometryBatch()
	reg.FlushGeometryBatch()

	calls := bridge.batchCalls()
	if len(calls) != 1 {
		t.Fatalf("frame 2: expected 1 batch call (hide), got %d", len(calls))
	}
	geos := extractGeometries(calls[0])
	if !hasEmptyClip(geos[0]) {
		t.Error("frame 2: off-screen view should be hidden")
	}
	bridge.reset()

	// Frame 3: view scrolls back with SAME position as frame 1.
	// The hide in frame 2 updated the geometry cache, so this must NOT be
	// deduped — the cache holds the hidden state, real geometry differs.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, clip)
	reg.FlushGeometryBatch()

	calls = bridge.batchCalls()
	if len(calls) != 1 {
		t.Fatalf("frame 3: expected 1 batch call (restore), got %d", len(calls))
	}
	geos = extractGeometries(calls[0])
	if hasEmptyClip(geos[0]) {
		t.Error("frame 3: restored view should not have empty clip")
	}
}
