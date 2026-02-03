package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/overlay"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildOverlaysPage demonstrates the overlay system including modals, tooltips, and barriers.
func buildOverlaysPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Overlays",
		// Section: Modal Dialog
		sectionTitle("Modal Dialog", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Tap to show a modal dialog with barrier:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Dialog", func() {
			showModalDialog(ctx)
		}),
		widgets.VSpace(24),

		// Section: Modal Route
		sectionTitle("Modal Route", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Modal as a navigation route (integrates with back button):", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Push Modal Route", func() {
			pushModalRoute(ctx)
		}),
		widgets.VSpace(24),

		// Section: Toast Overlay
		sectionTitle("Toast Overlay", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Temporary notification that auto-dismisses:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Toast", func() {
			showToast(ctx, "Hello from the overlay!")
		}),
		widgets.VSpace(24),

		// Section: Stacked Overlays
		sectionTitle("Stacked Overlays", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Multiple overlays can stack (toast + dialog):", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Stacked", func() {
			showStackedOverlays(ctx)
		}),
		widgets.VSpace(24),

		// Section: Non-Dismissible Barrier
		sectionTitle("Non-Dismissible Barrier", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Modal that requires action (can't dismiss by tapping barrier):", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Show Blocking Modal", func() {
			showBlockingModal(ctx)
		}),
		widgets.VSpace(40),
	)
}

// showModalDialog displays a modal dialog using overlay entries.
func showModalDialog(ctx core.BuildContext) {
	overlayState := overlay.OverlayOf(ctx)
	if overlayState == nil {
		return
	}

	colors := theme.ColorsOf(ctx)

	var barrierEntry, dialogEntry *overlay.OverlayEntry

	dismiss := func() {
		if barrierEntry != nil {
			barrierEntry.Remove()
		}
		if dialogEntry != nil {
			dialogEntry.Remove()
		}
	}

	// Create barrier entry
	barrierEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return overlay.ModalBarrier{
			Color:         graphics.RGBA(0, 0, 0, 0.5),
			Dismissible:   true,
			OnDismiss:     dismiss,
			SemanticLabel: "Close dialog",
		}
	})

	// Create dialog entry
	dialogEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Center{
			Child: widgets.Container{
				Width:        300,
				Color:        colors.Surface,
				BorderRadius: 16,
				Padding:      layout.EdgeInsetsAll(24),
				Shadow: &graphics.BoxShadow{
					Color:      graphics.RGBA(0, 0, 0, 0.3),
					BlurRadius: 20,
					Offset:     graphics.Offset{Y: 8},
				},
				Child: widgets.Column{
					MainAxisSize:       widgets.MainAxisSizeMin,
					CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
					Children: []core.Widget{
						widgets.Text{
							Content: "Modal Dialog",
							Style: graphics.TextStyle{
								Color:      colors.OnSurface,
								FontSize:   20,
								FontWeight: graphics.FontWeightBold,
							},
						},
						widgets.VSpace(12),
						widgets.Text{
							Content: "This dialog was created using overlay entries. Tap outside or the button to dismiss.",
							Wrap:    true,
							Style: graphics.TextStyle{
								Color:    colors.OnSurfaceVariant,
								FontSize: 14,
							},
						},
						widgets.VSpace(20),
						theme.ButtonOf(ctx, "Got it", dismiss),
					},
				},
			},
		}
	})
	dialogEntry.Opaque = true

	// Insert barrier then dialog (dialog on top)
	overlayState.Insert(barrierEntry, nil, nil)
	overlayState.Insert(dialogEntry, nil, nil)
}

