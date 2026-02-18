package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildWrapPage demonstrates the Wrap widget for flowing layouts.
func buildWrapPage(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Wrap",
		sectionTitle("Basic Wrap", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Children flow to the next line when space runs out:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Container{
			Color:   colors.SurfaceVariant,
			Padding: layout.EdgeInsetsAll(8),
			Child: widgets.WrapOf(8, 8,
				chip("Go", colors),
				chip("Rust", colors),
				chip("TypeScript", colors),
				chip("Python", colors),
				chip("Swift", colors),
				chip("Kotlin", colors),
			),
		},
		widgets.VSpace(24),

		sectionTitle("Alignment", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Control how items are positioned within each run:", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Text{Content: "Start (default):", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapAlignmentDemo(widgets.WrapAlignmentStart, colors),
		widgets.VSpace(8),

		widgets.Text{Content: "Center:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapAlignmentDemo(widgets.WrapAlignmentCenter, colors),
		widgets.VSpace(8),

		widgets.Text{Content: "SpaceBetween:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapAlignmentDemo(widgets.WrapAlignmentSpaceBetween, colors),
		widgets.VSpace(8),

		widgets.Text{Content: "SpaceEvenly:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapAlignmentDemo(widgets.WrapAlignmentSpaceEvenly, colors),
		widgets.VSpace(24),

		sectionTitle("Spacing", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Spacing (horizontal) and RunSpacing (vertical):", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Text{Content: "Spacing: 4, RunSpacing: 4", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapSpacingDemo(4, 4, colors),
		widgets.VSpace(8),

		widgets.Text{Content: "Spacing: 16, RunSpacing: 8", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapSpacingDemo(16, 8, colors),
		widgets.VSpace(24),

		sectionTitle("CrossAxisAlignment", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Align items within each run:", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Text{Content: "Start:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapCrossDemo(widgets.WrapCrossAlignmentStart, colors),
		widgets.VSpace(8),

		widgets.Text{Content: "Center:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapCrossDemo(widgets.WrapCrossAlignmentCenter, colors),
		widgets.VSpace(8),

		widgets.Text{Content: "End:", Style: labelStyle(colors)},
		widgets.VSpace(4),
		wrapCrossDemo(widgets.WrapCrossAlignmentEnd, colors),
		widgets.VSpace(40),
	)
}

// chip creates a styled chip/tag widget.
func chip(label string, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:        colors.PrimaryContainer,
		BorderRadius: 16,
		Padding:      layout.EdgeInsetsSymmetric(12, 8),
		Child: widgets.Text{Content: label, Style: graphics.TextStyle{
			Color:    colors.OnPrimaryContainer,
			FontSize: 14,
		}},
	}
}

// wrapAlignmentDemo shows Wrap with different alignment settings.
func wrapAlignmentDemo(alignment widgets.WrapAlignment, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:   colors.SurfaceVariant,
		Height:  80,
		Padding: layout.EdgeInsetsAll(8),
		Child: widgets.Wrap{
			Direction:  widgets.WrapAxisHorizontal,
			Alignment:  alignment,
			Spacing:    8,
			RunSpacing: 8,
			Children:   []core.Widget{chip("One", colors), chip("Two", colors), chip("Three", colors)},
		},
	}
}

// wrapSpacingDemo shows Wrap with different spacing values.
func wrapSpacingDemo(spacing, runSpacing float64, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:   colors.SurfaceVariant,
		Padding: layout.EdgeInsetsAll(8),
		Child: widgets.WrapOf(spacing, runSpacing,
			chip("Alpha", colors),
			chip("Beta", colors),
			chip("Gamma", colors),
			chip("Delta", colors),
			chip("Epsilon", colors),
		),
	}
}

// wrapCrossDemo shows Wrap with different cross-axis alignments using varied heights.
func wrapCrossDemo(cross widgets.WrapCrossAlignment, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:  colors.SurfaceVariant,
		Height: 64,
		Padding: layout.EdgeInsetsAll(8),
		Child: widgets.Wrap{
			Direction:          widgets.WrapAxisHorizontal,
			CrossAxisAlignment: cross,
			Spacing:            8,
			RunSpacing:         8,
			Children: []core.Widget{
				tallChip("Short", 32, colors),
				tallChip("Tall", 48, colors),
				tallChip("Medium", 40, colors),
			},
		},
	}
}

// tallChip creates a chip with specific height for cross-axis demos.
func tallChip(label string, height float64, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:        colors.PrimaryContainer,
		BorderRadius: 16,
		Height:       height,
		Padding:      layout.EdgeInsetsSymmetric(12, 8),
		Child: widgets.Text{Content: label, Style: graphics.TextStyle{
			Color:    colors.OnPrimaryContainer,
			FontSize: 14,
		}},
	}
}
