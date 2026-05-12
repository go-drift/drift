package platform

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	drifterrors "github.com/go-drift/drift/pkg/errors"
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

func (b *testBridge) InvokeMethod(_ context.Context, channel, method string, argsData []byte) ([]byte, error) {
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

// --- Dispatch + errors.Report capture helpers ---

// installImmediateDispatch swaps dispatchFunc for one that runs callbacks
// inline, so tests can observe view-callback side effects synchronously.
// Snapshots+restores under dispatchMu instead of RegisterDispatch(nil) to
// avoid leaking state between tests.
func installImmediateDispatch(t *testing.T) func() {
	t.Helper()
	dispatchMu.Lock()
	prev := dispatchFunc
	dispatchFunc = func(cb func()) { cb() }
	dispatchMu.Unlock()
	return func() {
		dispatchMu.Lock()
		dispatchFunc = prev
		dispatchMu.Unlock()
	}
}

// captureErrorReports installs a fresh capturingHandler (defined in
// permissions_test.go) and restores the previous handler on cleanup. Returns
// the handler so the caller can read accumulated errors via snapshot().
func captureErrorReports(t *testing.T) *capturingHandler {
	t.Helper()
	h := &capturingHandler{}
	prev := drifterrors.DefaultHandler
	drifterrors.SetHandler(h)
	t.Cleanup(func() { drifterrors.SetHandler(prev) })
	return h
}

// --- Handler tests: per-view fakes ---

type fakeTextInputClient struct {
	mu          sync.Mutex
	textCalls   []textCall
	actionCalls []TextInputAction
	focusCalls  []bool
}

type textCall struct {
	text            string
	selBase, selExt int
}

func (c *fakeTextInputClient) OnTextChanged(text string, b, e int) {
	c.mu.Lock()
	c.textCalls = append(c.textCalls, textCall{text, b, e})
	c.mu.Unlock()
}
func (c *fakeTextInputClient) OnAction(a TextInputAction) {
	c.mu.Lock()
	c.actionCalls = append(c.actionCalls, a)
	c.mu.Unlock()
}
func (c *fakeTextInputClient) OnFocusChanged(focused bool) {
	c.mu.Lock()
	c.focusCalls = append(c.focusCalls, focused)
	c.mu.Unlock()
}

type fakeSwitchClient struct {
	mu     sync.Mutex
	values []bool
}

func (c *fakeSwitchClient) OnValueChanged(v bool) {
	c.mu.Lock()
	c.values = append(c.values, v)
	c.mu.Unlock()
}

func registerView(r *PlatformViewRegistry, v PlatformView) {
	r.mu.Lock()
	r.views[v.ViewID()] = v
	r.mu.Unlock()
}

func validTextChangedArgs(viewID int64) map[string]any {
	return map[string]any{
		"viewId":          float64(viewID),
		"text":            "hello",
		"selectionBase":   float64(2),
		"selectionExtent": float64(3),
	}
}

// assertArgErrorReport asserts at least one report matches op/channel and
// wraps *argError. Returns the matched DriftError for further inspection.
func assertArgErrorReport(t *testing.T, h *capturingHandler, op, channel string) *drifterrors.DriftError {
	t.Helper()
	for _, e := range h.snapshot() {
		if e.Op != op || e.Channel != channel || e.Kind != drifterrors.KindParsing {
			continue
		}
		var ae *argError
		if errors.As(e.Err, &ae) {
			return e
		}
	}
	t.Fatalf("no matching argError report for op=%s channel=%s; got: %+v", op, channel, h.snapshot())
	return nil
}

func TestHandleTextChanged(t *testing.T) {
	defer installImmediateDispatch(t)()

	const viewID int64 = 7
	const op = "platform_view.handleTextChanged"

	t.Run("happy", func(t *testing.T) {
		reg := newTestRegistry()
		client := &fakeTextInputClient{}
		view := NewTextInputView(viewID, TextInputViewConfig{}, client)
		registerView(reg, view)

		_, err := reg.handleTextChanged(validTextChangedArgs(viewID))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(client.textCalls) != 1 || client.textCalls[0] != (textCall{"hello", 2, 3}) {
			t.Fatalf("client did not receive call: %+v", client.textCalls)
		}
	})

	t.Run("args not map", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		_, err := reg.handleTextChanged("garbage")
		if err == nil {
			t.Fatal("expected error")
		}
		var ae *argError
		if !errors.As(err, &ae) {
			t.Fatalf("expected *argError, got %T", err)
		}
		assertArgErrorReport(t, h, op, platformViewsChannel)
	})

	t.Run("missing viewId", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		args := validTextChangedArgs(viewID)
		delete(args, "viewId")
		_, err := reg.handleTextChanged(args)
		if err == nil {
			t.Fatal("expected error")
		}
		assertArgErrorReport(t, h, op, platformViewsChannel)
	})

	t.Run("non-integral viewId", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		args := validTextChangedArgs(viewID)
		args["viewId"] = 1.5
		_, err := reg.handleTextChanged(args)
		if err == nil {
			t.Fatal("expected error")
		}
		assertArgErrorReport(t, h, op, platformViewsChannel)
	})

	t.Run("wrong-type text", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		args := validTextChangedArgs(viewID)
		args["text"] = 42
		_, err := reg.handleTextChanged(args)
		if err == nil {
			t.Fatal("expected error")
		}
		assertArgErrorReport(t, h, op, platformViewsChannel)
	})

	t.Run("unknown view tolerated", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		// No view registered. Should be a silent no-op (dispose race).
		_, err := reg.handleTextChanged(validTextChangedArgs(viewID))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(h.snapshot()) != 0 {
			t.Fatalf("expected no reports, got %+v", h.snapshot())
		}
	})

	t.Run("wrong view type", func(t *testing.T) {
		reg := newTestRegistry(viewID) // registers stubView, not TextInputView
		h := captureErrorReports(t)
		_, err := reg.handleTextChanged(validTextChangedArgs(viewID))
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, errViewTypeMismatch) {
			t.Fatalf("expected errViewTypeMismatch, got %v", err)
		}
		// Type mismatch is reported (not an argError, just the wrapped sentinel).
		found := false
		for _, e := range h.snapshot() {
			if e.Op == op && e.Channel == platformViewsChannel && errors.Is(e.Err, errViewTypeMismatch) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected type-mismatch report, got %+v", h.snapshot())
		}
	})
}

