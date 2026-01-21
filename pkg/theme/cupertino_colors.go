package theme

import "github.com/go-drift/drift/pkg/rendering"

// CupertinoColors defines the iOS-style color palette.
// Based on Apple's Human Interface Guidelines system colors.
type CupertinoColors struct {
	// System colors - vibrant, accessible colors
	SystemBlue   rendering.Color
	SystemGreen  rendering.Color
	SystemIndigo rendering.Color
	SystemOrange rendering.Color
	SystemPink   rendering.Color
	SystemPurple rendering.Color
	SystemRed    rendering.Color
	SystemTeal   rendering.Color
	SystemYellow rendering.Color

	// Gray colors - semantic grays for various uses
	SystemGray  rendering.Color
	SystemGray2 rendering.Color
	SystemGray3 rendering.Color
	SystemGray4 rendering.Color
	SystemGray5 rendering.Color
	SystemGray6 rendering.Color

	// Background colors - for view hierarchies
	SystemBackground          rendering.Color
	SecondarySystemBackground rendering.Color
	TertiarySystemBackground  rendering.Color

	// Grouped background colors - for grouped content
	SystemGroupedBackground          rendering.Color
	SecondarySystemGroupedBackground rendering.Color
	TertiarySystemGroupedBackground  rendering.Color

	// Label colors - for text content
	Label           rendering.Color
	SecondaryLabel  rendering.Color
	TertiaryLabel   rendering.Color
	QuaternaryLabel rendering.Color

	// Fill colors - for shapes and controls
	SystemFill           rendering.Color
	SecondarySystemFill  rendering.Color
	TertiarySystemFill   rendering.Color
	QuaternarySystemFill rendering.Color

	// Separator colors - for dividers
	Separator       rendering.Color
	OpaqueSeparator rendering.Color

	// Link color
	Link rendering.Color

	// Placeholder text color
	PlaceholderText rendering.Color
}

// LightCupertinoColors returns the iOS light mode color palette.
func LightCupertinoColors() CupertinoColors {
	return CupertinoColors{
		// System colors (iOS light mode)
		SystemBlue:   rendering.RGB(0, 122, 255),
		SystemGreen:  rendering.RGB(52, 199, 89),
		SystemIndigo: rendering.RGB(88, 86, 214),
		SystemOrange: rendering.RGB(255, 149, 0),
		SystemPink:   rendering.RGB(255, 45, 85),
		SystemPurple: rendering.RGB(175, 82, 222),
		SystemRed:    rendering.RGB(255, 59, 48),
		SystemTeal:   rendering.RGB(90, 200, 250),
		SystemYellow: rendering.RGB(255, 204, 0),

		// Gray colors (iOS light mode)
		SystemGray:  rendering.RGB(142, 142, 147),
		SystemGray2: rendering.RGB(174, 174, 178),
		SystemGray3: rendering.RGB(199, 199, 204),
		SystemGray4: rendering.RGB(209, 209, 214),
		SystemGray5: rendering.RGB(229, 229, 234),
		SystemGray6: rendering.RGB(242, 242, 247),

		// Background colors (iOS light mode)
		SystemBackground:          rendering.RGB(255, 255, 255),
		SecondarySystemBackground: rendering.RGB(242, 242, 247),
		TertiarySystemBackground:  rendering.RGB(255, 255, 255),

		// Grouped background colors (iOS light mode)
		SystemGroupedBackground:          rendering.RGB(242, 242, 247),
		SecondarySystemGroupedBackground: rendering.RGB(255, 255, 255),
		TertiarySystemGroupedBackground:  rendering.RGB(242, 242, 247),

		// Label colors (iOS light mode)
		Label:           rendering.RGBA(0, 0, 0, 255),
		SecondaryLabel:  rendering.RGBA(60, 60, 67, 153),
		TertiaryLabel:   rendering.RGBA(60, 60, 67, 76),
		QuaternaryLabel: rendering.RGBA(60, 60, 67, 45),

		// Fill colors (iOS light mode)
		SystemFill:           rendering.RGBA(120, 120, 128, 51),
		SecondarySystemFill:  rendering.RGBA(120, 120, 128, 40),
		TertiarySystemFill:   rendering.RGBA(118, 118, 128, 30),
		QuaternarySystemFill: rendering.RGBA(116, 116, 128, 20),

		// Separator colors (iOS light mode)
		Separator:       rendering.RGBA(60, 60, 67, 73),
		OpaqueSeparator: rendering.RGB(198, 198, 200),

		// Link color
		Link: rendering.RGB(0, 122, 255),

		// Placeholder text
		PlaceholderText: rendering.RGBA(60, 60, 67, 76),
	}
}

// DarkCupertinoColors returns the iOS dark mode color palette.
func DarkCupertinoColors() CupertinoColors {
	return CupertinoColors{
		// System colors (iOS dark mode)
		SystemBlue:   rendering.RGB(10, 132, 255),
		SystemGreen:  rendering.RGB(48, 209, 88),
		SystemIndigo: rendering.RGB(94, 92, 230),
		SystemOrange: rendering.RGB(255, 159, 10),
		SystemPink:   rendering.RGB(255, 55, 95),
		SystemPurple: rendering.RGB(191, 90, 242),
		SystemRed:    rendering.RGB(255, 69, 58),
		SystemTeal:   rendering.RGB(100, 210, 255),
		SystemYellow: rendering.RGB(255, 214, 10),

		// Gray colors (iOS dark mode)
		SystemGray:  rendering.RGB(142, 142, 147),
		SystemGray2: rendering.RGB(99, 99, 102),
		SystemGray3: rendering.RGB(72, 72, 74),
		SystemGray4: rendering.RGB(58, 58, 60),
		SystemGray5: rendering.RGB(44, 44, 46),
		SystemGray6: rendering.RGB(28, 28, 30),

		// Background colors (iOS dark mode)
		SystemBackground:          rendering.RGB(0, 0, 0),
		SecondarySystemBackground: rendering.RGB(28, 28, 30),
		TertiarySystemBackground:  rendering.RGB(44, 44, 46),

		// Grouped background colors (iOS dark mode)
		SystemGroupedBackground:          rendering.RGB(0, 0, 0),
		SecondarySystemGroupedBackground: rendering.RGB(28, 28, 30),
		TertiarySystemGroupedBackground:  rendering.RGB(44, 44, 46),

		// Label colors (iOS dark mode)
		Label:           rendering.RGBA(255, 255, 255, 255),
		SecondaryLabel:  rendering.RGBA(235, 235, 245, 153),
		TertiaryLabel:   rendering.RGBA(235, 235, 245, 76),
		QuaternaryLabel: rendering.RGBA(235, 235, 245, 45),

		// Fill colors (iOS dark mode)
		SystemFill:           rendering.RGBA(120, 120, 128, 91),
		SecondarySystemFill:  rendering.RGBA(120, 120, 128, 81),
		TertiarySystemFill:   rendering.RGBA(118, 118, 128, 61),
		QuaternarySystemFill: rendering.RGBA(118, 118, 128, 45),

		// Separator colors (iOS dark mode)
		Separator:       rendering.RGBA(84, 84, 88, 153),
		OpaqueSeparator: rendering.RGB(56, 56, 58),

		// Link color
		Link: rendering.RGB(10, 132, 255),

		// Placeholder text
		PlaceholderText: rendering.RGBA(235, 235, 245, 76),
	}
}
