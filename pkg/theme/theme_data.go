package theme

// ThemeData contains all theme configuration for an application.
type ThemeData struct {
	// ColorScheme defines the color palette.
	ColorScheme ColorScheme

	// TextTheme defines text styles.
	TextTheme TextTheme

	// Brightness indicates if this is a light or dark theme.
	Brightness Brightness

	// Component themes - optional, derived from ColorScheme if nil.
	ButtonTheme      *ButtonThemeData
	CheckboxTheme    *CheckboxThemeData
	SwitchTheme      *SwitchThemeData
	TextFieldTheme   *TextFieldThemeData
	TabBarTheme      *TabBarThemeData
	RadioTheme       *RadioThemeData
	DropdownTheme    *DropdownThemeData
	BottomSheetTheme *BottomSheetThemeData
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
		ColorScheme:      t.ColorScheme,
		TextTheme:        t.TextTheme,
		Brightness:       t.Brightness,
		ButtonTheme:      t.ButtonTheme,
		CheckboxTheme:    t.CheckboxTheme,
		SwitchTheme:      t.SwitchTheme,
		TextFieldTheme:   t.TextFieldTheme,
		TabBarTheme:      t.TabBarTheme,
		RadioTheme:       t.RadioTheme,
		DropdownTheme:    t.DropdownTheme,
		BottomSheetTheme: t.BottomSheetTheme,
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

// ButtonThemeOf returns the button theme, deriving from ColorScheme if not set.
func (t *ThemeData) ButtonThemeOf() ButtonThemeData {
	if t.ButtonTheme != nil {
		return *t.ButtonTheme
	}
	return DefaultButtonTheme(t.ColorScheme)
}

// CheckboxThemeOf returns the checkbox theme, deriving from ColorScheme if not set.
func (t *ThemeData) CheckboxThemeOf() CheckboxThemeData {
	if t.CheckboxTheme != nil {
		return *t.CheckboxTheme
	}
	return DefaultCheckboxTheme(t.ColorScheme)
}

// SwitchThemeOf returns the switch theme, deriving from ColorScheme if not set.
func (t *ThemeData) SwitchThemeOf() SwitchThemeData {
	if t.SwitchTheme != nil {
		return *t.SwitchTheme
	}
	return DefaultSwitchTheme(t.ColorScheme)
}

// TextFieldThemeOf returns the text field theme, deriving from ColorScheme if not set.
func (t *ThemeData) TextFieldThemeOf() TextFieldThemeData {
	if t.TextFieldTheme != nil {
		return *t.TextFieldTheme
	}
	return DefaultTextFieldTheme(t.ColorScheme)
}

// TabBarThemeOf returns the tab bar theme, deriving from ColorScheme if not set.
func (t *ThemeData) TabBarThemeOf() TabBarThemeData {
	if t.TabBarTheme != nil {
		return *t.TabBarTheme
	}
	return DefaultTabBarTheme(t.ColorScheme)
}

// RadioThemeOf returns the radio theme, deriving from ColorScheme if not set.
func (t *ThemeData) RadioThemeOf() RadioThemeData {
	if t.RadioTheme != nil {
		return *t.RadioTheme
	}
	return DefaultRadioTheme(t.ColorScheme)
}

// DropdownThemeOf returns the dropdown theme, deriving from ColorScheme if not set.
func (t *ThemeData) DropdownThemeOf() DropdownThemeData {
	if t.DropdownTheme != nil {
		return *t.DropdownTheme
	}
	return DefaultDropdownTheme(t.ColorScheme)
}

// BottomSheetThemeOf returns the bottom sheet theme, deriving from ColorScheme if not set.
func (t *ThemeData) BottomSheetThemeOf() BottomSheetThemeData {
	if t.BottomSheetTheme != nil {
		return *t.BottomSheetTheme
	}
	return DefaultBottomSheetTheme(t.ColorScheme)
}
