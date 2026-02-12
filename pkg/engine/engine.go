package engine

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// backgroundColor uses atomic access to avoid deadlock when called from InitState/Build.
var backgroundColor atomic.Uint32

// platformScheduleFrameVal holds the platform schedule-frame callback.
// Stored as an atomic.Value for lock-free access from notifyPlatform().
var platformScheduleFrameVal atomic.Value // stores func()

// SetPlatformScheduleFrame registers a callback the engine invokes when a new
// frame is needed, enabling on-demand scheduling instead of continuous polling.
func SetPlatformScheduleFrame(fn func()) {
	if fn == nil {
		return
	}
	platformScheduleFrameVal.Store(fn)
}

// notifyPlatform calls the platform schedule-frame callback if one is registered.
func notifyPlatform() {
	if fn, ok := platformScheduleFrameVal.Load().(func()); ok && fn != nil {
		fn()
	}
}

// semanticsDeferralTimeout is the maximum time we defer semantics flushes during animations.
// After this timeout, we force a flush even if animations are still active to ensure
// screen readers receive updates for accessibility compliance.
const semanticsDeferralTimeout = 500 * time.Millisecond

// frameLock protects access to shared UI state across the engine package.
var frameLock sync.Mutex

var app = newAppRunner()

// SetDeviceScale updates the device pixel scale factor used for rendering and input.
func SetDeviceScale(scale float64) {
	app.SetDeviceScale(scale)
}

// RequestFrame marks the render tree as needing paint.
func RequestFrame() {
	if frameLock.TryLock() {
		defer frameLock.Unlock()
		app.requestFrameLocked()
		notifyPlatform()
		return
	}
	app.pendingFrameRequest.Store(true)
	notifyPlatform()
}

// NeedsFrame returns true if a new frame should be rendered.
// Call this before acquiring a drawable to skip unnecessary render cycles.
// Uses TryLock to avoid blocking the platform main thread when StepFrame or
// RenderFrame holds the lock. If the lock is held, a frame is actively being
// processed so we return true to keep the render loop alive.
func NeedsFrame() bool {
	if !frameLock.TryLock() {
		// The lock is held (typically by StepFrame/RenderFrame), so return
		// true rather than blocking the caller. At worst this schedules one
		// extra no-op frame; the alternative is stalling the platform main thread.
		return true
	}
	defer frameLock.Unlock()
	return app.needsFrameLocked()
}

func (a *appRunner) needsFrameLocked() bool {
	// Need frame for initial render (root not yet created)
	if a.root == nil {
		return true
	}
	// Need frame if there are pending dispatch callbacks
	a.dispatchMu.Lock()
	hasCallbacks := len(a.dispatchQueue) > 0
	a.dispatchMu.Unlock()
	if hasCallbacks {
		return true
	}
	// Need frame if explicitly requested
	if a.pendingFrameRequest.Load() {
		return true
	}
	// Need frame if animations are running
	if animation.HasActiveTickers() {
		return true
	}
	// Need frame if ballistics are active
	if widgets.HasActiveBallistics() {
		return true
	}
	// Need frame if build/layout/paint is needed
	if a.buildOwner != nil && a.buildOwner.NeedsWork() {
		return true
	}
	return false
}

// Dispatch schedules a callback to run on the UI thread
// during the next frame and is safe to call from any goroutine.
func Dispatch(callback func()) {
	app.dispatch(callback)
}

// SetApp configures the root widget for the application.
// The runtime calls this after the app has started.
func SetApp(root core.Widget) {
	app.setUserApp(root)
}

// SetBackgroundColor sets the color used to clear the canvas before each frame.
// This should match your app's background color so the status bar area (on iOS)
// or any gaps show the correct color. Safe to call from InitState or Build.
func SetBackgroundColor(color graphics.Color) {
	backgroundColor.Store(uint32(color))
}

// SetShowLayoutBounds enables or disables the layout bounds debug overlay.
func SetShowLayoutBounds(show bool) {
	frameLock.Lock()
	defer frameLock.Unlock()
	app.showLayoutBounds = show
	// Mark root for repaint to show/hide bounds
	if app.rootRender != nil {
		app.rootRender.MarkNeedsPaint()
	}
}

