package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildRichTextPage demonstrates the RichText widget with inline styled spans.
func buildRichTextPage(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	return demoPage(ctx, "Rich Text",
		// Section: Basic Inline Styles
		sectionTitle("Inline Styles", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Mix colors, weights, and sizes in a single paragraph:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("Go "),
				graphics.Span("bold").Bold(),
				graphics.Span(", "),
				graphics.Span("italic").Italic(),
				graphics.Span(", or "),
				graphics.Span("colored").Color(colors.Primary),
			).Size(16).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Font Weights
		sectionTitle("Font Weights", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Five standard weights in one paragraph:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("Thin ").Weight(graphics.FontWeightThin),
				graphics.Span("Regular ").Weight(graphics.FontWeightNormal),
				graphics.Span("Medium ").Weight(graphics.FontWeightMedium),
				graphics.Span("Bold ").Bold(),
				graphics.Span("Black").Weight(graphics.FontWeightBlack),
			).Size(16).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Text Decorations
		sectionTitle("Text Decorations", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Underline, overline, and line-through decorations:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("underline").Underline().DecorationColor(colors.Primary),
				graphics.Span("  "),
				graphics.Span("overline").Overline().DecorationColor(colors.Secondary),
				graphics.Span("  "),
				graphics.Span("strikethrough").Strikethrough().DecorationColor(colors.Error),
			).Size(16).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Decoration Styles
		sectionTitle("Decoration Styles", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Solid, double, dotted, dashed, and wavy:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("solid").Underline().DecorationColor(colors.Primary),
				graphics.Span("  "),
				graphics.Span("double").Underline().DecorationColor(colors.Primary).DecorationStyle(graphics.TextDecorationStyleDouble),
				graphics.Span("  "),
				graphics.Span("dotted").Underline().DecorationColor(colors.Primary).DecorationStyle(graphics.TextDecorationStyleDotted),
				graphics.Span("  "),
				graphics.Span("dashed").Underline().DecorationColor(colors.Primary).DecorationStyle(graphics.TextDecorationStyleDashed),
				graphics.Span("  "),
				graphics.Span("wavy").Underline().DecorationColor(colors.Primary).DecorationStyle(graphics.TextDecorationStyleWavy),
			).Size(16).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Mixed Sizes
		sectionTitle("Mixed Sizes", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Different font sizes within a paragraph:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("small ").Size(11),
				graphics.Span("normal ").Size(16),
				graphics.Span("large ").Size(22),
				graphics.Span("huge").Size(30),
			).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Letter & Word Spacing
		sectionTitle("Spacing", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Letter spacing and word spacing within a paragraph:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("normal "),
				graphics.Span("spaced").LetterSpacing(4).Color(colors.Primary),
				graphics.Span(" then "),
				graphics.Span("wide words").WordSpacing(10).Color(colors.Tertiary),
			).Size(16).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Background Color
		sectionTitle("Background Highlight", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Spans can have background colors for highlighting:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("Add "),
				graphics.Span("color").Color(colors.OnPrimaryContainer).Background(colors.PrimaryContainer),
				graphics.Span(" or "),
				graphics.Span("emphasis").Color(colors.OnTertiaryContainer).Background(colors.TertiaryContainer),
			).Size(16).Color(colors.OnSurface),
		}),
		widgets.VSpace(24),

		// Section: Wrapping Paragraph (using theme.RichTextOf)
		sectionTitle("Wrapping Paragraph", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "theme.RichTextOf applies themed color and size, inherited by children:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, theme.RichTextOf(ctx,
			graphics.Span("The "),
			graphics.Span("RichText").Bold().Color(colors.Primary),
			graphics.Span(" widget renders a tree of "),
			graphics.Span("TextSpan").Weight(graphics.FontWeightSemibold).Color(colors.Tertiary),
			graphics.Span(" nodes. Each span can set "),
			graphics.Span("color").Color(colors.Error),
			graphics.Span(", "),
			graphics.Span("weight").Weight(graphics.FontWeightBlack),
			graphics.Span(", "),
			graphics.Span("size").Size(20),
			graphics.Span(", and "),
			graphics.Span("decorations").Underline().DecorationColor(colors.Primary).DecorationStyle(graphics.TextDecorationStyleWavy),
			graphics.Span(" independently."),
		)),
		widgets.VSpace(24),

		// Section: Centered Alignment
		sectionTitle("Text Alignment", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Rich text supports paragraph alignment:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, widgets.RichText{
			Content: graphics.Spans(
				graphics.Span("This paragraph is "),
				graphics.Span("center-aligned").Bold().Color(colors.Primary),
				graphics.Span(" and wraps at the constraint width. Each line is centered within the available space."),
			).Size(15).Color(colors.OnSurface),
			Wrap:  true,
			Align: graphics.TextAlignCenter,
		}),
		widgets.VSpace(24),

		// Section: Themed Typography (using theme.RichTextOf with With* chaining)
		sectionTitle("Themed Typography", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Use theme.RichTextOf with .WithAlign() for themed defaults:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		richTextCard(colors, theme.RichTextOf(ctx,
			graphics.Span("Headline").Size(textTheme.HeadlineMedium.FontSize),
			graphics.Span(" meets "),
			graphics.Span("body text").Size(textTheme.BodyLarge.FontSize),
			graphics.Span(" in one paragraph."),
		).WithAlign(graphics.TextAlignCenter)),
		widgets.VSpace(40),
	)
}

// richTextCard wraps a widget in a styled card.
func richTextCard(colors theme.ColorScheme, content core.Widget) core.Widget {
	return widgets.Container{
		Color:        colors.SurfaceContainer,
		BorderRadius: 12,
		Padding:      layout.EdgeInsetsAll(16),
		Child:        content,
	}
}
