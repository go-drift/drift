package theme

// ThemeData contains all theme configuration for an application.
type ThemeData struct {
	// ColorScheme defines the color palette.
	ColorScheme ColorScheme

	// TextTheme defines text styles.
	TextTheme TextTheme

	// Brightness indicates if this is a light or dark theme.
	Brightness Brightness
}

// DefaultLightTheme returns the default light theme.
func DefaultLightTheme() *ThemeData {
	colors := LightColorScheme()
	return &ThemeData{
		ColorScheme: colors,
		TextTheme:   DefaultTextTheme(colors.OnBackground),
		Brightness:  BrightnessLight,
	}
}

// DefaultDarkTheme returns the default dark theme.
func DefaultDarkTheme() *ThemeData {
	colors := DarkColorScheme()
	return &ThemeData{
		ColorScheme: colors,
		TextTheme:   DefaultTextTheme(colors.OnBackground),
		Brightness:  BrightnessDark,
	}
}

// CopyWith returns a new ThemeData with the specified fields overridden.
func (t *ThemeData) CopyWith(colorScheme *ColorScheme, textTheme *TextTheme, brightness *Brightness) *ThemeData {
	result := &ThemeData{
		ColorScheme: t.ColorScheme,
		TextTheme:   t.TextTheme,
		Brightness:  t.Brightness,
	}
	if colorScheme != nil {
		result.ColorScheme = *colorScheme
	}
	if textTheme != nil {
		result.TextTheme = *textTheme
	}
	if brightness != nil {
		result.Brightness = *brightness
	}
	return result
}