// SetDiagnostics configures the diagnostics overlays.
// Pass nil to disable all diagnostics.
func SetDiagnostics(config *DiagnosticsConfig) {
	// Determine port changes outside the lock to avoid blocking frame/paint
	frameLock.Lock()
	oldPort := 0
	if app.diagnosticsConfig != nil {
		oldPort = app.diagnosticsConfig.DebugServerPort
	}
	frameLock.Unlock()

	newPort := 0
	if config != nil {
		newPort = config.DebugServerPort
	}

	interval, window := runtimeSampleConfig(config)
	enableRuntimeSampling := config != nil && config.DebugServerPort > 0 && interval > 0 && window > 0

	// Start/stop debug server outside frameLock (shutdown can block up to 2s)
	if oldPort != newPort {
		stopDebugServer()
		if newPort > 0 {
			if _, err := startDebugServer(newPort); err != nil {
				fmt.Printf("debug server failed to start on port %d: %v\n", newPort, err)
			}
		}
	}

	// Now update diagnostics state under lock
	frameLock.Lock()

	app.diagnosticsConfig = config
	if config != nil {
		app.showLayoutBounds = config.ShowLayoutBounds
		if app.frameTiming == nil && (config.ShowFPS || config.ShowFrameGraph) {
			samples := config.GraphSamples
			if samples <= 0 {
				samples = 60
			}
			app.frameTiming = NewFrameTimingBuffer(samples)
		}

		app.frameTraceEnabled = config.DebugServerPort > 0
		if app.frameTraceEnabled {
			threshold := config.TargetFrameTime
			if threshold == 0 {
				threshold = defaultFrameTraceThreshold
			}
			if app.frameTrace == nil || app.frameTrace.Capacity() != frameTraceSamplesDefault {
				app.frameTrace = NewFrameTraceBuffer(frameTraceSamplesDefault, threshold)
			} else {
				app.frameTrace.SetThreshold(threshold)
			}
		} else {
			app.frameTrace = nil
		}

		if enableRuntimeSampling {
			app.runtimeSamples = NewRuntimeSampleBuffer(window, interval)
		} else {
			app.runtimeSamples = nil
		}
	} else {
		// Clear state when diagnostics disabled
		app.showLayoutBounds = false
		app.hudRenderObject = nil
		app.frameTraceEnabled = false
		app.frameTrace = nil
		app.runtimeSamples = nil
	}
	if app.root != nil {
		app.root.MarkNeedsBuild()
	}
	if app.rootRender != nil {
		app.rootRender.MarkNeedsPaint()
	}
	runtimeSamples := app.runtimeSamples
	frameLock.Unlock()

	// Start/stop runtime sampler outside frameLock to avoid contention.
	if enableRuntimeSampling {
		startRuntimeSampler(runtimeSamples, interval)
	} else {
		stopRuntimeSampler()
	}
}

// diagnosticsDataSource implements widgets.DiagnosticsHUDDataSource
type diagnosticsDataSource struct {
	runner *appRunner
}

func (d *diagnosticsDataSource) FPSLabel() string {
	return d.runner.fpsLabel
}

func (d *diagnosticsDataSource) SamplesInto(dst []time.Duration) int {
	if d.runner.frameTiming == nil {
		return 0
	}
	return d.runner.frameTiming.SamplesInto(dst)
}

func (d *diagnosticsDataSource) SampleCount() int {
	if d.runner.frameTiming == nil {
		return 0
	}
	return d.runner.frameTiming.Count()
}

func (d *diagnosticsDataSource) RegisterRenderObject(ro layout.RenderObject) {
	d.runner.hudRenderObject = ro
}

// RestartApp unmounts the entire widget tree and re-mounts from scratch.
// Use this for recovery from catastrophic errors. All state will be lost.
// This is safe to call from any goroutine.
func RestartApp() {
	// Dispatch runs inside StepFrame() which already holds frameLock,
	// so we don't need to acquire it here.
	Dispatch(func() {
		// Clear captured error and reset error screen state
		app.capturedError.Store(nil)
		app.errorScreenMounted = false

		// Unmount existing tree
		if app.root != nil {
			app.root.Unmount()
			app.root = nil
		}
		app.rootRender = nil

		// Next frame will re-mount the userApp
		app.pendingFrameRequest.Store(true)
	})
}