// pushModalRoute uses ModalRoute for navigation-integrated modals.
func pushModalRoute(ctx core.BuildContext) {
	nav := navigation.NavigatorOf(ctx)
	if nav == nil {
		return
	}

	route := navigation.NewModalRoute(
		func(ctx core.BuildContext) core.Widget {
			colors := theme.ColorsOf(ctx)
			return widgets.Center{
				Child: widgets.Container{
					Width:        300,
					Color:        colors.Surface,
					BorderRadius: 16,
					Padding:      layout.EdgeInsetsAll(24),
					Shadow: &graphics.BoxShadow{
						Color:      graphics.RGBA(0, 0, 0, 0.3),
						BlurRadius: 20,
						Offset:     graphics.Offset{Y: 8},
					},
					Child: widgets.Column{
						MainAxisSize:       widgets.MainAxisSizeMin,
						CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
						Children: []core.Widget{
							widgets.Text{
								Content: "Modal Route",
								Style: graphics.TextStyle{
									Color:      colors.OnSurface,
									FontSize:   20,
									FontWeight: graphics.FontWeightBold,
								},
							},
							widgets.VSpace(12),
							widgets.Text{
								Content: "This modal is a navigation route. Tap the barrier, use the system back button, or tap Close.",
								Wrap:    true,
								Style: graphics.TextStyle{
									Color:    colors.OnSurfaceVariant,
									FontSize: 14,
								},
							},
							widgets.VSpace(20),
							theme.ButtonOf(ctx, "Close", func() {
								navigation.NavigatorOf(ctx).Pop(nil)
							}),
						},
					},
				},
			}
		},
		navigation.RouteSettings{Name: "/modal-demo"},
	)
	route.BarrierDismissible = true
	barrierColor := graphics.RGBA(0, 0, 0, 0.5)
	route.BarrierColor = &barrierColor
	route.BarrierLabel = "Close modal"

	nav.Push(route)
}

// showToast displays a temporary toast notification.
// Tap the toast to dismiss it (in a real app, you'd use a timer for auto-dismiss).
func showToast(ctx core.BuildContext, message string) {
	overlayState := overlay.OverlayOf(ctx)
	if overlayState == nil {
		return
	}

	colors := theme.ColorsOf(ctx)

	var entry *overlay.OverlayEntry

	entry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		// Use a Stack that fills the overlay to properly position the toast
		return widgets.Stack{
			Fit: widgets.StackFitExpand,
			Children: []core.Widget{
				widgets.Positioned{
					Alignment: &graphics.AlignBottomCenter,
					Bottom:    widgets.Ptr(100),
					Child: widgets.GestureDetector{
						OnTap: func() {
							entry.Remove()
						},
						Child: widgets.Container{
							Color:        colors.InverseSurface,
							BorderRadius: 8,
							Padding:      layout.EdgeInsetsSymmetric(20, 12),
							Child: widgets.Text{
								Content: message + " (tap to dismiss)",
								Style: graphics.TextStyle{
									Color:    colors.OnInverseSurface,
									FontSize: 14,
								},
							},
						},
					},
				},
			},
		}
	})

	overlayState.Insert(entry, nil, nil)
}

