package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildScrollPage demonstrates scrollable content with physics.
func buildScrollPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Scrolling",
		sectionTitle("ListView", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Static list with explicit children:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Child: widgets.SizedBox{
				Height: 200,
				Child: widgets.ListView{
					Padding: layout.EdgeInsetsAll(8),
					Physics: widgets.BouncingScrollPhysics{},
					Children: []core.Widget{
						listItem(1, colors.Surface, colors),
						widgets.VSpace(6),
						listItem(2, colors.Surface, colors),
						widgets.VSpace(6),
						listItem(3, colors.Surface, colors),
						widgets.VSpace(6),
						listItem(4, colors.Surface, colors),
						widgets.VSpace(6),
						listItem(5, colors.Surface, colors),
					},
				},
			},
		},
		widgets.VSpace(24),

		sectionTitle("ListViewBuilder", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Dynamic list with lazy item generation:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Child: widgets.SizedBox{
				Height: 200,
				Child: widgets.ListViewBuilder{
					Padding:     layout.EdgeInsetsAll(8),
					ItemCount:   50,
					ItemExtent:  56,
					CacheExtent: 112,
					ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
						return widgets.Padding{
							Padding: layout.EdgeInsets{Bottom: 6},
							Child:   listItem(index+1, colors.Surface, colors),
						}
					},
				},
			},
		},
		widgets.VSpace(40),
	)
}

// listItem creates a styled list item.
func listItem(index int, bgColor graphics.Color, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:        bgColor,
		BorderRadius: 6,
		Child: widgets.PaddingSym(12, 12,
			widgets.Row{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				Children: []core.Widget{
					widgets.Container{
						Color:        colors.PrimaryContainer,
						BorderRadius: 4,
						Width:        32,
						Height:       32,
						Alignment:    layout.AlignmentCenter,
						Child: widgets.Text{Content: itoa(index), Style: graphics.TextStyle{
							Color:    colors.OnPrimaryContainer,
							FontSize: 14,
						}},
					},
					widgets.HSpace(12),
					widgets.Text{Content: "Item " + itoa(index), Style: graphics.TextStyle{
						Color:    colors.OnSurface,
						FontSize: 15,
					}},
				},
			},
		),
	}
}
