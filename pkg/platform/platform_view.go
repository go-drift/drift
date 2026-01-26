package platform

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/rendering"
)

// geometryUpdate represents a pending geometry change for a platform view.
type geometryUpdate struct {
	viewID     int64
	offset     rendering.Offset
	size       rendering.Size
	clipBounds *rendering.Rect // nil = no clipping
}

// viewGeometryCache tracks the last sent geometry to avoid redundant updates.
type viewGeometryCache struct {
	offset     rendering.Offset
	size       rendering.Size
	clipBounds *rendering.Rect
}

// rectsEqual compares two clip bounds with tolerance (handles nil).
// Uses epsilon to avoid defeating dedupe due to sub-pixel drift from animation/scroll.
func rectsEqual(a, b *rendering.Rect) bool {
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
	SetSize(size rendering.Size)

	// SetOffset updates the view position in logical pixels.
	SetOffset(offset rendering.Offset)

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

	// Geometry batching for synchronized updates
	batchMu       sync.Mutex
	batchMode     bool
	batchUpdates  []geometryUpdate
	frameSeq      uint64
	geometryCache map[int64]viewGeometryCache

	// Track views for culling detection (views not painted should be hidden)
	viewsAtFrameStart    map[int64]struct{}
	viewsUpdatedThisFrame map[int64]struct{}

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
	r := &PlatformViewRegistry{
		factories:     make(map[string]PlatformViewFactory),
		views:         make(map[int64]PlatformView),
		channel:       NewMethodChannel("drift/platform_views"),
		geometryCache: make(map[int64]viewGeometryCache),
	}

	// Handle incoming calls from native
	r.channel.SetHandler(r.handleMethodCall)

	// Also listen for events from native (text changes, focus, etc.)
	eventChannel := NewEventChannel("drift/platform_views")
	eventChannel.Listen(EventHandler{
		OnEvent: func(data any) {
			r.handleEvent(data)
		},
	})

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
		// Clear geometry cache so next frame resends position.
		if viewID, ok := toInt64(dataMap["viewId"]); ok {
			r.ClearGeometryCache(viewID)
		}
	case "onTextChanged":
		r.handleTextChanged(dataMap)
	case "onAction":
		r.handleAction(dataMap)
	case "onFocusChanged":
		r.handleFocusChanged(dataMap)
	case "onSwitchChanged":
		r.handleSwitchChanged(dataMap)
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

// UpdateViewGeometry notifies native of a view's position, size, and clip bounds.
// If batching is active, the update is queued; otherwise sent immediately.
func (r *PlatformViewRegistry) UpdateViewGeometry(viewID int64, offset rendering.Offset, size rendering.Size, clipBounds *rendering.Rect) error {
	r.batchMu.Lock()

	// Mark this view as updated this frame (for culling detection)
	if r.viewsUpdatedThisFrame != nil {
		r.viewsUpdatedThisFrame[viewID] = struct{}{}
	}

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
	// Track which views existed at frame start to detect culled views
	r.viewsAtFrameStart = make(map[int64]struct{})
	r.mu.RLock()
	for id := range r.views {
		r.viewsAtFrameStart[id] = struct{}{}
	}
	r.mu.RUnlock()
	r.viewsUpdatedThisFrame = make(map[int64]struct{})
	r.batchMu.Unlock()
}

// FlushGeometryBatch sends all queued geometry updates to native and waits
// for them to be applied. This ensures native views are positioned correctly
// before the frame is displayed. Also hides any views that were paint-culled
// (existed at frame start but weren't updated during paint).
func (r *PlatformViewRegistry) FlushGeometryBatch() {
	r.batchMu.Lock()
	updates := r.batchUpdates
	frameSeq := r.frameSeq
	viewsAtStart := r.viewsAtFrameStart
	viewsUpdated := r.viewsUpdatedThisFrame
	r.batchMode = false
	r.batchUpdates = nil
	r.viewsAtFrameStart = nil
	r.viewsUpdatedThisFrame = nil
	r.batchMu.Unlock()

	// Find views that were culled (existed at frame start but not painted)
	var culledViews []int64
	for viewID := range viewsAtStart {
		if _, updated := viewsUpdated[viewID]; !updated {
			culledViews = append(culledViews, viewID)
		}
	}

	// Hide culled views - they're outside the visible viewport
	for _, viewID := range culledViews {
		r.SetViewVisible(viewID, false)
		// Clear geometry cache so view repositions correctly when scrolled back
		r.ClearGeometryCache(viewID)
	}

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

	// Send batch to native - this call blocks until native applies all geometries
	// or timeout occurs. The frameSeq allows native to skip stale batches.
	_, err := r.channel.Invoke("batchSetGeometry", map[string]any{
		"frameSeq":   frameSeq,
		"geometries": batch,
	})
	if err != nil {
		// Timeout or error - increment stat counter
		r.BatchTimeouts.Add(1)
	}
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
		// Clear geometry cache so next frame resends position.
		// This handles the race where geometry was sent before the native
		// view was added to the views map (Android's async host.post).
		if viewID, ok := toInt64(argsMap["viewId"]); ok {
			r.ClearGeometryCache(viewID)
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

// basePlatformView provides common implementation for platform views.
type basePlatformView struct {
	viewID   int64
	viewType string
	offset   rendering.Offset
	size     rendering.Size
	visible  bool
}

func (v *basePlatformView) ViewID() int64 {
	return v.viewID
}

func (v *basePlatformView) ViewType() string {
	return v.viewType
}

func (v *basePlatformView) SetSize(size rendering.Size) {
	v.size = size
	GetPlatformViewRegistry().UpdateViewGeometry(v.viewID, v.offset, v.size, nil)
}

func (v *basePlatformView) SetOffset(offset rendering.Offset) {
	v.offset = offset
	GetPlatformViewRegistry().UpdateViewGeometry(v.viewID, v.offset, v.size, nil)
}

// SetGeometry updates position, size, and clip bounds in a single call.
func (v *basePlatformView) SetGeometry(offset rendering.Offset, size rendering.Size, clipBounds *rendering.Rect) {
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
