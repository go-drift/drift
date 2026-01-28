package theme

import "github.com/go-drift/drift/pkg/graphics"

// TextTheme defines text styles for various purposes.
// Based on Material Design 3 type scale.
type TextTheme struct {
	// Display styles are for short, important text.
	DisplayLarge  graphics.TextStyle
	DisplayMedium graphics.TextStyle
	DisplaySmall  graphics.TextStyle

	// Headline styles are for high-emphasis text.
	HeadlineLarge  graphics.TextStyle
	HeadlineMedium graphics.TextStyle
	HeadlineSmall  graphics.TextStyle

	// Title styles are for medium-emphasis text.
	TitleLarge  graphics.TextStyle
	TitleMedium graphics.TextStyle
	TitleSmall  graphics.TextStyle

	// Body styles are for long-form text.
	BodyLarge  graphics.TextStyle
	BodyMedium graphics.TextStyle
	BodySmall  graphics.TextStyle

	// Label styles are for small text like buttons.
	LabelLarge  graphics.TextStyle
	LabelMedium graphics.TextStyle
	LabelSmall  graphics.TextStyle
}

// DefaultTextTheme creates a TextTheme with default sizes and the given text color.
func DefaultTextTheme(textColor graphics.Color) TextTheme {
	return TextTheme{
		DisplayLarge: graphics.TextStyle{
			Color:    textColor,
			FontSize: 57,
		},
		DisplayMedium: graphics.TextStyle{
			Color:    textColor,
			FontSize: 45,
		},
		DisplaySmall: graphics.TextStyle{
			Color:    textColor,
			FontSize: 36,
		},
		HeadlineLarge: graphics.TextStyle{
			Color:    textColor,
			FontSize: 32,
		},
		HeadlineMedium: graphics.TextStyle{
			Color:    textColor,
			FontSize: 28,
		},
		HeadlineSmall: graphics.TextStyle{
			Color:    textColor,
			FontSize: 24,
		},
		TitleLarge: graphics.TextStyle{
			Color:    textColor,
			FontSize: 22,
		},
		TitleMedium: graphics.TextStyle{
			Color:    textColor,
			FontSize: 16,
		},
		TitleSmall: graphics.TextStyle{
			Color:    textColor,
			FontSize: 14,
		},
		BodyLarge: graphics.TextStyle{
			Color:    textColor,
			FontSize: 16,
		},
		BodyMedium: graphics.TextStyle{
			Color:    textColor,
			FontSize: 14,
		},
		BodySmall: graphics.TextStyle{
			Color:    textColor,
			FontSize: 12,
		},
		LabelLarge: graphics.TextStyle{
			Color:    textColor,
			FontSize: 14,
		},
		LabelMedium: graphics.TextStyle{
			Color:    textColor,
			FontSize: 12,
		},
		LabelSmall: graphics.TextStyle{
			Color:    textColor,
			FontSize: 11,
		},
	}
}

// Apply applies a scale factor to all text sizes in the theme.
func (t TextTheme) Apply(scale float64) TextTheme {
	return TextTheme{
		DisplayLarge:   scaleTextStyle(t.DisplayLarge, scale),
		DisplayMedium:  scaleTextStyle(t.DisplayMedium, scale),
		DisplaySmall:   scaleTextStyle(t.DisplaySmall, scale),
		HeadlineLarge:  scaleTextStyle(t.HeadlineLarge, scale),
		HeadlineMedium: scaleTextStyle(t.HeadlineMedium, scale),
		HeadlineSmall:  scaleTextStyle(t.HeadlineSmall, scale),
		TitleLarge:     scaleTextStyle(t.TitleLarge, scale),
		TitleMedium:    scaleTextStyle(t.TitleMedium, scale),
		TitleSmall:     scaleTextStyle(t.TitleSmall, scale),
		BodyLarge:      scaleTextStyle(t.BodyLarge, scale),
		BodyMedium:     scaleTextStyle(t.BodyMedium, scale),
		BodySmall:      scaleTextStyle(t.BodySmall, scale),
		LabelLarge:     scaleTextStyle(t.LabelLarge, scale),
		LabelMedium:    scaleTextStyle(t.LabelMedium, scale),
		LabelSmall:     scaleTextStyle(t.LabelSmall, scale),
	}
}

func scaleTextStyle(style graphics.TextStyle, scale float64) graphics.TextStyle {
	style.FontSize = style.FontSize * scale
	return style
}
