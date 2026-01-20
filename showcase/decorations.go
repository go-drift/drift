package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildDecorationsPage demonstrates ClipRRect and DecoratedBox.
func buildDecorationsPage(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)
	logo := loadGoLogo()

	return demoPage(ctx, "Decorations",
		widgets.TextOf("Decorations", textTheme.TitleLarge),
		widgets.VSpace(8),
		widgets.TextOf("Rounded corners and borders for any widget.", labelStyle(colors)),
		widgets.VSpace(24),
		sectionTitle("ClipRRect", colors),
		widgets.VSpace(12),
		widgets.TextOf("Clip images or content with rounded corners.", labelStyle(colors)),
		widgets.VSpace(12),
		widgets.ClipRRect{
			Radius: 16,
			ChildWidget: widgets.Image{
				Source: logo,
				Width:  240,
				Height: 140,
				Fit:    widgets.ImageFitCover,
			},
		},
		widgets.VSpace(24),
		sectionTitle("DecoratedBox", colors),
		widgets.VSpace(12),
		widgets.TextOf("Paint fills, gradients, and borders behind content.", labelStyle(colors)),
		widgets.VSpace(12),
		widgets.DecoratedBox{
			Color:        colors.SurfaceVariant,
			BorderColor:  colors.Outline,
			BorderWidth:  1,
			BorderRadius: 16,
			ChildWidget: widgets.PaddingAll(16,
				widgets.ColumnOf(
					widgets.MainAxisAlignmentStart,
					widgets.CrossAxisAlignmentStart,
					widgets.MainAxisSizeMin,
					widgets.TextOf("Card title", rendering.TextStyle{
						Color:      colors.OnSurface,
						FontSize:   16,
						FontWeight: rendering.FontWeightBold,
					}),
					widgets.VSpace(8),
					widgets.TextOf("Use border radius for cards and panels.", rendering.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 14,
					}).WithWrap(true),
				),
			),
		},
		widgets.VSpace(24),
		widgets.DecoratedBox{
			Gradient: rendering.NewLinearGradient(
				rendering.Offset{X: 0, Y: 0},
				rendering.Offset{X: 240, Y: 0},
				[]rendering.GradientStop{
					{Position: 0, Color: colors.Primary},
					{Position: 1, Color: colors.Secondary},
				},
			),
			BorderRadius: 20,
			ChildWidget: widgets.SizedBox{
				Width:  240,
				Height: 52,
				ChildWidget: widgets.Center{
					ChildWidget: widgets.TextOf("Gradient surface", rendering.TextStyle{
						Color:      colors.OnPrimary,
						FontSize:   14,
						FontWeight: rendering.FontWeightBold,
					}),
				},
			},
		},
		widgets.VSpace(40),
	)
}
