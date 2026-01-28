package theme

import "github.com/go-drift/drift/pkg/graphics"

// CupertinoTextThemeData defines text styles for iOS-style interfaces.
// Based on Apple's Human Interface Guidelines typography.
type CupertinoTextThemeData struct {
	// TextStyle is the default body text style.
	TextStyle graphics.TextStyle

	// ActionTextStyle is for interactive text elements.
	ActionTextStyle graphics.TextStyle

	// NavTitleTextStyle is for navigation bar titles.
	NavTitleTextStyle graphics.TextStyle

	// NavLargeTitleTextStyle is for large navigation bar titles.
	NavLargeTitleTextStyle graphics.TextStyle

	// TabLabelTextStyle is for tab bar labels.
	TabLabelTextStyle graphics.TextStyle

	// PickerTextStyle is for picker/wheel items.
	PickerTextStyle graphics.TextStyle

	// DateTimePickerTextStyle is for date/time picker items.
	DateTimePickerTextStyle graphics.TextStyle

	// LargeTitleTextStyle is for large prominent text.
	LargeTitleTextStyle graphics.TextStyle

	// Title1TextStyle is for title level 1.
	Title1TextStyle graphics.TextStyle

	// Title2TextStyle is for title level 2.
	Title2TextStyle graphics.TextStyle

	// Title3TextStyle is for title level 3.
	Title3TextStyle graphics.TextStyle

	// HeadlineTextStyle is for headline text.
	HeadlineTextStyle graphics.TextStyle

	// SubheadlineTextStyle is for subheadline text.
	SubheadlineTextStyle graphics.TextStyle

	// BodyTextStyle is for body text.
	BodyTextStyle graphics.TextStyle

	// CalloutTextStyle is for callout text.
	CalloutTextStyle graphics.TextStyle

	// FootnoteTextStyle is for footnote text.
	FootnoteTextStyle graphics.TextStyle

	// Caption1TextStyle is for caption level 1.
	Caption1TextStyle graphics.TextStyle

	// Caption2TextStyle is for caption level 2.
	Caption2TextStyle graphics.TextStyle
}

// DefaultCupertinoTextTheme creates a text theme with iOS-style defaults.
func DefaultCupertinoTextTheme(textColor graphics.Color) CupertinoTextThemeData {
	return CupertinoTextThemeData{
		TextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 17,
		},
		ActionTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 17,
		},
		NavTitleTextStyle: graphics.TextStyle{
			Color:      textColor,
			FontSize:   17,
			FontWeight: graphics.FontWeightSemibold,
		},
		NavLargeTitleTextStyle: graphics.TextStyle{
			Color:      textColor,
			FontSize:   34,
			FontWeight: graphics.FontWeightBold,
		},
		TabLabelTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 10,
		},
		PickerTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 21,
		},
		DateTimePickerTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 21,
		},
		LargeTitleTextStyle: graphics.TextStyle{
			Color:      textColor,
			FontSize:   34,
			FontWeight: graphics.FontWeightBold,
		},
		Title1TextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 28,
		},
		Title2TextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 22,
		},
		Title3TextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 20,
		},
		HeadlineTextStyle: graphics.TextStyle{
			Color:      textColor,
			FontSize:   17,
			FontWeight: graphics.FontWeightSemibold,
		},
		SubheadlineTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 15,
		},
		BodyTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 17,
		},
		CalloutTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 16,
		},
		FootnoteTextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 13,
		},
		Caption1TextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 12,
		},
		Caption2TextStyle: graphics.TextStyle{
			Color:    textColor,
			FontSize: 11,
		},
	}
}
