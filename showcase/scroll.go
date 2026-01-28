package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildScrollPage demonstrates scrollable content with physics.
func buildScrollPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	// Build a list of items
	items := make([]core.Widget, 0, 50)

	items = append(items,
		sectionTitle("ListView", colors),
		widgets.VSpace(8),
		widgets.TextOf("ListView builds a scrollable column for simple lists:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.SizedBox{
			Height: 220,
			ChildWidget: widgets.ListView{
				Physics: widgets.ClampingScrollPhysics{},
				ChildrenWidgets: []core.Widget{
					listItem(1, colors.Surface, colors),
					widgets.VSpace(4),
					listItem(2, colors.SurfaceVariant, colors),
					widgets.VSpace(4),
					listItem(3, colors.Surface, colors),
					widgets.VSpace(4),
					listItem(4, colors.SurfaceVariant, colors),
				},
			},
		},
		widgets.VSpace(20),
		sectionTitle("ListView Builder", colors),
		widgets.VSpace(8),
		widgets.TextOf("Use ListViewBuilder for item generation:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.SizedBox{
			Height: 220,
			ChildWidget: widgets.ListViewBuilder{
				ItemCount:   12,
				ItemExtent:  52,
				CacheExtent: 104,
				ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
					bg := colors.Surface
					if index%2 == 1 {
						bg = colors.SurfaceVariant
					}
					return listItem(index+1, bg, colors)
				},
			},
		},
		widgets.VSpace(20),
		sectionTitle("Scrollable List", colors),
		widgets.VSpace(8),
		widgets.TextOf("Drag to scroll through 40 items", labelStyle(colors)),
		widgets.VSpace(16),
	)

	for i := 1; i <= 40; i++ {
		bgColor := colors.Surface
		if i%2 == 0 {
			bgColor = colors.SurfaceVariant
		}
		items = append(items, listItem(i, bgColor, colors))
		items = append(items, widgets.VSpace(4))
	}

	items = append(items,
		widgets.VSpace(40),
	)

	content := widgets.ListView{
		Padding:         layout.EdgeInsetsAll(20),
		Physics:         widgets.BouncingScrollPhysics{},
		ChildrenWidgets: items,
	}

	return pageScaffold(ctx, "Scrolling", content)
}

// listItem creates a styled list item.
func listItem(index int, bgColor graphics.Color, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color: bgColor,
		ChildWidget: widgets.PaddingSym(16, 14,
			widgets.RowOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				widgets.Container{
					Color: colors.Primary,
					ChildWidget: widgets.PaddingAll(8,
						widgets.TextOf(itoa(index), graphics.TextStyle{
							Color:    colors.OnPrimary,
							FontSize: 12,
						}),
					),
				},
				widgets.HSpace(16),
				widgets.TextOf("List Item "+itoa(index), graphics.TextStyle{
					Color:    colors.OnSurface,
					FontSize: 16,
				}),
			),
		),
	}
}
