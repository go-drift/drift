package theme

import "github.com/go-drift/drift/pkg/rendering"

// CupertinoTextThemeData defines text styles for iOS-style interfaces.
// Based on Apple's Human Interface Guidelines typography.
type CupertinoTextThemeData struct {
	// TextStyle is the default body text style.
	TextStyle rendering.TextStyle

	// ActionTextStyle is for interactive text elements.
	ActionTextStyle rendering.TextStyle

	// NavTitleTextStyle is for navigation bar titles.
	NavTitleTextStyle rendering.TextStyle

	// NavLargeTitleTextStyle is for large navigation bar titles.
	NavLargeTitleTextStyle rendering.TextStyle

	// TabLabelTextStyle is for tab bar labels.
	TabLabelTextStyle rendering.TextStyle

	// PickerTextStyle is for picker/wheel items.
	PickerTextStyle rendering.TextStyle

	// DateTimePickerTextStyle is for date/time picker items.
	DateTimePickerTextStyle rendering.TextStyle

	// LargeTitleTextStyle is for large prominent text.
	LargeTitleTextStyle rendering.TextStyle

	// Title1TextStyle is for title level 1.
	Title1TextStyle rendering.TextStyle

	// Title2TextStyle is for title level 2.
	Title2TextStyle rendering.TextStyle

	// Title3TextStyle is for title level 3.
	Title3TextStyle rendering.TextStyle

	// HeadlineTextStyle is for headline text.
	HeadlineTextStyle rendering.TextStyle

	// SubheadlineTextStyle is for subheadline text.
	SubheadlineTextStyle rendering.TextStyle

	// BodyTextStyle is for body text.
	BodyTextStyle rendering.TextStyle

	// CalloutTextStyle is for callout text.
	CalloutTextStyle rendering.TextStyle

	// FootnoteTextStyle is for footnote text.
	FootnoteTextStyle rendering.TextStyle

	// Caption1TextStyle is for caption level 1.
	Caption1TextStyle rendering.TextStyle

	// Caption2TextStyle is for caption level 2.
	Caption2TextStyle rendering.TextStyle
}

// DefaultCupertinoTextTheme creates a text theme with iOS-style defaults.
func DefaultCupertinoTextTheme(textColor rendering.Color) CupertinoTextThemeData {
	return CupertinoTextThemeData{
		TextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 17,
		},
		ActionTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 17,
		},
		NavTitleTextStyle: rendering.TextStyle{
			Color:      textColor,
			FontSize:   17,
			FontWeight: rendering.FontWeightSemibold,
		},
		NavLargeTitleTextStyle: rendering.TextStyle{
			Color:      textColor,
			FontSize:   34,
			FontWeight: rendering.FontWeightBold,
		},
		TabLabelTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 10,
		},
		PickerTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 21,
		},
		DateTimePickerTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 21,
		},
		LargeTitleTextStyle: rendering.TextStyle{
			Color:      textColor,
			FontSize:   34,
			FontWeight: rendering.FontWeightBold,
		},
		Title1TextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 28,
		},
		Title2TextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 22,
		},
		Title3TextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 20,
		},
		HeadlineTextStyle: rendering.TextStyle{
			Color:      textColor,
			FontSize:   17,
			FontWeight: rendering.FontWeightSemibold,
		},
		SubheadlineTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 15,
		},
		BodyTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 17,
		},
		CalloutTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 16,
		},
		FootnoteTextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 13,
		},
		Caption1TextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 12,
		},
		Caption2TextStyle: rendering.TextStyle{
			Color:    textColor,
			FontSize: 11,
		},
	}
}
