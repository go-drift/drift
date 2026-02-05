package engine

import (
	"fmt"
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
		return
	}
	app.pendingFrameRequest.Store(true)
}

// NeedsFrame returns true if a new frame should be rendered.
// Call this before acquiring a drawable to skip unnecessary render cycles.
func NeedsFrame() bool {
	frameLock.Lock()
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
	defer frameLock.Unlock()

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
	} else {
		// Clear state when diagnostics disabled
		app.showLayoutBounds = false
		app.hudRenderObject = nil
	}
	if app.root != nil {
		app.root.MarkNeedsBuild()
	}
	if app.rootRender != nil {
		app.rootRender.MarkNeedsPaint()
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
	// Dispatch runs inside Paint() which already holds frameLock,
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

	// Error recovery state (atomic for safe access from HandlePointer without lock)
	capturedError      atomic.Pointer[errors.BoundaryError]
	errorScreenMounted bool // true once we've transitioned to the error screen

	// Diagnostics state
	diagnosticsConfig *DiagnosticsConfig
	frameTiming       *FrameTimingBuffer
	lastFrameStart    time.Time
	hudRenderObject   layout.RenderObject // Reference to HUD for targeted repaints
	showLayoutBounds  bool                // Debug overlay for widget bounds (independent of HUD)
}

func init() {
	// Default background color to black
	backgroundColor.Store(uint32(graphics.RGB(0, 0, 0)))
	// Register dispatch function for platform package
	platform.RegisterDispatch(Dispatch)
	// Register RestartApp for error widget
	widgets.RegisterRestartAppFn(RestartApp)
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

	// Quick exit if nothing needs updating and nothing deferred
	if !pipeline.NeedsSemantics() && !a.semanticsDeferred {
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
	} else if !a.semanticsDeferred {
		// Start deferring
		a.semanticsDeferred = true
		a.semanticsDeferredAt = time.Now()
	}
}

func (a *appRunner) Paint(canvas graphics.Canvas, size graphics.Size) (err error) {
	frameLock.Lock()
	defer frameLock.Unlock()

	// In debug mode, recover panics and show error screen
	// In prod mode, let panics crash the app (unless user adds ErrorBoundary)
	// Note: This defer is set up after Lock(), so it runs before Unlock()
	if core.DebugMode {
		defer func() {
			if r := recover(); r != nil {
				err := &errors.BoundaryError{
					Phase:      "frame",
					Recovered:  r,
					StackTrace: errors.CaptureStack(),
					Timestamp:  time.Now(),
				}
				a.capturedError.Store(err)
				errors.ReportBoundaryError(err)
				// Unmount broken tree
				if a.root != nil {
					a.root.Unmount()
					a.root = nil
				}
				a.rootRender = nil
				// Request new frame to render error screen
				a.pendingFrameRequest.Store(true)
			}
		}()
	}

	// If an error was captured (e.g., from HandlePointer) and we still have the OLD tree,
	// unmount it. HandlePointer can't safely unmount due to lock concerns, so we do it here.
	// Note: We only unmount if the tree still exists AND we have an error. Once we've
	// remounted with the error screen (root becomes non-nil again), we don't unmount again.
	// The errorScreenMounted flag tracks whether we've already transitioned to the error screen.
	if a.capturedError.Load() != nil && a.root != nil && !a.errorScreenMounted {
		a.root.Unmount()
		a.root = nil
		a.rootRender = nil
		a.errorScreenMounted = true
	}

	// Track frame timing for diagnostics
	now := time.Now()
	if a.frameTiming != nil && !a.lastFrameStart.IsZero() {
		a.frameTiming.Add(now.Sub(a.lastFrameStart))
		// Mark only the HUD for repaint (not the whole tree)
		if a.hudRenderObject != nil {
			a.hudRenderObject.MarkNeedsPaint()
		}
	}
	a.lastFrameStart = now

	scale := a.deviceScale
	logicalSize := graphics.Size{
		Width:  size.Width / scale,
		Height: size.Height / scale,
	}

	callbacks := a.drainDispatchQueue()
	for _, callback := range callbacks {
		callback()
	}
	if a.consumePendingFrameRequest() {
		a.requestFrameLocked()
	}

	widgets.StepBallistics()
	animation.StepTickers()
	a.updateFPS()
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
		// Initialize accessibility system on first frame
		initializeAccessibility()
	}

	a.buildOwner.FlushBuild()

	if a.rootRender != nil {
		pipeline := a.buildOwner.Pipeline()
		pipeline.FlushLayoutForRoot(a.rootRender, layout.Tight(logicalSize))

		// Flush semantics after layout, with deferral during animations
		a.flushSemanticsIfNeeded(pipeline, scale)

		// Begin collecting platform view geometry updates for synchronized batch apply
		platform.GetPlatformViewRegistry().BeginGeometryBatch()

		// Process dirty repaint boundaries
		showLayoutBounds := a.showLayoutBounds
		debugStrokeWidth := 1.0
		if showLayoutBounds {
			debugStrokeWidth = 1.0 / scale // Scale-independent 1px stroke
		}

		// Phase B: Record dirty layers
		dirtyBoundaries := pipeline.FlushPaint()
		recordDirtyLayers(dirtyBoundaries, showLayoutBounds, debugStrokeWidth)

		// Phase C: Composite layer tree via CompositingCanvas
		canvas.Clear(graphics.Color(backgroundColor.Load()))
		canvas.Save()
		canvas.Scale(scale, scale)
		compCanvas := NewCompositingCanvas(canvas, platform.GetPlatformViewRegistry())
		compositeLayerTree(compCanvas, a.rootRender)
		canvas.Restore()

		// Flush geometry batch - blocks until native applies all updates.
		// This ensures native views are positioned before the frame is displayed.
		platform.GetPlatformViewRegistry().FlushGeometryBatch()
	}
	return nil
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
	label := "FPS: " + itoa(int(fps+0.5))
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

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	neg := false
	if value < 0 {
		neg = true
		value = -value
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
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
