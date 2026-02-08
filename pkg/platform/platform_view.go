package platform

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/graphics"
)

// geometryUpdate represents a pending geometry change for a platform view.
type geometryUpdate struct {
	viewID     int64
	offset     graphics.Offset
	size       graphics.Size
	clipBounds *graphics.Rect // nil = no clipping
}

// viewGeometryCache tracks the last sent geometry to avoid redundant updates.
type viewGeometryCache struct {
	offset     graphics.Offset
	size       graphics.Size
	clipBounds *graphics.Rect
}

// rectsEqual compares two clip bounds with tolerance (handles nil).
// Uses epsilon to avoid defeating dedupe due to sub-pixel drift from animation/scroll.
func rectsEqual(a, b *graphics.Rect) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	const epsilon = 0.0001 // Same as geometry.go
	return math.Abs(a.Left-b.Left) <= epsilon &&
		math.Abs(a.Top-b.Top) <= epsilon &&
		math.Abs(a.Right-b.Right) <= epsilon &&
		math.Abs(a.Bottom-b.Bottom) <= epsilon
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

	// SetSize updates the view size in logical pixels.
	SetSize(size graphics.Size)

	// SetOffset updates the view position in logical pixels.
	SetOffset(offset graphics.Offset)

	// SetVisible shows or hides the native view.
	SetVisible(visible bool)
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
	// queued during compositing and sent to native as a single batch.
	batchMu       sync.Mutex
	batchMode     bool
	batchUpdates  []geometryUpdate
	frameSeq      uint64
	geometryCache map[int64]viewGeometryCache
	// viewsSeenThisFrame tracks which views received geometry updates this frame.
	// Views NOT seen are sent empty clip bounds in FlushGeometryBatch, causing the
	// native side to hide them. This prevents culled (off-screen) platform views
	// from staying visible at stale positions.
	viewsSeenThisFrame map[int64]struct{}

	// Geometry-applied signal: native signals after applying geometry so
	// the render thread can defer surface presentation until both Skia
	// content and native view geometry land before the same vsync.
	geometryPending atomic.Bool
	geometrySignal  chan struct{} // buffered, size 1
	geometryTimer   *time.Timer   // reusable timer for WaitGeometryApplied (render thread only)

	// Stats for monitoring
	BatchTimeouts atomic.Uint64
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
	timer := time.NewTimer(0)
	<-timer.C // drain initial fire so the timer is ready for Reset

	r := &PlatformViewRegistry{
		factories:          make(map[string]PlatformViewFactory),
		views:              make(map[int64]PlatformView),
		channel:            NewMethodChannel("drift/platform_views"),
		geometryCache:      make(map[int64]viewGeometryCache),
		viewsSeenThisFrame: make(map[int64]struct{}),
		geometrySignal:     make(chan struct{}, 1),
		geometryTimer:      timer,
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

// HasViews returns true if any platform views are registered.
func (r *PlatformViewRegistry) HasViews() bool {
	r.mu.RLock()
	hasViews := len(r.views) > 0
	r.mu.RUnlock()
	return hasViews
}

// ViewCount returns the number of active platform views.
func (r *PlatformViewRegistry) ViewCount() int {
	r.mu.RLock()
	count := len(r.views)
	r.mu.RUnlock()
	return count
}

// UpdateViewGeometry notifies native of a view's position, size, and clip bounds.
// If batching is active, the update is queued; otherwise sent immediately.
// Gracefully ignores disposed or unknown viewIDs.
func (r *PlatformViewRegistry) UpdateViewGeometry(viewID int64, offset graphics.Offset, size graphics.Size, clipBounds *graphics.Rect) error {
	// Guard: ignore disposed/unknown views
	r.mu.RLock()
	_, exists := r.views[viewID]
	r.mu.RUnlock()
	if !exists {
		return nil
	}

	r.batchMu.Lock()

	// Mark as seen this frame (before dedup check, so culled-then-visible
	// views don't get hidden even if geometry hasn't changed)
	r.viewsSeenThisFrame[viewID] = struct{}{}

	// Check if geometry has actually changed (deduplication)
	if cached, ok := r.geometryCache[viewID]; ok {
		if cached.offset == offset && cached.size == size && rectsEqual(cached.clipBounds, clipBounds) {
			r.batchMu.Unlock()
			return nil // No change, skip update
		}
	}

	// Update cache
	r.geometryCache[viewID] = viewGeometryCache{offset: offset, size: size, clipBounds: clipBounds}

	if r.batchMode {
		// Queue for batch send
		r.batchUpdates = append(r.batchUpdates, geometryUpdate{
			viewID:     viewID,
			offset:     offset,
			size:       size,
			clipBounds: clipBounds,
		})
		r.batchMu.Unlock()
		return nil
	}
	r.batchMu.Unlock()

	// Not batching, send immediately (fallback for non-frame updates)
	args := map[string]any{
		"viewId": viewID,
		"x":      offset.X,
		"y":      offset.Y,
		"width":  size.Width,
		"height": size.Height,
	}
	if clipBounds != nil {
		args["clipLeft"] = clipBounds.Left
		args["clipTop"] = clipBounds.Top
		args["clipRight"] = clipBounds.Right
		args["clipBottom"] = clipBounds.Bottom
	}
	_, err := r.channel.Invoke("setGeometry", args)
	return err
}

// BeginGeometryBatch starts collecting geometry updates for batch processing.
// Call this at the start of each frame before paint.
func (r *PlatformViewRegistry) BeginGeometryBatch() {
	r.batchMu.Lock()
	r.batchMode = true
	r.batchUpdates = r.batchUpdates[:0] // Reset slice, keep capacity
	r.frameSeq++
	// Clear seen set (reuse map to avoid allocation)
	for k := range r.viewsSeenThisFrame {
		delete(r.viewsSeenThisFrame, k)
	}
	r.batchMu.Unlock()
}

// FlushGeometryBatch sends all queued geometry updates to native code.
// Native applies updates asynchronously on its main thread — this call
// returns as soon as the message is delivered. The frameSeq mechanism
// ensures stale batches are skipped if native falls behind.
func (r *PlatformViewRegistry) FlushGeometryBatch() {
	r.batchMu.Lock()
	updates := r.batchUpdates
	frameSeq := r.frameSeq
	r.batchMode = false
	r.batchUpdates = nil

	// Snapshot the seen set under batchMu
	viewsSeen := r.viewsSeenThisFrame
	r.batchMu.Unlock()

	// Hide unseen views by sending empty clip bounds.
	// This ensures culled platform views (scrolled off-screen) don't remain
	// visible at their last-known position.
	r.mu.RLock()
	for viewID := range r.views {
		if _, seen := viewsSeen[viewID]; !seen {
			emptyClip := graphics.Rect{} // 0,0,0,0 → empty → native hides the view
			updates = append(updates, geometryUpdate{
				viewID:     viewID,
				clipBounds: &emptyClip,
			})
		}
	}
	r.mu.RUnlock()

	// Update geometry cache for hidden views so that when the view scrolls
	// back into view, the real geometry will differ from cached hidden state
	// and the dedup check will allow the update through.
	r.batchMu.Lock()
	for _, u := range updates {
		if _, seen := viewsSeen[u.viewID]; !seen {
			r.geometryCache[u.viewID] = viewGeometryCache{
				clipBounds: u.clipBounds,
			}
		}
	}
	r.batchMu.Unlock()

	if len(updates) == 0 {
		return
	}

	// Convert to format for native
	batch := make([]map[string]any, len(updates))
	for i, u := range updates {
		entry := map[string]any{
			"viewId": u.viewID,
			"x":      u.offset.X,
			"y":      u.offset.Y,
			"width":  u.size.Width,
			"height": u.size.Height,
		}
		if u.clipBounds != nil {
			entry["clipLeft"] = u.clipBounds.Left
			entry["clipTop"] = u.clipBounds.Top
			entry["clipRight"] = u.clipBounds.Right
			entry["clipBottom"] = u.clipBounds.Bottom
		}
		batch[i] = entry
	}

	// Signal infrastructure: mark pending and drain any stale signal before sending.
	r.geometryPending.Store(true)
	select {
	case <-r.geometrySignal:
	default:
	}

	// Send batch to native. Native posts to its main thread and returns immediately.
	// The frameSeq allows native to skip stale batches.
	_, err := r.channel.Invoke("batchSetGeometry", map[string]any{
		"frameSeq":   frameSeq,
		"geometries": batch,
	})
	if err != nil {
		r.geometryPending.Store(false) // no signal will come — don't wait 8ms
		r.BatchTimeouts.Add(1)
	}
}

// WaitGeometryApplied blocks until native confirms geometry has been applied,
// or until timeout expires. No-op if no geometry batch was sent this frame.
// GPU work (surface.Flush) is already submitted and pipelines with this wait.
func (r *PlatformViewRegistry) WaitGeometryApplied(timeout time.Duration) {
	if !r.geometryPending.Load() {
		return
	}
	r.geometryTimer.Reset(timeout)
	select {
	case <-r.geometrySignal:
	case <-r.geometryTimer.C:
	}
	// Stop + drain to leave the timer in a clean state for next frame.
	if !r.geometryTimer.Stop() {
		select {
		case <-r.geometryTimer.C:
		default:
		}
	}
	r.geometryPending.Store(false)
}

// SignalGeometryApplied is called by native (via DriftGeometryApplied CGo export)
// after geometry has been applied on the native main thread.
func (r *PlatformViewRegistry) SignalGeometryApplied() {
	select {
	case r.geometrySignal <- struct{}{}:
	default:
	}
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
	// Clear cache first so the next UpdateViewGeometry actually sends
	r.ClearGeometryCache(viewID)
	r.UpdateViewGeometry(viewID, cached.offset, cached.size, cached.clipBounds)
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
	offset   graphics.Offset
	size     graphics.Size
	visible  bool
}

func (v *basePlatformView) ViewID() int64 {
	return v.viewID
}

func (v *basePlatformView) ViewType() string {
	return v.viewType
}

func (v *basePlatformView) SetSize(size graphics.Size) {
	v.size = size
	GetPlatformViewRegistry().UpdateViewGeometry(v.viewID, v.offset, v.size, nil)
}

func (v *basePlatformView) SetOffset(offset graphics.Offset) {
	v.offset = offset
	GetPlatformViewRegistry().UpdateViewGeometry(v.viewID, v.offset, v.size, nil)
}

// SetGeometry updates position, size, and clip bounds in a single call.
func (v *basePlatformView) SetGeometry(offset graphics.Offset, size graphics.Size, clipBounds *graphics.Rect) {
	v.offset = offset
	v.size = size
	GetPlatformViewRegistry().UpdateViewGeometry(v.viewID, v.offset, v.size, clipBounds)
}

func (v *basePlatformView) SetVisible(visible bool) {
	v.visible = visible
	GetPlatformViewRegistry().SetViewVisible(v.viewID, visible)
}

func (v *basePlatformView) SetEnabled(enabled bool) {
	GetPlatformViewRegistry().SetViewEnabled(v.viewID, enabled)
}
