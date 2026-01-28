package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildLayoutsPage demonstrates Row, Column, Stack, and other layout widgets.
func buildLayoutsPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Layouts",
		// Row section
		sectionTitle("Row Layout", colors),
		widgets.VSpace(12),
		widgets.TextOf("Horizontal arrangement with MainAxisAlignment:", labelStyle(colors)),
		widgets.VSpace(8),

		// Row - Start
		widgets.TextOf("Start:", labelStyle(colors)),
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				colorBox(colors.Primary, "A"),
				colorBox(colors.Secondary, "B"),
				colorBox(colors.Error, "C"),
			),
			colors,
		),
		widgets.VSpace(12),

		// Row - Center
		widgets.TextOf("Center:", labelStyle(colors)),
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentCenter,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				colorBox(colors.Primary, "A"),
				colorBox(colors.Secondary, "B"),
				colorBox(colors.Error, "C"),
			),
			colors,
		),
		widgets.VSpace(12),

		// Row - SpaceBetween
		widgets.TextOf("SpaceBetween:", labelStyle(colors)),
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentSpaceBetween,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				colorBox(colors.Primary, "A"),
				colorBox(colors.Secondary, "B"),
				colorBox(colors.Error, "C"),
			),
			colors,
		),
		widgets.VSpace(12),

		// Row - SpaceEvenly
		widgets.TextOf("SpaceEvenly:", labelStyle(colors)),
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentSpaceEvenly,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				colorBox(colors.Primary, "A"),
				colorBox(colors.Secondary, "B"),
				colorBox(colors.Error, "C"),
			),
			colors,
		),
		widgets.VSpace(24),

		// Column section
		sectionTitle("Column Layout", colors),
		widgets.VSpace(12),
		widgets.TextOf("Vertical arrangement:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.Container{
			Color: colors.SurfaceVariant,
			ChildWidget: widgets.PaddingAll(8,
				widgets.ColumnOf(
					widgets.MainAxisAlignmentStart,
					widgets.CrossAxisAlignmentStart,
					widgets.MainAxisSizeMin,
					colorBox(colors.Primary, "First"),
					widgets.VSpace(8),
					colorBox(colors.Secondary, "Second"),
					widgets.VSpace(8),
					colorBox(colors.Error, "Third"),
				),
			),
		},
		widgets.VSpace(24),

		// Stack section
		sectionTitle("Stack Layout", colors),
		widgets.VSpace(12),
		widgets.TextOf("Overlay widgets on top of each other:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.SizedBox{
			Width:  200,
			Height: 120,
			ChildWidget: widgets.Stack{
				Alignment: layout.AlignmentCenter,
				ChildrenWidgets: []core.Widget{
					widgets.Container{Color: colors.Primary, Width: 200, Height: 120},
					widgets.Container{Color: colors.Secondary, Width: 140, Height: 80},
					widgets.Container{Color: colors.Error, Width: 80, Height: 40},
					widgets.TextOf("Stacked", graphics.TextStyle{
						Color:    colors.OnError,
						FontSize: 14,
					}),
				},
			},
		},
		widgets.VSpace(40),
	)
}

// layoutContainer wraps layout demos in a styled container.
func layoutContainer(child core.Widget, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:       colors.SurfaceVariant,
		ChildWidget: widgets.PaddingAll(8, child),
	}
}

// colorBox creates a small colored box with a label.
func colorBox(color graphics.Color, label string) core.Widget {
	return widgets.Container{
		Color: color,
		ChildWidget: widgets.PaddingAll(12,
			widgets.TextOf(label, graphics.TextStyle{
				Color:    graphics.ColorWhite,
				FontSize: 14,
			}),
		),
	}
}
