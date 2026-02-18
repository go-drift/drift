package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildDecorationsPage demonstrates ClipRRect and DecoratedBox.
func buildDecorationsPage(ctx core.BuildContext) core.Widget {
	colors, textTheme := theme.ColorsOf(ctx), theme.TextThemeOf(ctx)
	logo := loadGoLogo()

	return demoPage(ctx, "Decorations",
		widgets.Text{Content: "Decorations", Style: textTheme.TitleLarge},
		widgets.VSpace(8),
		widgets.Text{Content: "Rounded corners and borders for any widget.", Style: labelStyle(colors)},
		widgets.VSpace(24),
		sectionTitle("ClipRRect", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Clip images or content with rounded corners.", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.ClipRRect{
			Radius: 16,
			Child: widgets.Image{
				Source: logo,
				Width:  240,
				Height: 140,
				Fit:    widgets.ImageFitCover,
			},
		},
		widgets.VSpace(24),
		sectionTitle("DecoratedBox", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Paint fills, gradients, and borders behind content.", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.DecoratedBox{
			Color:        colors.SurfaceVariant,
			BorderColor:  colors.Outline,
			BorderWidth:  1,
			BorderRadius: 16,
			Child: widgets.PaddingAll(16,
				widgets.ColumnOf(
					widgets.MainAxisAlignmentStart,
					widgets.CrossAxisAlignmentStart,
					widgets.MainAxisSizeMin,
					widgets.Text{Content: "Card title", Style: graphics.TextStyle{
						Color:      colors.OnSurface,
						FontSize:   16,
						FontWeight: graphics.FontWeightBold,
					}},
					widgets.VSpace(8),
					widgets.Text{Content: "Use border radius for cards and panels.", Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 14,
					}},
				),
			),
		},
		widgets.VSpace(24),
		widgets.DecoratedBox{
			Color: graphics.ColorWhite,
			Gradient: graphics.NewLinearGradient(
				graphics.AlignCenterLeft,
				graphics.AlignCenterRight,
				[]graphics.GradientStop{
					{Position: 0, Color: colors.Primary},
					{Position: 1, Color: colors.Tertiary},
				},
			),
			BorderRadius: 16,
			Child: widgets.SizedBox{
				Width:  240,
				Height: 52,
				Child: widgets.Center{
					Child: widgets.Text{Content: "Gradient surface", Style: graphics.TextStyle{
						Color:      colors.OnPrimary,
						FontSize:   14,
						FontWeight: graphics.FontWeightBold,
					}},
				},
			},
		},
		widgets.VSpace(24),
		sectionTitle("Drop Shadows", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Material elevation levels 1-5 using BoxShadowElevation.", Style: labelStyle(colors)},
		widgets.VSpace(16),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,
			elevationBox("1", 1, colors),
			widgets.HSpace(16),
			elevationBox("2", 2, colors),
			widgets.HSpace(16),
			elevationBox("3", 3, colors),
			widgets.HSpace(16),
			elevationBox("5", 5, colors),
		),
		widgets.VSpace(24),
		sectionTitle("Backdrop Blur", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Frosted glass effect over content.", Style: labelStyle(colors)},
		widgets.VSpace(16),
		widgets.SizedBox{
			Width:  280,
			Height: 160,
			Child: widgets.Stack{
				Children: []core.Widget{
					widgets.ClipRRect{
						Radius: 16,
						Child: widgets.Image{
							Source: logo,
							Width:  280,
							Height: 160,
							Fit:    widgets.ImageFitCover,
						},
					},
					widgets.Positioned{
						Left:   widgets.Ptr(40),
						Top:    widgets.Ptr(40),
						Right:  widgets.Ptr(40),
						Bottom: widgets.Ptr(40),
						Child: widgets.ClipRRect{
							Radius: 12,
							Child: widgets.NewBackdropFilter(10,
								widgets.DecoratedBox{
									Color: graphics.RGBA(255, 255, 255, 0.3),
									Child: widgets.Center{
										Child: widgets.Text{Content: "Frosted Glass", Style: graphics.TextStyle{
											Color:      graphics.RGBA(10, 10, 10, 0.4),
											FontSize:   14,
											FontWeight: graphics.FontWeightBold,
										}},
									},
								},
							),
						},
					},
				},
			},
		},
		widgets.VSpace(24),
		sectionTitle("Text Shadows", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Drop shadows for text elements.", Style: labelStyle(colors)},
		widgets.VSpace(16),
		widgets.ColumnOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentStart,
			widgets.MainAxisSizeMin,
			widgets.Text{Content: "Hard Shadow", Style: graphics.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   24,
				FontWeight: graphics.FontWeightBold,
				Shadow: &graphics.TextShadow{
					Color:  colors.Primary.WithAlpha(0.31),
					Offset: graphics.Offset{X: 2, Y: 2},
				},
			}},
			widgets.VSpace(16),
			widgets.Text{Content: "Soft Shadow", Style: graphics.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   24,
				FontWeight: graphics.FontWeightBold,
				Shadow: &graphics.TextShadow{
					Color:      colors.Primary.WithAlpha(0.39),
					Offset:     graphics.Offset{X: 2, Y: 3},
					BlurRadius: 4,
				},
			}},
			widgets.VSpace(16),
			widgets.Text{Content: "Glow Effect", Style: graphics.TextStyle{
				Color:      colors.Primary,
				FontSize:   24,
				FontWeight: graphics.FontWeightBold,
				Shadow: &graphics.TextShadow{
					Color:      colors.Tertiary.WithAlpha(0.9),
					Offset:     graphics.Offset{X: 0, Y: 0},
					BlurRadius: 8,
				},
			}},
		),
		widgets.VSpace(40),
	)
}

// elevationBox creates a 72x72 box demonstrating a Material elevation level.
func elevationBox(label string, level int, colors theme.ColorScheme) core.Widget {
	return widgets.DecoratedBox{
		BorderColor:  colors.SecondaryContainer,
		BorderWidth:  2,
		BorderDash:   &graphics.DashPattern{Intervals: []float64{5, 3}},
		BorderRadius: 12,
		Color:        colors.SurfaceVariant,
		Shadow:       graphics.BoxShadowElevation(level, colors.SurfaceTint.WithAlpha(0.31)),
		Child: widgets.SizedBox{
			Width:  72,
			Height: 72,
			Child: widgets.Center{
				Child: widgets.Text{Content: label, Style: graphics.TextStyle{
					Color:    colors.OnSurface,
					FontSize: 14,
				}},
			},
		},
	}
}
