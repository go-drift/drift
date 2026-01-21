package theme

import (
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// ButtonThemeData defines default styling for Button widgets.
type ButtonThemeData struct {
	// BackgroundColor is the default button background.
	BackgroundColor rendering.Color
	// ForegroundColor is the default text/icon color.
	ForegroundColor rendering.Color
	// DisabledBackgroundColor is the background when disabled.
	DisabledBackgroundColor rendering.Color
	// DisabledForegroundColor is the text color when disabled.
	DisabledForegroundColor rendering.Color
	// Padding is the default button padding.
	Padding layout.EdgeInsets
	// BorderRadius is the default corner radius.
	BorderRadius float64
	// FontSize is the default label font size.
	FontSize float64
}

// CheckboxThemeData defines default styling for Checkbox widgets.
type CheckboxThemeData struct {
	// ActiveColor is the fill color when checked.
	ActiveColor rendering.Color
	// CheckColor is the checkmark color.
	CheckColor rendering.Color
	// BorderColor is the outline color when unchecked.
	BorderColor rendering.Color
	// BackgroundColor is the fill color when unchecked.
	BackgroundColor rendering.Color
	// DisabledActiveColor is the fill color when checked and disabled.
	DisabledActiveColor rendering.Color
	// DisabledCheckColor is the checkmark color when disabled.
	DisabledCheckColor rendering.Color
	// Size is the default checkbox size.
	Size float64
	// BorderRadius is the default corner radius.
	BorderRadius float64
}

// SwitchThemeData defines default styling for Switch widgets.
type SwitchThemeData struct {
	// ActiveTrackColor is the track color when on.
	ActiveTrackColor rendering.Color
	// InactiveTrackColor is the track color when off.
	InactiveTrackColor rendering.Color
	// ThumbColor is the thumb fill color.
	ThumbColor rendering.Color
	// DisabledActiveTrackColor is the track color when on and disabled.
	DisabledActiveTrackColor rendering.Color
	// DisabledInactiveTrackColor is the track color when off and disabled.
	DisabledInactiveTrackColor rendering.Color
	// DisabledThumbColor is the thumb color when disabled.
	DisabledThumbColor rendering.Color
	// Width is the default switch width.
	Width float64
	// Height is the default switch height.
	Height float64
}

// TextFieldThemeData defines default styling for TextField widgets.
type TextFieldThemeData struct {
	// BackgroundColor is the field background.
	BackgroundColor rendering.Color
	// BorderColor is the default border color.
	BorderColor rendering.Color
	// FocusColor is the border color when focused.
	FocusColor rendering.Color
	// ErrorColor is the border color when in error state.
	ErrorColor rendering.Color
	// LabelColor is the label text color.
	LabelColor rendering.Color
	// TextColor is the input text color.
	TextColor rendering.Color
	// PlaceholderColor is the placeholder text color.
	PlaceholderColor rendering.Color
	// Padding is the default inner padding.
	Padding layout.EdgeInsets
	// BorderRadius is the default corner radius.
	BorderRadius float64
	// Height is the default field height.
	Height float64
}

// TabBarThemeData defines default styling for TabBar widgets.
type TabBarThemeData struct {
	// BackgroundColor is the tab bar background.
	BackgroundColor rendering.Color
	// ActiveColor is the color for the selected tab.
	ActiveColor rendering.Color
	// InactiveColor is the color for unselected tabs.
	InactiveColor rendering.Color
	// IndicatorColor is the color for the selection indicator.
	IndicatorColor rendering.Color
	// IndicatorHeight is the height of the selection indicator.
	IndicatorHeight float64
	// Padding is the default tab item padding.
	Padding layout.EdgeInsets
	// Height is the default tab bar height.
	Height float64
}

// RadioThemeData defines default styling for Radio widgets.
type RadioThemeData struct {
	// ActiveColor is the fill color when selected.
	ActiveColor rendering.Color
	// InactiveColor is the border color when unselected.
	InactiveColor rendering.Color
	// BackgroundColor is the fill color when unselected.
	BackgroundColor rendering.Color
	// DisabledActiveColor is the fill color when selected and disabled.
	DisabledActiveColor rendering.Color
	// DisabledInactiveColor is the border color when disabled.
	DisabledInactiveColor rendering.Color
	// Size is the default radio diameter.
	Size float64
}

// DropdownThemeData defines default styling for Dropdown widgets.
type DropdownThemeData struct {
	// BackgroundColor is the trigger background.
	BackgroundColor rendering.Color
	// BorderColor is the trigger border color.
	BorderColor rendering.Color
	// MenuBackgroundColor is the dropdown menu background.
	MenuBackgroundColor rendering.Color
	// MenuBorderColor is the dropdown menu border color.
	MenuBorderColor rendering.Color
	// SelectedItemColor is the background for the selected item.
	SelectedItemColor rendering.Color
	// TextColor is the default text color.
	TextColor rendering.Color
	// DisabledTextColor is the text color when disabled.
	DisabledTextColor rendering.Color
	// BorderRadius is the default corner radius.
	BorderRadius float64
	// ItemPadding is the default padding for menu items.
	ItemPadding layout.EdgeInsets
	// Height is the default trigger/item height.
	Height float64
}

// DefaultButtonTheme returns ButtonThemeData derived from a ColorScheme.
func DefaultButtonTheme(colors ColorScheme) ButtonThemeData {
	return ButtonThemeData{
		BackgroundColor:         colors.Primary,
		ForegroundColor:         colors.OnPrimary,
		DisabledBackgroundColor: colors.SurfaceVariant,
		DisabledForegroundColor: colors.OnSurfaceVariant,
		Padding:                 layout.EdgeInsetsSymmetric(24, 14),
		BorderRadius:            8,
		FontSize:                16,
	}
}

// DefaultCheckboxTheme returns CheckboxThemeData derived from a ColorScheme.
func DefaultCheckboxTheme(colors ColorScheme) CheckboxThemeData {
	return CheckboxThemeData{
		ActiveColor:         colors.Primary,
		CheckColor:          colors.OnPrimary,
		BorderColor:         colors.Outline,
		BackgroundColor:     colors.Surface,
		DisabledActiveColor: colors.SurfaceVariant,
		DisabledCheckColor:  colors.OnSurfaceVariant,
		Size:                20,
		BorderRadius:        4,
	}
}

// DefaultSwitchTheme returns SwitchThemeData derived from a ColorScheme.
func DefaultSwitchTheme(colors ColorScheme) SwitchThemeData {
	return SwitchThemeData{
		ActiveTrackColor:           colors.Primary,
		InactiveTrackColor:         colors.SurfaceVariant,
		ThumbColor:                 colors.Surface,
		DisabledActiveTrackColor:   colors.SurfaceVariant,
		DisabledInactiveTrackColor: colors.SurfaceVariant,
		DisabledThumbColor:         colors.OnSurfaceVariant,
		Width:                      44,
		Height:                     26,
	}
}

// DefaultTextFieldTheme returns TextFieldThemeData derived from a ColorScheme.
func DefaultTextFieldTheme(colors ColorScheme) TextFieldThemeData {
	return TextFieldThemeData{
		BackgroundColor:  colors.Surface,
		BorderColor:      colors.Outline,
		FocusColor:       colors.Primary,
		ErrorColor:       colors.Error,
		LabelColor:       colors.OnSurfaceVariant,
		TextColor:        colors.OnSurface,
		PlaceholderColor: colors.OnSurfaceVariant,
		Padding:          layout.EdgeInsetsSymmetric(12, 8),
		BorderRadius:     8,
		Height:           48,
	}
}

// DefaultTabBarTheme returns TabBarThemeData derived from a ColorScheme.
func DefaultTabBarTheme(colors ColorScheme) TabBarThemeData {
	return TabBarThemeData{
		BackgroundColor: colors.SurfaceVariant,
		ActiveColor:     colors.Primary,
		InactiveColor:   colors.OnSurfaceVariant,
		IndicatorColor:  colors.Primary,
		IndicatorHeight: 3,
		Padding:         layout.EdgeInsetsSymmetric(12, 8),
		Height:          56,
	}
}

// DefaultRadioTheme returns RadioThemeData derived from a ColorScheme.
func DefaultRadioTheme(colors ColorScheme) RadioThemeData {
	return RadioThemeData{
		ActiveColor:           colors.Primary,
		InactiveColor:         colors.Outline,
		BackgroundColor:       colors.Surface,
		DisabledActiveColor:   colors.SurfaceVariant,
		DisabledInactiveColor: colors.Outline,
		Size:                  20,
	}
}

// DefaultDropdownTheme returns DropdownThemeData derived from a ColorScheme.
func DefaultDropdownTheme(colors ColorScheme) DropdownThemeData {
	return DropdownThemeData{
		BackgroundColor:     colors.Surface,
		BorderColor:         colors.Outline,
		MenuBackgroundColor: colors.Surface,
		MenuBorderColor:     colors.Outline,
		SelectedItemColor:   colors.SurfaceVariant,
		TextColor:           colors.OnSurface,
		DisabledTextColor:   colors.OnSurfaceVariant,
		BorderRadius:        8,
		ItemPadding:         layout.EdgeInsetsSymmetric(12, 8),
		Height:              44,
	}
}