type appRunner struct {
	buildOwner          *core.BuildOwner
	root                core.Element
	rootRender          layout.RenderObject
	deviceScale         float64
	userApp             core.Widget
	pointerHandlers     map[int64][]layout.PointerHandler
	pointerPositions    map[int64]graphics.Offset
	lastFPSUpdate       time.Time
	fpsLabel            string
	dispatchMu          sync.Mutex
	dispatchQueue       []func()
	pendingFrameRequest atomic.Bool

	// Semantics deferral state for animation optimization
	semanticsDeferred   bool      // true if we skipped a semantics flush
	semanticsDeferredAt time.Time // when we first started deferring
	semanticsForceFlush bool      // true when accessibility was just enabled and needs initial build

	// Error recovery state (atomic for safe access from HandlePointer without lock)
	capturedError      atomic.Pointer[errors.BoundaryError]
	errorScreenMounted bool // true once we've transitioned to the error screen

	// Diagnostics state
	diagnosticsConfig     *DiagnosticsConfig
	frameTiming           *FrameTimingBuffer
	lastFrameStart        time.Time
	hudRenderObject       layout.RenderObject // Reference to HUD for targeted repaints
	showLayoutBounds      bool                // Debug overlay for widget bounds (independent of HUD)
	frameTrace            *FrameTraceBuffer
	frameTraceEnabled     bool
	lastLifecycleState    platform.LifecycleState
	runtimeSamples        *RuntimeSampleBuffer
	treeCountFrame        int
	cachedRenderNodeCount int
	cachedWidgetNodeCount int
}

func init() {
	// Default background color to black
	backgroundColor.Store(uint32(graphics.RGB(0, 0, 0)))
	// Register dispatch function for platform package
	platform.RegisterDispatch(Dispatch)
	// Register RestartApp for error widget
	widgets.RegisterRestartAppFn(RestartApp)
	// Wire up frame scheduling so SetState triggers a render under on-demand scheduling
	app.buildOwner.OnNeedsFrame = RequestFrame
}

func newAppRunner() *appRunner {
	return &appRunner{
		buildOwner:       core.NewBuildOwner(),
		deviceScale:      1,
		pointerHandlers:  make(map[int64][]layout.PointerHandler),
		pointerPositions: make(map[int64]graphics.Offset),
	}
}

func (a *appRunner) SetDeviceScale(scale float64) {
	if scale <= 0 {
		scale = 1
	}
	frameLock.Lock()
	defer frameLock.Unlock()
	if a.deviceScale == scale {
		return
	}
	a.deviceScale = scale
	if a.root != nil {
		a.root.MarkNeedsBuild()
	}
}

func (a *appRunner) setUserApp(root core.Widget) {
	frameLock.Lock()
	defer frameLock.Unlock()
	a.userApp = root
	if a.root != nil {
		a.root.MarkNeedsBuild()
	}
}

func (a *appRunner) dispatch(callback func()) {
	if callback == nil {
		return
	}
	a.dispatchMu.Lock()
	a.dispatchQueue = append(a.dispatchQueue, callback)
	a.dispatchMu.Unlock()
	RequestFrame()
}

func (a *appRunner) drainDispatchQueue() []func() {
	a.dispatchMu.Lock()
	callbacks := append([]func(){}, a.dispatchQueue...)
	a.dispatchQueue = nil
	a.dispatchMu.Unlock()
	return callbacks
}

func (a *appRunner) consumePendingFrameRequest() bool {
	return a.pendingFrameRequest.Swap(false)
}

func (a *appRunner) requestFrameLocked() {
	if a.rootRender != nil {
		a.rootRender.MarkNeedsPaint()
	}
}

// flushSemanticsIfNeeded defers semantics updates during animations to avoid
// expensive O(n) to O(n^2) rebuilds every frame. Screen readers don't benefit
// from 60 updates/second mid-animation, so we defer until animations settle
// or a 500ms timeout for accessibility compliance.
func (a *appRunner) flushSemanticsIfNeeded(pipeline *layout.PipelineOwner, scale float64) {
	animationActive := animation.HasActiveTickers() || widgets.HasActiveBallistics()

	// Quick exit if nothing needs updating, nothing deferred, and no forced flush
	if !pipeline.NeedsSemantics() && !a.semanticsDeferred && !a.semanticsForceFlush {
		return
	}

	// Determine if we should flush
	shouldFlush := !animationActive
	if !shouldFlush && a.semanticsDeferred {
		// Check timeout only when actively deferring during animation
		if time.Since(a.semanticsDeferredAt) >= semanticsDeferralTimeout {
			shouldFlush = true
		}
	}

	if shouldFlush {
		dirtySemantics := pipeline.FlushSemantics()
		flushSemanticsWithScale(a.rootRender, scale, dirtySemantics)
		a.semanticsDeferred = false
		a.semanticsDeferredAt = time.Time{}
		a.semanticsForceFlush = false
	} else if !a.semanticsDeferred {
		// Start deferring
		a.semanticsDeferred = true
		a.semanticsDeferredAt = time.Now()
	}
}

