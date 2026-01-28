// Package theme provides theming support for the Drift framework.
package theme

import "github.com/go-drift/drift/pkg/graphics"

// Brightness indicates whether a theme is light or dark.
type Brightness int

const (
	// BrightnessLight is a light theme with dark text on light backgrounds.
	BrightnessLight Brightness = iota
	// BrightnessDark is a dark theme with light text on dark backgrounds.
	BrightnessDark
)

// ColorScheme defines the color palette for a theme.
// Based on Material Design 3 color system with full 45-color support.
type ColorScheme struct {
	// Primary is the main brand color.
	Primary graphics.Color
	// OnPrimary is the color for text/icons on primary.
	OnPrimary graphics.Color
	// PrimaryContainer is a tonal container for primary.
	PrimaryContainer graphics.Color
	// OnPrimaryContainer is the color for text/icons on primary container.
	OnPrimaryContainer graphics.Color

	// Secondary is a supporting brand color.
	Secondary graphics.Color
	// OnSecondary is the color for text/icons on secondary.
	OnSecondary graphics.Color
	// SecondaryContainer is a tonal container for secondary.
	SecondaryContainer graphics.Color
	// OnSecondaryContainer is the color for text/icons on secondary container.
	OnSecondaryContainer graphics.Color

	// Tertiary is an accent color for contrast.
	Tertiary graphics.Color
	// OnTertiary is the color for text/icons on tertiary.
	OnTertiary graphics.Color
	// TertiaryContainer is a tonal container for tertiary.
	TertiaryContainer graphics.Color
	// OnTertiaryContainer is the color for text/icons on tertiary container.
	OnTertiaryContainer graphics.Color

	// Surface is the color for cards, sheets, menus.
	Surface graphics.Color
	// OnSurface is the color for text/icons on surface.
	OnSurface graphics.Color
	// SurfaceVariant is an alternative surface color.
	SurfaceVariant graphics.Color
	// OnSurfaceVariant is the color for text/icons on surface variant.
	OnSurfaceVariant graphics.Color

	// SurfaceDim is a dimmed surface for less emphasis.
	SurfaceDim graphics.Color
	// SurfaceBright is a bright surface for more emphasis.
	SurfaceBright graphics.Color
	// SurfaceContainerLowest is the lowest emphasis container surface.
	SurfaceContainerLowest graphics.Color
	// SurfaceContainerLow is a low emphasis container surface.
	SurfaceContainerLow graphics.Color
	// SurfaceContainer is the default container surface.
	SurfaceContainer graphics.Color
	// SurfaceContainerHigh is a high emphasis container surface.
	SurfaceContainerHigh graphics.Color
	// SurfaceContainerHighest is the highest emphasis container surface.
	SurfaceContainerHighest graphics.Color

	// Background is the app background color.
	Background graphics.Color
	// OnBackground is the color for text/icons on background.
	OnBackground graphics.Color

	// Error is the color for error states.
	Error graphics.Color
	// OnError is the color for text/icons on error.
	OnError graphics.Color
	// ErrorContainer is a tonal container for error.
	ErrorContainer graphics.Color
	// OnErrorContainer is the color for text/icons on error container.
	OnErrorContainer graphics.Color

	// Outline is the color for borders and dividers.
	Outline graphics.Color
	// OutlineVariant is a subtle outline for decorative elements.
	OutlineVariant graphics.Color

	// Shadow is the color for elevation shadows.
	Shadow graphics.Color
	// Scrim is the color for modal barriers.
	Scrim graphics.Color

	// InverseSurface is the surface color for inverse contexts.
	InverseSurface graphics.Color
	// OnInverseSurface is the color for text/icons on inverse surface.
	OnInverseSurface graphics.Color
	// InversePrimary is the primary color for inverse contexts.
	InversePrimary graphics.Color

	// SurfaceTint is the tint color applied to surfaces.
	SurfaceTint graphics.Color

	// Brightness indicates if this is a light or dark scheme.
	Brightness Brightness
}

