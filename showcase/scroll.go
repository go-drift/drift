package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
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
		widgets.VSpace(12),
		codeBlock(`widgets.ListView{
    Padding:         layout.EdgeInsetsAll(20),
    Physics:         widgets.BouncingScrollPhysics{},
    ChildrenWidgets: items,
}`, colors),
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
		widgets.VSpace(12),
		codeBlock(`widgets.ListViewBuilder{
    ItemCount:   40,
    ItemExtent:  52,
    CacheExtent: 104,
    ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
        return listItem(index+1, colors.Surface, colors)
    },
}`, colors),
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
		widgets.VSpace(20),
		sectionTitle("Scroll Physics", colors),
		widgets.VSpace(12),
		widgets.TextOf("ScrollView uses BouncingScrollPhysics for natural feel:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`widgets.ScrollView{
    ScrollDirection: widgets.AxisVertical,
    Physics:         widgets.BouncingScrollPhysics{},
    ChildWidget:     content,
}`, colors),
		widgets.VSpace(20),
		sectionTitle("Scroll Controller", colors),
		widgets.VSpace(12),
		widgets.TextOf("Programmatic scroll control:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`ctrl := &widgets.ScrollController{}

// In widget
widgets.ScrollView{
    Controller:  ctrl,
    ChildWidget: content,
}

// Jump to position
ctrl.JumpTo(100)

// Animate to position
ctrl.AnimateTo(200, 300*time.Millisecond)

// Get current offset
offset := ctrl.Offset()`, colors),
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
func listItem(index int, bgColor rendering.Color, colors theme.ColorScheme) core.Widget {
	return widgets.NewContainer(
		widgets.PaddingSym(16, 14,
			widgets.RowOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				widgets.NewContainer(
					widgets.PaddingAll(8,
						widgets.TextOf(itoa(index), rendering.TextStyle{
							Color:    colors.OnPrimary,
							FontSize: 12,
						}),
					),
				).WithColor(colors.Primary).Build(),
				widgets.HSpace(16),
				widgets.TextOf("List Item "+itoa(index), rendering.TextStyle{
					Color:    colors.OnSurface,
					FontSize: 16,
				}),
			),
		),
	).WithColor(bgColor).Build()
}