// runPipeline executes the shared engine pipeline phases: error handling, frame
// timing, dispatch, animate, root mounting, build, layout, semantics, geometry
// batch setup, and dirty layer recording. Must be called with frameLock held.
//
// If traceSample is non-nil, per-phase timing is recorded into it.
//
// Returns false if the render tree is not yet available.
func (a *appRunner) runPipeline(size graphics.Size, traceSample *FrameSample) bool {
	tracing := traceSample != nil

	// Handle captured errors (e.g. from HandlePointer)
	if a.capturedError.Load() != nil && a.root != nil && !a.errorScreenMounted {
		a.root.Unmount()
		a.root = nil
		a.rootRender = nil
		a.errorScreenMounted = true
	}

	// Frame timing
	frameStart := time.Now()
	frameInterval := time.Duration(0)
	if !a.lastFrameStart.IsZero() {
		frameInterval = frameStart.Sub(a.lastFrameStart)
	}
	if a.frameTiming != nil && frameInterval > 0 {
		a.frameTiming.Add(frameInterval)
		if a.hudRenderObject != nil {
			a.hudRenderObject.MarkNeedsPaint()
		}
	}
	a.lastFrameStart = frameStart

	scale := a.deviceScale
	logicalSize := graphics.Size{
		Width:  size.Width / scale,
		Height: size.Height / scale,
	}

	// Dispatch
	var phaseStart time.Time
	if tracing {
		phaseStart = time.Now()
	}
	callbacks := a.drainDispatchQueue()
	for _, callback := range callbacks {
		callback()
	}
	if a.consumePendingFrameRequest() {
		a.requestFrameLocked()
	}
	if tracing {
		traceSample.Phases.DispatchMs = durationToMillis(time.Since(phaseStart))
	}

	// Animate
	if tracing {
		phaseStart = time.Now()
	}
	widgets.StepBallistics()
	animation.StepTickers()
	if tracing {
		traceSample.Phases.AnimateMs = durationToMillis(time.Since(phaseStart))
	}
	a.updateFPS()

	// Mount root
	if a.root == nil {
		rootWidget := widgets.Root(engineApp{runner: a})
		a.root = core.MountRoot(rootWidget, a.buildOwner)
		if renderElement, ok := a.root.(interface{ RenderObject() layout.RenderObject }); ok {
			a.rootRender = renderElement.RenderObject()
		}
		if a.rootRender != nil {
			pipeline := a.buildOwner.Pipeline()
			pipeline.ScheduleLayout(a.rootRender)
			pipeline.SchedulePaint(a.rootRender)
		}
		initializeAccessibility()
	}

	// Build
	if tracing {
		phaseStart = time.Now()
	}
	a.buildOwner.FlushBuild()
	if tracing {
		traceSample.Phases.BuildMs = durationToMillis(time.Since(phaseStart))
	}

	if a.rootRender == nil {
		return false
	}

	pipeline := a.buildOwner.Pipeline()

	// Trace overhead (counts, dirty types)
	if tracing {
		traceOverheadStart := time.Now()
		traceSample.Counts.DirtyLayout = pipeline.DirtyLayoutCount()
		traceSample.Counts.DirtyPaintBoundaries = pipeline.DirtyPaintCount()
		traceSample.Counts.DirtySemantics = pipeline.DirtySemanticsCount()
		if a.treeCountFrame%10 == 0 {
			a.cachedRenderNodeCount = countRenderTree(a.rootRender)
			a.cachedWidgetNodeCount = countWidgetTree(a.root)
		}
		a.treeCountFrame++
		traceSample.Counts.RenderNodeCount = a.cachedRenderNodeCount
		traceSample.Counts.WidgetNodeCount = a.cachedWidgetNodeCount
		traceSample.Counts.PlatformViewCount = platform.GetPlatformViewRegistry().ViewCount()
		traceSample.DirtyTypes.Layout = pipeline.DirtyLayoutTypes(5)
		traceSample.DirtyTypes.Semantics = pipeline.DirtySemanticsTypes(5)
		traceSample.Phases.TraceOverheadMs = durationToMillis(time.Since(traceOverheadStart))
	}

	// Layout
	if tracing {
		phaseStart = time.Now()
	}
	pipeline.FlushLayoutForRoot(a.rootRender, layout.Tight(logicalSize))
	if tracing {
		traceSample.Phases.LayoutMs = durationToMillis(time.Since(phaseStart))
	}

	// Semantics
	if tracing {
		phaseStart = time.Now()
	}
	a.flushSemanticsIfNeeded(pipeline, scale)
	if tracing {
		traceSample.Phases.SemanticsMs = durationToMillis(time.Since(phaseStart))
	}

	// Record dirty layers
	showLayoutBounds := a.showLayoutBounds
	debugStrokeWidth := 1.0
	if showLayoutBounds {
		debugStrokeWidth = 1.0 / scale
	}
	if tracing {
		phaseStart = time.Now()
		traceSample.DirtyTypes.Paint = pipeline.DirtyPaintTypes(5)
	}
	dirtyBoundaries := pipeline.FlushPaint()
	recordDirtyLayers(dirtyBoundaries, showLayoutBounds, debugStrokeWidth)
	if tracing {
		traceSample.Phases.RecordMs = durationToMillis(time.Since(phaseStart))
		traceSample.Counts.DirtyPaintBoundaries = len(dirtyBoundaries)
	}

	return true
}

