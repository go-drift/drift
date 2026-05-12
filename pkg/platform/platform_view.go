package platform

import (
	"context"
	"errors"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/graphics"
)

// CapturedViewGeometry holds the resolved geometry for one platform view,
// captured during a StepFrame pass for synchronous application on the UI thread.
type CapturedViewGeometry struct {
	ViewID         int64
	Offset         graphics.Offset
	Size           graphics.Size
	ClipBounds     *graphics.Rect   // collapsed single-rect after occlusion subtraction (Android fallback); nil if unclipped
	VisibleRect    graphics.Rect    // view bounds intersected with parent clips; {0,0,0,0} if hidden
	OcclusionPaths []*graphics.Path // path-based occlusion masks; [] if none
}

// PlatformView represents a native view embedded in Drift UI.
type PlatformView interface {
	// ViewID returns the unique identifier for this view.
	ViewID() int64

	// ViewType returns the type identifier for this view (e.g., "native_webview").
	ViewType() string

	// Create initializes the native view with given parameters.
	Create(params map[string]any) error

	// Dispose cleans up the native view.
	Dispose()
}

// PlatformViewFactory creates platform views of a specific type.
type PlatformViewFactory interface {
	// Create creates a new platform view instance.
	Create(viewID int64, params map[string]any) (PlatformView, error)

	// ViewType returns the view type this factory creates.
	ViewType() string
}

// PlatformViewRegistry manages platform view types and instances.
type PlatformViewRegistry struct {
	factories map[string]PlatformViewFactory
	views     map[int64]PlatformView
	nextID    atomic.Int64
	mu        sync.RWMutex
	channel   *MethodChannel

	// Geometry batching for synchronized updates.
	// BeginGeometryBatch/FlushGeometryBatch bracket each frame. Updates are
	// queued during compositing and collected into capturedViews.
	batchMu      sync.Mutex
	batchUpdates []CapturedViewGeometry
	// geometryCache stores the last geometry for each view, used by
	// resendGeometry to replay position when a native view finishes creating.
	geometryCache map[int64]CapturedViewGeometry
	// viewsSeenThisFrame tracks which views received geometry updates this frame.
	// Views NOT seen get empty clip bounds in FlushGeometryBatch, signaling hidden.
	viewsSeenThisFrame map[int64]struct{}
	capturedViews      []CapturedViewGeometry
}

var platformViewRegistry *PlatformViewRegistry

// GetPlatformViewRegistry returns the global platform view registry.
func GetPlatformViewRegistry() *PlatformViewRegistry {
	if platformViewRegistry == nil {
		platformViewRegistry = newPlatformViewRegistry()
	}
	return platformViewRegistry
}

func newPlatformViewRegistry() *PlatformViewRegistry {
	r := &PlatformViewRegistry{
		factories:          make(map[string]PlatformViewFactory),
		views:              make(map[int64]PlatformView),
		channel:            NewMethodChannel("drift/platform_views"),
		geometryCache:      make(map[int64]CapturedViewGeometry),
		viewsSeenThisFrame: make(map[int64]struct{}),
	}

	// Handle incoming calls from native
	r.channel.SetHandler(r.handleMethodCall)

	// Also listen for events from native (text changes, focus, etc.)
	eventChannel := NewEventChannel("drift/platform_views")
	listenForViewEvents := func() {
		eventChannel.Listen(EventHandler{
			OnEvent: func(data any) {
				r.handleEvent(data)
			},
		})
	}
	listenForViewEvents()
	registerBuiltinInit(listenForViewEvents)

	return r
}

// handleEvent processes events from native platform views.
//
// Per-handler errors are already reported through reportPlatformViewArg
// before being returned, so the (any, error) results are discarded here.
func (r *PlatformViewRegistry) handleEvent(data any) {
	const op = "handleEvent"
	args, err := requireMap(op, data)
	if err != nil {
		reportPlatformViewArg(op, err)
		return
	}
	method, err := requireString(op, args, "method")
	if err != nil {
		reportPlatformViewArg(op, err)
		return
	}

	switch method {
	case "onViewCreated":
		r.handleViewCreated(args)
	case "onTextChanged":
		r.handleTextChanged(args)
	case "onAction":
		r.handleAction(args)
	case "onFocusChanged":
		r.handleFocusChanged(args)
	case "onSwitchChanged":
		r.handleSwitchChanged(args)
	case "onPlaybackStateChanged":
		r.handleVideoPlaybackStateChanged(args)
	case "onPositionChanged":
		r.handleVideoPositionChanged(args)
	case "onVideoError":
		r.handleVideoError(args)
	case "onPageStarted":
		r.handleWebViewPageStarted(args)
	case "onPageFinished":
		r.handleWebViewPageFinished(args)
	case "onWebViewError":
		r.handleWebViewError(args)
	default:
		reportPlatformViewArg(op, &argError{
			Op: op, Key: "method", Want: "known event method", Got: method,
		})
	}
}

