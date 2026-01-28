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
		widgets.VSpace(24),
		sectionTitle("Drop Shadows", colors),
		widgets.VSpace(12),
		widgets.TextOf("Material elevation levels 1-5 using BoxShadowElevation.", labelStyle(colors)),
		widgets.VSpace(16),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,
			widgets.DecoratedBox{
				BorderColor:  colors.SecondaryContainer,
				BorderWidth:  2,
				BorderDash:   &rendering.DashPattern{Intervals: []float64{5, 3}}, // 5px on, 3px off
				BorderRadius: 12,
				Color:        colors.SurfaceVariant,
				Shadow:       rendering.BoxShadowElevation(1, colors.SurfaceTint.WithAlpha(80)),
				ChildWidget: widgets.SizedBox{
					Width:  72,
					Height: 72,
					ChildWidget: widgets.Center{
						ChildWidget: widgets.TextOf("1", rendering.TextStyle{
							Color:    colors.OnSurface,
							FontSize: 14,
						}),
					},
				},
			},
			widgets.HSpace(16),
			widgets.DecoratedBox{
				BorderColor:  colors.SecondaryContainer,
				BorderWidth:  2,
				BorderDash:   &rendering.DashPattern{Intervals: []float64{5, 3}}, // 5px on, 3px off
				BorderRadius: 12,
				Color:        colors.SurfaceVariant,
				Shadow:       rendering.BoxShadowElevation(2, colors.SurfaceTint.WithAlpha(80)),
				ChildWidget: widgets.SizedBox{
					Width:  72,
					Height: 72,
					ChildWidget: widgets.Center{
						ChildWidget: widgets.TextOf("2", rendering.TextStyle{
							Color:    colors.OnSurface,
							FontSize: 14,
						}),
					},
				},
			},
			widgets.HSpace(16),
			widgets.DecoratedBox{
				BorderColor:  colors.SecondaryContainer,
				BorderWidth:  2,
				BorderDash:   &rendering.DashPattern{Intervals: []float64{5, 3}}, // 5px on, 3px off
				BorderRadius: 12,
				Color:        colors.SurfaceVariant,
				Shadow:       rendering.BoxShadowElevation(3, colors.SurfaceTint.WithAlpha(80)),
				ChildWidget: widgets.SizedBox{
					Width:  72,
					Height: 72,
					ChildWidget: widgets.Center{
						ChildWidget: widgets.TextOf("3", rendering.TextStyle{
							Color:    colors.OnSurface,
							FontSize: 14,
						}),
					},
				},
			},
			widgets.HSpace(16),
			widgets.DecoratedBox{
				BorderColor:  colors.SecondaryContainer,
				BorderWidth:  2,
				BorderDash:   &rendering.DashPattern{Intervals: []float64{5, 3}}, // 5px on, 3px off
				BorderRadius: 12,
				Color:        colors.SurfaceVariant,
				Shadow:       rendering.BoxShadowElevation(5, colors.SurfaceTint.WithAlpha(80)),
				ChildWidget: widgets.SizedBox{
					Width:  72,
					Height: 72,
					ChildWidget: widgets.Center{
						ChildWidget: widgets.TextOf("5", rendering.TextStyle{
							Color:    colors.OnSurface,
							FontSize: 14,
						}),
					},
				},
			},
		),
		widgets.VSpace(24),
		sectionTitle("Backdrop Blur", colors),
		widgets.VSpace(12),
		widgets.TextOf("Frosted glass effect over content.", labelStyle(colors)),
		widgets.VSpace(16),
		widgets.SizedBox{
			Width:  280,
			Height: 160,
			ChildWidget: widgets.Stack{
				ChildrenWidgets: []core.Widget{
					widgets.ClipRRect{
						Radius: 16,
						ChildWidget: widgets.Image{
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
						ChildWidget: widgets.ClipRRect{
							Radius: 12,
							ChildWidget: widgets.NewBackdropFilter(10,
								widgets.DecoratedBox{
									Color: rendering.RGBA(255, 255, 255, 77),
									ChildWidget: widgets.Center{
										ChildWidget: widgets.TextOf("Frosted Glass", rendering.TextStyle{
											Color:      rendering.RGBA(10, 10, 10, 90),
											FontSize:   14,
											FontWeight: rendering.FontWeightBold,
										}),
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
		widgets.TextOf("Drop shadows for text elements.", labelStyle(colors)),
		widgets.VSpace(16),
		widgets.ColumnOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentStart,
			widgets.MainAxisSizeMin,
			widgets.TextOf("Hard Shadow", rendering.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   24,
				FontWeight: rendering.FontWeightBold,
				Shadow: &rendering.TextShadow{
					Color:  colors.Primary.WithAlpha(80),
					Offset: rendering.Offset{X: 2, Y: 2},
				},
			}),
			widgets.VSpace(16),
			widgets.TextOf("Soft Shadow", rendering.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   24,
				FontWeight: rendering.FontWeightBold,
				Shadow: &rendering.TextShadow{
					Color:      colors.Primary.WithAlpha(100),
					Offset:     rendering.Offset{X: 2, Y: 3},
					BlurRadius: 4,
				},
			}),
			widgets.VSpace(16),
			widgets.TextOf("Glow Effect", rendering.TextStyle{
				Color:      colors.Primary,
				FontSize:   24,
				FontWeight: rendering.FontWeightBold,
				Shadow: &rendering.TextShadow{
					Color:      colors.SurfaceTint.WithAlpha(200),
					Offset:     rendering.Offset{X: 0, Y: 0},
					BlurRadius: 8,
				},
			}),
		),
		widgets.VSpace(40),
	)
}
