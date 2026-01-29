package main

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
)

// Seed colors for gradients - constant across both themes
var (
	PinkSeed = graphics.RGB(238, 23, 130) // #EE1782
	CyanSeed = graphics.RGB(47, 249, 238) // #2FF9EE
)

// ShowcaseDarkColorScheme returns the dark theme color scheme.
// Matches the colors from the HTML prototype.
func ShowcaseDarkColorScheme() theme.ColorScheme {
	return theme.ColorScheme{
		// Primary - Pink
		Primary:            graphics.RGB(255, 177, 200), // #FFB1C8
		OnPrimary:          graphics.RGB(101, 0, 51),    // #650033
		PrimaryContainer:   graphics.RGB(255, 72, 150),  // #FF4896
		OnPrimaryContainer: graphics.RGB(45, 0, 19),     // #2D0013

		// Secondary - Pink (similar to primary)
		Secondary:            graphics.RGB(255, 177, 200), // #FFB1C8
		OnSecondary:          graphics.RGB(101, 0, 51),    // #650033
		SecondaryContainer:   graphics.RGB(136, 21, 74),   // #88154A
		OnSecondaryContainer: graphics.RGB(255, 149, 184), // #FF95B8

		// Tertiary - Cyan
		Tertiary:            graphics.RGB(47, 249, 238), // #2FF9EE
		OnTertiary:          graphics.RGB(0, 55, 52),    // #003734
		TertiaryContainer:   graphics.RGB(0, 221, 211),  // #00DDD3 (tertiary-dim)
		OnTertiaryContainer: graphics.RGB(0, 111, 106),  // #006F6A

		// Surface - from prototype
		Surface:                 graphics.RGB(21, 19, 19),    // #151313
		OnSurface:               graphics.RGB(231, 225, 225), // #E7E1E1
		SurfaceVariant:          graphics.RGB(69, 71, 75),    // #45474B
		OnSurfaceVariant:        graphics.RGB(197, 198, 204), // #C5C6CC
		SurfaceDim:              graphics.RGB(21, 19, 19),    // #151313
		SurfaceBright:           graphics.RGB(58, 57, 57),    // #3A3939
		SurfaceContainerLowest:  graphics.RGB(15, 14, 14),    // #0F0E0E
		SurfaceContainerLow:     graphics.RGB(28, 27, 27),    // #1C1B1B
		SurfaceContainer:        graphics.RGB(33, 31, 32),    // #211F20
		SurfaceContainerHigh:    graphics.RGB(43, 41, 42),    // #2B292A
		SurfaceContainerHighest: graphics.RGB(54, 52, 53),    // #363435

		// Background - Dark with pink tint
		Background:   graphics.RGB(20, 15, 20),    // #1E0F14
		OnBackground: graphics.RGB(231, 225, 225), // #E7E1E1

		// Error
		Error:            graphics.RGB(255, 179, 175), // #FFB3AF
		OnError:          graphics.RGB(104, 0, 14),    // #68000E
		ErrorContainer:   graphics.RGB(253, 86, 88),   // #FD5658
		OnErrorContainer: graphics.RGB(70, 0, 6),      // #460006

		// Outline
		Outline:        graphics.RGB(143, 144, 150), // #8F9096
		OutlineVariant: graphics.RGB(69, 71, 75),    // #45474B

		// Shadow and Scrim
		Shadow: graphics.RGB(0, 0, 0),
		Scrim:  graphics.RGB(0, 0, 0),

		// Inverse
		InverseSurface:   graphics.RGB(231, 225, 225), // #E7E1E1
		OnInverseSurface: graphics.RGB(50, 48, 48),    // #323030
		InversePrimary:   graphics.RGB(185, 0, 99),    // #B90063

		// Surface Tint
		SurfaceTint: graphics.RGB(255, 177, 200), // #FFB1C8

		Brightness: theme.BrightnessDark,
	}
}

// ShowcaseDarkTheme returns a ThemeData using the dark color scheme.
func ShowcaseDarkTheme() *theme.ThemeData {
	colors := ShowcaseDarkColorScheme()
	return &theme.ThemeData{
		ColorScheme: colors,
		TextTheme:   theme.DefaultTextTheme(colors.OnBackground),
		Brightness:  theme.BrightnessDark,
	}
}