func TestHandleSwitchChanged(t *testing.T) {
	defer installImmediateDispatch(t)()

	const viewID int64 = 11
	t.Run("happy", func(t *testing.T) {
		reg := newTestRegistry()
		client := &fakeSwitchClient{}
		view := NewSwitchView(viewID, SwitchViewConfig{}, client)
		registerView(reg, view)

		args := map[string]any{
			"viewId": float64(viewID),
			"value":  true,
		}
		_, err := reg.handleSwitchChanged(args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(client.values) != 1 || client.values[0] != true {
			t.Fatalf("client did not receive call: %+v", client.values)
		}
	})

	t.Run("missing value", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		args := map[string]any{"viewId": float64(viewID)}
		_, err := reg.handleSwitchChanged(args)
		if err == nil {
			t.Fatal("expected error")
		}
		assertArgErrorReport(t, h, "platform_view.handleSwitchChanged", platformViewsChannel)
	})

	t.Run("wrong-type value (string 'true' rejected)", func(t *testing.T) {
		reg := newTestRegistry()
		h := captureErrorReports(t)
		args := map[string]any{"viewId": float64(viewID), "value": "true"}
		_, err := reg.handleSwitchChanged(args)
		if err == nil {
			t.Fatal("expected error for non-bool value")
		}
		assertArgErrorReport(t, h, "platform_view.handleSwitchChanged", platformViewsChannel)
	})
}

func TestHandleEvent_UnknownMethodReported(t *testing.T) {
	reg := newTestRegistry()
	h := captureErrorReports(t)
	reg.handleEvent(map[string]any{"method": "onSomethingNobodyKnows"})
	assertArgErrorReport(t, h, "platform_view.handleEvent", platformViewsChannel)
}

func TestHandleEvent_BadMethodTypeReported(t *testing.T) {
	reg := newTestRegistry()
	h := captureErrorReports(t)
	reg.handleEvent(map[string]any{"method": 42})
	assertArgErrorReport(t, h, "platform_view.handleEvent", platformViewsChannel)
}

func TestHandleEvent_NonMapReported(t *testing.T) {
	reg := newTestRegistry()
	h := captureErrorReports(t)
	reg.handleEvent("not a map")
	assertArgErrorReport(t, h, "platform_view.handleEvent", platformViewsChannel)
}

func TestHandleMethodCall_OnViewDisposedNoArgs(t *testing.T) {
	reg := newTestRegistry()
	h := captureErrorReports(t)
	res, err := reg.handleMethodCall("onViewDisposed", nil)
	if err != nil || res != nil {
		t.Fatalf("expected (nil, nil); got (%v, %v)", res, err)
	}
	if len(h.snapshot()) != 0 {
		t.Fatalf("expected no reports for onViewDisposed; got %+v", h.snapshot())
	}
}

func TestHandleMethodCall_UnknownMethodReturnsNotFound(t *testing.T) {
	reg := newTestRegistry()
	_, err := reg.handleMethodCall("noSuchMethod", map[string]any{})
	if !errors.Is(err, ErrMethodNotFound) {
		t.Fatalf("expected ErrMethodNotFound, got %v", err)
	}
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
		graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
		[]*graphics.Path{},
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
	reg.UpdateViewGeometry(1, graphics.Offset{X: 10, Y: 20}, graphics.Size{Width: 100, Height: 50}, nil,
		graphics.Rect{Left: 10, Top: 20, Right: 110, Bottom: 70}, []*graphics.Path{})
	reg.UpdateViewGeometry(2, graphics.Offset{X: 10, Y: 80}, graphics.Size{Width: 100, Height: 50}, nil,
		graphics.Rect{Left: 10, Top: 80, Right: 110, Bottom: 130}, []*graphics.Path{})
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
		graphics.Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
		[]*graphics.Path{},
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
	reg.UpdateViewGeometry(1, pos, size, nil,
		graphics.Rect{Left: 10, Top: 20, Right: 110, Bottom: 70}, []*graphics.Path{})
	reg.FlushGeometryBatch()
	reg.TakeCapturedSnapshot()

	// Frame 2: same geometry, view still seen.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, nil,
		graphics.Rect{Left: 10, Top: 20, Right: 110, Bottom: 70}, []*graphics.Path{})
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

	visRect := graphics.Rect{Left: 10, Top: 200, Right: 200, Bottom: 240}

	// Frame 1: view visible.
	reg.BeginGeometryBatch()
	reg.UpdateViewGeometry(1, pos, size, clip, visRect, []*graphics.Path{})
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
	reg.UpdateViewGeometry(1, pos, size, clip, visRect, []*graphics.Path{})
	reg.FlushGeometryBatch()
	captured = reg.TakeCapturedSnapshot()

	if len(captured) != 1 {
		t.Fatalf("frame 3: expected 1 geometry (restore), got %d", len(captured))
	}
	if isEmptyClip(&captured[0]) {
		t.Error("frame 3: restored view should not have empty clip")
	}
}
