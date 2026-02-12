package platform

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/graphics"
)

// CapturedViewGeometry holds the resolved geometry for one platform view,
// captured during a StepFrame pass for synchronous application on the UI thread.
type CapturedViewGeometry struct {
	ViewID     int64
	Offset     graphics.Offset
	Size       graphics.Size
	ClipBounds *graphics.Rect
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
func (r *PlatformViewRegistry) handleEvent(data any) {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return
	}

	method, _ := dataMap["method"].(string)
	if method == "" {
		return
	}

	switch method {
	case "onViewCreated":
		// Native has finished creating the view.
		// Resend geometry immediately so the view appears at the correct position.
		if viewID, ok := toInt64(dataMap["viewId"]); ok {
			r.resendGeometry(viewID)
		}
	case "onTextChanged":
		r.handleTextChanged(dataMap)
	case "onAction":
		r.handleAction(dataMap)
	case "onFocusChanged":
		r.handleFocusChanged(dataMap)
	case "onSwitchChanged":
		r.handleSwitchChanged(dataMap)
	case "onPlaybackStateChanged":
		r.handleVideoPlaybackStateChanged(dataMap)
	case "onPositionChanged":
		r.handleVideoPositionChanged(dataMap)
	case "onVideoError":
		r.handleVideoError(dataMap)
	case "onPageStarted":
		r.handleWebViewPageStarted(dataMap)
	case "onPageFinished":
		r.handleWebViewPageFinished(dataMap)
	case "onWebViewError":
		r.handleWebViewError(dataMap)
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
	_, err = r.channel.Invoke("create", map[string]any{
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
		r.channel.Invoke("dispose", map[string]any{
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
func (r *PlatformViewRegistry) UpdateViewGeometry(viewID int64, offset graphics.Offset, size graphics.Size, clipBounds *graphics.Rect) error {
	// Guard: ignore disposed/unknown views
	r.mu.RLock()
	_, exists := r.views[viewID]
	r.mu.RUnlock()
	if !exists {
		return nil
	}

	entry := CapturedViewGeometry{
		ViewID:     viewID,
		Offset:     offset,
		Size:       size,
		ClipBounds: clipBounds,
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
	viewsSeen := r.viewsSeenThisFrame
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
				ViewID:     viewID,
				ClipBounds: &emptyClip,
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
	r.UpdateViewGeometry(viewID, cached.Offset, cached.Size, cached.ClipBounds)
}

// ClearGeometryCache removes cached geometry for a view (call on dispose).
func (r *PlatformViewRegistry) ClearGeometryCache(viewID int64) {
	r.batchMu.Lock()
	delete(r.geometryCache, viewID)
	r.batchMu.Unlock()
}

// SetViewVisible notifies native to show or hide a view.
func (r *PlatformViewRegistry) SetViewVisible(viewID int64, visible bool) error {
	_, err := r.channel.Invoke("setVisible", map[string]any{
		"viewId":  viewID,
		"visible": visible,
	})
	return err
}

// SetViewEnabled notifies native to enable or disable a view.
func (r *PlatformViewRegistry) SetViewEnabled(viewID int64, enabled bool) error {
	_, err := r.channel.Invoke("setEnabled", map[string]any{
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
	for k, v := range args { // safe: range over nil map is no-op
		invokeArgs[k] = v
	}
	invokeArgs["viewId"] = viewID
	invokeArgs["method"] = method
	return r.channel.Invoke("invokeViewMethod", invokeArgs)
}

// handleMethodCall processes incoming method calls from native code.
func (r *PlatformViewRegistry) handleMethodCall(method string, args any) (any, error) {
	argsMap, _ := args.(map[string]any)

	switch method {
	case "onViewCreated":
		// Native has finished creating the view.
		// Resend geometry immediately so the view appears at the correct position.
		if viewID, ok := toInt64(argsMap["viewId"]); ok {
			r.resendGeometry(viewID)
		}
		return nil, nil

	case "onViewDisposed":
		// Native has finished disposing the view
		return nil, nil

	case "onTextChanged":
		return r.handleTextChanged(argsMap)

	case "onAction":
		return r.handleAction(argsMap)

	case "onFocusChanged":
		return r.handleFocusChanged(argsMap)

	case "onSwitchChanged":
		return r.handleSwitchChanged(argsMap)

	case "onPlaybackStateChanged":
		return r.handleVideoPlaybackStateChanged(argsMap)

	case "onPositionChanged":
		return r.handleVideoPositionChanged(argsMap)

	case "onVideoError":
		return r.handleVideoError(argsMap)

	case "onPageStarted":
		return r.handleWebViewPageStarted(argsMap)

	case "onPageFinished":
		return r.handleWebViewPageFinished(argsMap)

	case "onWebViewError":
		return r.handleWebViewError(argsMap)

	default:
		return nil, ErrMethodNotFound
	}
}

func (r *PlatformViewRegistry) handleTextChanged(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	text, _ := args["text"].(string)
	selBase, _ := toInt(args["selectionBase"])
	selExt, _ := toInt(args["selectionExtent"])

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if textInput, ok := view.(*TextInputView); ok {
		textInput.handleTextChanged(text, selBase, selExt)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleAction(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	action, _ := toInt(args["action"])

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if textInput, ok := view.(*TextInputView); ok {
		textInput.handleAction(TextInputAction(action))
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleFocusChanged(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	focused, _ := args["focused"].(bool)

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if textInput, ok := view.(*TextInputView); ok {
		textInput.handleFocusChanged(focused)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleSwitchChanged(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	value, _ := args["value"].(bool)

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if switchView, ok := view.(*SwitchView); ok {
		switchView.handleValueChanged(value)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleVideoPlaybackStateChanged(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	stateInt, _ := toInt(args["state"])

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if videoView, ok := view.(*videoPlayerView); ok {
		videoView.handlePlaybackStateChanged(PlaybackState(stateInt))
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleVideoPositionChanged(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	positionMs, _ := toInt64(args["positionMs"])
	durationMs, _ := toInt64(args["durationMs"])
	bufferedMs, _ := toInt64(args["bufferedMs"])

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if videoView, ok := view.(*videoPlayerView); ok {
		videoView.handlePositionChanged(
			time.Duration(positionMs)*time.Millisecond,
			time.Duration(durationMs)*time.Millisecond,
			time.Duration(bufferedMs)*time.Millisecond,
		)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleVideoError(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	code, _ := args["code"].(string)
	message, _ := args["message"].(string)

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if videoView, ok := view.(*videoPlayerView); ok {
		videoView.handleError(code, message)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleWebViewPageStarted(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	url, _ := args["url"].(string)

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if webView, ok := view.(*nativeWebView); ok {
		webView.handlePageStarted(url)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleWebViewPageFinished(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	url, _ := args["url"].(string)

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if webView, ok := view.(*nativeWebView); ok {
		webView.handlePageFinished(url)
	}
	return nil, nil
}

func (r *PlatformViewRegistry) handleWebViewError(args map[string]any) (any, error) {
	viewID, _ := toInt64(args["viewId"])
	code, _ := args["code"].(string)
	message, _ := args["message"].(string)

	r.mu.RLock()
	view := r.views[viewID]
	r.mu.RUnlock()

	if webView, ok := view.(*nativeWebView); ok {
		webView.handleError(code, message)
	}
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
