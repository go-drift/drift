package theme

import "github.com/go-drift/drift/pkg/rendering"

// TextTheme defines text styles for various purposes.
// Based on Material Design 3 type scale.
type TextTheme struct {
	// Display styles are for short, important text.
	DisplayLarge  rendering.TextStyle
	DisplayMedium rendering.TextStyle
	DisplaySmall  rendering.TextStyle

	// Headline styles are for high-emphasis text.
	HeadlineLarge  rendering.TextStyle
	HeadlineMedium rendering.TextStyle
	HeadlineSmall  rendering.TextStyle

	// Title styles are for medium-emphasis text.
	TitleLarge  rendering.TextStyle
	TitleMedium rendering.TextStyle
	TitleSmall  rendering.TextStyle

	// Body styles are for long-form text.
	BodyLarge  rendering.TextStyle
	BodyMedium rendering.TextStyle
	BodySmall  rendering.TextStyle

	// Label styles are for small text like buttons.
	LabelLarge  rendering.TextStyle
	LabelMedium rendering.TextStyle
	LabelSmall  rendering.TextStyle
}

// DefaultTextTheme creates a TextTheme with default sizes and the given text color.
func DefaultTextTheme(textColor rendering.Color) TextTheme {
	return TextTheme{
		DisplayLarge: rendering.TextStyle{
			Color:    textColor,
			FontSize: 57,
		},
		DisplayMedium: rendering.TextStyle{
			Color:    textColor,
			FontSize: 45,
		},
		DisplaySmall: rendering.TextStyle{
			Color:    textColor,
			FontSize: 36,
		},
		HeadlineLarge: rendering.TextStyle{
			Color:    textColor,
			FontSize: 32,
		},
		HeadlineMedium: rendering.TextStyle{
			Color:    textColor,
			FontSize: 28,
		},
		HeadlineSmall: rendering.TextStyle{
			Color:    textColor,
			FontSize: 24,
		},
		TitleLarge: rendering.TextStyle{
			Color:    textColor,
			FontSize: 22,
		},
		TitleMedium: rendering.TextStyle{
			Color:    textColor,
			FontSize: 16,
		},
		TitleSmall: rendering.TextStyle{
			Color:    textColor,
			FontSize: 14,
		},
		BodyLarge: rendering.TextStyle{
			Color:    textColor,
			FontSize: 16,
		},
		BodyMedium: rendering.TextStyle{
			Color:    textColor,
			FontSize: 14,
		},
		BodySmall: rendering.TextStyle{
			Color:    textColor,
			FontSize: 12,
		},
		LabelLarge: rendering.TextStyle{
			Color:    textColor,
			FontSize: 14,
		},
		LabelMedium: rendering.TextStyle{
			Color:    textColor,
			FontSize: 12,
		},
		LabelSmall: rendering.TextStyle{
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

func scaleTextStyle(style rendering.TextStyle, scale float64) rendering.TextStyle {
	style.FontSize = style.FontSize * scale
	return style
}