// ShowcaseLightColorScheme returns the light theme color scheme.
// Matches the colors from the HTML prototype.
func ShowcaseLightColorScheme() theme.ColorScheme {
	return theme.ColorScheme{
		// Primary - Deep pink
		Primary:            graphics.RGB(181, 0, 96),    // #B50060
		OnPrimary:          graphics.RGB(255, 255, 255), // #FFFFFF
		PrimaryContainer:   graphics.RGB(226, 0, 122),   // #E2007A
		OnPrimaryContainer: graphics.RGB(255, 255, 255), // #FFFFFF

		// Secondary
		Secondary:            graphics.RGB(181, 0, 96),    // #B50060
		OnSecondary:          graphics.RGB(255, 255, 255), // #FFFFFF
		SecondaryContainer:   graphics.RGB(255, 216, 227), // #FFD8E3
		OnSecondaryContainer: graphics.RGB(62, 0, 31),     // #3E001F

		// Tertiary - Teal
		Tertiary:            graphics.RGB(0, 106, 101),   // #006A65
		OnTertiary:          graphics.RGB(255, 255, 255), // #FFFFFF
		TertiaryContainer:   graphics.RGB(0, 80, 76),     // #00504C
		OnTertiaryContainer: graphics.RGB(255, 255, 255), // #FFFFFF

		// Surface
		Surface:                 graphics.RGB(254, 248, 248), // #FEF8F8
		OnSurface:               graphics.RGB(29, 27, 28),    // #1D1B1C
		SurfaceVariant:          graphics.RGB(226, 226, 232), // #E2E2E8
		OnSurfaceVariant:        graphics.RGB(69, 71, 75),    // #45474B
		SurfaceDim:              graphics.RGB(222, 216, 216), // #DED8D8
		SurfaceBright:           graphics.RGB(254, 248, 248), // #FEF8F8
		SurfaceContainerLowest:  graphics.RGB(255, 255, 255), // #FFFFFF
		SurfaceContainerLow:     graphics.RGB(248, 242, 242), // #F8F2F2
		SurfaceContainer:        graphics.RGB(242, 236, 237), // #F2ECED
		SurfaceContainerHigh:    graphics.RGB(236, 231, 231), // #ECE7E7
		SurfaceContainerHighest: graphics.RGB(231, 225, 225), // #E7E1E1

		// Background
		Background:   graphics.RGB(255, 248, 248), // #FFF8F8
		OnBackground: graphics.RGB(29, 27, 28),    // #1D1B1C

		// Error
		Error:            graphics.RGB(186, 26, 26),   // #BA1A1A
		OnError:          graphics.RGB(255, 255, 255), // #FFFFFF
		ErrorContainer:   graphics.RGB(255, 218, 214), // #FFDAD6
		OnErrorContainer: graphics.RGB(65, 0, 2),      // #410002

		// Outline
		Outline:        graphics.RGB(117, 119, 124), // #75777C
		OutlineVariant: graphics.RGB(197, 198, 204), // #C5C6CC

		// Shadow and Scrim
		Shadow: graphics.RGB(0, 0, 0),
		Scrim:  graphics.RGB(0, 0, 0),

		// Inverse
		InverseSurface:   graphics.RGB(50, 48, 49),    // #323031
		OnInverseSurface: graphics.RGB(246, 240, 240), // #F6F0F0
		InversePrimary:   graphics.RGB(255, 177, 200), // #FFB1C8

		// Surface Tint
		SurfaceTint: graphics.RGB(181, 0, 96), // #B50060

		Brightness: theme.BrightnessLight,
	}
}

// ShowcaseLightTheme returns a ThemeData using the light color scheme.
func ShowcaseLightTheme() *theme.ThemeData {
	colors := ShowcaseLightColorScheme()
	return &theme.ThemeData{
		ColorScheme: colors,
		TextTheme:   theme.DefaultTextTheme(colors.OnBackground),
		Brightness:  theme.BrightnessLight,
	}
}
