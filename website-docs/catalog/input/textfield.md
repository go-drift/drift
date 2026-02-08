---
id: textfield
title: TextField
---

# TextField

Native text input with decoration, label, and helper text. `TextInput` is the base native control; `TextField` wraps it with label, placeholder, and styling.

## Basic Usage

```go
// Themed (recommended)
theme.TextFieldOf(ctx).
    WithLabel("Email").
    WithPlaceholder("you@example.com").
    WithOnChanged(func(value string) {
        s.SetState(func() { email = value })
    })

// Explicit (must provide all visual properties)
widgets.TextField{
    Label:           "Email",
    Placeholder:     "you@example.com",
    Height:          48,
    Padding:         layout.EdgeInsetsSymmetric(12, 8),
    BackgroundColor: colors.Surface,
    BorderColor:     colors.Outline,
    FocusColor:      colors.Primary,
    BorderWidth:     1,
    BorderRadius:    8,
    Style:           graphics.TextStyle{FontSize: 16, Color: colors.OnSurface},
    PlaceholderColor: colors.OnSurfaceVariant,
    OnChanged: func(value string) {
        s.SetState(func() { email = value })
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Label` | `string` | Label text above the field |
| `Placeholder` | `string` | Placeholder text when empty |
| `Height` | `float64` | Field height |
| `Padding` | `layout.EdgeInsets` | Inner padding |
| `BackgroundColor` | `color.Color` | Background color |
| `BorderColor` | `color.Color` | Border color |
| `FocusColor` | `color.Color` | Border color when focused |
| `BorderWidth` | `float64` | Border width |
| `BorderRadius` | `float64` | Corner radius |
| `Style` | `graphics.TextStyle` | Text style (font size, color) |
| `PlaceholderColor` | `color.Color` | Placeholder text color |
| `OnChanged` | `func(string)` | Called when text changes |
| `KeyboardType` | `KeyboardType` | Keyboard type (`KeyboardTypeEmail`, `KeyboardTypeNumber`, etc.) |
| `InputAction` | `TextInputAction` | Action button (`TextInputActionNext`, `TextInputActionDone`, etc.) |
| `Obscure` | `bool` | Hide text (for passwords) |

## Explicit Styling Requirements

Explicit text fields only render what you set. If colors, sizes, or text styles are zero, the widget can be invisible or collapsed. You must set `Height`, `Padding`, `BackgroundColor`, `BorderColor`, `FocusColor`, `BorderWidth`, `Style` (FontSize + Color), and `PlaceholderColor`.

If you want defaults from the theme, prefer `theme.TextFieldOf(ctx)`.

## Common Patterns

### Password Field

```go
theme.TextFieldOf(ctx).
    WithLabel("Password").
    WithPlaceholder("Enter password").
    WithObscure(true).
    WithInputAction(widgets.TextInputActionDone)
```

### Email Field

```go
theme.TextFieldOf(ctx).
    WithLabel("Email").
    WithPlaceholder("you@example.com").
    WithKeyboardType(widgets.KeyboardTypeEmail).
    WithInputAction(widgets.TextInputActionNext)
```

## Related

- [Forms & Validation](/docs/guides/forms) for TextFormField with validation
- [Button](/docs/catalog/input/button) for form submission
