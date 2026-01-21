// Package theme provides theming support for the Drift framework.
package theme

import "github.com/go-drift/drift/pkg/rendering"

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
	Primary rendering.Color
	// OnPrimary is the color for text/icons on primary.
	OnPrimary rendering.Color
	// PrimaryContainer is a tonal container for primary.
	PrimaryContainer rendering.Color
	// OnPrimaryContainer is the color for text/icons on primary container.
	OnPrimaryContainer rendering.Color

	// Secondary is a supporting brand color.
	Secondary rendering.Color
	// OnSecondary is the color for text/icons on secondary.
	OnSecondary rendering.Color
	// SecondaryContainer is a tonal container for secondary.
	SecondaryContainer rendering.Color
	// OnSecondaryContainer is the color for text/icons on secondary container.
	OnSecondaryContainer rendering.Color

	// Tertiary is an accent color for contrast.
	Tertiary rendering.Color
	// OnTertiary is the color for text/icons on tertiary.
	OnTertiary rendering.Color
	// TertiaryContainer is a tonal container for tertiary.
	TertiaryContainer rendering.Color
	// OnTertiaryContainer is the color for text/icons on tertiary container.
	OnTertiaryContainer rendering.Color

	// Surface is the color for cards, sheets, menus.
	Surface rendering.Color
	// OnSurface is the color for text/icons on surface.
	OnSurface rendering.Color
	// SurfaceVariant is an alternative surface color.
	SurfaceVariant rendering.Color
	// OnSurfaceVariant is the color for text/icons on surface variant.
	OnSurfaceVariant rendering.Color

	// SurfaceDim is a dimmed surface for less emphasis.
	SurfaceDim rendering.Color
	// SurfaceBright is a bright surface for more emphasis.
	SurfaceBright rendering.Color
	// SurfaceContainerLowest is the lowest emphasis container surface.
	SurfaceContainerLowest rendering.Color
	// SurfaceContainerLow is a low emphasis container surface.
	SurfaceContainerLow rendering.Color
	// SurfaceContainer is the default container surface.
	SurfaceContainer rendering.Color
	// SurfaceContainerHigh is a high emphasis container surface.
	SurfaceContainerHigh rendering.Color
	// SurfaceContainerHighest is the highest emphasis container surface.
	SurfaceContainerHighest rendering.Color

	// Background is the app background color.
	Background rendering.Color
	// OnBackground is the color for text/icons on background.
	OnBackground rendering.Color

	// Error is the color for error states.
	Error rendering.Color
	// OnError is the color for text/icons on error.
	OnError rendering.Color
	// ErrorContainer is a tonal container for error.
	ErrorContainer rendering.Color
	// OnErrorContainer is the color for text/icons on error container.
	OnErrorContainer rendering.Color

	// Outline is the color for borders and dividers.
	Outline rendering.Color
	// OutlineVariant is a subtle outline for decorative elements.
	OutlineVariant rendering.Color

	// Shadow is the color for elevation shadows.
	Shadow rendering.Color
	// Scrim is the color for modal barriers.
	Scrim rendering.Color

	// InverseSurface is the surface color for inverse contexts.
	InverseSurface rendering.Color
	// OnInverseSurface is the color for text/icons on inverse surface.
	OnInverseSurface rendering.Color
	// InversePrimary is the primary color for inverse contexts.
	InversePrimary rendering.Color

	// SurfaceTint is the tint color applied to surfaces.
	SurfaceTint rendering.Color

	// Brightness indicates if this is a light or dark scheme.
	Brightness Brightness
}

