package theme

import "github.com/go-drift/drift/pkg/core"

// TargetPlatform identifies the design language/platform style.
type TargetPlatform int

const (
	// TargetPlatformMaterial uses Material Design (Android/default).
	TargetPlatformMaterial TargetPlatform = iota
	// TargetPlatformCupertino uses iOS/Cupertino design.
	TargetPlatformCupertino
)

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

// PlatformOf returns the target platform based on which theme is active.
// If a CupertinoTheme is found in the widget tree, returns TargetPlatformCupertino.
// Otherwise, returns TargetPlatformMaterial.
func PlatformOf(ctx core.BuildContext) TargetPlatform {
	// Check AppTheme first (unified provider)
	if appTheme := AppThemeMaybeOf(ctx); appTheme != nil {
		return appTheme.Platform
	}
	// Fall back to checking CupertinoTheme presence
	if CupertinoMaybeOf(ctx) != nil {
		return TargetPlatformCupertino
	}
	return TargetPlatformMaterial
}

// IsCupertino returns true if a CupertinoTheme is active in the widget tree.
func IsCupertino(ctx core.BuildContext) bool {
	return CupertinoMaybeOf(ctx) != nil
}

// IsMaterial returns true if no CupertinoTheme is active (Material is the default).
func IsMaterial(ctx core.BuildContext) bool {
	return CupertinoMaybeOf(ctx) == nil
}