// recoverFromFramePanic returns a deferred function that catches panics during
// frame processing (StepFrame) and transitions to the error screen.
func (a *appRunner) recoverFromFramePanic() func() {
	return func() {
		if r := recover(); r != nil {
			err := &errors.BoundaryError{
				Phase:      "frame",
				Recovered:  r,
				StackTrace: errors.CaptureStack(),
				Timestamp:  time.Now(),
			}
			a.capturedError.Store(err)
			errors.ReportBoundaryError(err)
			if a.root != nil {
				a.root.Unmount()
				a.root = nil
			}
			a.rootRender = nil
			a.pendingFrameRequest.Store(true)
		}
	}
}

func (a *appRunner) HandlePointer(event PointerEvent) {
	// In debug mode, recover panics and show error screen
	// In prod mode, let panics crash the app (unless user adds ErrorBoundary)
	if core.DebugMode {
		defer func() {
			if r := recover(); r != nil {
				err := &errors.BoundaryError{
					Phase:      "pointer",
					Recovered:  r,
					StackTrace: errors.CaptureStack(),
					Timestamp:  time.Now(),
				}
				a.capturedError.Store(err)
				errors.ReportBoundaryError(err)
				a.pendingFrameRequest.Store(true)
			}
		}()
	}

	pointerID := event.PointerID
	var handlers []layout.PointerHandler
	delta := graphics.Offset{}

	frameLock.Lock()
	rootRender := a.rootRender
	if rootRender == nil {
		frameLock.Unlock()
		return
	}
	scale := a.deviceScale
	position := graphics.Offset{X: event.X / scale, Y: event.Y / scale}

	if event.Phase != PointerPhaseDown {
		if last, ok := a.pointerPositions[pointerID]; ok {
			delta = graphics.Offset{X: position.X - last.X, Y: position.Y - last.Y}
		}
	}
	a.pointerPositions[pointerID] = position

	if event.Phase == PointerPhaseDown {
		result := &layout.HitTestResult{}
		if rootRender.HitTest(position, result) && len(result.Entries) > 0 {
			handlers = collectPointerHandlers(result.Entries)
			if len(handlers) > 0 {
				a.pointerHandlers[pointerID] = handlers
			}

			// Auto-unfocus text inputs when tapping outside them
			if focusedTarget := platform.GetFocusedTarget(); focusedTarget != nil {
				if !containsEntry(result.Entries, focusedTarget) {
					platform.UnfocusAll()
				}
			}
		} else if platform.HasFocus() {
			// Tapped on empty space - unfocus
			platform.UnfocusAll()
		}
	} else {
		handlers = a.pointerHandlers[pointerID]
	}

	if event.Phase == PointerPhaseUp || event.Phase == PointerPhaseCancel {
		delete(a.pointerHandlers, pointerID)
		delete(a.pointerPositions, pointerID)
	}
	frameLock.Unlock()

	if len(handlers) == 0 {
		return
	}

	gestureEvent := gestures.PointerEvent{
		PointerID: pointerID,
		Position:  position,
		Delta:     delta,
		Phase:     convertPointerPhase(event.Phase),
	}

	for _, handler := range handlers {
		handler.HandlePointer(gestureEvent)
	}

	if event.Phase == PointerPhaseDown {
		gestures.DefaultArena.Close(pointerID)
	}
	if event.Phase == PointerPhaseUp || event.Phase == PointerPhaseCancel {
		gestures.DefaultArena.Sweep(pointerID)
	}
}

