package theme

import "github.com/go-drift/drift/pkg/core"

// ColorsOf returns the ColorScheme from the nearest Theme ancestor.
// If no Theme is found, returns the default light color scheme.
func ColorsOf(ctx core.BuildContext) ColorScheme {
	return ThemeOf(ctx).ColorScheme
}

// TextThemeOf returns the TextTheme from the nearest Theme ancestor.
// If no Theme is found, returns the default text theme.
func TextThemeOf(ctx core.BuildContext) TextTheme {
	return ThemeOf(ctx).TextTheme
}

// UseTheme returns all theme components in a single call.
// This replaces the common pattern:
//
//	themeData := theme.ThemeOf(ctx)
//	colors := themeData.ColorScheme
//	textTheme := themeData.TextTheme
//
// With:
//
//	_, colors, textTheme := theme.UseTheme(ctx)
func UseTheme(ctx core.BuildContext) (*ThemeData, ColorScheme, TextTheme) {
	data := ThemeOf(ctx)
	return data, data.ColorScheme, data.TextTheme
}