// LightColorScheme returns the default light color scheme.
// Colors are based on Material Design 3 baseline purple theme.
func LightColorScheme() ColorScheme {
	return ColorScheme{
		// Primary
		Primary:            graphics.RGB(103, 80, 164),  // MD3 Purple primary
		OnPrimary:          graphics.RGB(255, 255, 255), // White
		PrimaryContainer:   graphics.RGB(234, 221, 255), // Light purple
		OnPrimaryContainer: graphics.RGB(33, 0, 94),     // Dark purple

		// Secondary
		Secondary:            graphics.RGB(98, 91, 113),   // Muted purple
		OnSecondary:          graphics.RGB(255, 255, 255), // White
		SecondaryContainer:   graphics.RGB(232, 222, 248), // Light lavender
		OnSecondaryContainer: graphics.RGB(30, 25, 43),    // Dark purple

		// Tertiary
		Tertiary:            graphics.RGB(125, 82, 96),   // Rose
		OnTertiary:          graphics.RGB(255, 255, 255), // White
		TertiaryContainer:   graphics.RGB(255, 216, 228), // Light pink
		OnTertiaryContainer: graphics.RGB(49, 17, 29),    // Dark rose

		// Surface
		Surface:                 graphics.RGB(254, 247, 255), // Near white with purple tint
		OnSurface:               graphics.RGB(28, 27, 31),    // Near black
		SurfaceVariant:          graphics.RGB(231, 224, 236), // Light purple gray
		OnSurfaceVariant:        graphics.RGB(73, 69, 79),    // Dark gray
		SurfaceDim:              graphics.RGB(222, 216, 225), // Dimmed surface
		SurfaceBright:           graphics.RGB(254, 247, 255), // Bright surface
		SurfaceContainerLowest:  graphics.RGB(255, 255, 255), // Pure white
		SurfaceContainerLow:     graphics.RGB(247, 242, 250), // Slight tint
		SurfaceContainer:        graphics.RGB(243, 237, 247), // Light container
		SurfaceContainerHigh:    graphics.RGB(236, 230, 240), // Medium container
		SurfaceContainerHighest: graphics.RGB(230, 224, 233), // Dark container

		// Background
		Background:   graphics.RGB(254, 247, 255), // Near white
		OnBackground: graphics.RGB(28, 27, 31),    // Near black

		// Error
		Error:            graphics.RGB(179, 38, 30),   // MD3 Red
		OnError:          graphics.RGB(255, 255, 255), // White
		ErrorContainer:   graphics.RGB(249, 222, 220), // Light red
		OnErrorContainer: graphics.RGB(65, 14, 11),    // Dark red

		// Outline
		Outline:        graphics.RGB(121, 116, 126), // Medium gray
		OutlineVariant: graphics.RGB(196, 199, 197), // Light gray

		// Shadow and Scrim
		Shadow: graphics.RGB(0, 0, 0), // Black
		Scrim:  graphics.RGB(0, 0, 0), // Black

		// Inverse
		InverseSurface:   graphics.RGB(49, 48, 51),    // Dark surface
		OnInverseSurface: graphics.RGB(244, 239, 244), // Light text
		InversePrimary:   graphics.RGB(208, 188, 255), // Light purple

		// Surface Tint
		SurfaceTint: graphics.RGB(103, 80, 164), // Primary color

		Brightness: BrightnessLight,
	}
}

// DarkColorScheme returns the default dark color scheme.
// Colors are based on Material Design 3 baseline purple theme.
func DarkColorScheme() ColorScheme {
	return ColorScheme{
		// Primary
		Primary:            graphics.RGB(208, 188, 255), // Light purple
		OnPrimary:          graphics.RGB(56, 30, 114),   // Dark purple
		PrimaryContainer:   graphics.RGB(79, 55, 139),   // Medium purple
		OnPrimaryContainer: graphics.RGB(234, 221, 255), // Light purple

		// Secondary
		Secondary:            graphics.RGB(204, 194, 220), // Light lavender
		OnSecondary:          graphics.RGB(51, 45, 65),    // Dark purple
		SecondaryContainer:   graphics.RGB(74, 68, 88),    // Medium purple
		OnSecondaryContainer: graphics.RGB(232, 222, 248), // Light lavender

		// Tertiary
		Tertiary:            graphics.RGB(239, 184, 200), // Light rose
		OnTertiary:          graphics.RGB(73, 37, 50),    // Dark rose
		TertiaryContainer:   graphics.RGB(99, 59, 72),    // Medium rose
		OnTertiaryContainer: graphics.RGB(255, 216, 228), // Light pink

		// Surface
		Surface:                 graphics.RGB(20, 18, 24),    // Dark with purple tint
		OnSurface:               graphics.RGB(230, 225, 229), // Off white
		SurfaceVariant:          graphics.RGB(73, 69, 79),    // Dark gray
		OnSurfaceVariant:        graphics.RGB(202, 196, 208), // Light gray
		SurfaceDim:              graphics.RGB(20, 18, 24),    // Dimmed surface
		SurfaceBright:           graphics.RGB(59, 56, 62),    // Bright surface
		SurfaceContainerLowest:  graphics.RGB(15, 13, 19),    // Darkest container
		SurfaceContainerLow:     graphics.RGB(29, 27, 32),    // Dark container
		SurfaceContainer:        graphics.RGB(33, 31, 38),    // Medium container
		SurfaceContainerHigh:    graphics.RGB(43, 41, 48),    // Light container
		SurfaceContainerHighest: graphics.RGB(54, 52, 59),    // Lightest container

		// Background
		Background:   graphics.RGB(20, 18, 24),    // Dark
		OnBackground: graphics.RGB(230, 225, 229), // Off white

		// Error
		Error:            graphics.RGB(242, 184, 181), // Light red
		OnError:          graphics.RGB(96, 20, 16),    // Dark red
		ErrorContainer:   graphics.RGB(140, 29, 24),   // Medium red
		OnErrorContainer: graphics.RGB(249, 222, 220), // Light red

		// Outline
		Outline:        graphics.RGB(147, 143, 153), // Medium gray
		OutlineVariant: graphics.RGB(73, 69, 79),    // Dark gray

		// Shadow and Scrim
		Shadow: graphics.RGB(0, 0, 0), // Black
		Scrim:  graphics.RGB(0, 0, 0), // Black

		// Inverse
		InverseSurface:   graphics.RGB(230, 225, 229), // Light surface
		OnInverseSurface: graphics.RGB(49, 48, 51),    // Dark text
		InversePrimary:   graphics.RGB(103, 80, 164),  // Dark purple

		// Surface Tint
		SurfaceTint: graphics.RGB(208, 188, 255), // Primary color

		Brightness: BrightnessDark,
	}
}
