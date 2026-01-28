package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildButtonsPage demonstrates button variants and the builder pattern.
func buildButtonsPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	buttonGradient := graphics.NewLinearGradient(
		graphics.Offset{X: 0, Y: 0},
		graphics.Offset{X: 100, Y: 0},
		[]graphics.GradientStop{
			{Position: 0, Color: colors.Primary},
			{Position: 1, Color: colors.Tertiary},
		},
	)

	return demoPage(ctx, "Buttons",
		// Section: Basic Buttons
		sectionTitle("Basic Buttons", colors),
		widgets.VSpace(12),
		widgets.TextOf("The simplest way to create a button:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.ButtonOf("Tap Me", func() {
			platform.Haptics.LightImpact()
		}),
		widgets.VSpace(20),

		// Section: Colored Buttons
		sectionTitle("Colored Buttons", colors),
		widgets.VSpace(12),
		widgets.TextOf("Use WithColor() to set background and text:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.ButtonOf("Primary", func() {}).
			WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		widgets.ButtonOf("Secondary", func() {}).
			WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(8),
		widgets.ButtonOf("Error", func() {}).
			WithColor(colors.Error, colors.OnError),
		widgets.VSpace(8),
		widgets.ButtonOf("Surface", func() {}).
			WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
		widgets.VSpace(20),

		// Section: Gradient Buttons
		sectionTitle("Gradient Buttons", colors),
		widgets.VSpace(12),
		widgets.TextOf("Use WithGradient() for colorful backgrounds:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.ButtonOf("Sunset", func() {}).
			WithColor(colors.Primary, colors.OnPrimary).
			WithGradient(buttonGradient),
		widgets.VSpace(20),

		// Section: Custom Sizing
		sectionTitle("Custom Sizing", colors),
		widgets.VSpace(12),
		widgets.TextOf("Adjust padding and font size:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.ButtonOf("Small", func() {}).
			WithColor(colors.Primary, colors.OnPrimary).
			WithPadding(layout.EdgeInsetsSymmetric(12, 8)).
			WithFontSize(12),
		widgets.VSpace(8),
		widgets.ButtonOf("Large", func() {}).
			WithColor(colors.Primary, colors.OnPrimary).
			WithPadding(layout.EdgeInsetsSymmetric(32, 18)).
			WithFontSize(20),
		widgets.VSpace(20),

		// Section: Haptic Feedback
		sectionTitle("Haptic Feedback", colors),
		widgets.VSpace(12),
		widgets.TextOf("Buttons include haptic feedback by default:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.ButtonOf("With Haptics (default)", func() {
			platform.Haptics.LightImpact()
		}).WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		widgets.ButtonOf("No Haptics", func() {}).
			WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant).
			WithHaptic(false),
		widgets.VSpace(40),
	)
}