func (a *appRunner) updateFPS() {
	now := time.Now()
	if a.lastFPSUpdate.IsZero() {
		a.lastFPSUpdate = now
		a.fpsLabel = "FPS: --"
		return
	}
	// Calculate instant FPS from last frame duration
	elapsed := now.Sub(a.lastFPSUpdate)
	a.lastFPSUpdate = now
	if elapsed <= 0 {
		return
	}
	fps := float64(time.Second) / float64(elapsed)
	label := "FPS: " + strconv.Itoa(int(fps+0.5))
	if label != a.fpsLabel {
		a.fpsLabel = label
	}
}

func convertPointerPhase(phase PointerPhase) gestures.PointerPhase {
	switch phase {
	case PointerPhaseDown:
		return gestures.PointerPhaseDown
	case PointerPhaseMove:
		return gestures.PointerPhaseMove
	case PointerPhaseUp:
		return gestures.PointerPhaseUp
	case PointerPhaseCancel:
		return gestures.PointerPhaseCancel
	default:
		return gestures.PointerPhaseCancel
	}
}

func collectPointerHandlers(entries []layout.RenderObject) []layout.PointerHandler {
	handlers := make([]layout.PointerHandler, 0, len(entries))
	seen := make(map[layout.PointerHandler]struct{})
	for _, entry := range entries {
		if handler, ok := entry.(layout.PointerHandler); ok {
			if _, exists := seen[handler]; exists {
				continue
			}
			seen[handler] = struct{}{}
			handlers = append(handlers, handler)
		}
	}
	return handlers
}

func containsEntry(entries []layout.RenderObject, target any) bool {
	for _, entry := range entries {
		if entry == target {
			return true
		}
	}
	return false
}

type engineApp struct {
	runner *appRunner
}

func (e engineApp) CreateElement() core.Element {
	return core.NewStatelessElement(e, nil)
}

func (e engineApp) Key() any {
	return nil
}

func (e engineApp) Build(ctx core.BuildContext) core.Widget {
	scale := 1.0
	var child core.Widget
	var diagnosticsConfig *DiagnosticsConfig
	if e.runner != nil {
		scale = e.runner.deviceScale
		diagnosticsConfig = e.runner.diagnosticsConfig

		// If we have a captured error (debug mode only), show error screen
		if err := e.runner.capturedError.Load(); err != nil {
			child = widgets.DebugErrorScreen{Error: err}
		} else {
			child = e.runner.userApp
		}
	}
	if child == nil {
		child = defaultPlaceholder{}
	}

	// Wrap with diagnostics HUD if FPS or frame graph is enabled
	if diagnosticsConfig != nil && (diagnosticsConfig.ShowFPS || diagnosticsConfig.ShowFrameGraph) {
		targetTime := diagnosticsConfig.TargetFrameTime
		if targetTime == 0 {
			targetTime = 16667 * time.Microsecond
		}

		graphWidth := 120.0
		graphHeight := 60.0

		// Create data source that pulls directly from runner
		dataSource := &diagnosticsDataSource{runner: e.runner}

		hud := widgets.DiagnosticsHUD{
			DataSource:     dataSource,
			TargetTime:     targetTime,
			GraphWidth:     graphWidth,
			GraphHeight:    graphHeight,
			ShowFPS:        diagnosticsConfig.ShowFPS,
			ShowFrameGraph: diagnosticsConfig.ShowFrameGraph,
		}

		// Wrap HUD in a positioner that reads safe area from context
		hudPositioner := diagnosticsHUDPositioner{
			position: diagnosticsConfig.Position,
			hud:      hud,
		}

		child = widgets.Stack{
			Children: []core.Widget{
				child,
				hudPositioner,
			},
		}
	}

	return widgets.DeviceScale{
		Scale: scale,
		Child: widgets.SafeAreaProvider{
			Child: child,
		},
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}

// diagnosticsHUDPositioner reads safe area from context and positions the HUD accordingly.
type diagnosticsHUDPositioner struct {
	position DiagnosticsPosition
	hud      core.Widget
}

func (d diagnosticsHUDPositioner) CreateElement() core.Element {
	return core.NewStatelessElement(d, nil)
}

func (d diagnosticsHUDPositioner) Key() any {
	return nil
}

func (d diagnosticsHUDPositioner) Build(ctx core.BuildContext) core.Widget {
	insets := widgets.SafeAreaOf(ctx)
	padding := 8.0

	switch d.position {
	case DiagnosticsTopRight:
		return widgets.Positioned{
			Right: ptrFloat64(insets.Right + padding),
			Top:   ptrFloat64(insets.Top + padding),
			Child: d.hud,
		}
	case DiagnosticsBottomLeft:
		return widgets.Positioned{
			Left:   ptrFloat64(insets.Left + padding),
			Bottom: ptrFloat64(insets.Bottom + padding),
			Child:  d.hud,
		}
	case DiagnosticsBottomRight:
		return widgets.Positioned{
			Right:  ptrFloat64(insets.Right + padding),
			Bottom: ptrFloat64(insets.Bottom + padding),
			Child:  d.hud,
		}
	default: // DiagnosticsTopLeft or invalid
		return widgets.Positioned{
			Left:  ptrFloat64(insets.Left + padding),
			Top:   ptrFloat64(insets.Top + padding),
			Child: d.hud,
		}
	}
}

// defaultPlaceholder is shown when no app is registered via SetApp.
type defaultPlaceholder struct{}

func (d defaultPlaceholder) CreateElement() core.Element {
	return core.NewStatelessElement(d, nil)
}

func (d defaultPlaceholder) Key() any {
	return nil
}

func (d defaultPlaceholder) Build(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)
	return theme.Theme{
		Data: theme.DefaultDarkTheme(),
		Child: widgets.Expanded{
			Child: widgets.Container{
				Color: colors.Background,
				Child: widgets.Centered(
					widgets.ColumnOf(
						widgets.MainAxisAlignmentCenter,
						widgets.CrossAxisAlignmentStart,
						widgets.MainAxisSizeMin,
						widgets.Text{Content: "Drift", Style: graphics.TextStyle{
							Color:      colors.Primary,
							FontSize:   48,
							FontWeight: graphics.FontWeightBold,
						}},
						widgets.VSpace(16),
						widgets.Text{Content: "No app registered", Style: textTheme.BodyLarge},
						widgets.VSpace(8),
						widgets.Text{Content: "Call drift.NewApp(...).Run() to set your root widget", Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						}},
					),
				),
			},
		},
	}
}

