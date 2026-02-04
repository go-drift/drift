package main

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildBottomSheetsPage demonstrates bottom sheets with various configurations.
func buildBottomSheetsPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Bottom Sheets",
		// Section: Basic Bottom Sheet
		sectionTitle("Basic Bottom Sheet", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Slides up from bottom, drag to dismiss:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Bottom Sheet", func() {
			showBasicBottomSheet(ctx)
		}),
		widgets.VSpace(24),

		// Section: Bottom Sheet with Snap Points
		sectionTitle("Snap Points", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Drag handle to half or full screen, scroll content:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Multi-Snap Sheet", func() {
			showSnapPointsBottomSheet(ctx)
		}),
		widgets.VSpace(24),

		// Section: Bottom Sheet with Result
		sectionTitle("Selection Sheet", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Returns a value when dismissed:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Selection Sheet", func() {
			showSelectionBottomSheet(ctx)
		}),
		widgets.VSpace(24),

		// Section: Non-Dismissible Sheet
		sectionTitle("Non-Dismissible", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Barrier tap disabled, must use button:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Required Action Sheet", func() {
			showNonDismissibleBottomSheet(ctx)
		}),
		widgets.VSpace(24),

		// Section: No Handle
		sectionTitle("Custom Appearance", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Without drag handle:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Sheet Without Handle", func() {
			showNoHandleBottomSheet(ctx)
		}),
		widgets.VSpace(40),
	)
}

// showBasicBottomSheet displays a simple bottom sheet with drag-to-dismiss.
func showBasicBottomSheet(ctx core.BuildContext) {
	colors := theme.ColorsOf(ctx)

	navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
		return widgets.Padding{
			Padding: layout.EdgeInsetsAll(24),
			Child: widgets.Column{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				Children: []core.Widget{
					widgets.Text{
						Content: "Bottom Sheet",
						Style: graphics.TextStyle{
							Color:      colors.OnSurface,
							FontSize:   20,
							FontWeight: graphics.FontWeightBold,
						},
					},
					widgets.VSpace(12),
					widgets.Text{
						Content: "This is a modal bottom sheet. You can drag it down to dismiss, or tap the barrier behind it.",
						Wrap:    true,
						Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						},
					},
					widgets.VSpace(20),
					widgets.Row{
						MainAxisAlignment: widgets.MainAxisAlignmentCenter,
						Children: []core.Widget{
							theme.ButtonOf(ctx, "Close", func() {
								widgets.BottomSheetScope{}.Of(ctx).Close(nil)
							}),
						},
					},
				},
			},
		}
	})
}

// showSnapPointsBottomSheet displays a bottom sheet with snap points and scrollable content.
func showSnapPointsBottomSheet(ctx core.BuildContext) {
	colors := theme.ColorsOf(ctx)

	navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
		// Build items list with header included
		items := make([]core.Widget, 0, 32)

		// Add header as first items in the list
		items = append(items,
			widgets.Padding{
				Padding: layout.EdgeInsets{Left: 24, Right: 24, Top: 24},
				Child: widgets.Text{
					Content: "Scrollable List",
					Style: graphics.TextStyle{
						Color:      colors.OnSurface,
						FontSize:   20,
						FontWeight: graphics.FontWeightBold,
					},
				},
			},
			widgets.Padding{
				Padding: layout.EdgeInsets{Left: 24, Right: 24, Bottom: 16},
				Child: widgets.Text{
					Content: "Drag the handle to resize. Scroll the list below.",
					Wrap:    true,
					Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 14,
					},
				},
			},
		)

		// Add list items
		for i := range 30 {
			items = append(items, sheetItemRow(colors,
				fmt.Sprintf("Item %d", i+1),
				fmt.Sprintf("Description for item %d", i+1)))
		}

		return widgets.BottomSheetScrollable{
			Builder: func(controller *widgets.ScrollController) core.Widget {
				return widgets.ListView{
					Controller: controller,
					Padding:    layout.EdgeInsets{Left: 24, Right: 24, Bottom: 24},
					Physics:    widgets.BouncingScrollPhysics{},
					Children:   items,
				}
			},
		}
	},
		navigation.WithSnapPoints(widgets.SnapHalf, widgets.SnapFull),
		navigation.WithInitialSnapPoint(0),
		navigation.WithDragMode(widgets.DragModeContentAware),
	)
}

// showSelectionBottomSheet displays a bottom sheet that returns a selected value.
func showSelectionBottomSheet(ctx core.BuildContext) {
	colors := theme.ColorsOf(ctx)

	go func() {
		result := <-navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
			return widgets.Padding{
				Padding: layout.EdgeInsetsAll(24),
				Child: widgets.Column{
					MainAxisSize:       widgets.MainAxisSizeMin,
					CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
					Children: []core.Widget{
						widgets.Text{
							Content: "Select an Option",
							Style: graphics.TextStyle{
								Color:      colors.OnSurface,
								FontSize:   20,
								FontWeight: graphics.FontWeightBold,
							},
						},
						widgets.VSpace(16),
						sheetSelectionItem(ctx, colors, "Option A", "First choice", "A"),
						widgets.VSpace(8),
						sheetSelectionItem(ctx, colors, "Option B", "Second choice", "B"),
						widgets.VSpace(8),
						sheetSelectionItem(ctx, colors, "Option C", "Third choice", "C"),
						widgets.VSpace(12),
						widgets.Row{
							MainAxisAlignment: widgets.MainAxisAlignmentCenter,
							Children: []core.Widget{
								theme.ButtonOf(ctx, "Cancel", func() {
									widgets.BottomSheetScope{}.Of(ctx).Close(nil)
								}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
							},
						},
					},
				},
			}
		},
			navigation.WithSnapPoints(widgets.SnapPoint{FractionalHeight: 0.5, Name: "selection"}),
		)

		// Show toast with result
		if result != nil {
			drift.Dispatch(func() {
				showToast(ctx, "Selected: "+result.(string))
			})
		} else {
			drift.Dispatch(func() {
				showToast(ctx, "Selection cancelled")
			})
		}
	}()
}

