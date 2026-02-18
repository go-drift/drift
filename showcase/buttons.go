package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildButtonsPage demonstrates button variants and the builder pattern.
func buildButtonsPage(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
	buttonGradient := graphics.NewLinearGradient(
		graphics.AlignCenterLeft,
		graphics.AlignCenterRight,
		[]graphics.GradientStop{
			{Position: 0, Color: colors.Primary},
			{Position: 1, Color: colors.Tertiary},
		},
	)

	return demoPage(ctx, "Buttons",
		// Section: Themed Buttons
		sectionTitle("Themed Buttons", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Use theme.ButtonOf(ctx, ...) for styled buttons:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Tap Me", func() {
			platform.Haptics.LightImpact()
		}),
		widgets.VSpace(24),

		// Section: Colored Buttons
		sectionTitle("Colored Buttons", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Override theme with WithColor():", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Primary", func() {}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Secondary", func() {}).
					WithColor(colors.Secondary, colors.OnSecondary),
			},
		},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Error", func() {}).
					WithColor(colors.Error, colors.OnError),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Surface", func() {}).
					WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			},
		},
		widgets.VSpace(24),

		// Section: Gradient Buttons
		sectionTitle("Gradient Buttons", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Use WithGradient() for colorful backgrounds:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Sunset", func() {}).
			WithGradient(buttonGradient),
		widgets.VSpace(24),

		// Section: Custom Sizing
		sectionTitle("Custom Sizing", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Adjust padding and font size:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Small", func() {}).
					WithPadding(layout.EdgeInsetsSymmetric(12, 8)).
					WithFontSize(12),
				widgets.HSpace(12),
				theme.ButtonOf(ctx, "Large", func() {}).
					WithPadding(layout.EdgeInsetsSymmetric(32, 18)).
					WithFontSize(20),
			},
		},
		widgets.VSpace(24),

		// Section: Explicit Buttons
		sectionTitle("Explicit Buttons", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Fully explicit with struct literal:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Button{
			Label:        "Explicit",
			OnTap:        func() { platform.Haptics.LightImpact() },
			Color:        graphics.RGB(156, 39, 176),
			TextColor:    graphics.ColorWhite,
			Padding:      layout.EdgeInsetsSymmetric(24, 14),
			FontSize:     16,
			BorderRadius: 8,
			Haptic:       true,
		},
		widgets.VSpace(24),

		// Section: Haptic Feedback
		sectionTitle("Haptic Feedback", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Buttons include haptic feedback by default:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "With Haptics", func() {
					platform.Haptics.LightImpact()
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "No Haptics", func() {}).
					WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant).
					WithHaptic(false),
			},
		},
		widgets.VSpace(40),
	)
}
