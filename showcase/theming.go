package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildThemingPage demonstrates the theming system.
func buildThemingPage(ctx core.BuildContext, isDark bool) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	modeLabel := "Dark Mode"
	if !isDark {
		modeLabel = "Light Mode"
	}
	gradientText := rendering.NewLinearGradient(
		rendering.Offset{X: 0, Y: 0},
		rendering.Offset{X: 280, Y: 0},
		[]rendering.GradientStop{
			{Position: 0, Color: colors.Primary},
			{Position: 1, Color: colors.Secondary},
		},
	)

	return demoPage(ctx, "Theming",
		// Current mode indicator
		widgets.NewContainer(
			widgets.PaddingAll(16,
				widgets.RowOf(
					widgets.MainAxisAlignmentCenter,
					widgets.CrossAxisAlignmentStart,
					widgets.MainAxisSizeMax,

					widgets.TextOf(modeLabel, rendering.TextStyle{
						Color:      colors.OnPrimary,
						FontSize:   18,
						FontWeight: rendering.FontWeightBold,
					}),
				),
			),
		).WithColor(colors.Primary).Build(),
		widgets.VSpace(24),

		// Color palette section
		sectionTitle("Color Palette", colors),
		widgets.VSpace(12),
		colorSwatch("Primary", colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		colorSwatch("Secondary", colors.Secondary, colors.OnSecondary),
		widgets.VSpace(8),
		colorSwatch("Error", colors.Error, colors.OnError),
		widgets.VSpace(8),
		colorSwatch("Background", colors.Background, colors.OnBackground),
		widgets.VSpace(8),
		colorSwatch("Surface", colors.Surface, colors.OnSurface),
		widgets.VSpace(8),
		colorSwatch("SurfaceVariant", colors.SurfaceVariant, colors.OnSurfaceVariant),
		widgets.VSpace(24),

		// Text theme section
		sectionTitle("Text Theme", colors),
		widgets.VSpace(12),
		widgets.TextOf("HeadlineLarge", textTheme.HeadlineLarge),
		widgets.VSpace(8),
		widgets.TextOf("HeadlineMedium", textTheme.HeadlineMedium),
		widgets.VSpace(8),
		widgets.TextOf("HeadlineSmall", textTheme.HeadlineSmall),
		widgets.VSpace(8),
		widgets.TextOf("TitleLarge", textTheme.TitleLarge),
		widgets.VSpace(8),
		widgets.TextOf("TitleMedium", textTheme.TitleMedium),
		widgets.VSpace(8),
		widgets.TextOf("BodyLarge", textTheme.BodyLarge),
		widgets.VSpace(8),
		widgets.TextOf("BodyMedium", textTheme.BodyMedium),
		widgets.VSpace(8),
		widgets.TextOf("LabelLarge", textTheme.LabelLarge),
		widgets.VSpace(24),

		// Gradient text section
		sectionTitle("Gradient Text", colors),
		widgets.VSpace(12),
		widgets.TextOf("Gradient headlines", rendering.TextStyle{
			Color:      colors.OnSurface,
			Gradient:   gradientText,
			FontSize:   28,
			FontWeight: rendering.FontWeightBold,
		}),
		widgets.VSpace(24),

		// Using themes section
		sectionTitle("Using Themes", colors),
		widgets.VSpace(12),
		widgets.TextOf("Access theme in Build() via context:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`func (s *myState) Build(ctx core.BuildContext) core.Widget {
    // Get all theme parts at once
    _, colors, textTheme := theme.UseTheme(ctx)

    // Or get parts individually
    colors := theme.ColorsOf(ctx)
    textTheme := theme.TextThemeOf(ctx)

    return widgets.TextOf("Hello", textTheme.HeadlineLarge)
}`, colors),
		widgets.VSpace(24),

		// Theme provider section
		sectionTitle("Providing Theme", colors),
		widgets.VSpace(12),
		widgets.TextOf("Wrap your app with Theme:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`theme.Theme{
    Data: theme.DefaultDarkTheme(),  // or DefaultLightTheme()
    ChildWidget: myApp,
}`, colors),
		widgets.VSpace(40),
	)
}

// colorSwatch displays a color with its name.
func colorSwatch(name string, bg, fg rendering.Color) core.Widget {
	return widgets.NewContainer(
		widgets.PaddingSym(16, 12,
			widgets.RowOf(
				widgets.MainAxisAlignmentSpaceBetween,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				widgets.TextOf(name, rendering.TextStyle{
					Color:    fg,
					FontSize: 16,
				}),
				widgets.TextOf(colorHex(bg), rendering.TextStyle{
					Color:    fg,
					FontSize: 12,
				}),
			),
		),
	).WithColor(bg).Build()
}

// colorHex formats a color as a hex string.
func colorHex(c rendering.Color) string {
	r := (c >> 16) & 0xFF
	g := (c >> 8) & 0xFF
	b := c & 0xFF
	return "#" + hexByte(uint8(r)) + hexByte(uint8(g)) + hexByte(uint8(b))
}

func hexByte(b uint8) string {
	const hexChars = "0123456789ABCDEF"
	return string([]byte{hexChars[b>>4], hexChars[b&0x0F]})
}
