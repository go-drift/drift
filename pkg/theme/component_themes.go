package theme

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// ButtonThemeData defines default styling for Button widgets.
type ButtonThemeData struct {
	// BackgroundColor is the default button background.
	BackgroundColor graphics.Color
	// ForegroundColor is the default text/icon color.
	ForegroundColor graphics.Color
	// DisabledBackgroundColor is the background when disabled.
	DisabledBackgroundColor graphics.Color
	// DisabledForegroundColor is the text color when disabled.
	DisabledForegroundColor graphics.Color
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
	ActiveColor graphics.Color
	// CheckColor is the checkmark color.
	CheckColor graphics.Color
	// BorderColor is the outline color when unchecked.
	BorderColor graphics.Color
	// BackgroundColor is the fill color when unchecked.
	BackgroundColor graphics.Color
	// DisabledActiveColor is the fill color when checked and disabled.
	DisabledActiveColor graphics.Color
	// DisabledCheckColor is the checkmark color when disabled.
	DisabledCheckColor graphics.Color
	// Size is the default checkbox size.
	Size float64
	// BorderRadius is the default corner radius.
	BorderRadius float64
}

// SwitchThemeData defines default styling for Switch widgets.
type SwitchThemeData struct {
	// ActiveTrackColor is the track color when on.
	ActiveTrackColor graphics.Color
	// InactiveTrackColor is the track color when off.
	InactiveTrackColor graphics.Color
	// ThumbColor is the thumb fill color.
	ThumbColor graphics.Color
	// DisabledActiveTrackColor is the track color when on and disabled.
	DisabledActiveTrackColor graphics.Color
	// DisabledInactiveTrackColor is the track color when off and disabled.
	DisabledInactiveTrackColor graphics.Color
	// DisabledThumbColor is the thumb color when disabled.
	DisabledThumbColor graphics.Color
	// Width is the default switch width.
	Width float64
	// Height is the default switch height.
	Height float64
}

// TextFieldThemeData defines default styling for TextField widgets.
type TextFieldThemeData struct {
	// BackgroundColor is the field background.
	BackgroundColor graphics.Color
	// BorderColor is the default border color.
	BorderColor graphics.Color
	// FocusColor is the border color when focused.
	FocusColor graphics.Color
	// ErrorColor is the border color when in error state.
	ErrorColor graphics.Color
	// LabelColor is the label text color.
	LabelColor graphics.Color
	// TextColor is the input text color.
	TextColor graphics.Color
	// PlaceholderColor is the placeholder text color.
	PlaceholderColor graphics.Color
	// Padding is the default inner padding.
	Padding layout.EdgeInsets
	// BorderRadius is the default corner radius.
	BorderRadius float64
	// BorderWidth is the default border stroke width.
	BorderWidth float64
	// Height is the default field height.
	Height float64
}

// TabBarThemeData defines default styling for TabBar widgets.
type TabBarThemeData struct {
	// BackgroundColor is the tab bar background.
	BackgroundColor graphics.Color
	// ActiveColor is the color for the selected tab.
	ActiveColor graphics.Color
	// InactiveColor is the color for unselected tabs.
	InactiveColor graphics.Color
	// IndicatorColor is the color for the selection indicator.
	IndicatorColor graphics.Color
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
	ActiveColor graphics.Color
	// InactiveColor is the border color when unselected.
	InactiveColor graphics.Color
	// BackgroundColor is the fill color when unselected.
	BackgroundColor graphics.Color
	// DisabledActiveColor is the fill color when selected and disabled.
	DisabledActiveColor graphics.Color
	// DisabledInactiveColor is the border color when disabled.
	DisabledInactiveColor graphics.Color
	// Size is the default radio diameter.
	Size float64
}

// DropdownThemeData defines default styling for Dropdown widgets.
type DropdownThemeData struct {
	// BackgroundColor is the trigger background.
	BackgroundColor graphics.Color
	// BorderColor is the trigger border color.
	BorderColor graphics.Color
	// MenuBackgroundColor is the dropdown menu background.
	MenuBackgroundColor graphics.Color
	// MenuBorderColor is the dropdown menu border color.
	MenuBorderColor graphics.Color
	// SelectedItemColor is the background for the selected item.
	SelectedItemColor graphics.Color
	// TextColor is the default text color.
	TextColor graphics.Color
	// DisabledTextColor is the text color when disabled.
	DisabledTextColor graphics.Color
	// BorderRadius is the default corner radius.
	BorderRadius float64
	// ItemPadding is the default padding for menu items.
	ItemPadding layout.EdgeInsets
	// Height is the default trigger/item height.
	Height float64
	// FontSize is the default text font size.
	FontSize float64
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
		BorderWidth:      1,
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
		FontSize:            16,
	}
}

// BottomSheetThemeData defines default styling for BottomSheet widgets.
type BottomSheetThemeData struct {
	// BackgroundColor is the sheet's background color.
	BackgroundColor graphics.Color
	// HandleColor is the drag handle indicator color.
	HandleColor graphics.Color
	// BarrierColor is the color of the semi-transparent barrier behind the sheet.
	BarrierColor graphics.Color
	// BorderRadius is the corner radius for the top corners of the sheet.
	BorderRadius float64
	// HandleWidth is the width of the drag handle indicator.
	HandleWidth float64
	// HandleHeight is the height of the drag handle indicator.
	HandleHeight float64
	// HandleTopPadding is the padding above the drag handle.
	HandleTopPadding float64
	// HandleBottomPadding is the padding below the drag handle.
	HandleBottomPadding float64
}

// DefaultBottomSheetTheme returns BottomSheetThemeData derived from a ColorScheme.
func DefaultBottomSheetTheme(colors ColorScheme) BottomSheetThemeData {
	return BottomSheetThemeData{
		BackgroundColor:     colors.Surface,
		HandleColor:         colors.OnSurfaceVariant,
		BarrierColor:        graphics.RGBA(0, 0, 0, 0.5),
		BorderRadius:        16,
		HandleWidth:         32,
		HandleHeight:        4,
		HandleTopPadding:    8,
		HandleBottomPadding: 8,
	}
}