// recordLayerContent records a boundary's content into its layer.
func recordLayerContent(boundary layout.RenderObject, showLayoutBounds bool, strokeWidth float64) {
	layerGetter, ok := boundary.(interface{ EnsureLayer() *graphics.Layer })
	if !ok {
		return
	}
	layer := layerGetter.EnsureLayer()

	if !layer.Dirty {
		return
	}

	size := boundary.Size()
	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(size)

	ctx := &layout.PaintContext{
		Canvas:           recordCanvas,
		ShowLayoutBounds: showLayoutBounds,
		DebugStrokeWidth: strokeWidth,
		RecordingLayer:   layer,
	}
	boundary.Paint(ctx)

	layer.SetContent(recorder.EndRecording())
	layer.Size = size

	if clearer, ok := boundary.(interface{ ClearNeedsPaint() }); ok {
		clearer.ClearNeedsPaint()
	}
}

// recordDirtyLayers records content for dirty boundaries.
// Processes boundaries in reverse depth order (children before parents)
// so DrawChildLayer ops reference valid content.
func recordDirtyLayers(dirtyBoundaries []layout.RenderObject, showLayoutBounds bool, strokeWidth float64) {
	if len(dirtyBoundaries) == 0 {
		return
	}

	// dirtyBoundaries is sorted by depth (parents first from FlushPaint).
	// We need children recorded before parents, so process in reverse order.
	for i := len(dirtyBoundaries) - 1; i >= 0; i-- {
		recordDirtyLayersDFS(dirtyBoundaries[i], showLayoutBounds, strokeWidth, true)
	}
}

// recordDirtyLayersDFS traverses a subtree depth-first, recording dirty layers.
// Children are visited before their parent's layer is recorded.
// Stops at child boundaries (they are in dirtyBoundaries and processed independently).
func recordDirtyLayersDFS(node layout.RenderObject, showLayoutBounds bool, strokeWidth float64, isRoot bool) {
	var isBoundary bool
	var needsPaint bool
	if bn, ok := node.(layout.RepaintBoundaryNode); ok && bn.IsRepaintBoundary() {
		isBoundary = true
		needsPaint = bn.NeedsPaint()
	}

	// Stop at child boundaries (not the root of this DFS)
	if isBoundary && !isRoot {
		return
	}

	// Recurse into children first (DFS post-order)
	if visitor, ok := node.(layout.ChildVisitor); ok {
		visitor.VisitChildren(func(child layout.RenderObject) {
			recordDirtyLayersDFS(child, showLayoutBounds, strokeWidth, false)
		})
	}

	// Then record this boundary if dirty
	if isBoundary && needsPaint {
		recordLayerContent(node, showLayoutBounds, strokeWidth)
	}
}

