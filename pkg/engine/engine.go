// Package engine provides the core rendering logic for the Drift framework.
// It renders a widget tree into backend-specific surfaces consumed by native embedders.
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
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// backgroundColor uses atomic access to avoid deadlock when called from InitState/Build.
var backgroundColor atomic.Uint32

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
func SetBackgroundColor(color rendering.Color) {
	backgroundColor.Store(uint32(color))
}

type appRunner struct {
	buildOwner          *core.BuildOwner
	root                core.Element
	rootRender          layout.RenderObject
	deviceScale         float64
	safeAreaInsets      layout.EdgeInsets
	userApp             core.Widget
	pointerHandlers     map[int64][]layout.PointerHandler
	pointerPositions    map[int64]rendering.Offset
	frameCount          int
	lastFPSUpdate       time.Time
	fpsLabel            string
	dispatchMu          sync.Mutex
	dispatchQueue       []func()
	pendingFrameRequest atomic.Bool
}

func init() {
	// Default background color to black
	backgroundColor.Store(uint32(rendering.RGB(0, 0, 0)))
}

func newAppRunner() *appRunner {
	a := &appRunner{
		buildOwner:       core.NewBuildOwner(),
		deviceScale:      1,
		pointerHandlers:  make(map[int64][]layout.PointerHandler),
		pointerPositions: make(map[int64]rendering.Offset),
	}
	a.safeAreaInsets = toLayoutEdgeInsets(platform.SafeArea.Insets())
	platform.SafeArea.AddHandler(func(insets platform.EdgeInsets) {
		a.setSafeAreaInsets(toLayoutEdgeInsets(insets))
	})
	return a
}

func toLayoutEdgeInsets(insets platform.EdgeInsets) layout.EdgeInsets {
	return layout.EdgeInsets{
		Top:    insets.Top,
		Bottom: insets.Bottom,
		Left:   insets.Left,
		Right:  insets.Right,
	}
}

func (a *appRunner) setSafeAreaInsets(insets layout.EdgeInsets) {
	frameLock.Lock()
	defer frameLock.Unlock()
	if a.safeAreaInsets == insets {
		return
	}
	a.safeAreaInsets = insets
	if a.root != nil {
		a.root.MarkNeedsBuild()
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

func (a *appRunner) Paint(canvas rendering.Canvas, size rendering.Size) (err error) {
	defer func() {
		if r := recover(); r != nil {
			panicErr := &errors.PanicError{
				Value:      r,
				StackTrace: errors.CaptureStack(),
				Timestamp:  time.Now(),
			}
			errors.ReportPanic(panicErr)
			err = fmt.Errorf("panic during paint: %v", r)
		}
	}()

	frameLock.Lock()
	defer frameLock.Unlock()

	scale := a.deviceScale
	logicalSize := rendering.Size{
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
	}

	a.buildOwner.FlushBuild()

	if a.rootRender != nil {
		pipeline := a.buildOwner.Pipeline()
		pipeline.FlushLayoutForRoot(a.rootRender, layout.Tight(logicalSize))

		// Always clear and paint - with triple buffering, each drawable may contain
		// stale content from previous frames. Skipping paint causes flicker.
		canvas.Clear(rendering.Color(backgroundColor.Load()))
		canvas.Save()
		canvas.Scale(scale, scale)
		a.rootRender.Paint(&layout.PaintContext{Canvas: canvas})
		canvas.Restore()
		pipeline.FlushPaint()
	}
	return nil
}

func (a *appRunner) HandlePointer(event PointerEvent) {
	defer errors.Recover("engine.HandlePointer")

	pointerID := event.PointerID
	var handlers []layout.PointerHandler
	delta := rendering.Offset{}

	frameLock.Lock()
	rootRender := a.rootRender
	if rootRender == nil {
		frameLock.Unlock()
		return
	}
	scale := a.deviceScale
	position := rendering.Offset{X: event.X / scale, Y: event.Y / scale}

	if event.Phase != PointerPhaseDown {
		if last, ok := a.pointerPositions[pointerID]; ok {
			delta = rendering.Offset{X: position.X - last.X, Y: position.Y - last.Y}
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
		a.frameCount = 0
		a.fpsLabel = "FPS: --"
		return
	}
	a.frameCount++
	elapsed := now.Sub(a.lastFPSUpdate)
	if elapsed < 500*time.Millisecond {
		return
	}
	fps := float64(a.frameCount) / elapsed.Seconds()
	a.lastFPSUpdate = now
	a.frameCount = 0
	label := "FPS: " + itoa(int(fps+0.5))
	if label != a.fpsLabel {
		a.fpsLabel = label
		if a.root != nil {
			a.root.MarkNeedsBuild()
		}
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
	safeAreaInsets := layout.EdgeInsets{}
	var child core.Widget
	if e.runner != nil {
		scale = e.runner.deviceScale
		safeAreaInsets = e.runner.safeAreaInsets
		child = e.runner.userApp
	}
	if child == nil {
		child = defaultPlaceholder{}
	}
	return widgets.DeviceScale{
		Scale: scale,
		ChildWidget: widgets.SafeAreaData{
			Insets:      safeAreaInsets,
			ChildWidget: child,
		},
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
		ChildWidget: widgets.Expanded{
			ChildWidget: widgets.NewContainer(
				widgets.Centered(
					widgets.ColumnOf(
						widgets.MainAxisAlignmentCenter,
						widgets.CrossAxisAlignmentStart,
						widgets.MainAxisSizeMin,
						widgets.TextOf("Drift", rendering.TextStyle{
							Color:      colors.Primary,
							FontSize:   48,
							FontWeight: rendering.FontWeightBold,
						}),
						widgets.VSpace(16),
						widgets.TextOf("No app registered", textTheme.BodyLarge),
						widgets.VSpace(8),
						widgets.TextOf("Call drift.NewApp(...).Run() to set your root widget", rendering.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						}),
					),
				),
			).WithColor(colors.Background).Build(),
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