// showNonDismissibleBottomSheet displays a bottom sheet that can't be dismissed by tapping the barrier.
func showNonDismissibleBottomSheet(ctx core.BuildContext) {
	colors := theme.ColorsOf(ctx)

	navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
		return widgets.Padding{
			Padding: layout.EdgeInsetsAll(24),
			Child: widgets.Column{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				Children: []core.Widget{
					widgets.Text{
						Content: "Required Action",
						Style: graphics.TextStyle{
							Color:      colors.Error,
							FontSize:   20,
							FontWeight: graphics.FontWeightBold,
						},
					},
					widgets.VSpace(12),
					widgets.Text{
						Content: "This sheet requires you to take action. Tapping the barrier does NOT dismiss it. You must use the button below.",
						Wrap:    true,
						Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						},
					},
					widgets.VSpace(20),
					widgets.Row{
						MainAxisAlignment: widgets.MainAxisAlignmentCenter,
						Children: []core.Widget{
							theme.ButtonOf(ctx, "Cancel", func() {
								widgets.BottomSheetScope{}.Of(ctx).Close(nil)
							}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
							widgets.HSpace(12),
							theme.ButtonOf(ctx, "Confirm", func() {
								widgets.BottomSheetScope{}.Of(ctx).Close("confirmed")
							}),
						},
					},
				},
			},
		}
	},
		navigation.WithBarrierDismissible(false),
		navigation.WithDragEnabled(false),
	)
}

// showNoHandleBottomSheet displays a bottom sheet without a drag handle.
func showNoHandleBottomSheet(ctx core.BuildContext) {
	colors := theme.ColorsOf(ctx)

	navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
		return widgets.Padding{
			Padding: layout.EdgeInsetsAll(24),
			Child: widgets.Column{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				Children: []core.Widget{
					widgets.Text{
						Content: "No Handle",
						Style: graphics.TextStyle{
							Color:      colors.OnSurface,
							FontSize:   20,
							FontWeight: graphics.FontWeightBold,
						},
					},
					widgets.VSpace(12),
					widgets.Text{
						Content: "This sheet has no drag handle at the top. You can still drag it to dismiss, or tap the barrier.",
						Wrap:    true,
						Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						},
					},
					widgets.VSpace(20),
					widgets.Row{
						MainAxisAlignment: widgets.MainAxisAlignmentCenter,
						Children: []core.Widget{
							theme.ButtonOf(ctx, "Close", func() {
								widgets.BottomSheetScope{}.Of(ctx).Close(nil)
							}),
						},
					},
				},
			},
		}
	},
		navigation.WithHandle(false),
	)
}

// sheetItemRow creates a list item row for the bottom sheet demo.
func sheetItemRow(colors theme.ColorScheme, title, subtitle string) core.Widget {
	return widgets.Padding{
		Padding: layout.EdgeInsetsSymmetric(0, 8),
		Child: widgets.Row{
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Container{
					Width:        40,
					Height:       40,
					Color:        colors.SurfaceVariant,
					BorderRadius: 20,
				},
				widgets.HSpace(12),
				widgets.Column{
					CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
					MainAxisSize:       widgets.MainAxisSizeMin,
					Children: []core.Widget{
						widgets.Text{
							Content: title,
							Style: graphics.TextStyle{
								Color:      colors.OnSurface,
								FontSize:   16,
								FontWeight: graphics.FontWeightMedium,
							},
						},
						widgets.Text{
							Content: subtitle,
							Style: graphics.TextStyle{
								Color:    colors.OnSurfaceVariant,
								FontSize: 12,
							},
						},
					},
				},
			},
		},
	}
}

// sheetSelectionItem creates a tappable selection item for the bottom sheet demo.
func sheetSelectionItem(ctx core.BuildContext, colors theme.ColorScheme, title, subtitle, value string) core.Widget {
	return widgets.GestureDetector{
		OnTap: func() {
			widgets.BottomSheetScope{}.Of(ctx).Close(value)
		},
		Child: widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Padding:      layout.EdgeInsetsAll(12),
			Child: widgets.Row{
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				Children: []core.Widget{
					widgets.Expanded{
						Child: widgets.Column{
							CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
							MainAxisSize:       widgets.MainAxisSizeMin,
							Children: []core.Widget{
								widgets.Text{
									Content: title,
									Style: graphics.TextStyle{
										Color:      colors.OnSurface,
										FontSize:   16,
										FontWeight: graphics.FontWeightMedium,
									},
								},
								widgets.Text{
									Content: subtitle,
									Style: graphics.TextStyle{
										Color:    colors.OnSurfaceVariant,
										FontSize: 12,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