// LightColorScheme returns the default light color scheme.
// Colors are based on Material Design 3 baseline purple theme.
func LightColorScheme() ColorScheme {
	return ColorScheme{
		// Primary
		Primary:            rendering.RGB(103, 80, 164),  // MD3 Purple primary
		OnPrimary:          rendering.RGB(255, 255, 255), // White
		PrimaryContainer:   rendering.RGB(234, 221, 255), // Light purple
		OnPrimaryContainer: rendering.RGB(33, 0, 94),     // Dark purple

		// Secondary
		Secondary:            rendering.RGB(98, 91, 113),   // Muted purple
		OnSecondary:          rendering.RGB(255, 255, 255), // White
		SecondaryContainer:   rendering.RGB(232, 222, 248), // Light lavender
		OnSecondaryContainer: rendering.RGB(30, 25, 43),    // Dark purple

		// Tertiary
		Tertiary:            rendering.RGB(125, 82, 96),   // Rose
		OnTertiary:          rendering.RGB(255, 255, 255), // White
		TertiaryContainer:   rendering.RGB(255, 216, 228), // Light pink
		OnTertiaryContainer: rendering.RGB(49, 17, 29),    // Dark rose

		// Surface
		Surface:                 rendering.RGB(254, 247, 255), // Near white with purple tint
		OnSurface:               rendering.RGB(28, 27, 31),    // Near black
		SurfaceVariant:          rendering.RGB(231, 224, 236), // Light purple gray
		OnSurfaceVariant:        rendering.RGB(73, 69, 79),    // Dark gray
		SurfaceDim:              rendering.RGB(222, 216, 225), // Dimmed surface
		SurfaceBright:           rendering.RGB(254, 247, 255), // Bright surface
		SurfaceContainerLowest:  rendering.RGB(255, 255, 255), // Pure white
		SurfaceContainerLow:     rendering.RGB(247, 242, 250), // Slight tint
		SurfaceContainer:        rendering.RGB(243, 237, 247), // Light container
		SurfaceContainerHigh:    rendering.RGB(236, 230, 240), // Medium container
		SurfaceContainerHighest: rendering.RGB(230, 224, 233), // Dark container

		// Background
		Background:   rendering.RGB(254, 247, 255), // Near white
		OnBackground: rendering.RGB(28, 27, 31),    // Near black

		// Error
		Error:            rendering.RGB(179, 38, 30),   // MD3 Red
		OnError:          rendering.RGB(255, 255, 255), // White
		ErrorContainer:   rendering.RGB(249, 222, 220), // Light red
		OnErrorContainer: rendering.RGB(65, 14, 11),    // Dark red

		// Outline
		Outline:        rendering.RGB(121, 116, 126), // Medium gray
		OutlineVariant: rendering.RGB(196, 199, 197), // Light gray

		// Shadow and Scrim
		Shadow: rendering.RGB(0, 0, 0), // Black
		Scrim:  rendering.RGB(0, 0, 0), // Black

		// Inverse
		InverseSurface:   rendering.RGB(49, 48, 51),    // Dark surface
		OnInverseSurface: rendering.RGB(244, 239, 244), // Light text
		InversePrimary:   rendering.RGB(208, 188, 255), // Light purple

		// Surface Tint
		SurfaceTint: rendering.RGB(103, 80, 164), // Primary color

		Brightness: BrightnessLight,
	}
}

// DarkColorScheme returns the default dark color scheme.
// Colors are based on Material Design 3 baseline purple theme.
func DarkColorScheme() ColorScheme {
	return ColorScheme{
		// Primary
		Primary:            rendering.RGB(208, 188, 255), // Light purple
		OnPrimary:          rendering.RGB(56, 30, 114),   // Dark purple
		PrimaryContainer:   rendering.RGB(79, 55, 139),   // Medium purple
		OnPrimaryContainer: rendering.RGB(234, 221, 255), // Light purple

		// Secondary
		Secondary:            rendering.RGB(204, 194, 220), // Light lavender
		OnSecondary:          rendering.RGB(51, 45, 65),    // Dark purple
		SecondaryContainer:   rendering.RGB(74, 68, 88),    // Medium purple
		OnSecondaryContainer: rendering.RGB(232, 222, 248), // Light lavender

		// Tertiary
		Tertiary:            rendering.RGB(239, 184, 200), // Light rose
		OnTertiary:          rendering.RGB(73, 37, 50),    // Dark rose
		TertiaryContainer:   rendering.RGB(99, 59, 72),    // Medium rose
		OnTertiaryContainer: rendering.RGB(255, 216, 228), // Light pink

		// Surface
		Surface:                 rendering.RGB(20, 18, 24),    // Dark with purple tint
		OnSurface:               rendering.RGB(230, 225, 229), // Off white
		SurfaceVariant:          rendering.RGB(73, 69, 79),    // Dark gray
		OnSurfaceVariant:        rendering.RGB(202, 196, 208), // Light gray
		SurfaceDim:              rendering.RGB(20, 18, 24),    // Dimmed surface
		SurfaceBright:           rendering.RGB(59, 56, 62),    // Bright surface
		SurfaceContainerLowest:  rendering.RGB(15, 13, 19),    // Darkest container
		SurfaceContainerLow:     rendering.RGB(29, 27, 32),    // Dark container
		SurfaceContainer:        rendering.RGB(33, 31, 38),    // Medium container
		SurfaceContainerHigh:    rendering.RGB(43, 41, 48),    // Light container
		SurfaceContainerHighest: rendering.RGB(54, 52, 59),    // Lightest container

		// Background
		Background:   rendering.RGB(20, 18, 24),    // Dark
		OnBackground: rendering.RGB(230, 225, 229), // Off white

		// Error
		Error:            rendering.RGB(242, 184, 181), // Light red
		OnError:          rendering.RGB(96, 20, 16),    // Dark red
		ErrorContainer:   rendering.RGB(140, 29, 24),   // Medium red
		OnErrorContainer: rendering.RGB(249, 222, 220), // Light red

		// Outline
		Outline:        rendering.RGB(147, 143, 153), // Medium gray
		OutlineVariant: rendering.RGB(73, 69, 79),    // Dark gray

		// Shadow and Scrim
		Shadow: rendering.RGB(0, 0, 0), // Black
		Scrim:  rendering.RGB(0, 0, 0), // Black

		// Inverse
		InverseSurface:   rendering.RGB(230, 225, 229), // Light surface
		OnInverseSurface: rendering.RGB(49, 48, 51),    // Dark text
		InversePrimary:   rendering.RGB(103, 80, 164),  // Dark purple

		// Surface Tint
		SurfaceTint: rendering.RGB(208, 188, 255), // Primary color

		Brightness: BrightnessDark,
	}
}
