package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
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
		widgets.NewContainer(
			widgets.PaddingAll(8,
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
		).WithColor(colors.SurfaceVariant).Build(),
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
					widgets.NewContainer(nil).
						WithColor(colors.Primary).
						WithSize(200, 120).Build(),
					widgets.NewContainer(nil).
						WithColor(colors.Secondary).
						WithSize(140, 80).Build(),
					widgets.NewContainer(nil).
						WithColor(colors.Error).
						WithSize(80, 40).Build(),
					widgets.TextOf("Stacked", rendering.TextStyle{
						Color:    colors.OnError,
						FontSize: 14,
					}),
				},
			},
		},
		widgets.VSpace(24),

		// Spacers section
		sectionTitle("Spacing Helpers", colors),
		widgets.VSpace(12),
		widgets.TextOf("VSpace and HSpace for consistent spacing:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    widgets.TextOf("Title", style),
    widgets.VSpace(16),  // 16px vertical gap
    widgets.TextOf("Body", style),
)

widgets.RowOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    button1,
    widgets.HSpace(12),  // 12px horizontal gap
    button2,
)`, colors),
		widgets.VSpace(24),

		// Composition section
		sectionTitle("Composition Pattern", colors),
		widgets.VSpace(12),
		widgets.TextOf("Build complex layouts by nesting simple widgets:", labelStyle(colors)),
		widgets.VSpace(8),
		codeBlock(`widgets.PaddingAll(20,
    widgets.ColumnOf(
        widgets.MainAxisAlignmentStart,
        widgets.CrossAxisAlignmentStart,
        widgets.MainAxisSizeMin,
        widgets.RowOf(
            widgets.MainAxisAlignmentStart,
            widgets.CrossAxisAlignmentStart,
            widgets.MainAxisSizeMin,
            items...,
        ),
        widgets.VSpace(16),
        widgets.Stack{...},
    ),
)`, colors),
		widgets.VSpace(40),
	)
}

// layoutContainer wraps layout demos in a styled container.
func layoutContainer(child core.Widget, colors theme.ColorScheme) core.Widget {
	return widgets.NewContainer(
		widgets.PaddingAll(8, child),
	).WithColor(colors.SurfaceVariant).Build()
}

// colorBox creates a small colored box with a label.
func colorBox(color rendering.Color, label string) core.Widget {
	return widgets.NewContainer(
		widgets.PaddingAll(12,
			widgets.TextOf(label, rendering.TextStyle{
				Color:    rendering.ColorWhite,
				FontSize: 14,
			}),
		),
	).WithColor(color).Build()
}
