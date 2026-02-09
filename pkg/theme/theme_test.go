package theme

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

// --- DefaultLightTheme / DefaultDarkTheme ---

func TestDefaultLightTheme(t *testing.T) {
	th := DefaultLightTheme()
	if th == nil {
		t.Fatal("DefaultLightTheme returned nil")
	}
	if th.Brightness != BrightnessLight {
		t.Errorf("Brightness = %v, want BrightnessLight", th.Brightness)
	}
	if th.ColorScheme.Primary == 0 {
		t.Error("Primary color should be non-zero")
	}
	if th.ColorScheme.Brightness != BrightnessLight {
		t.Error("ColorScheme.Brightness should be BrightnessLight")
	}
}

func TestDefaultDarkTheme(t *testing.T) {
	th := DefaultDarkTheme()
	if th == nil {
		t.Fatal("DefaultDarkTheme returned nil")
	}
	if th.Brightness != BrightnessDark {
		t.Errorf("Brightness = %v, want BrightnessDark", th.Brightness)
	}
	if th.ColorScheme.Primary == 0 {
		t.Error("Primary color should be non-zero")
	}
	if th.ColorScheme.Brightness != BrightnessDark {
		t.Error("ColorScheme.Brightness should be BrightnessDark")
	}
}

// --- CopyWith ---

func TestThemeData_CopyWith_NilArgs(t *testing.T) {
	orig := DefaultLightTheme()
	copied := orig.CopyWith(nil, nil, nil)

	if copied.Brightness != orig.Brightness {
		t.Error("nil brightness should preserve original")
	}
	if copied.ColorScheme.Primary != orig.ColorScheme.Primary {
		t.Error("nil colorScheme should preserve original")
	}
}

func TestThemeData_CopyWith_OverrideBrightness(t *testing.T) {
	orig := DefaultLightTheme()
	dark := BrightnessDark
	copied := orig.CopyWith(nil, nil, &dark)

	if copied.Brightness != BrightnessDark {
		t.Error("brightness should be overridden to dark")
	}
	// Original unchanged
	if orig.Brightness != BrightnessLight {
		t.Error("original should be unchanged")
	}
}

func TestThemeData_CopyWith_OverrideColorScheme(t *testing.T) {
	orig := DefaultLightTheme()
	customColors := DarkColorScheme()
	copied := orig.CopyWith(&customColors, nil, nil)

	if copied.ColorScheme.Primary != customColors.Primary {
		t.Error("color scheme should be overridden")
	}
}

func TestThemeData_CopyWith_PreservesComponentThemes(t *testing.T) {
	orig := DefaultLightTheme()
	customButton := &ButtonThemeData{BorderRadius: 99}
	orig.ButtonTheme = customButton

	copied := orig.CopyWith(nil, nil, nil)

	if copied.ButtonTheme != customButton {
		t.Error("component themes should be preserved by CopyWith")
	}
}

// --- ThemeOf methods (derive defaults vs. use custom) ---

func TestButtonThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	bt := th.ButtonThemeOf()

	if bt.BackgroundColor != th.ColorScheme.Primary {
		t.Error("default ButtonTheme.BackgroundColor should match ColorScheme.Primary")
	}
	if bt.ForegroundColor != th.ColorScheme.OnPrimary {
		t.Error("default ButtonTheme.ForegroundColor should match ColorScheme.OnPrimary")
	}
}

func TestButtonThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &ButtonThemeData{BackgroundColor: graphics.RGB(1, 2, 3)}
	th.ButtonTheme = custom

	bt := th.ButtonThemeOf()
	if bt.BackgroundColor != custom.BackgroundColor {
		t.Error("should return custom button theme")
	}
}

func TestCheckboxThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	ct := th.CheckboxThemeOf()

	if ct.ActiveColor != th.ColorScheme.Primary {
		t.Error("default CheckboxTheme.ActiveColor should match Primary")
	}
}

func TestCheckboxThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &CheckboxThemeData{ActiveColor: graphics.RGB(1, 2, 3)}
	th.CheckboxTheme = custom

	if th.CheckboxThemeOf().ActiveColor != custom.ActiveColor {
		t.Error("should return custom checkbox theme")
	}
}

func TestSwitchThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	st := th.SwitchThemeOf()

	if st.ActiveTrackColor != th.ColorScheme.Primary {
		t.Error("default SwitchTheme.ActiveTrackColor should match Primary")
	}
}

func TestSwitchThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &SwitchThemeData{ActiveTrackColor: graphics.RGB(1, 2, 3)}
	th.SwitchTheme = custom

	if th.SwitchThemeOf().ActiveTrackColor != custom.ActiveTrackColor {
		t.Error("should return custom switch theme")
	}
}

func TestTextFieldThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	tf := th.TextFieldThemeOf()

	if tf.FocusColor != th.ColorScheme.Primary {
		t.Error("default TextFieldTheme.FocusColor should match Primary")
	}
	if tf.ErrorColor != th.ColorScheme.Error {
		t.Error("default TextFieldTheme.ErrorColor should match Error")
	}
}

func TestTextFieldThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &TextFieldThemeData{FocusColor: graphics.RGB(1, 2, 3)}
	th.TextFieldTheme = custom

	if th.TextFieldThemeOf().FocusColor != custom.FocusColor {
		t.Error("should return custom text field theme")
	}
}

func TestTabBarThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	tb := th.TabBarThemeOf()

	if tb.ActiveColor != th.ColorScheme.Primary {
		t.Error("default TabBarTheme.ActiveColor should match Primary")
	}
}

func TestTabBarThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &TabBarThemeData{ActiveColor: graphics.RGB(1, 2, 3)}
	th.TabBarTheme = custom

	if th.TabBarThemeOf().ActiveColor != custom.ActiveColor {
		t.Error("should return custom tab bar theme")
	}
}

func TestRadioThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	rt := th.RadioThemeOf()

	if rt.ActiveColor != th.ColorScheme.Primary {
		t.Error("default RadioTheme.ActiveColor should match Primary")
	}
}

func TestRadioThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &RadioThemeData{ActiveColor: graphics.RGB(1, 2, 3)}
	th.RadioTheme = custom

	if th.RadioThemeOf().ActiveColor != custom.ActiveColor {
		t.Error("should return custom radio theme")
	}
}

func TestDropdownThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	dt := th.DropdownThemeOf()

	if dt.TextColor != th.ColorScheme.OnSurface {
		t.Error("default DropdownTheme.TextColor should match OnSurface")
	}
}

func TestDropdownThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &DropdownThemeData{TextColor: graphics.RGB(1, 2, 3)}
	th.DropdownTheme = custom

	if th.DropdownThemeOf().TextColor != custom.TextColor {
		t.Error("should return custom dropdown theme")
	}
}

func TestBottomSheetThemeOf_Default(t *testing.T) {
	th := DefaultLightTheme()
	bs := th.BottomSheetThemeOf()

	if bs.BackgroundColor != th.ColorScheme.Surface {
		t.Error("default BottomSheetTheme.BackgroundColor should match Surface")
	}
}

func TestBottomSheetThemeOf_Custom(t *testing.T) {
	th := DefaultLightTheme()
	custom := &BottomSheetThemeData{BackgroundColor: graphics.RGB(1, 2, 3)}
	th.BottomSheetTheme = custom

	if th.BottomSheetThemeOf().BackgroundColor != custom.BackgroundColor {
		t.Error("should return custom bottom sheet theme")
	}
}

// --- ColorScheme sanity ---

func TestLightColorScheme(t *testing.T) {
	cs := LightColorScheme()
	if cs.Brightness != BrightnessLight {
		t.Error("LightColorScheme should have BrightnessLight")
	}
	if cs.Primary == 0 {
		t.Error("Primary should be non-zero")
	}
	if cs.OnPrimary == 0 {
		t.Error("OnPrimary should be non-zero")
	}
	if cs.Background == 0 {
		t.Error("Background should be non-zero")
	}
}

func TestDarkColorScheme(t *testing.T) {
	cs := DarkColorScheme()
	if cs.Brightness != BrightnessDark {
		t.Error("DarkColorScheme should have BrightnessDark")
	}
	if cs.Primary == 0 {
		t.Error("Primary should be non-zero")
	}
	if cs.OnPrimary == 0 {
		t.Error("OnPrimary should be non-zero")
	}
	if cs.Background == 0 {
		t.Error("Background should be non-zero")
	}
}

// --- Default*Theme constructors ---

func TestDefaultButtonTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	bt := DefaultButtonTheme(cs)

	if bt.BackgroundColor != cs.Primary {
		t.Error("BackgroundColor should be Primary")
	}
	if bt.ForegroundColor != cs.OnPrimary {
		t.Error("ForegroundColor should be OnPrimary")
	}
	if bt.DisabledBackgroundColor != cs.SurfaceVariant {
		t.Error("DisabledBackgroundColor should be SurfaceVariant")
	}
}

func TestDefaultCheckboxTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	ct := DefaultCheckboxTheme(cs)

	if ct.ActiveColor != cs.Primary {
		t.Error("ActiveColor should be Primary")
	}
	if ct.CheckColor != cs.OnPrimary {
		t.Error("CheckColor should be OnPrimary")
	}
	if ct.BorderColor != cs.Outline {
		t.Error("BorderColor should be Outline")
	}
}

func TestDefaultSwitchTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	st := DefaultSwitchTheme(cs)

	if st.ActiveTrackColor != cs.Primary {
		t.Error("ActiveTrackColor should be Primary")
	}
	if st.ThumbColor != cs.Surface {
		t.Error("ThumbColor should be Surface")
	}
}

func TestDefaultTextFieldTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	tf := DefaultTextFieldTheme(cs)

	if tf.FocusColor != cs.Primary {
		t.Error("FocusColor should be Primary")
	}
	if tf.ErrorColor != cs.Error {
		t.Error("ErrorColor should be Error")
	}
	if tf.TextColor != cs.OnSurface {
		t.Error("TextColor should be OnSurface")
	}
}

func TestDefaultRadioTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	rt := DefaultRadioTheme(cs)

	if rt.ActiveColor != cs.Primary {
		t.Error("ActiveColor should be Primary")
	}
	if rt.InactiveColor != cs.Outline {
		t.Error("InactiveColor should be Outline")
	}
}

func TestDefaultDropdownTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	dt := DefaultDropdownTheme(cs)

	if dt.BackgroundColor != cs.Surface {
		t.Error("BackgroundColor should be Surface")
	}
	if dt.TextColor != cs.OnSurface {
		t.Error("TextColor should be OnSurface")
	}
}

func TestDefaultBottomSheetTheme_UsesColorScheme(t *testing.T) {
	cs := LightColorScheme()
	bs := DefaultBottomSheetTheme(cs)

	if bs.BackgroundColor != cs.Surface {
		t.Error("BackgroundColor should be Surface")
	}
	if bs.HandleColor != cs.OnSurfaceVariant {
		t.Error("HandleColor should be OnSurfaceVariant")
	}
}
