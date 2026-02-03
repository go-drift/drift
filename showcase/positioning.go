package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildPositioningPage demonstrates single-child positioning widgets.
func buildPositioningPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	boxColor := CyanSeed

	return demoPage(ctx, "Positioning",
		// Center
		sectionTitle("Center", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Centers child within available space:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		positioningContainer(
			widgets.Center{
				Child: colorBox(boxColor, "Centered"),
			},
			colors,
		),
		widgets.VSpace(24),

		// Align
		sectionTitle("Align", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Position child with any alignment:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentStart,
			widgets.MainAxisSizeMax,
			alignDemo("TopLeft", layout.AlignmentTopLeft, boxColor, colors),
			widgets.HSpace(8),
			alignDemo("TopRight", layout.AlignmentTopRight, boxColor, colors),
		),
		widgets.VSpace(8),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentStart,
			widgets.MainAxisSizeMax,
			alignDemo("BottomLeft", layout.AlignmentBottomLeft, boxColor, colors),
			widgets.HSpace(8),
			alignDemo("BottomRight", layout.AlignmentBottomRight, boxColor, colors),
		),
		widgets.VSpace(24),

		// SizedBox
		sectionTitle("SizedBox", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Constrain child to specific dimensions:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentEnd,
			widgets.MainAxisSizeMax,
			sizedBoxDemo("50x50", 50, 50, boxColor, colors),
			widgets.HSpace(8),
			sizedBoxDemo("100x30", 100, 30, boxColor, colors),
			widgets.HSpace(8),
			sizedBoxDemo("60x80", 60, 80, boxColor, colors),
		),
		widgets.VSpace(16),
		widgets.Text{Content: "Also useful as spacers (no child):", Style: labelStyle(colors)},
		widgets.VSpace(8),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentCenter,
				widgets.MainAxisSizeMax,
				colorBox(boxColor, "A"),
				widgets.SizedBox{Width: 40}, // Horizontal spacer
				colorBox(PinkSeed, "B"),
			),
			colors,
		),
		widgets.VSpace(24),

		// Expanded
		sectionTitle("Expanded", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Fill remaining space in Row/Column:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentCenter,
				widgets.MainAxisSizeMax,
				colorBox(boxColor, "Fixed"),
				widgets.Expanded{
					Child: widgets.Container{
						Color: PinkSeed,
						Child: widgets.PaddingAll(12,
							widgets.Text{Content: "Expanded", Style: graphics.TextStyle{Color: graphics.ColorWhite, FontSize: 14}},
						),
					},
				},
				colorBox(boxColor, "Fixed"),
			),
			colors,
		),
		widgets.VSpace(16),
		widgets.Text{Content: "With flex factors (1:2 ratio):", Style: labelStyle(colors)},
		widgets.VSpace(8),
		layoutContainer(
			widgets.RowOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentCenter,
				widgets.MainAxisSizeMax,
				widgets.Expanded{
					Flex: 1,
					Child: widgets.Container{
						Color: boxColor,
						Child: widgets.PaddingAll(12,
							widgets.Text{Content: "Flex 1", Style: graphics.TextStyle{Color: graphics.ColorBlack, FontSize: 14}},
						),
					},
				},
				widgets.HSpace(8),
				widgets.Expanded{
					Flex: 2,
					Child: widgets.Container{
						Color: PinkSeed,
						Child: widgets.PaddingAll(12,
							widgets.Text{Content: "Flex 2", Style: graphics.TextStyle{Color: graphics.ColorWhite, FontSize: 14}},
						),
					},
				},
			),
			colors,
		),
		widgets.VSpace(40),
	)
}

// positioningContainer creates a fixed-size container for positioning demos.
func positioningContainer(child core.Widget, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:  colors.SurfaceVariant,
		Width:  200,
		Height: 100,
		Child:  child,
	}
}

// alignDemo shows Align with a specific alignment.
func alignDemo(label string, alignment layout.Alignment, boxColor graphics.Color, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		MainAxisSize:       widgets.MainAxisSizeMin,
		Children: []core.Widget{
			widgets.Text{Content: label, Style: labelStyle(colors)},
			widgets.VSpace(4),
			widgets.Container{
				Color:  colors.SurfaceVariant,
				Width:  120,
				Height: 80,
				Child: widgets.Align{
					Alignment: alignment,
					Child: widgets.Container{
						Color: boxColor,
						Child: widgets.PaddingAll(8,
							widgets.Text{Content: "X", Style: graphics.TextStyle{Color: graphics.ColorBlack, FontSize: 12}},
						),
					},
				},
			},
		},
	}
}

// sizedBoxDemo shows a SizedBox with specific dimensions.
func sizedBoxDemo(label string, width, height float64, boxColor graphics.Color, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		MainAxisSize:       widgets.MainAxisSizeMin,
		Children: []core.Widget{
			widgets.SizedBox{
				Width:  width,
				Height: height,
				Child: widgets.Container{
					Color: boxColor,
					Child: widgets.Center{
						Child: widgets.Text{Content: label, Style: graphics.TextStyle{
							Color:    textColorFor(boxColor),
							FontSize: 11,
						}},
					},
				},
			},
			widgets.VSpace(4),
			widgets.Text{Content: label, Style: labelStyle(colors)},
		},
	}
}
