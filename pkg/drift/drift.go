// Package drift provides the main entry point for Drift applications.
package drift

import (
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
	if app.Diagnostics != nil {
		engine.SetDiagnostics(app.Diagnostics)
	}
	if app.Root != nil {
		// Wrap the root widget with the theme
		themedRoot := theme.Theme{
			Data:        app.Theme,
			ChildWidget: app.Root,
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
