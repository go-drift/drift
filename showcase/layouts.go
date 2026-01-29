package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildLayoutsPage demonstrates Row, Column, Stack, and other layout widgets.
func buildLayoutsPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	// Distinct colors for layout demos
	boxA := CyanSeed                    // Cyan
	boxB := PinkSeed                    // Pink/Magenta
	boxC := graphics.RGB(239, 154, 154) // Soft coral

	return demoPage(ctx, "Layouts",
		// MainAxisAlignment section
		sectionTitle("MainAxisAlignment", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Controls spacing along the main axis (horizontal for Row):", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Text{Content: "Start:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(widgets.MainAxisAlignmentStart, widgets.CrossAxisAlignmentStart, widgets.MainAxisSizeMax,
				colorBox(boxA, "A"), colorBox(boxB, "B"), colorBox(boxC, "C")),
			colors,
		),
		widgets.VSpace(8),

		widgets.Text{Content: "Center:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(widgets.MainAxisAlignmentCenter, widgets.CrossAxisAlignmentStart, widgets.MainAxisSizeMax,
				colorBox(boxA, "A"), colorBox(boxB, "B"), colorBox(boxC, "C")),
			colors,
		),
		widgets.VSpace(8),

		widgets.Text{Content: "SpaceBetween:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(widgets.MainAxisAlignmentSpaceBetween, widgets.CrossAxisAlignmentStart, widgets.MainAxisSizeMax,
				colorBox(boxA, "A"), colorBox(boxB, "B"), colorBox(boxC, "C")),
			colors,
		),
		widgets.VSpace(8),

		widgets.Text{Content: "SpaceEvenly:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		layoutContainer(
			widgets.RowOf(widgets.MainAxisAlignmentSpaceEvenly, widgets.CrossAxisAlignmentStart, widgets.MainAxisSizeMax,
				colorBox(boxA, "A"), colorBox(boxB, "B"), colorBox(boxC, "C")),
			colors,
		),
		widgets.VSpace(24),

		// CrossAxisAlignment section
		sectionTitle("CrossAxisAlignment", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Controls positioning along the cross axis (vertical for Row):", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Text{Content: "Start:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		crossAxisContainer(
			widgets.RowOf(widgets.MainAxisAlignmentStart, widgets.CrossAxisAlignmentStart, widgets.MainAxisSizeMax,
				tallBox(boxA, "A", 60), tallBox(boxB, "B", 40), tallBox(boxC, "C", 50)),
			colors,
		),
		widgets.VSpace(8),

		widgets.Text{Content: "Center:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		crossAxisContainer(
			widgets.RowOf(widgets.MainAxisAlignmentStart, widgets.CrossAxisAlignmentCenter, widgets.MainAxisSizeMax,
				tallBox(boxA, "A", 60), tallBox(boxB, "B", 40), tallBox(boxC, "C", 50)),
			colors,
		),
		widgets.VSpace(8),

		widgets.Text{Content: "End:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		crossAxisContainer(
			widgets.RowOf(widgets.MainAxisAlignmentStart, widgets.CrossAxisAlignmentEnd, widgets.MainAxisSizeMax,
				tallBox(boxA, "A", 60), tallBox(boxB, "B", 40), tallBox(boxC, "C", 50)),
			colors,
		),
		widgets.VSpace(8),

		widgets.Text{Content: "Stretch:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		crossAxisContainer(
			widgets.RowOf(widgets.MainAxisAlignmentStart, widgets.CrossAxisAlignmentStretch, widgets.MainAxisSizeMax,
				colorBox(boxA, "A"), colorBox(boxB, "B"), colorBox(boxC, "C")),
			colors,
		),
		widgets.VSpace(24),

		// Column section
		sectionTitle("Column", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Same alignments work vertically:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentStart,
			widgets.MainAxisSizeMax,
			columnDemo("Start", widgets.CrossAxisAlignmentStart, boxA, boxB, boxC, colors),
			widgets.HSpace(12),
			columnDemo("Center", widgets.CrossAxisAlignmentCenter, boxA, boxB, boxC, colors),
			widgets.HSpace(12),
			columnDemo("End", widgets.CrossAxisAlignmentEnd, boxA, boxB, boxC, colors),
		),
		widgets.VSpace(24),

		// Stack section
		sectionTitle("Stack", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Overlay widgets on top of each other:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.SizedBox{
			Width:  200,
			Height: 120,
			ChildWidget: widgets.Stack{
				Alignment: layout.AlignmentCenter,
				ChildrenWidgets: []core.Widget{
					widgets.Container{Color: boxA, Width: 200, Height: 120},
					widgets.Container{Color: boxB, Width: 140, Height: 80},
					widgets.Container{Color: boxC, Width: 80, Height: 40},
					widgets.Text{Content: "Stacked", Style: graphics.TextStyle{Color: graphics.ColorWhite, FontSize: 14}},
				},
			},
		},
		widgets.VSpace(40),
	)
}

// columnDemo creates a labeled column demo with CrossAxisAlignment.
func columnDemo(label string, cross widgets.CrossAxisAlignment, a, b, c graphics.Color, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		MainAxisSize:       widgets.MainAxisSizeMin,
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: label, Style: labelStyle(colors)},
			widgets.VSpace(4),
			widgets.Container{
				Color:  colors.SurfaceVariant,
				Width:  80,
				Height: 120,
				ChildWidget: widgets.Column{
					MainAxisAlignment:  widgets.MainAxisAlignmentStart,
					CrossAxisAlignment: cross,
					MainAxisSize:       widgets.MainAxisSizeMin,
					ChildrenWidgets: []core.Widget{
						widgets.Container{Color: a, Width: 50, Height: 30},
						widgets.VSpace(4),
						widgets.Container{Color: b, Width: 30, Height: 30},
						widgets.VSpace(4),
						widgets.Container{Color: c, Width: 40, Height: 30},
					},
				},
			},
		},
	}
}

// layoutContainer wraps layout demos in a styled container.
func layoutContainer(child core.Widget, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:       colors.SurfaceVariant,
		ChildWidget: widgets.PaddingAll(8, child),
	}
}

// crossAxisContainer wraps layout demos with fixed height for cross-axis demos.
func crossAxisContainer(child core.Widget, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:       colors.SurfaceVariant,
		Height:      80,
		ChildWidget: widgets.PaddingAll(8, child),
	}
}

// textColorFor returns dark text for light backgrounds, white for dark backgrounds.
func textColorFor(bg graphics.Color) graphics.Color {
	if bg == CyanSeed {
		return graphics.ColorBlack
	}
	return graphics.ColorWhite
}

// colorBox creates a small colored box with a label.
func colorBox(color graphics.Color, label string) core.Widget {
	return widgets.Container{
		Color: color,
		ChildWidget: widgets.PaddingAll(12,
			widgets.Text{Content: label, Style: graphics.TextStyle{
				Color:    textColorFor(color),
				FontSize: 14,
			}},
		),
	}
}

// tallBox creates a colored box with specific height for cross-axis demos.
func tallBox(color graphics.Color, label string, height float64) core.Widget {
	return widgets.Container{
		Color:  color,
		Height: height,
		ChildWidget: widgets.PaddingAll(12,
			widgets.Text{Content: label, Style: graphics.TextStyle{
				Color:    textColorFor(color),
				FontSize: 14,
			}},
		),
	}
}