// RegisterFactory registers a factory for a platform view type.
func (r *PlatformViewRegistry) RegisterFactory(factory PlatformViewFactory) {
	r.mu.Lock()
	r.factories[factory.ViewType()] = factory
	r.mu.Unlock()
}

// Create creates a new platform view of the given type.
func (r *PlatformViewRegistry) Create(viewType string, params map[string]any) (PlatformView, error) {
	r.mu.RLock()
	factory, ok := r.factories[viewType]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrViewTypeNotFound
	}

	viewID := r.nextID.Add(1)

	view, err := factory.Create(viewID, params)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.views[viewID] = view
	r.mu.Unlock()

	// Notify native to create the view
	_, err = r.channel.Invoke(context.Background(), "create", map[string]any{
		"viewId":   viewID,
		"viewType": viewType,
		"params":   params,
	})
	if err != nil {
		r.mu.Lock()
		delete(r.views, viewID)
		r.mu.Unlock()
		return nil, err
	}

	return view, nil
}

// Dispose destroys a platform view.
func (r *PlatformViewRegistry) Dispose(viewID int64) {
	r.mu.Lock()
	view, ok := r.views[viewID]
	if ok {
		delete(r.views, viewID)
	}
	r.mu.Unlock()

	// Clear geometry cache to avoid stale skips if view is recreated
	r.ClearGeometryCache(viewID)

	if ok {
		view.Dispose()
		// Notify native to destroy the view
		r.channel.Invoke(context.Background(), "dispose", map[string]any{
			"viewId": viewID,
		})
	}
}

// GetView returns a platform view by ID.
func (r *PlatformViewRegistry) GetView(viewID int64) PlatformView {
	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()
	return view
}

// ViewCount returns the number of active platform views.
func (r *PlatformViewRegistry) ViewCount() int {
	r.mu.RLock()
	count := len(r.views)
	r.mu.RUnlock()
	return count
}

// UpdateViewGeometry queues a geometry update for a platform view.
// Updates are collected during compositing and flushed via FlushGeometryBatch.
// Gracefully ignores disposed or unknown viewIDs.
func (r *PlatformViewRegistry) UpdateViewGeometry(viewID int64, offset graphics.Offset, size graphics.Size,
	clipBounds *graphics.Rect, visibleRect graphics.Rect, occlusionPaths []*graphics.Path) error {
	// Guard: ignore disposed/unknown views
	r.mu.RLock()
	_, exists := r.views[viewID]
	r.mu.RUnlock()
	if !exists {
		return nil
	}

	entry := CapturedViewGeometry{
		ViewID:         viewID,
		Offset:         offset,
		Size:           size,
		ClipBounds:     clipBounds,
		VisibleRect:    visibleRect,
		OcclusionPaths: occlusionPaths,
	}

	r.batchMu.Lock()

	// Mark as seen this frame (before queuing, so culled-then-visible
	// views don't get hidden even if geometry hasn't changed)
	r.viewsSeenThisFrame[viewID] = struct{}{}

	// Update cache for resendGeometry
	r.geometryCache[viewID] = entry

	// Queue for batch send (geometry is always batched in the split pipeline)
	r.batchUpdates = append(r.batchUpdates, entry)
	r.batchMu.Unlock()
	return nil
}

// BeginGeometryBatch starts collecting geometry updates for batch processing.
// Call this at the start of each frame before the geometry compositing pass.
func (r *PlatformViewRegistry) BeginGeometryBatch() {
	r.batchMu.Lock()
	r.batchUpdates = r.batchUpdates[:0] // Reset slice, keep capacity
	// Clear seen set (reuse map to avoid allocation)
	for k := range r.viewsSeenThisFrame {
		delete(r.viewsSeenThisFrame, k)
	}
	r.batchMu.Unlock()
}

