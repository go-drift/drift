package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildThemingPage demonstrates the theming system.
func buildThemingPage(ctx core.BuildContext, isDark bool, isCupertino bool) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	modeLabel := "Dark Mode"
	if !isDark {
		modeLabel = "Light Mode"
	}

	platformLabel := "Material Design"
	if isCupertino {
		platformLabel = "Cupertino (iOS)"
	}

	gradientText := graphics.NewLinearGradient(
		graphics.AlignCenterLeft,
		graphics.AlignCenterRight,
		[]graphics.GradientStop{
			{Position: 0, Color: colors.Primary},
			{Position: 1, Color: colors.Tertiary},
		},
	)

	contentWidgets := []core.Widget{
		// Current mode indicator
		themeModeCard(modeLabel, platformLabel, colors),
		widgets.VSpace(24),

		// Color palette section
		sectionTitle("Color Palette", colors),
		widgets.VSpace(12),
	}

	// Add color swatches
	swatches := []struct {
		name   string
		bg, fg graphics.Color
	}{
		{"Primary", colors.Primary, colors.OnPrimary},
		{"PrimaryContainer", colors.PrimaryContainer, colors.OnPrimaryContainer},
		{"Secondary", colors.Secondary, colors.OnSecondary},
		{"SecondaryContainer", colors.SecondaryContainer, colors.OnSecondaryContainer},
		{"Tertiary", colors.Tertiary, colors.OnTertiary},
		{"TertiaryContainer", colors.TertiaryContainer, colors.OnTertiaryContainer},
		{"Error", colors.Error, colors.OnError},
		{"Background", colors.Background, colors.OnBackground},
		{"Surface", colors.Surface, colors.OnSurface},
		{"SurfaceVariant", colors.SurfaceVariant, colors.OnSurfaceVariant},
		{"SurfaceContainer", colors.SurfaceContainer, colors.OnSurface},
	}
	for _, s := range swatches {
		contentWidgets = append(contentWidgets, colorSwatch(s.name, s.bg, s.fg), widgets.VSpace(8))
	}

	// Text theme section
	contentWidgets = append(contentWidgets,
		widgets.VSpace(16),
		sectionTitle("Text Theme", colors),
		widgets.VSpace(12),
	)
	textStyles := []struct {
		name  string
		style graphics.TextStyle
	}{
		{"HeadlineLarge", textTheme.HeadlineLarge},
		{"HeadlineMedium", textTheme.HeadlineMedium},
		{"HeadlineSmall", textTheme.HeadlineSmall},
		{"TitleLarge", textTheme.TitleLarge},
		{"TitleMedium", textTheme.TitleMedium},
		{"BodyLarge", textTheme.BodyLarge},
		{"BodyMedium", textTheme.BodyMedium},
		{"LabelLarge", textTheme.LabelLarge},
	}
	for _, ts := range textStyles {
		contentWidgets = append(contentWidgets, widgets.Text{Content: ts.name, Style: ts.style}, widgets.VSpace(8))
	}

	// Gradient text section
	contentWidgets = append(contentWidgets,
		widgets.VSpace(16),
		sectionTitle("Gradient Text", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Gradient headlines", Style: graphics.TextStyle{
			Color:      colors.OnSurface,
			Gradient:   gradientText,
			FontSize:   28,
			FontWeight: graphics.FontWeightBold,
		}},
	)

	// Add Cupertino colors section if Cupertino theme is active
	if isCupertino {
		cupertinoColors := theme.CupertinoColorsOf(ctx)
		contentWidgets = append(contentWidgets,
			widgets.VSpace(24),
			sectionTitle("Cupertino System Colors", colors),
			widgets.VSpace(12),
			colorSwatch("SystemBlue", cupertinoColors.SystemBlue, graphics.RGB(255, 255, 255)),
			widgets.VSpace(8),
			colorSwatch("SystemGreen", cupertinoColors.SystemGreen, graphics.RGB(255, 255, 255)),
			widgets.VSpace(8),
			colorSwatch("SystemRed", cupertinoColors.SystemRed, graphics.RGB(255, 255, 255)),
			widgets.VSpace(8),
			colorSwatch("SystemOrange", cupertinoColors.SystemOrange, graphics.RGB(0, 0, 0)),
			widgets.VSpace(8),
			colorSwatch("SystemPurple", cupertinoColors.SystemPurple, graphics.RGB(255, 255, 255)),
			widgets.VSpace(8),
			colorSwatch("Label", cupertinoColors.Label, cupertinoColors.SystemBackground),
			widgets.VSpace(8),
			colorSwatch("SystemBackground", cupertinoColors.SystemBackground, cupertinoColors.Label),
		)
	}

	contentWidgets = append(contentWidgets, widgets.VSpace(40))

	return demoPage(ctx, "Theming", contentWidgets...)
}

// colorSwatch displays a color with its name.
func colorSwatch(name string, bg, fg graphics.Color) core.Widget {
	return widgets.Container{
		Color: bg,
		ChildWidget: widgets.PaddingSym(16, 12,
			widgets.RowOf(
				widgets.MainAxisAlignmentSpaceBetween,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				widgets.Text{Content: name, Style: graphics.TextStyle{
					Color:    fg,
					FontSize: 16,
				}},
				widgets.Text{Content: colorHex(bg), Style: graphics.TextStyle{
					Color:    fg,
					FontSize: 12,
				}},
			),
		),
	}
}

// colorHex formats a color as a hex string.
func colorHex(c graphics.Color) string {
	r := (c >> 16) & 0xFF
	g := (c >> 8) & 0xFF
	b := c & 0xFF
	return "#" + hexByte(uint8(r)) + hexByte(uint8(g)) + hexByte(uint8(b))
}

func hexByte(b uint8) string {
	const hexChars = "0123456789ABCDEF"
	return string([]byte{hexChars[b>>4], hexChars[b&0x0F]})
}

// themeModeCard displays the current theme mode and platform.
func themeModeCard(mode, platform string, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color: colors.Primary,
		ChildWidget: widgets.PaddingAll(16,
			widgets.ColumnOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentCenter,
				widgets.MainAxisSizeMin,
				widgets.Text{Content: mode, Style: graphics.TextStyle{
					Color:      colors.OnPrimary,
					FontSize:   18,
					FontWeight: graphics.FontWeightBold,
				}},
				widgets.VSpace(4),
				widgets.Text{Content: platform, Style: graphics.TextStyle{
					Color:    colors.OnPrimary,
					FontSize: 14,
				}},
			),
		),
	}
}
