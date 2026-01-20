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
// Based on Material Design 3 color system.
type ColorScheme struct {
	// Primary is the main brand color.
	Primary rendering.Color
	// OnPrimary is the color for text/icons on primary.
	OnPrimary rendering.Color

	// Secondary is a supporting brand color.
	Secondary rendering.Color
	// OnSecondary is the color for text/icons on secondary.
	OnSecondary rendering.Color

	// Surface is the color for cards, sheets, menus.
	Surface rendering.Color
	// OnSurface is the color for text/icons on surface.
	OnSurface rendering.Color

	// Background is the app background color.
	Background rendering.Color
	// OnBackground is the color for text/icons on background.
	OnBackground rendering.Color

	// Error is the color for error states.
	Error rendering.Color
	// OnError is the color for text/icons on error.
	OnError rendering.Color

	// Outline is the color for borders and dividers.
	Outline rendering.Color

	// SurfaceVariant is an alternative surface color.
	SurfaceVariant rendering.Color
	// OnSurfaceVariant is the color for text/icons on surface variant.
	OnSurfaceVariant rendering.Color

	// Brightness indicates if this is a light or dark scheme.
	Brightness Brightness
}

// LightColorScheme returns the default light color scheme.
func LightColorScheme() ColorScheme {
	return ColorScheme{
		Primary:          rendering.RGB(98, 0, 238),    // Purple
		OnPrimary:        rendering.RGB(255, 255, 255), // White
		Secondary:        rendering.RGB(3, 218, 198),   // Teal
		OnSecondary:      rendering.RGB(0, 0, 0),       // Black
		Surface:          rendering.RGB(255, 255, 255), // White
		OnSurface:        rendering.RGB(28, 28, 30),    // Near black
		Background:       rendering.RGB(250, 250, 250), // Off white
		OnBackground:     rendering.RGB(28, 28, 30),    // Near black
		Error:            rendering.RGB(176, 0, 32),    // Red
		OnError:          rendering.RGB(255, 255, 255), // White
		Outline:          rendering.RGB(121, 116, 126), // Gray
		SurfaceVariant:   rendering.RGB(231, 224, 236), // Light purple gray
		OnSurfaceVariant: rendering.RGB(73, 69, 79),    // Dark gray
		Brightness:       BrightnessLight,
	}
}

// DarkColorScheme returns the default dark color scheme.
func DarkColorScheme() ColorScheme {
	return ColorScheme{
		Primary:          rendering.RGB(187, 134, 252), // Light purple
		OnPrimary:        rendering.RGB(56, 0, 112),    // Dark purple
		Secondary:        rendering.RGB(3, 218, 198),   // Teal
		OnSecondary:      rendering.RGB(0, 55, 49),     // Dark teal
		Surface:          rendering.RGB(30, 30, 30),    // Dark gray
		OnSurface:        rendering.RGB(230, 225, 229), // Off white
		Background:       rendering.RGB(18, 18, 18),    // Near black
		OnBackground:     rendering.RGB(230, 225, 229), // Off white
		Error:            rendering.RGB(242, 184, 181), // Light red
		OnError:          rendering.RGB(96, 20, 16),    // Dark red
		Outline:          rendering.RGB(147, 143, 153), // Gray
		SurfaceVariant:   rendering.RGB(73, 69, 79),    // Dark gray
		OnSurfaceVariant: rendering.RGB(202, 196, 208), // Light gray
		Brightness:       BrightnessDark,
	}
}
