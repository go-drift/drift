package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildButtonsPage demonstrates button variants and the builder pattern.
func buildButtonsPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	buttonGradient := rendering.NewLinearGradient(
		rendering.Offset{X: 0, Y: 0},
		rendering.Offset{X: 220, Y: 0},
		[]rendering.GradientStop{
			{Position: 0, Color: colors.Primary},
			{Position: 1, Color: colors.Secondary},
		},
	)

	return demoPage(ctx, "Buttons",
		// Section: Basic Buttons
		sectionTitle("Basic Buttons", colors),
		widgets.VSpace(12),
		widgets.TextOf("The simplest way to create a button:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("Tap Me", func() {
			platform.Haptics.LightImpact()
		}),
		widgets.VSpace(20),

		// Section: Colored Buttons
		sectionTitle("Colored Buttons", colors),
		widgets.VSpace(12),
		widgets.TextOf("Use WithColor() to set background and text:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("Primary", func() {}).
			WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		widgets.NewButton("Secondary", func() {}).
			WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(8),
		widgets.NewButton("Error", func() {}).
			WithColor(colors.Error, colors.OnError),
		widgets.VSpace(8),
		widgets.NewButton("Surface", func() {}).
			WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
		widgets.VSpace(20),

		// Section: Gradient Buttons
		sectionTitle("Gradient Buttons", colors),
		widgets.VSpace(12),
		widgets.TextOf("Use WithGradient() for colorful backgrounds:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("Sunset", func() {}).
			WithColor(colors.Primary, colors.OnPrimary).
			WithGradient(buttonGradient),
		widgets.VSpace(20),

		// Section: Custom Sizing
		sectionTitle("Custom Sizing", colors),
		widgets.VSpace(12),
		widgets.TextOf("Adjust padding and font size:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("Small", func() {}).
			WithColor(colors.Primary, colors.OnPrimary).
			WithPadding(layout.EdgeInsetsSymmetric(12, 8)).
			WithFontSize(12),
		widgets.VSpace(8),
		widgets.NewButton("Large", func() {}).
			WithColor(colors.Primary, colors.OnPrimary).
			WithPadding(layout.EdgeInsetsSymmetric(32, 18)).
			WithFontSize(20),
		widgets.VSpace(20),

		// Section: Builder Pattern
		sectionTitle("Builder Pattern", colors),
		widgets.VSpace(12),
		widgets.TextOf("Chain methods for configuration:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`widgets.NewButton("Submit", onTap).
    WithColor(colors.Primary, colors.OnPrimary).
    WithFontSize(18).
    WithPadding(padding)`, colors),
		widgets.VSpace(20),

		// Section: Haptic Feedback
		sectionTitle("Haptic Feedback", colors),
		widgets.VSpace(12),
		widgets.TextOf("Buttons include haptic feedback by default:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("With Haptics (default)", func() {
			platform.Haptics.LightImpact()
		}).WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		widgets.NewButton("No Haptics", func() {}).
			WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant).
			WithHaptic(false),
		widgets.VSpace(40),
	)
}
