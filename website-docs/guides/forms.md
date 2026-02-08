---
id: forms
title: Forms & Validation
sidebar_position: 7
---

# Forms & Validation

Use `Form` with `TextFormField` for validated text input. The form tracks all fields and provides `Validate()`, `Save()`, and `Reset()` methods.

## Basic Usage

```go
type loginState struct {
    core.StateBase
    email    string
    password string
}

func (s *loginState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Form{
        Autovalidate: true,
        Child:  loginForm{state: s},
    }
}

// Separate widget to access FormOf(ctx)
type loginForm struct {
    state *loginState
}

func (f loginForm) Build(ctx core.BuildContext) core.Widget {
    form := widgets.FormOf(ctx)

    return widgets.ColumnOf(
        widgets.MainAxisAlignmentStart,
        widgets.CrossAxisAlignmentStretch,
        widgets.MainAxisSizeMin,

        // Themed form field (recommended)
        theme.TextFormFieldOf(ctx).
            WithLabel("Email").
            WithPlaceholder("you@example.com").
            WithValidator(func(value string) string {
                if value == "" {
                    return "Email is required"
                }
                if !strings.Contains(value, "@") {
                    return "Please enter a valid email"
                }
                return ""
            }).
            WithOnSaved(func(value string) {
                f.state.email = value
            }),
        widgets.VSpace(16),

        theme.TextFormFieldOf(ctx).
            WithLabel("Password").
            WithPlaceholder("Enter password").
            WithObscure(true).
            WithValidator(func(value string) string {
                if len(value) < 8 {
                    return "Password must be at least 8 characters"
                }
                return ""
            }).
            WithOnSaved(func(value string) {
                f.state.password = value
            }),
        widgets.VSpace(24),

        theme.ButtonOf(ctx, "Submit", func() {
            if form.Validate() {
                form.Save()
                // Use f.state.email and f.state.password
            }
        }),
    )
}
```

## Form Methods

| Method | Description |
|--------|-------------|
| `Validate()` | Run validators on all fields, returns `bool` |
| `Save()` | Call `OnSaved` for all fields |
| `Reset()` | Reset all fields to initial values |

## TextFormField Options

| Field | Description |
|-------|-------------|
| `TextField` | Base TextField for theme styling (use with `theme.TextFieldOf`) |
| `InitialValue` | Starting value when no Controller is provided |
| `Validator` | Returns error message or empty string if valid |
| `OnSaved` | Called when the form is saved |
| `OnChanged` | Called when the field value changes |
| `Autovalidate` | Validate on every change |
| `Obscure` | Hide text (for passwords) |
| `KeyboardType` | Keyboard type (`KeyboardTypeEmail`, `KeyboardTypeNumber`, etc.) |
| `InputAction` | Action button (`TextInputActionNext`, `TextInputActionDone`, etc.) |
| `LabelStyle` | Style for the label text above the field |
| `HelperStyle` | Style for helper/error text below the field |
| `ErrorColor` | Color for error text and border when validation fails |

## Themed vs Explicit

```go
// Themed (recommended) - inherits colors, typography from theme
theme.TextFormFieldOf(ctx).
    WithLabel("Email").
    WithValidator(validateEmail)

// Explicit - must provide all visual properties
widgets.TextFormField{
    Label:      "Email",
    LabelStyle: graphics.TextStyle{Color: labelColor, FontSize: 14},
    HelperStyle: graphics.TextStyle{Color: helperColor, FontSize: 12},
    ErrorColor: errorColor,
    Validator:  validateEmail,
}
```

## Next Steps

- [TextField](/docs/catalog/input/textfield) - Text input widget details
- [Checkbox & Radio](/docs/catalog/input/checkbox-radio) - Selection controls
- [Dropdown](/docs/catalog/input/dropdown) - Selection menus
- [State Management](/docs/guides/state-management) - Managing widget state
