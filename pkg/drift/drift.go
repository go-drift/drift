// Package drift provides the main entry point for Drift applications.
package drift

import (
	"sync"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/engine"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// App defines the configuration for a Drift application.
type App struct {
	// Root is the root widget of the application.
	Root core.Widget
	// Theme is the application theme. Defaults to DefaultLightTheme if nil.
	Theme *theme.ThemeData
	// DeviceScale is the device pixel ratio. Defaults to 1.0 if zero.
	DeviceScale float64
}

// Engine is the core runtime for a Drift application.
// It manages the widget tree, layout, and rendering.
type Engine struct {
	buildOwner       *core.BuildOwner
	root             core.Element
	rootRender       layout.RenderObject
	deviceScale      float64
	app              App
	pointerHandlers  map[int64][]layout.PointerHandler
	pointerPositions map[int64]rendering.Offset
	mu               sync.Mutex
}

// NewApp creates a default App with the given root widget.
func NewApp(root core.Widget) App {
	return App{Root: root}
}

// Run starts the app using the package-level runtime.
func (app App) Run() *Engine {
	return Run(app)
}

// Run creates and initializes a new Engine with the given App configuration.
func Run(app App) *Engine {
	if app.DeviceScale <= 0 {
		app.DeviceScale = 1.0
	}
	if app.Theme == nil {
		app.Theme = theme.DefaultLightTheme()
	}
	if app.Root != nil {
		engine.SetApp(app.Root)
	}
	return &Engine{
		buildOwner:       core.NewBuildOwner(),
		deviceScale:      app.DeviceScale,
		app:              app,
		pointerHandlers:  make(map[int64][]layout.PointerHandler),
		pointerPositions: make(map[int64]rendering.Offset),
	}
}

// Dispatch schedules a callback to run on the UI thread
// during the next frame and is safe to call from any goroutine.
func Dispatch(callback func()) {
	engine.Dispatch(callback)
}

// SetDeviceScale updates the device pixel ratio.
func (e *Engine) SetDeviceScale(scale float64) {
	if scale <= 0 {
		scale = 1.0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.deviceScale == scale {
		return
	}
	e.deviceScale = scale
	if e.root != nil {
		e.root.MarkNeedsBuild()
	}
}

// Paint renders the application to the given canvas.
// This should be called by the platform embedder on each frame.
func (e *Engine) Paint(canvas rendering.Canvas, size rendering.Size) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	scale := e.deviceScale
	logicalSize := rendering.Size{
		Width:  size.Width / scale,
		Height: size.Height / scale,
	}

	widgets.StepBallistics()
	animation.StepTickers()

	if e.root == nil {
		rootWidget := widgets.Root(appWidget{engine: e})
		e.root = core.MountRoot(rootWidget, e.buildOwner)
		if renderElement, ok := e.root.(interface{ RenderObject() layout.RenderObject }); ok {
			e.rootRender = renderElement.RenderObject()
		}
		if e.rootRender != nil {
			pipeline := e.buildOwner.Pipeline()
			pipeline.ScheduleLayout(e.rootRender)
			pipeline.SchedulePaint(e.rootRender)
		}
	}

	e.buildOwner.FlushBuild()

	if e.rootRender != nil {
		pipeline := e.buildOwner.Pipeline()
		pipeline.FlushLayoutForRoot(e.rootRender, layout.Tight(logicalSize))

		needsPaint := pipeline.NeedsPaint() || animation.HasActiveTickers()
		if needsPaint {
			canvas.Clear(rendering.RGB(18, 18, 22))
			canvas.Save()
			canvas.Scale(scale, scale)
			e.rootRender.Paint(&layout.PaintContext{Canvas: canvas})
			canvas.Restore()
			pipeline.FlushPaint()
		}
	}
	return nil
}

// PointerPhase represents the phase of a pointer event.
type PointerPhase int

const (
	PointerDown PointerPhase = iota
	PointerMove
	PointerUp
	PointerCancel
)

// PointerEvent represents a touch or mouse event.
type PointerEvent struct {
	Phase    PointerPhase
	X, Y     float64
	Pressure float64
}

// HandlePointer processes a pointer (touch/mouse) event.
// This should be called by the platform embedder for input events.
func (e *Engine) HandlePointer(event PointerEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	scale := e.deviceScale
	logicalX := event.X / scale
	logicalY := event.Y / scale
	position := rendering.Offset{X: logicalX, Y: logicalY}

	gesturePhase := toGesturePhase(event.Phase)
	gestureEvent := gestures.PointerEvent{
		Position: position,
		Phase:    gesturePhase,
	}

	pointerID := int64(0)
	var handlers []layout.PointerHandler

	switch event.Phase {
	case PointerDown:
		if e.rootRender != nil {
			result := &layout.HitTestResult{}
			e.rootRender.HitTest(position, result)
			widgets.HandleDropdownPointerDown(result.Entries)
			handlers = collectHandlers(result.Entries)
			e.pointerHandlers[pointerID] = handlers
		}
		e.pointerPositions[pointerID] = position
	case PointerMove, PointerUp, PointerCancel:
		handlers = e.pointerHandlers[pointerID]
		if event.Phase == PointerUp || event.Phase == PointerCancel {
			delete(e.pointerHandlers, pointerID)
			delete(e.pointerPositions, pointerID)
		} else {
			e.pointerPositions[pointerID] = position
		}
	}

	for _, handler := range handlers {
		handler.HandlePointer(gestureEvent)
	}
}

func toGesturePhase(phase PointerPhase) gestures.PointerPhase {
	switch phase {
	case PointerDown:
		return gestures.PointerPhaseDown
	case PointerMove:
		return gestures.PointerPhaseMove
	case PointerUp:
		return gestures.PointerPhaseUp
	case PointerCancel:
		return gestures.PointerPhaseCancel
	default:
		return gestures.PointerPhaseCancel
	}
}

func collectHandlers(entries []layout.RenderObject) []layout.PointerHandler {
	var handlers []layout.PointerHandler
	seen := make(map[layout.PointerHandler]struct{})
	for _, entry := range entries {
		if handler, ok := entry.(layout.PointerHandler); ok {
			if _, exists := seen[handler]; !exists {
				seen[handler] = struct{}{}
				handlers = append(handlers, handler)
			}
		}
	}
	return handlers
}

// appWidget wraps the user's Root widget with DeviceScale and Theme.
type appWidget struct {
	engine *Engine
}

func (a appWidget) CreateElement() core.Element {
	return core.NewStatelessElement(a, nil)
}

func (a appWidget) Key() any {
	return nil
}

func (a appWidget) Build(ctx core.BuildContext) core.Widget {
	root := a.engine.app.Root
	if root == nil {
		return nil
	}

	// Wrap with theme
	themed := theme.Theme{
		Data:        a.engine.app.Theme,
		ChildWidget: root,
	}

	// Wrap with device scale
	return widgets.DeviceScale{
		Scale:       a.engine.deviceScale,
		ChildWidget: themed,
	}
}
