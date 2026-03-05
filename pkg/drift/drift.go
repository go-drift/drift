// Package drift provides the main entry point for Drift applications.
package drift

import (
	"context"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/engine"
	"github.com/go-drift/drift/pkg/theme"
)

// App defines the configuration for a Drift application.
type App struct {
	// Root is the root widget of the application.
	Root core.Widget
	// Theme is the application theme. Defaults to DefaultLightTheme if nil.
	Theme *theme.ThemeData
	// DeviceScale is the device pixel ratio. Defaults to 1.0 if zero.
	DeviceScale float64
	// Diagnostics enables the performance diagnostics HUD overlay.
	// Use engine.DefaultDiagnosticsConfig() for sensible defaults.
	Diagnostics *engine.DiagnosticsConfig
	// OnInit is called once in a background goroutine before the root widget
	// is mounted. Use it for one-time setup such as opening a database,
	// loading configuration, or restoring authentication state.
	//
	// While OnInit executes, the engine defers root mounting so the
	// platform's native splash screen stays visible. When OnInit returns
	// nil, the root widget mounts on the next frame. If OnInit returns a
	// non-nil error, a [widgets.DebugErrorScreen] is shown instead.
	// Tapping "Restart" on the error screen mounts the root widget
	// directly; OnInit is not retried.
	//
	// The provided context is cancelled when the app is disposed (see
	// OnDispose). Long-running work inside OnInit should select on
	// ctx.Done() to exit promptly during teardown.
	OnInit func(ctx context.Context) error

	// OnDispose is called once when the app lifecycle reaches
	// [platform.LifecycleStateDetached]. Use it to release resources
	// acquired in OnInit, such as closing database connections or
	// flushing caches.
	//
	// The context passed to OnInit is cancelled before OnDispose runs,
	// so any in-flight OnInit work observes cancellation first.
	// OnDispose is guaranteed to run at most once, even if the
	// lifecycle transitions to Detached multiple times.
	OnDispose func()
}

// NewApp creates a default App with the given root widget.
func NewApp(root core.Widget) App {
	return App{Root: root}
}

// Run starts the app using the package-level runtime.
func (app App) Run() {
	Run(app)
}

// Run initializes the Drift engine with the given App configuration.
func Run(app App) {
	if app.DeviceScale <= 0 {
		app.DeviceScale = 1.0
	}
	if app.Theme == nil {
		app.Theme = theme.DefaultLightTheme()
	}
	if app.OnInit != nil {
		engine.SetOnInit(app.OnInit)
	}
	if app.OnDispose != nil {
		engine.SetOnDispose(app.OnDispose)
	}
	if app.Diagnostics != nil {
		engine.SetDiagnostics(app.Diagnostics)
	}
	if app.Root != nil {
		// Wrap the root widget with the theme
		themedRoot := theme.Theme{
			Data:  app.Theme,
			Child: app.Root,
		}
		engine.SetApp(themedRoot)
	}
	if app.DeviceScale != 1.0 {
		engine.SetDeviceScale(app.DeviceScale)
	}
}

// Dispatch schedules a callback to run on the UI thread
// during the next frame and is safe to call from any goroutine.
func Dispatch(callback func()) {
	engine.Dispatch(callback)
}
