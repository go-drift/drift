package drift_test

import (
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// This example shows how to create and configure a Drift application.
func ExampleNewApp() {
	// Create the root widget for the application
	root := widgets.Center{
		ChildWidget: widgets.Text{Content: "Hello, Drift!"},
	}

	// Create an app with default settings
	app := drift.NewApp(root)
	_ = app
}

// This example shows how to create an app with a custom theme.
func ExampleApp_withTheme() {
	root := widgets.Center{
		ChildWidget: widgets.Text{Content: "Dark Mode App"},
	}

	app := drift.App{
		Root:        root,
		Theme:       theme.DefaultDarkTheme(),
		DeviceScale: 2.0, // For high-DPI displays
	}
	_ = app
}

// This example shows how to dispatch work to the UI thread from a background goroutine.
// Use Dispatch when you need to update UI state from async operations like network calls.
func ExampleDispatch() {
	// Simulating an async operation that needs to update UI
	go func() {
		// ... do some work in the background ...

		// Schedule UI update on the main thread
		drift.Dispatch(func() {
			// This code runs on the UI thread and can safely update state
			// state.SetState(func() {
			//     state.data = result
			// })
		})
	}()
}
