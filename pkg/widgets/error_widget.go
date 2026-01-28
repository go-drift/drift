package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

func init() {
	// Register the default error widget builder
	core.SetErrorWidgetBuilder(func(err *errors.BuildError) core.Widget {
		return ErrorWidget{Error: err}
	})
}

// ErrorWidget displays error information when a widget build fails.
// It shows a red background with error details in debug mode,
// or a minimal error indicator in release mode.
type ErrorWidget struct {
	// Error is the build error that occurred.
	Error *errors.BuildError
	// Verbose overrides DebugMode for this widget instance.
	// If not explicitly set, defaults to core.DebugMode.
	Verbose *bool
}

func (e ErrorWidget) CreateElement() core.Element {
	return core.NewStatelessElement(e, nil)
}

func (e ErrorWidget) Key() any {
	return nil
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
			Wrap: true,
		},
	}

	if verbose {
		children = append(children,
			SizedBox{Height: 8},
			Text{
				Content: errorText,
				Style: graphics.TextStyle{
					Color:    graphics.RGBA(255, 255, 255, 200),
					FontSize: 12,
				},
				Wrap:     true,
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
		Color:   graphics.RGBA(180, 0, 0, 255), // Dark red
		Padding: layout.EdgeInsetsAll(16),
		ChildWidget: Column{
			MainAxisAlignment:  MainAxisAlignmentCenter,
			CrossAxisAlignment: CrossAxisAlignmentCenter,
			MainAxisSize:       MainAxisSizeMin,
			ChildrenWidgets:    children,
		},
	}
}

// errorRestartButton is a stateful widget for the restart button.
// It's separated to import the engine package lazily and avoid circular imports.
type errorRestartButton struct{}

func (b errorRestartButton) CreateElement() core.Element {
	return core.NewStatefulElement(b, nil)
}

func (b errorRestartButton) Key() any {
	return nil
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
			Color:   graphics.RGBA(100, 100, 100, 200),
			Padding: layout.EdgeInsetsSymmetric(16, 8),
			ChildWidget: Text{
				Content: "Restart unavailable",
				Style: graphics.TextStyle{
					Color:    graphics.RGBA(200, 200, 200, 255),
					FontSize: 14,
				},
			},
		}
	}

	return GestureDetector{
		OnTap: s.restartFn,
		ChildWidget: Container{
			Color:   graphics.RGBA(255, 255, 255, 220),
			Padding: layout.EdgeInsetsSymmetric(16, 8),
			ChildWidget: Text{
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
