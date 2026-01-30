package theme

import "github.com/go-drift/drift/pkg/graphics"

// CupertinoColors defines the iOS-style color palette.
// Based on Apple's Human Interface Guidelines system colors.
type CupertinoColors struct {
	// System colors - vibrant, accessible colors
	SystemBlue   graphics.Color
	SystemGreen  graphics.Color
	SystemIndigo graphics.Color
	SystemOrange graphics.Color
	SystemPink   graphics.Color
	SystemPurple graphics.Color
	SystemRed    graphics.Color
	SystemTeal   graphics.Color
	SystemYellow graphics.Color

	// Gray colors - semantic grays for various uses
	SystemGray  graphics.Color
	SystemGray2 graphics.Color
	SystemGray3 graphics.Color
	SystemGray4 graphics.Color
	SystemGray5 graphics.Color
	SystemGray6 graphics.Color

	// Background colors - for view hierarchies
	SystemBackground          graphics.Color
	SecondarySystemBackground graphics.Color
	TertiarySystemBackground  graphics.Color

	// Grouped background colors - for grouped content
	SystemGroupedBackground          graphics.Color
	SecondarySystemGroupedBackground graphics.Color
	TertiarySystemGroupedBackground  graphics.Color

	// Label colors - for text content
	Label           graphics.Color
	SecondaryLabel  graphics.Color
	TertiaryLabel   graphics.Color
	QuaternaryLabel graphics.Color

	// Fill colors - for shapes and controls
	SystemFill           graphics.Color
	SecondarySystemFill  graphics.Color
	TertiarySystemFill   graphics.Color
	QuaternarySystemFill graphics.Color

	// Separator colors - for dividers
	Separator       graphics.Color
	OpaqueSeparator graphics.Color

	// Link color
	Link graphics.Color

	// Placeholder text color
	PlaceholderText graphics.Color
}

// LightCupertinoColors returns the iOS light mode color palette.
func LightCupertinoColors() CupertinoColors {
	return CupertinoColors{
		// System colors (iOS light mode)
		SystemBlue:   graphics.RGB(0, 122, 255),
		SystemGreen:  graphics.RGB(52, 199, 89),
		SystemIndigo: graphics.RGB(88, 86, 214),
		SystemOrange: graphics.RGB(255, 149, 0),
		SystemPink:   graphics.RGB(255, 45, 85),
		SystemPurple: graphics.RGB(175, 82, 222),
		SystemRed:    graphics.RGB(255, 59, 48),
		SystemTeal:   graphics.RGB(90, 200, 250),
		SystemYellow: graphics.RGB(255, 204, 0),

		// Gray colors (iOS light mode)
		SystemGray:  graphics.RGB(142, 142, 147),
		SystemGray2: graphics.RGB(174, 174, 178),
		SystemGray3: graphics.RGB(199, 199, 204),
		SystemGray4: graphics.RGB(209, 209, 214),
		SystemGray5: graphics.RGB(229, 229, 234),
		SystemGray6: graphics.RGB(242, 242, 247),

		// Background colors (iOS light mode)
		SystemBackground:          graphics.RGB(255, 255, 255),
		SecondarySystemBackground: graphics.RGB(242, 242, 247),
		TertiarySystemBackground:  graphics.RGB(255, 255, 255),

		// Grouped background colors (iOS light mode)
		SystemGroupedBackground:          graphics.RGB(242, 242, 247),
		SecondarySystemGroupedBackground: graphics.RGB(255, 255, 255),
		TertiarySystemGroupedBackground:  graphics.RGB(242, 242, 247),

		// Label colors (iOS light mode)
		Label:           graphics.RGBA(0, 0, 0, 1.0),
		SecondaryLabel:  graphics.RGBA(60, 60, 67, 0.6),
		TertiaryLabel:   graphics.RGBA(60, 60, 67, 0.3),
		QuaternaryLabel: graphics.RGBA(60, 60, 67, 0.18),

		// Fill colors (iOS light mode)
		SystemFill:           graphics.RGBA(120, 120, 128, 0.2),
		SecondarySystemFill:  graphics.RGBA(120, 120, 128, 0.16),
		TertiarySystemFill:   graphics.RGBA(118, 118, 128, 0.12),
		QuaternarySystemFill: graphics.RGBA(116, 116, 128, 0.08),

		// Separator colors (iOS light mode)
		Separator:       graphics.RGBA(60, 60, 67, 0.29),
		OpaqueSeparator: graphics.RGB(198, 198, 200),

		// Link color
		Link: graphics.RGB(0, 122, 255),

		// Placeholder text
		PlaceholderText: graphics.RGBA(60, 60, 67, 0.3),
	}
}

// DarkCupertinoColors returns the iOS dark mode color palette.
func DarkCupertinoColors() CupertinoColors {
	return CupertinoColors{
		// System colors (iOS dark mode)
		SystemBlue:   graphics.RGB(10, 132, 255),
		SystemGreen:  graphics.RGB(48, 209, 88),
		SystemIndigo: graphics.RGB(94, 92, 230),
		SystemOrange: graphics.RGB(255, 159, 10),
		SystemPink:   graphics.RGB(255, 55, 95),
		SystemPurple: graphics.RGB(191, 90, 242),
		SystemRed:    graphics.RGB(255, 69, 58),
		SystemTeal:   graphics.RGB(100, 210, 255),
		SystemYellow: graphics.RGB(255, 214, 10),

		// Gray colors (iOS dark mode)
		SystemGray:  graphics.RGB(142, 142, 147),
		SystemGray2: graphics.RGB(99, 99, 102),
		SystemGray3: graphics.RGB(72, 72, 74),
		SystemGray4: graphics.RGB(58, 58, 60),
		SystemGray5: graphics.RGB(44, 44, 46),
		SystemGray6: graphics.RGB(28, 28, 30),

		// Background colors (iOS dark mode)
		SystemBackground:          graphics.RGB(0, 0, 0),
		SecondarySystemBackground: graphics.RGB(28, 28, 30),
		TertiarySystemBackground:  graphics.RGB(44, 44, 46),

		// Grouped background colors (iOS dark mode)
		SystemGroupedBackground:          graphics.RGB(0, 0, 0),
		SecondarySystemGroupedBackground: graphics.RGB(28, 28, 30),
		TertiarySystemGroupedBackground:  graphics.RGB(44, 44, 46),

		// Label colors (iOS dark mode)
		Label:           graphics.RGBA(255, 255, 255, 1.0),
		SecondaryLabel:  graphics.RGBA(235, 235, 245, 0.6),
		TertiaryLabel:   graphics.RGBA(235, 235, 245, 0.3),
		QuaternaryLabel: graphics.RGBA(235, 235, 245, 0.18),

		// Fill colors (iOS dark mode)
		SystemFill:           graphics.RGBA(120, 120, 128, 0.36),
		SecondarySystemFill:  graphics.RGBA(120, 120, 128, 0.32),
		TertiarySystemFill:   graphics.RGBA(118, 118, 128, 0.24),
		QuaternarySystemFill: graphics.RGBA(118, 118, 128, 0.18),

		// Separator colors (iOS dark mode)
		Separator:       graphics.RGBA(84, 84, 88, 0.6),
		OpaqueSeparator: graphics.RGB(56, 56, 58),

		// Link color
		Link: graphics.RGB(10, 132, 255),

		// Placeholder text
		PlaceholderText: graphics.RGBA(235, 235, 245, 0.3),
	}
}