// showStackedOverlays demonstrates multiple overlay layers.
func showStackedOverlays(ctx core.BuildContext) {
	overlayState := overlay.OverlayOf(ctx)
	if overlayState == nil {
		return
	}

	colors := theme.ColorsOf(ctx)

	var toastEntry, barrierEntry, dialogEntry *overlay.OverlayEntry

	dismiss := func() {
		if toastEntry != nil {
			toastEntry.Remove()
		}
		if barrierEntry != nil {
			barrierEntry.Remove()
		}
		if dialogEntry != nil {
			dialogEntry.Remove()
		}
	}

	// First: Toast at bottom
	toastEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Stack{
			Fit: widgets.StackFitExpand,
			Children: []core.Widget{
				widgets.Positioned{
					Alignment: &graphics.AlignBottomCenter,
					Bottom:    widgets.Ptr(100),
					Child: widgets.Container{
						Color:        colors.Tertiary,
						BorderRadius: 8,
						Padding:      layout.EdgeInsetsSymmetric(20, 12),
						Child: widgets.Text{
							Content: "Toast is below the dialog",
							Style: graphics.TextStyle{
								Color:    colors.OnTertiary,
								FontSize: 14,
							},
						},
					},
				},
			},
		}
	})

	// Second: Barrier
	barrierEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return overlay.ModalBarrier{
			Color:         graphics.RGBA(0, 0, 0, 0.5),
			Dismissible:   true,
			OnDismiss:     dismiss,
			SemanticLabel: "Close stacked overlays",
		}
	})

	// Third: Dialog on top
	dialogEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Center{
			Child: widgets.Container{
				Width:        280,
				Color:        colors.Surface,
				BorderRadius: 16,
				Padding:      layout.EdgeInsetsAll(20),
				Shadow: &graphics.BoxShadow{
					Color:      graphics.RGBA(0, 0, 0, 0.3),
					BlurRadius: 20,
					Offset:     graphics.Offset{Y: 8},
				},
				Child: widgets.Column{
					MainAxisSize:       widgets.MainAxisSizeMin,
					CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
					Children: []core.Widget{
						widgets.Text{
							Content: "Stacked Overlays",
							Style: graphics.TextStyle{
								Color:      colors.OnSurface,
								FontSize:   18,
								FontWeight: graphics.FontWeightBold,
							},
						},
						widgets.VSpace(12),
						widgets.Text{
							Content: "Notice the toast below this dialog. Multiple overlays stack in order.",
							Wrap:    true,
							Style: graphics.TextStyle{
								Color:    colors.OnSurfaceVariant,
								FontSize: 14,
							},
						},
						widgets.VSpace(16),
						theme.ButtonOf(ctx, "Dismiss All", dismiss),
					},
				},
			},
		}
	})
	dialogEntry.Opaque = true

	// Insert in order: toast, barrier, dialog
	overlayState.Insert(toastEntry, nil, nil)
	overlayState.Insert(barrierEntry, nil, nil)
	overlayState.Insert(dialogEntry, nil, nil)
}

// showBlockingModal displays a modal that can't be dismissed by tapping the barrier.
func showBlockingModal(ctx core.BuildContext) {
	overlayState := overlay.OverlayOf(ctx)
	if overlayState == nil {
		return
	}

	colors := theme.ColorsOf(ctx)

	var barrierEntry, dialogEntry *overlay.OverlayEntry

	dismiss := func() {
		if barrierEntry != nil {
			barrierEntry.Remove()
		}
		if dialogEntry != nil {
			dialogEntry.Remove()
		}
	}

	// Create non-dismissible barrier (darker to indicate it can't be dismissed)
	barrierEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return overlay.ModalBarrier{
			Color:         graphics.RGBA(0, 0, 0, 0.65),
			Dismissible:   false, // Can't dismiss by tapping
			SemanticLabel: "Required action dialog",
		}
	})

	// Create dialog
	dialogEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Center{
			Child: widgets.Container{
				Width:        300,
				Color:        colors.Surface,
				BorderRadius: 16,
				Padding:      layout.EdgeInsetsAll(24),
				Shadow: &graphics.BoxShadow{
					Color:      graphics.RGBA(0, 0, 0, 0.3),
					BlurRadius: 20,
					Offset:     graphics.Offset{Y: 8},
				},
				Child: widgets.Column{
					MainAxisSize:       widgets.MainAxisSizeMin,
					CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
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
							Content: "This modal requires you to take action. Tapping outside does NOT dismiss it.",
							Wrap:    true,
							Style: graphics.TextStyle{
								Color:    colors.OnSurfaceVariant,
								FontSize: 14,
							},
						},
						widgets.VSpace(20),
						widgets.Row{
							MainAxisSize:       widgets.MainAxisSizeMin,
							MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
							CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
							Children: []core.Widget{
								theme.ButtonOf(ctx, "Cancel", dismiss).
									WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
								widgets.HSpace(12),
								theme.ButtonOf(ctx, "Confirm", dismiss).
									WithColor(colors.Primary, colors.OnPrimary),
							},
						},
					},
				},
			},
		}
	})
	dialogEntry.Opaque = true

	// Insert barrier then dialog
	overlayState.Insert(barrierEntry, nil, nil)
	overlayState.Insert(dialogEntry, nil, nil)
}