// FlushGeometryBatch collects all queued geometry updates (including hide
// entries for unseen views) into the captured snapshot. The caller retrieves
// the result via TakeCapturedSnapshot.
func (r *PlatformViewRegistry) FlushGeometryBatch() {
	r.batchMu.Lock()
	// Move batch updates directly into captured views
	r.capturedViews = append(r.capturedViews, r.batchUpdates...)
	// Copy the seen set so we can release batchMu before the r.mu.RLock below.
	// Without the copy, a concurrent resendGeometry (triggered by native
	// onViewCreated) could write to viewsSeenThisFrame while we read it.
	viewsSeen := make(map[int64]struct{}, len(r.viewsSeenThisFrame))
	for id := range r.viewsSeenThisFrame {
		viewsSeen[id] = struct{}{}
	}
	r.batchUpdates = nil
	r.batchMu.Unlock()

	// Hide unseen views by adding empty clip bounds.
	// This ensures culled platform views (scrolled off-screen) don't remain
	// visible at their last-known position.
	r.mu.RLock()
	var hidden []CapturedViewGeometry
	for viewID := range r.views {
		if _, seen := viewsSeen[viewID]; !seen {
			emptyClip := graphics.Rect{} // 0,0,0,0 signals hidden
			hidden = append(hidden, CapturedViewGeometry{
				ViewID:         viewID,
				ClipBounds:     &emptyClip,
				VisibleRect:    graphics.Rect{},
				OcclusionPaths: []*graphics.Path{},
			})
		}
	}
	r.mu.RUnlock()

	if len(hidden) > 0 {
		r.batchMu.Lock()
		for _, h := range hidden {
			// Update geometry cache for hidden views so that when the view scrolls
			// back into view, the real geometry will differ from cached hidden state.
			r.geometryCache[h.ViewID] = h
		}
		r.capturedViews = append(r.capturedViews, hidden...)
		r.batchMu.Unlock()
	}
}

// TakeCapturedSnapshot returns the geometry captured during the last frame
// and resets the capture buffer.
func (r *PlatformViewRegistry) TakeCapturedSnapshot() []CapturedViewGeometry {
	r.batchMu.Lock()
	result := r.capturedViews
	r.capturedViews = nil
	r.batchMu.Unlock()
	return result
}

// resendGeometry replays the cached geometry for a view.
// Called when a native view finishes creation and needs its position.
func (r *PlatformViewRegistry) resendGeometry(viewID int64) {
	r.batchMu.Lock()
	cached, ok := r.geometryCache[viewID]
	r.batchMu.Unlock()
	if !ok {
		return
	}
	r.UpdateViewGeometry(viewID, cached.Offset, cached.Size, cached.ClipBounds, cached.VisibleRect, cached.OcclusionPaths)
}

// ClearGeometryCache removes cached geometry for a view (call on dispose).
func (r *PlatformViewRegistry) ClearGeometryCache(viewID int64) {
	r.batchMu.Lock()
	delete(r.geometryCache, viewID)
	r.batchMu.Unlock()
}

// SetViewVisible notifies native to show or hide a view.
func (r *PlatformViewRegistry) SetViewVisible(viewID int64, visible bool) error {
	_, err := r.channel.Invoke(context.Background(), "setVisible", map[string]any{
		"viewId":  viewID,
		"visible": visible,
	})
	return err
}

// SetViewEnabled notifies native to enable or disable a view.
func (r *PlatformViewRegistry) SetViewEnabled(viewID int64, enabled bool) error {
	_, err := r.channel.Invoke(context.Background(), "setEnabled", map[string]any{
		"viewId":  viewID,
		"enabled": enabled,
	})
	return err
}

// InvokeViewMethod invokes a method on a specific platform view.
func (r *PlatformViewRegistry) InvokeViewMethod(viewID int64, method string, args map[string]any) (any, error) {
	// Clone the args map to avoid mutating the caller's map
	size := 2
	if args != nil {
		size += len(args)
	}
	invokeArgs := make(map[string]any, size)
	// safe: range over nil map is no-op
	maps.Copy(invokeArgs, args)
	invokeArgs["viewId"] = viewID
	invokeArgs["method"] = method
	return r.channel.Invoke(context.Background(), "invokeViewMethod", invokeArgs)
}

// handleMethodCall processes incoming method calls from native code.
func (r *PlatformViewRegistry) handleMethodCall(method string, args any) (any, error) {
	switch method {
	case "onViewDisposed":
		// Native has finished disposing the view. No args expected.
		return nil, nil
	case "onViewCreated":
		return r.handleViewCreated(args)
	case "onTextChanged":
		return r.handleTextChanged(args)
	case "onAction":
		return r.handleAction(args)
	case "onFocusChanged":
		return r.handleFocusChanged(args)
	case "onSwitchChanged":
		return r.handleSwitchChanged(args)
	case "onPlaybackStateChanged":
		return r.handleVideoPlaybackStateChanged(args)
	case "onPositionChanged":
		return r.handleVideoPositionChanged(args)
	case "onVideoError":
		return r.handleVideoError(args)
	case "onPageStarted":
		return r.handleWebViewPageStarted(args)
	case "onPageFinished":
		return r.handleWebViewPageFinished(args)
	case "onWebViewError":
		return r.handleWebViewError(args)
	default:
		return nil, ErrMethodNotFound
	}
}

