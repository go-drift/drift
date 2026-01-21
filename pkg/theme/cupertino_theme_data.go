package theme

import "github.com/go-drift/drift/pkg/rendering"

// CupertinoThemeData contains all theme configuration for iOS-style interfaces.
type CupertinoThemeData struct {
	// Brightness indicates if this is a light or dark theme.
	Brightness Brightness

	// PrimaryColor is the primary accent color (defaults to SystemBlue).
	PrimaryColor rendering.Color

	// PrimaryContrastingColor is used for text/icons on primary color.
	PrimaryContrastingColor rendering.Color

	// Colors provides the full iOS color palette.
	Colors CupertinoColors

	// TextTheme provides iOS-style text styles.
	TextTheme CupertinoTextThemeData

	// BarBackgroundColor is the color for navigation/tab bars.
	BarBackgroundColor rendering.Color

	// ScaffoldBackgroundColor is the default page background color.
	ScaffoldBackgroundColor rendering.Color
}

// DefaultCupertinoLightTheme returns the default iOS light theme.
func DefaultCupertinoLightTheme() *CupertinoThemeData {
	colors := LightCupertinoColors()
	return &CupertinoThemeData{
		Brightness:              BrightnessLight,
		PrimaryColor:            colors.SystemBlue,
		PrimaryContrastingColor: rendering.RGB(255, 255, 255),
		Colors:                  colors,
		TextTheme:               DefaultCupertinoTextTheme(colors.Label),
		BarBackgroundColor:      rendering.RGBA(249, 249, 249, 244), // iOS translucent bar
		ScaffoldBackgroundColor: colors.SystemBackground,
	}
}

// DefaultCupertinoDarkTheme returns the default iOS dark theme.
func DefaultCupertinoDarkTheme() *CupertinoThemeData {
	colors := DarkCupertinoColors()
	return &CupertinoThemeData{
		Brightness:              BrightnessDark,
		PrimaryColor:            colors.SystemBlue,
		PrimaryContrastingColor: rendering.RGB(255, 255, 255),
		Colors:                  colors,
		TextTheme:               DefaultCupertinoTextTheme(colors.Label),
		BarBackgroundColor:      rendering.RGBA(30, 30, 30, 244), // iOS translucent bar
		ScaffoldBackgroundColor: colors.SystemBackground,
	}
}

// CopyWith returns a new CupertinoThemeData with the specified fields overridden.
func (t *CupertinoThemeData) CopyWith(
	brightness *Brightness,
	primaryColor *rendering.Color,
	primaryContrastingColor *rendering.Color,
	colors *CupertinoColors,
	textTheme *CupertinoTextThemeData,
	barBackgroundColor *rendering.Color,
	scaffoldBackgroundColor *rendering.Color,
) *CupertinoThemeData {
	result := &CupertinoThemeData{
		Brightness:              t.Brightness,
		PrimaryColor:            t.PrimaryColor,
		PrimaryContrastingColor: t.PrimaryContrastingColor,
		Colors:                  t.Colors,
		TextTheme:               t.TextTheme,
		BarBackgroundColor:      t.BarBackgroundColor,
		ScaffoldBackgroundColor: t.ScaffoldBackgroundColor,
	}
	if brightness != nil {
		result.Brightness = *brightness
	}
	if primaryColor != nil {
		result.PrimaryColor = *primaryColor
	}
	if primaryContrastingColor != nil {
		result.PrimaryContrastingColor = *primaryContrastingColor
	}
	if colors != nil {
		result.Colors = *colors
	}
	if textTheme != nil {
		result.TextTheme = *textTheme
	}
	if barBackgroundColor != nil {
		result.BarBackgroundColor = *barBackgroundColor
	}
	if scaffoldBackgroundColor != nil {
		result.ScaffoldBackgroundColor = *scaffoldBackgroundColor
	}
	return result
}

// ResolveFrom creates a CupertinoThemeData by resolving colors for the given brightness.
func (t *CupertinoThemeData) ResolveFrom(brightness Brightness) *CupertinoThemeData {
	if brightness == t.Brightness {
		return t
	}
	if brightness == BrightnessLight {
		return DefaultCupertinoLightTheme()
	}
	return DefaultCupertinoDarkTheme()
}
