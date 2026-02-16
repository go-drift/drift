package theme

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/widgets"
)

// TextOf creates a [widgets.Text] with the given content and style, with
// wrapping enabled by default.
//
// This is a convenient way to create text that follows the app's typography.
// Pass a style from the theme's [TextTheme] for consistent typography:
//
//	_, _, textTheme := theme.UseTheme(ctx)
//	theme.TextOf(ctx, "Welcome", textTheme.HeadlineMedium)
//
// The returned text has Wrap set to true (text wraps to multiple lines by default).
//
// For fully explicit text without wrapping, use [widgets.Text] struct literals.
//
// Example:
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    _, _, textTheme := theme.UseTheme(ctx)
//	    return widgets.Column{
//	        Children: []core.Widget{
//	            theme.TextOf(ctx, "Welcome", textTheme.HeadlineMedium),
//	            widgets.VSpace(8),
//	            theme.TextOf(ctx, "Please sign in to continue", textTheme.BodyLarge),
//	        },
//	    }
//	}
func TextOf(ctx core.BuildContext, content string, style graphics.TextStyle) widgets.Text {
	return widgets.Text{
		Content: content,
		Style:   style,
		Wrap:    true,
	}
}

// ButtonOf creates a [widgets.Button] with visual properties filled from the
// current theme's [ButtonThemeData].
//
// This is the recommended way to create buttons that follow the app's theme.
// The returned button has:
//   - Color set to ButtonThemeData.BackgroundColor
//   - TextColor set to ButtonThemeData.ForegroundColor
//   - Padding set to ButtonThemeData.Padding
//   - FontSize set to ButtonThemeData.FontSize
//   - BorderRadius set to ButtonThemeData.BorderRadius
//   - Haptic set to true
//
// To override specific properties, chain WithX methods on the returned button.
// WithX always takes precedence over theme values:
//
//	theme.ButtonOf(ctx, "Save", onSave).
//	    WithBorderRadius(0).           // sharp corners instead of themed radius
//	    WithPadding(layout.EdgeInsetsAll(20))  // custom padding
//
// For fully explicit buttons without theme styling, use [widgets.Button]
// struct literals.
//
// Example:
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Column{
//	        Children: []core.Widget{
//	            theme.ButtonOf(ctx, "Primary Action", s.onPrimary),
//	            widgets.VSpace(12),
//	            theme.ButtonOf(ctx, "Secondary", s.onSecondary).
//	                WithColor(colors.Secondary, colors.OnSecondary),
//	        },
//	    }
//	}
func ButtonOf(ctx core.BuildContext, label string, onTap func()) widgets.Button {
	th := ThemeOf(ctx).ButtonThemeOf()
	return widgets.Button{
		Label:             label,
		OnTap:             onTap,
		Color:             th.BackgroundColor,
		TextColor:         th.ForegroundColor,
		Padding:           th.Padding,
		FontSize:          th.FontSize,
		BorderRadius:      th.BorderRadius,
		Haptic:            true,
		DisabledColor:     th.DisabledBackgroundColor,
		DisabledTextColor: th.DisabledForegroundColor,
	}
}

// CheckboxOf creates a [widgets.Checkbox] with visual properties filled from
// the current theme's [CheckboxThemeData].
//
// This is the recommended way to create checkboxes that follow the app's theme.
// The returned checkbox has:
//   - ActiveColor set to CheckboxThemeData.ActiveColor
//   - CheckColor set to CheckboxThemeData.CheckColor
//   - BorderColor set to CheckboxThemeData.BorderColor
//   - BackgroundColor set to CheckboxThemeData.BackgroundColor
//   - Size set to CheckboxThemeData.Size
//   - BorderRadius set to CheckboxThemeData.BorderRadius
//
// To override specific properties, chain WithX methods on the returned checkbox.
//
// For fully explicit checkboxes without theme styling, use [widgets.Checkbox]
// struct literals.
//
// Example:
//
//	theme.CheckboxOf(ctx, isChecked, func(value bool) {
//	    s.SetState(func() { s.isChecked = value })
//	})
func CheckboxOf(ctx core.BuildContext, value bool, onChanged func(bool)) widgets.Checkbox {
	th := ThemeOf(ctx).CheckboxThemeOf()
	return widgets.Checkbox{
		Value:               value,
		OnChanged:           onChanged,
		ActiveColor:         th.ActiveColor,
		CheckColor:          th.CheckColor,
		BorderColor:         th.BorderColor,
		BackgroundColor:     th.BackgroundColor,
		Size:                th.Size,
		BorderRadius:        th.BorderRadius,
		DisabledActiveColor: th.DisabledActiveColor,
		DisabledCheckColor:  th.DisabledCheckColor,
	}
}

// DropdownOf creates a [widgets.Dropdown] with visual properties filled from
// the current theme's [DropdownThemeData].
//
// This is the recommended way to create dropdowns that follow the app's theme.
// The returned dropdown has:
//   - BackgroundColor set to DropdownThemeData.BackgroundColor
//   - BorderColor set to DropdownThemeData.BorderColor
//   - MenuBackgroundColor set to DropdownThemeData.MenuBackgroundColor
//   - MenuBorderColor set to DropdownThemeData.MenuBorderColor
//   - BorderRadius set to DropdownThemeData.BorderRadius
//   - Height set to DropdownThemeData.Height
//   - ItemPadding set to DropdownThemeData.ItemPadding
//   - TextStyle.Color set to DropdownThemeData.TextColor
//   - SelectedItemColor set to DropdownThemeData.SelectedItemColor
//
// To override specific properties, chain WithX methods on the returned dropdown.
//
// For fully explicit dropdowns without theme styling, use [widgets.Dropdown]
// struct literals.
//
// Example:
//
//	theme.DropdownOf(ctx, selectedPlan, []widgets.DropdownItem[string]{
//	    {Value: "starter", Label: "Starter"},
//	    {Value: "pro", Label: "Pro"},
//	}, func(value string) {
//	    s.SetState(func() { s.selectedPlan = value })
//	})
func DropdownOf[T any](ctx core.BuildContext, value T, items []widgets.DropdownItem[T], onChanged func(T)) widgets.Dropdown[T] {
	th := ThemeOf(ctx).DropdownThemeOf()
	return widgets.Dropdown[T]{
		Value:               value,
		Items:               items,
		OnChanged:           onChanged,
		BackgroundColor:     th.BackgroundColor,
		BorderColor:         th.BorderColor,
		MenuBackgroundColor: th.MenuBackgroundColor,
		MenuBorderColor:     th.MenuBorderColor,
		BorderRadius:        th.BorderRadius,
		Height:              th.Height,
		ItemPadding:         th.ItemPadding,
		TextStyle:           graphics.TextStyle{Color: th.TextColor, FontSize: th.FontSize},
		SelectedItemColor:   th.SelectedItemColor,
		DisabledTextColor:   th.DisabledTextColor,
	}
}

// TextFieldOf creates a [widgets.TextField] with visual properties filled from
// the current theme's [TextFieldThemeData].
//
// This is the recommended way to create text fields that follow the app's theme.
// The returned text field has all visual properties pre-filled from the theme,
// including colors, dimensions, and typography styles.
//
// To override specific properties, chain WithX methods on the returned text field.
//
// For fully explicit text fields without theme styling, use [widgets.TextField]
// struct literals (you must provide all visual properties).
//
// Example:
//
//	theme.TextFieldOf(ctx, controller).
//	    WithPlaceholder("Enter email").
//	    WithLabel("Email")
func TextFieldOf(ctx core.BuildContext, controller *platform.TextEditingController) widgets.TextField {
	th := ThemeOf(ctx).TextFieldThemeOf()
	_, _, textTheme := UseTheme(ctx)
	return widgets.TextField{
		Controller:       controller,
		BackgroundColor:  th.BackgroundColor,
		BorderColor:      th.BorderColor,
		FocusColor:       th.FocusColor,
		PlaceholderColor: th.PlaceholderColor,
		BorderRadius:     th.BorderRadius,
		BorderWidth:      th.BorderWidth,
		Height:           th.Height,
		Padding:          th.Padding,
		Style:            graphics.TextStyle{FontSize: textTheme.BodyLarge.FontSize, Color: th.TextColor},
		LabelStyle:       graphics.TextStyle{FontSize: textTheme.LabelMedium.FontSize, Color: th.LabelColor},
		HelperStyle:      graphics.TextStyle{FontSize: textTheme.BodySmall.FontSize, Color: th.LabelColor},
		ErrorColor:       th.ErrorColor,
	}
}

// TextFormFieldOf creates a [widgets.TextFormField] with visual properties filled
// from the current theme's [TextFieldThemeData].
//
// This is the recommended way to create form text fields that follow the app's theme.
// The returned text form field has all visual properties pre-filled from the theme
// via an embedded TextField, including colors, dimensions, and typography styles.
//
// To override specific properties, chain WithX methods on the returned text form field.
//
// For fully explicit text form fields without theme styling, use [widgets.TextFormField]
// struct literals (you must provide all visual properties).
//
// Example:
//
//	theme.TextFormFieldOf(ctx).
//	    WithLabel("Email").
//	    WithPlaceholder("Enter your email").
//	    WithValidator(func(value string) string {
//	        if !strings.Contains(value, "@") {
//	            return "Invalid email address"
//	        }
//	        return ""
//	    })
func TextFormFieldOf(ctx core.BuildContext) widgets.TextFormField {
	return widgets.TextFormField{
		TextField: TextFieldOf(ctx, nil),
	}
}

// ToggleOf creates a [widgets.Toggle] with visual properties filled from the
// current theme's [SwitchThemeData].
//
// This is the recommended way to create toggles that follow the app's theme.
// The returned toggle has:
//   - ActiveColor set to SwitchThemeData.ActiveTrackColor
//   - InactiveColor set to SwitchThemeData.InactiveTrackColor
//   - ThumbColor set to SwitchThemeData.ThumbColor
//   - Width set to SwitchThemeData.Width
//   - Height set to SwitchThemeData.Height
//
// For fully explicit toggles without theme styling, use [widgets.Toggle]
// struct literals.
//
// Example:
//
//	theme.ToggleOf(ctx, isEnabled, func(value bool) {
//	    s.SetState(func() { s.isEnabled = value })
//	})
func ToggleOf(ctx core.BuildContext, value bool, onChanged func(bool)) widgets.Toggle {
	th := ThemeOf(ctx).SwitchThemeOf()
	return widgets.Toggle{
		Value:                 value,
		OnChanged:             onChanged,
		ActiveColor:           th.ActiveTrackColor,
		InactiveColor:         th.InactiveTrackColor,
		ThumbColor:            th.ThumbColor,
		Width:                 th.Width,
		Height:                th.Height,
		DisabledActiveColor:   th.DisabledActiveTrackColor,
		DisabledInactiveColor: th.DisabledInactiveTrackColor,
		DisabledThumbColor:    th.DisabledThumbColor,
	}
}

// RadioOf creates a [widgets.Radio] with visual properties filled from the
// current theme's [RadioThemeData].
//
// This is the recommended way to create radio buttons that follow the app's theme.
// The returned radio has:
//   - ActiveColor set to RadioThemeData.ActiveColor
//   - InactiveColor set to RadioThemeData.InactiveColor
//   - BackgroundColor set to RadioThemeData.BackgroundColor
//   - Size set to RadioThemeData.Size
//
// For fully explicit radio buttons without theme styling, use [widgets.Radio]
// struct literals.
//
// Example:
//
//	theme.RadioOf(ctx, "email", selectedMethod, func(value string) {
//	    s.SetState(func() { s.selectedMethod = value })
//	})
func RadioOf[T any](ctx core.BuildContext, value, groupValue T, onChanged func(T)) widgets.Radio[T] {
	th := ThemeOf(ctx).RadioThemeOf()
	return widgets.Radio[T]{
		Value:                 value,
		GroupValue:            groupValue,
		OnChanged:             onChanged,
		ActiveColor:           th.ActiveColor,
		InactiveColor:         th.InactiveColor,
		BackgroundColor:       th.BackgroundColor,
		Size:                  th.Size,
		DisabledActiveColor:   th.DisabledActiveColor,
		DisabledInactiveColor: th.DisabledInactiveColor,
	}
}

// TabBarOf creates a [widgets.TabBar] with visual properties filled from the
// current theme's [TabBarThemeData].
//
// This is the recommended way to create tab bars that follow the app's theme.
// The returned tab bar has:
//   - BackgroundColor set to TabBarThemeData.BackgroundColor
//   - ActiveColor set to TabBarThemeData.ActiveColor
//   - InactiveColor set to TabBarThemeData.InactiveColor
//   - IndicatorColor set to TabBarThemeData.IndicatorColor
//   - IndicatorHeight set to TabBarThemeData.IndicatorHeight
//   - Padding set to TabBarThemeData.Padding
//   - Height set to TabBarThemeData.Height
//   - LabelStyle set to TextTheme.LabelSmall
//
// For fully explicit tab bars without theme styling, use [widgets.TabBar]
// struct literals.
//
// Example:
//
//	theme.TabBarOf(ctx, []widgets.TabItem{
//	    {Label: "Home"},
//	    {Label: "Search"},
//	    {Label: "Profile"},
//	}, currentIndex, func(index int) {
//	    s.SetState(func() { s.currentIndex = index })
//	})
func TabBarOf(ctx core.BuildContext, items []widgets.TabItem, currentIndex int, onTap func(int)) widgets.TabBar {
	th := ThemeOf(ctx).TabBarThemeOf()
	_, _, textTheme := UseTheme(ctx)
	return widgets.TabBar{
		Items:           items,
		CurrentIndex:    currentIndex,
		OnTap:           onTap,
		BackgroundColor: th.BackgroundColor,
		ActiveColor:     th.ActiveColor,
		InactiveColor:   th.InactiveColor,
		IndicatorColor:  th.IndicatorColor,
		IndicatorHeight: th.IndicatorHeight,
		Padding:         th.Padding,
		Height:          th.Height,
		LabelStyle:      textTheme.LabelSmall,
	}
}

// DatePickerOf creates a [widgets.DatePicker] with visual properties filled from
// the current theme's colors.
//
// This is the recommended way to create date pickers that follow the app's theme.
// The returned date picker has a pre-configured InputDecoration with themed colors
// and a placeholder of "Select date".
//
// Example:
//
//	theme.DatePickerOf(ctx, selectedDate, func(date time.Time) {
//	    s.SetState(func() { s.selectedDate = &date })
//	})
func DatePickerOf(ctx core.BuildContext, value *time.Time, onChanged func(time.Time)) widgets.DatePicker {
	_, colors, _ := UseTheme(ctx)
	return widgets.DatePicker{
		Value:       value,
		OnChanged:   onChanged,
		Placeholder: "Select date",
		TextStyle:   graphics.TextStyle{FontSize: 16, Color: colors.OnSurface},
		Decoration: &widgets.InputDecoration{
			BorderRadius:    8,
			BorderColor:     colors.Outline,
			BackgroundColor: colors.Surface,
			HintStyle:       graphics.TextStyle{FontSize: 16, Color: colors.OnSurfaceVariant},
			LabelStyle:      graphics.TextStyle{FontSize: 14, Color: colors.OnSurfaceVariant},
			HelperStyle:     graphics.TextStyle{FontSize: 12, Color: colors.OnSurfaceVariant},
			ErrorColor:      colors.Error,
		},
	}
}

// TimePickerOf creates a [widgets.TimePicker] with visual properties filled from
// the current theme's colors.
//
// This is the recommended way to create time pickers that follow the app's theme.
// The returned time picker has a pre-configured InputDecoration with themed colors.
//
// Example:
//
//	theme.TimePickerOf(ctx, hour, minute, func(h, m int) {
//	    s.SetState(func() { s.hour, s.minute = h, m })
//	})
func TimePickerOf(ctx core.BuildContext, hour, minute int, onChanged func(hour, minute int)) widgets.TimePicker {
	_, colors, _ := UseTheme(ctx)
	return widgets.TimePicker{
		Hour:      hour,
		Minute:    minute,
		OnChanged: onChanged,
		TextStyle: graphics.TextStyle{FontSize: 16, Color: colors.OnSurface},
		Decoration: &widgets.InputDecoration{
			BorderRadius:    8,
			BorderColor:     colors.Outline,
			BackgroundColor: colors.Surface,
			HintStyle:       graphics.TextStyle{FontSize: 16, Color: colors.OnSurfaceVariant},
			LabelStyle:      graphics.TextStyle{FontSize: 14, Color: colors.OnSurfaceVariant},
			HelperStyle:     graphics.TextStyle{FontSize: 12, Color: colors.OnSurfaceVariant},
			ErrorColor:      colors.Error,
		},
	}
}

// IconOf creates a [widgets.Icon] with visual properties filled from the
// current theme's colors.
//
// This is the recommended way to create icons that follow the app's theme.
// The returned icon has:
//   - Size set to 24 (standard icon size)
//   - Color set to ColorScheme.OnSurface
//
// To override specific properties, set fields on the returned icon:
//
//	icon := theme.IconOf(ctx, "★")
//	icon.Size = 32
//	icon.Color = colors.Primary
//
// For fully explicit icons without theme styling, use [widgets.Icon]
// struct literals.
//
// Example:
//
//	theme.IconOf(ctx, "✓")
func IconOf(ctx core.BuildContext, glyph string) widgets.Icon {
	_, colors, _ := UseTheme(ctx)
	return widgets.Icon{
		Glyph: glyph,
		Size:  24,
		Color: colors.OnSurface,
	}
}

// CircularProgressIndicatorOf creates a [widgets.CircularProgressIndicator] with
// visual properties filled from the current theme's colors.
//
// This is the recommended way to create circular progress indicators that follow
// the app's theme. The returned indicator has:
//   - Color set to ColorScheme.Primary
//   - TrackColor set to ColorScheme.SurfaceVariant
//   - Size set to 36 (standard size)
//   - StrokeWidth set to 4
//
// Pass nil for value to create an indeterminate (spinning) indicator, or pass
// a pointer to a float64 (0.0 to 1.0) for determinate progress.
//
// Example:
//
//	// Indeterminate (spinning)
//	theme.CircularProgressIndicatorOf(ctx, nil)
//
//	// Determinate (50% complete)
//	progress := 0.5
//	theme.CircularProgressIndicatorOf(ctx, &progress)
func CircularProgressIndicatorOf(ctx core.BuildContext, value *float64) widgets.CircularProgressIndicator {
	_, colors, _ := UseTheme(ctx)
	return widgets.CircularProgressIndicator{
		Value:       value,
		Color:       colors.Primary,
		TrackColor:  colors.SurfaceVariant,
		Size:        36,
		StrokeWidth: 4,
	}
}

// DividerOf creates a [widgets.Divider] with visual properties filled from the
// current theme's [DividerThemeData].
//
// This is the recommended way to create horizontal dividers that follow the
// app's theme. The returned divider has:
//   - Height set to DividerThemeData.Space
//   - Thickness set to DividerThemeData.Thickness
//   - Color set to DividerThemeData.Color
//   - Indent set to DividerThemeData.Indent
//   - EndIndent set to DividerThemeData.EndIndent
//
// For fully explicit dividers without theme styling, use [widgets.Divider]
// struct literals.
//
// Example:
//
//	theme.DividerOf(ctx)
func DividerOf(ctx core.BuildContext) widgets.Divider {
	th := ThemeOf(ctx).DividerThemeOf()
	return widgets.Divider{
		Height:    th.Space,
		Thickness: th.Thickness,
		Color:     th.Color,
		Indent:    th.Indent,
		EndIndent: th.EndIndent,
	}
}

// VerticalDividerOf creates a [widgets.VerticalDivider] with visual properties
// filled from the current theme's [DividerThemeData].
//
// This is the recommended way to create vertical dividers that follow the
// app's theme. The returned divider has:
//   - Width set to DividerThemeData.Space
//   - Thickness set to DividerThemeData.Thickness
//   - Color set to DividerThemeData.Color
//   - Indent set to DividerThemeData.Indent
//   - EndIndent set to DividerThemeData.EndIndent
//
// For fully explicit vertical dividers without theme styling, use
// [widgets.VerticalDivider] struct literals.
//
// Example:
//
//	theme.VerticalDividerOf(ctx)
func VerticalDividerOf(ctx core.BuildContext) widgets.VerticalDivider {
	th := ThemeOf(ctx).DividerThemeOf()
	return widgets.VerticalDivider{
		Width:     th.Space,
		Thickness: th.Thickness,
		Color:     th.Color,
		Indent:    th.Indent,
		EndIndent: th.EndIndent,
	}
}

// RichTextOf creates a [widgets.RichText] with themed defaults for color and
// font size. Theme values are set on the widget-level [widgets.RichText.Style]
// field, which acts as the lowest-priority base: the Content span tree's own
// styles override it, and child spans override their parents as usual.
//
// To override layout properties, chain With* methods on the returned widget:
//
//	theme.RichTextOf(ctx, spans...).WithAlign(graphics.TextAlignCenter).WithMaxLines(3)
//
// Example:
//
//	theme.RichTextOf(ctx,
//	    graphics.Span("Hello "),
//	    graphics.Span("World").Bold(),
//	)
func RichTextOf(ctx core.BuildContext, spans ...graphics.TextSpan) widgets.RichText {
	_, colors, textTheme := UseTheme(ctx)
	return widgets.RichText{
		Content: graphics.Spans(spans...),
		Style: graphics.SpanStyle{
			Color:    colors.OnSurface,
			FontSize: textTheme.BodyMedium.FontSize,
		},
		Wrap: true,
	}
}

// LinearProgressIndicatorOf creates a [widgets.LinearProgressIndicator] with
// visual properties filled from the current theme's colors.
//
// This is the recommended way to create linear progress indicators that follow
// the app's theme. The returned indicator has:
//   - Color set to ColorScheme.Primary
//   - TrackColor set to ColorScheme.SurfaceVariant
//   - Height set to 4
//   - BorderRadius set to 2
//
// Pass nil for value to create an indeterminate (animating) indicator, or pass
// a pointer to a float64 (0.0 to 1.0) for determinate progress.
//
// Example:
//
//	// Indeterminate (animating)
//	theme.LinearProgressIndicatorOf(ctx, nil)
//
//	// Determinate (75% complete)
//	progress := 0.75
//	theme.LinearProgressIndicatorOf(ctx, &progress)
func LinearProgressIndicatorOf(ctx core.BuildContext, value *float64) widgets.LinearProgressIndicator {
	_, colors, _ := UseTheme(ctx)
	return widgets.LinearProgressIndicator{
		Value:        value,
		Color:        colors.Primary,
		TrackColor:   colors.SurfaceVariant,
		Height:       4,
		BorderRadius: 2,
	}
}