// Invariant for all handle* below: any error returned has already been
// surfaced through reportPlatformViewArg, so callers can discard the error
// (handleEvent does) and the report path is the single source of truth.

func (r *PlatformViewRegistry) handleViewCreated(raw any) (any, error) {
	const op = "handleViewCreated"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	r.resendGeometry(viewID)
	return nil, nil
}

func (r *PlatformViewRegistry) handleTextChanged(raw any) (any, error) {
	const op = "handleTextChanged"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	text, err := requireString(op, args, "text")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	selBase, err := requireInt(op, args, "selectionBase")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	selExt, err := requireInt(op, args, "selectionExtent")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*TextInputView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handleTextChanged(text, selBase, selExt)
	return nil, nil
}

func (r *PlatformViewRegistry) handleAction(raw any) (any, error) {
	const op = "handleAction"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	action, err := requireInt(op, args, "action")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*TextInputView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handleAction(TextInputAction(action))
	return nil, nil
}

func (r *PlatformViewRegistry) handleFocusChanged(raw any) (any, error) {
	const op = "handleFocusChanged"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	focused, err := requireBool(op, args, "focused")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*TextInputView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handleFocusChanged(focused)
	return nil, nil
}

func (r *PlatformViewRegistry) handleSwitchChanged(raw any) (any, error) {
	const op = "handleSwitchChanged"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	value, err := requireBool(op, args, "value")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*SwitchView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handleValueChanged(value)
	return nil, nil
}

func (r *PlatformViewRegistry) handleVideoPlaybackStateChanged(raw any) (any, error) {
	const op = "handleVideoPlaybackStateChanged"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	stateInt, err := requireInt(op, args, "state")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*videoPlayerView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handlePlaybackStateChanged(PlaybackState(stateInt))
	return nil, nil
}

func (r *PlatformViewRegistry) handleVideoPositionChanged(raw any) (any, error) {
	const op = "handleVideoPositionChanged"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	positionMs, err := requireInt64(op, args, "positionMs")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	durationMs, err := requireInt64(op, args, "durationMs")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	bufferedMs, err := requireInt64(op, args, "bufferedMs")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*videoPlayerView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handlePositionChanged(
		time.Duration(positionMs)*time.Millisecond,
		time.Duration(durationMs)*time.Millisecond,
		time.Duration(bufferedMs)*time.Millisecond,
	)
	return nil, nil
}

func (r *PlatformViewRegistry) handleVideoError(raw any) (any, error) {
	const op = "handleVideoError"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	code, err := requireString(op, args, "code")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	message, err := requireString(op, args, "message")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*videoPlayerView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handleError(code, message)
	return nil, nil
}

func (r *PlatformViewRegistry) handleWebViewPageStarted(raw any) (any, error) {
	const op = "handleWebViewPageStarted"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	url, err := requireString(op, args, "url")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*nativeWebView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handlePageStarted(url)
	return nil, nil
}

func (r *PlatformViewRegistry) handleWebViewPageFinished(raw any) (any, error) {
	const op = "handleWebViewPageFinished"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	url, err := requireString(op, args, "url")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*nativeWebView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handlePageFinished(url)
	return nil, nil
}

func (r *PlatformViewRegistry) handleWebViewError(raw any) (any, error) {
	const op = "handleWebViewError"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	viewID, err := requireInt64(op, args, "viewId")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	code, err := requireString(op, args, "code")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}
	message, err := requireString(op, args, "message")
	if err != nil {
		return nil, reportPlatformViewArg(op, err)
	}

	view, err := lookupView[*nativeWebView](r, viewID)
	if err != nil {
		if errors.Is(err, errViewNotFound) {
			return nil, nil
		}
		return nil, reportPlatformViewArg(op, err)
	}
	view.handleError(code, message)
	return nil, nil
}

// basePlatformView provides common implementation for platform views.
type basePlatformView struct {
	viewID   int64
	viewType string
}

func (v *basePlatformView) ViewID() int64 {
	return v.viewID
}

func (v *basePlatformView) ViewType() string {
	return v.viewType
}

func (v *basePlatformView) SetEnabled(enabled bool) {
	GetPlatformViewRegistry().SetViewEnabled(v.viewID, enabled)
}