// compositeLayerTree draws the layer tree starting from root.
// Child layers are drawn via DrawChildLayer ops recorded in each layer.
func compositeLayerTree(canvas graphics.Canvas, root layout.RenderObject) {
	layerGetter, ok := root.(interface{ EnsureLayer() *graphics.Layer })
	if !ok {
		panic("drift: root render object must implement EnsureLayer() - ensure root is a repaint boundary")
	}

	layer := layerGetter.EnsureLayer()
	if layer.Content == nil {
		panic("drift: root layer has no recorded content - recording phase failed to run or was skipped")
	}

	layer.Composite(canvas)
}

// StepFrame runs the engine pipeline (dispatch, animate, build, layout,
// semantics, record dirty layers) and composites through a geometry-only canvas
// to extract platform view positions. Returns a FrameSnapshot with the geometry.
//
// After StepFrame, call RenderFrame to composite into the actual GPU canvas.
// This split allows the Android UI thread to position platform views synchronously
// between StepFrame and RenderFrame, eliminating visual lag.
func (a *appRunner) StepFrame(size graphics.Size) (*FrameSnapshot, error) {
	frameLock.Lock()
	defer frameLock.Unlock()

	if core.DebugMode {
		defer a.recoverFromFramePanic()()
	}

	traceEnabled := a.frameTraceEnabled && a.frameTrace != nil
	var traceSample FrameSample
	var frameWorkStart time.Time
	if traceEnabled {
		frameWorkStart = time.Now()
		traceSample.Timestamp = frameWorkStart.UnixMilli()
		currentState := platform.Lifecycle.State()
		traceSample.Flags.LifecycleState = string(currentState)
		traceSample.Flags.ResumedThisFrame = a.lastLifecycleState != platform.LifecycleStateResumed && currentState == platform.LifecycleStateResumed
		a.lastLifecycleState = currentState
	}

	var ts *FrameSample
	if traceEnabled {
		ts = &traceSample
	}

	hasRenderTree := a.runPipeline(size, ts)

	snapshot := &FrameSnapshot{
		FrameID: frameCounter.Add(1),
	}

	if hasRenderTree {
		reg := platform.GetPlatformViewRegistry()

		// Begin/Flush geometry batch brackets the compositing pass.
		// Both calls live in StepFrame so the batch is always paired,
		// even when runPipeline returns nil on an earlier frame.
		reg.BeginGeometryBatch()

		// Composite through geometry canvas to extract platform view positions.
		// GeometryCanvas feeds into the registry's batch system via PlatformViewSink.
		// Geometry is in logical coordinates; the consumer (Android UI thread)
		// applies device density scaling.
		var compositeStart time.Time
		if traceEnabled {
			compositeStart = time.Now()
		}
		geoCanvas := NewGeometryCanvas(size, reg)
		compositeLayerTree(geoCanvas, a.rootRender)
		if traceEnabled {
			traceSample.Phases.GeometryMs = durationToMillis(time.Since(compositeStart))
		}

		// FlushGeometryBatch collects both visible views (from compositing
		// above) and hidden views (unseen, with empty clips).
		reg.FlushGeometryBatch()
		captured := reg.TakeCapturedSnapshot()

		for _, cv := range captured {
			snapshot.Views = append(snapshot.Views, viewSnapshotFromCapture(cv))
		}
	}

	if traceEnabled {
		traceSample.Flags.SemanticsDeferred = a.semanticsDeferred
		frameWorkDuration := time.Since(frameWorkStart)
		traceSample.FrameMs = durationToMillis(frameWorkDuration)
		a.frameTrace.Add(traceSample, frameWorkDuration)
	}

	return snapshot, nil
}

// RenderFrame composites the layer tree into the provided canvas.
// Must be called after a successful StepFrame.
func (a *appRunner) RenderFrame(canvas graphics.Canvas) error {
	frameLock.Lock()
	defer frameLock.Unlock()

	if a.rootRender == nil {
		return fmt.Errorf("RenderFrame called before root render tree is available")
	}

	scale := a.deviceScale

	canvas.Clear(graphics.Color(backgroundColor.Load()))
	canvas.Save()
	canvas.Scale(scale, scale)

	// Geometry was already captured in StepFrame; composite directly.
	compositeLayerTree(canvas, a.rootRender)

	canvas.Restore()
	return nil
}
