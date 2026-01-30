package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// DebugErrorScreen is a full-screen error display for development mode.
// It shows detailed error information including:
//   - Error phase (Build, Layout, Paint, HitTest, Frame, Pointer)
//   - Widget or RenderObject type that failed
//   - Error message
//   - Scrollable stack trace
//   - Restart button to recover the app
//
// This screen is automatically shown in debug mode (core.DebugMode = true)
// when an uncaught panic occurs. In production mode, panics crash the app
// unless caught by an [ErrorBoundary].
//
// The restart button calls [engine.RestartApp] to unmount the entire widget
// tree and re-mount from scratch, clearing all state.
type DebugErrorScreen struct {
	// Error is the boundary error to display. If nil, shows "Unknown error".
	Error *errors.BoundaryError
}

func (d DebugErrorScreen) CreateElement() core.Element {
	return core.NewStatelessElement(d, nil)
}

func (d DebugErrorScreen) Key() any {
	return nil
}

func (d DebugErrorScreen) Build(ctx core.BuildContext) core.Widget {
	// Colors
	bgColor := graphics.RGBA(30, 30, 30, 1.0)       // Dark background
	headerColor := graphics.RGBA(200, 50, 50, 1.0)  // Red header
	errorBoxColor := graphics.RGBA(50, 50, 50, 1.0) // Darker box for error
	textColor := graphics.ColorWhite
	dimTextColor := graphics.RGBA(180, 180, 180, 1.0)
	stackBgColor := graphics.RGBA(40, 40, 40, 1.0)

	// Build header text based on error phase
	headerText := "Error"
	if d.Error != nil {
		switch d.Error.Phase {
		case "build":
			headerText = "Build Error"
		case "layout":
			headerText = "Layout Error"
		case "paint":
			headerText = "Paint Error"
		case "hittest":
			headerText = "HitTest Error"
		case "frame":
			headerText = "Frame Error"
		case "pointer":
			headerText = "Pointer Error"
		default:
			if d.Error.Recovered != nil {
				headerText = "Panic"
			}
		}
	}

	// Error message
	errorMessage := "Unknown error"
	if d.Error != nil {
		errorMessage = d.Error.Error()
	}

	// Type name (widget or render object)
	typeName := ""
	if d.Error != nil {
		if d.Error.Widget != "" {
			typeName = d.Error.Widget
		} else if d.Error.RenderObject != "" {
			typeName = d.Error.RenderObject
		}
	}

	// Stack trace
	stackTrace := ""
	if d.Error != nil && d.Error.StackTrace != "" {
		stackTrace = d.Error.StackTrace
	}

	// Build the content
	children := []core.Widget{
		// Header section
		Container{
			Color:   headerColor,
			Padding: layout.EdgeInsetsAll(16),
			ChildWidget: Row{
				ChildrenWidgets: []core.Widget{
					Text{
						Content: "!",
						Style: graphics.TextStyle{
							Color:      textColor,
							FontSize:   28,
							FontWeight: graphics.FontWeightBold,
						},
					},
					SizedBox{Width: 12},
					Expanded{
						ChildWidget: Column{
							CrossAxisAlignment: CrossAxisAlignmentStart,
							MainAxisSize:       MainAxisSizeMin,
							ChildrenWidgets: []core.Widget{
								Text{
									Content: headerText,
									Style: graphics.TextStyle{
										Color:      textColor,
										FontSize:   20,
										FontWeight: graphics.FontWeightBold,
									},
								},
								if_(typeName != "", func() core.Widget {
									return Text{
										Content: typeName,
										Style: graphics.TextStyle{
											Color:    dimTextColor,
											FontSize: 14,
										},
									}
								}),
							},
						},
					},
				},
			},
		},
	}

	// Error message box
	children = append(children,
		Container{
			Padding: layout.EdgeInsetsAll(16),
			ChildWidget: Container{
				Color:   errorBoxColor,
				Padding: layout.EdgeInsetsAll(12),
				ChildWidget: Text{
					Content: errorMessage,
					Style: graphics.TextStyle{
						Color:    textColor,
						FontSize: 14,
					},
					Wrap: true,
				},
			},
		},
	)

	// Stack trace section (scrollable)
	if stackTrace != "" {
		children = append(children,
			Padding{
				Padding: layout.EdgeInsetsSymmetric(16, 0),
				ChildWidget: Text{
					Content: "Stack Trace",
					Style: graphics.TextStyle{
						Color:      dimTextColor,
						FontSize:   12,
						FontWeight: graphics.FontWeightBold,
					},
				},
			},
			SizedBox{Height: 8},
			Expanded{
				ChildWidget: Padding{
					Padding: layout.EdgeInsetsSymmetric(16, 0),
					ChildWidget: Container{
						Color: stackBgColor,
						ChildWidget: ScrollView{
							Padding: layout.EdgeInsetsAll(12),
							ChildWidget: Text{
								Content: stackTrace,
								Style: graphics.TextStyle{
									Color:    dimTextColor,
									FontSize: 11,
								},
								Wrap: true,
							},
						},
					},
				},
			},
		)
	}

	// Restart button - use Row with centered alignment instead of Center
	// (Center expands to fill available space, consuming space meant for Expanded)
	children = append(children,
		Padding{
			Padding: layout.EdgeInsetsAll(16),
			ChildWidget: Row{
				MainAxisAlignment: MainAxisAlignmentCenter,
				MainAxisSize:      MainAxisSizeMax,
				ChildrenWidgets: []core.Widget{
					errorRestartButton{},
				},
			},
		},
	)

	// SafeArea wrapper to handle notches/status bars
	return Container{
		Color: bgColor,
		ChildWidget: SafeArea{
			ChildWidget: Column{
				MainAxisSize:       MainAxisSizeMax, // Required for Expanded children
				CrossAxisAlignment: CrossAxisAlignmentStretch,
				ChildrenWidgets:    children,
			},
		},
	}
}

// if_ is a helper for conditional widget inclusion
func if_(condition bool, builder func() core.Widget) core.Widget {
	if condition {
		return builder()
	}
	return SizedBox{}
}
