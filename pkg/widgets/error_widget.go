package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

func init() {
	// Register the default error widget builder
	core.SetErrorWidgetBuilder(func(err *errors.BoundaryError) core.Widget {
		return ErrorWidget{Error: err}
	})
}

// ErrorWidget displays inline error information when a widget fails.
// Unlike [DebugErrorScreen] which takes over the entire screen, ErrorWidget
// renders as a compact red box that can be embedded in layouts.
//
// It shows a red background with:
//   - "Something went wrong" message
//   - Detailed error text (in debug mode or when Verbose is true)
//   - Restart button to recover the app
//
// This is the default fallback widget used by [ErrorBoundary] when no
// FallbackBuilder is provided.
type ErrorWidget struct {
	core.StatelessBase

	// Error is the boundary error that occurred. If nil, shows "Unknown error".
	Error *errors.BoundaryError
	// Verbose overrides DebugMode for this widget instance.
	// When true, shows detailed error messages. When false, shows generic text.
	// If nil (default), uses core.DebugMode.
	Verbose *bool
}

func (e ErrorWidget) Build(ctx core.BuildContext) core.Widget {
	verbose := core.DebugMode
	if e.Verbose != nil {
		verbose = *e.Verbose
	}

	var errorText string
	if e.Error != nil {
		if verbose {
			errorText = e.Error.Error()
		} else {
			errorText = "An error occurred"
		}
	} else {
		errorText = "Unknown error"
	}

	children := []core.Widget{
		// Error indicator
		Text{
			Content: "!",
			Style: graphics.TextStyle{
				Color:      graphics.ColorWhite,
				FontSize:   24,
				FontWeight: graphics.FontWeightBold,
			},
		},
		SizedBox{Height: 8},
		// Error message
		Text{
			Content: "Something went wrong",
			Style: graphics.TextStyle{
				Color:      graphics.ColorWhite,
				FontSize:   16,
				FontWeight: graphics.FontWeightBold,
			},
		},
	}

	if verbose {
		children = append(children,
			SizedBox{Height: 8},
			Text{
				Content: errorText,
				Style: graphics.TextStyle{
					Color:    graphics.RGBA(255, 255, 255, 0.78),
					FontSize: 12,
				},
				MaxLines: 5,
			},
		)
	}

	// Add restart button
	children = append(children,
		SizedBox{Height: 16},
		errorRestartButton{},
	)

	return Container{
		Color:   graphics.RGBA(180, 0, 0, 1.0), // Dark red
		Padding: layout.EdgeInsetsAll(16),
		Child: Column{
			MainAxisAlignment:  MainAxisAlignmentCenter,
			CrossAxisAlignment: CrossAxisAlignmentCenter,
			MainAxisSize:       MainAxisSizeMin,
			Children:           children,
		},
	}
}

// errorRestartButton is a stateful widget for the restart button.
// It's separated to import the engine package lazily and avoid circular imports.
type errorRestartButton struct {
	core.StatefulBase
}

func (b errorRestartButton) CreateState() core.State {
	return &errorRestartButtonState{}
}

type errorRestartButtonState struct {
	core.StateBase
	restartFn func()
}

func (s *errorRestartButtonState) InitState() {
	// Import engine.RestartApp lazily through a registration mechanism
	s.restartFn = getRestartAppFn()
}

func (s *errorRestartButtonState) Build(ctx core.BuildContext) core.Widget {
	if s.restartFn == nil {
		// No restart function registered, show disabled button
		return Container{
			Color:   graphics.RGBA(100, 100, 100, 0.78),
			Padding: layout.EdgeInsetsSymmetric(16, 8),
			Child: Text{
				Content: "Restart unavailable",
				Style: graphics.TextStyle{
					Color:    graphics.RGBA(200, 200, 200, 1.0),
					FontSize: 14,
				},
			},
		}
	}

	return GestureDetector{
		OnTap: s.restartFn,
		Child: Container{
			Color:   graphics.RGBA(255, 255, 255, 0.86),
			Padding: layout.EdgeInsetsSymmetric(16, 8),
			Child: Text{
				Content: "Restart App",
				Style: graphics.TextStyle{
					Color:      graphics.ColorBlack,
					FontSize:   14,
					FontWeight: graphics.FontWeightNormal,
				},
			},
		},
	}
}

// restartAppFn holds the registered restart function
var restartAppFn func()

// RegisterRestartAppFn registers the function to restart the app.
// This is called by the engine package during initialization.
func RegisterRestartAppFn(fn func()) {
	restartAppFn = fn
}

func getRestartAppFn() func() {
	return restartAppFn
}
