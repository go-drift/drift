---
id: theming
title: Theming
sidebar_position: 5
---

# Theming

Drift provides a Material Design 3 inspired theming system.

## Using Theme

Get all theme parts in one call:

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    _, colors, textTheme := theme.UseTheme(ctx)

    return widgets.Container{
        Color:       colors.Surface,
        Child: widgets.Text{Content: "Hello", Style: textTheme.HeadlineLarge},
    }
}
```

### Individual Accessors

When you only need one part:

```go
colors := theme.ColorsOf(ctx)
textTheme := theme.TextThemeOf(ctx)
themeData := theme.ThemeOf(ctx)
```

## Providing Theme

Wrap your app with a Theme widget:

```go
theme.Theme{
    Data: theme.DefaultDarkTheme(),  // or DefaultLightTheme()
    Child: myApp,
}
```

## Built-in Themes

Drift includes light and dark themes:

```go
// Light theme
theme.DefaultLightTheme()

// Dark theme
theme.DefaultDarkTheme()
```

## Color Scheme

The color scheme follows Material Design 3:

| Color | Purpose |
|-------|---------|
| `Primary` | Main brand color |
| `OnPrimary` | Text/icons on primary |
| `Secondary` | Accent color |
| `OnSecondary` | Text/icons on secondary |
| `Surface` | Background for cards, sheets |
| `OnSurface` | Text/icons on surface |
| `Background` | App background |
| `OnBackground` | Text/icons on background |
| `Error` | Error states |
| `OnError` | Text/icons on error |

Usage:

```go
colors := theme.ColorsOf(ctx)

widgets.Container{
    Color:       colors.Surface,
    Child: child,
}

widgets.Text{Content: "Error!", Style: graphics.TextStyle{
    Color: colors.Error,
}}
```

## Text Theme

Typography follows Material Design 3:

| Style | Typical Use |
|-------|-------------|
| `DisplayLarge` | Hero text |
| `DisplayMedium` | Large headlines |
| `DisplaySmall` | Section headers |
| `HeadlineLarge` | Page titles |
| `HeadlineMedium` | Section titles |
| `HeadlineSmall` | Subsection titles |
| `TitleLarge` | Card titles |
| `TitleMedium` | List item titles |
| `TitleSmall` | Captions |
| `BodyLarge` | Primary body text |
| `BodyMedium` | Secondary body text |
| `BodySmall` | Tertiary text |
| `LabelLarge` | Button text |
| `LabelMedium` | Tab labels |
| `LabelSmall` | Chip text |

Usage:

```go
textTheme := theme.TextThemeOf(ctx)

widgets.Text{Content: "Welcome", Style: textTheme.HeadlineLarge}
widgets.Text{Content: "Body content", Style: textTheme.BodyMedium}
```

## Custom Themes

Create a custom theme by building `ThemeData`:

```go
myTheme := theme.ThemeData{
    ColorScheme: theme.ColorScheme{
        Primary:      graphics.RGB(0x67, 0x50, 0xA7),  // Purple
        OnPrimary:    graphics.ColorWhite,
        Secondary:    graphics.RGB(0x62, 0x5B, 0x71),
        OnSecondary:  graphics.ColorWhite,
        Surface:      graphics.RGB(0xFE, 0xF7, 0xFF),
        OnSurface:    graphics.RGB(0x1D, 0x1B, 0x20),
        Background:   graphics.ColorWhite,
        OnBackground: graphics.ColorBlack,
        Error:        graphics.RGB(0xB3, 0x26, 0x1E),
        OnError:      graphics.ColorWhite,
    },
    TextTheme: theme.DefaultTextTheme(),
}

theme.Theme{
    Data:        myTheme,
    Child: myApp,
}
```

## Dynamic Theming

Switch themes at runtime:

```go
type appState struct {
    core.StateBase
    isDark bool
}

func (s *appState) Build(ctx core.BuildContext) core.Widget {
    var themeData theme.ThemeData
    if s.isDark {
        themeData = theme.DefaultDarkTheme()
    } else {
        themeData = theme.DefaultLightTheme()
    }

    return theme.Theme{
        Data: themeData,
        Child: widgets.Column{
            Children: []core.Widget{
                widgets.Switch{
                    Value: s.isDark,
                    OnChanged: func(value bool) {
                        s.SetState(func() {
                            s.isDark = value
                        })
                    },
                },
                // Rest of your app
            },
        },
    }
}
```

## Nested Themes

Override theme for a subtree:

```go
theme.Theme{
    Data: theme.DefaultLightTheme(),
    Child: widgets.Column{
        Children: []core.Widget{
            lightContent,
            // Dark section within light app
            theme.Theme{
                Data: theme.DefaultDarkTheme(),
                Child: darkSection,
            },
        },
    },
}
```

## Themed Widget Constructors

Most Drift widgets are **explicit by default** — zero values mean zero, not "use theme default."
For theme-styled widgets, use the themed constructors in `pkg/theme`.

When using explicit widgets, you must provide the visual properties you want rendered.
If you omit colors, sizes, or text styles, the widget may render transparently or with zero size.
In particular, explicit `TextField`/`TextInput`, `Dropdown`, `DatePicker`, and `TimePicker`
require you to set their colors, sizes, and text styles.

### Available Constructors

| Constructor | Returns | Theme Data Used |
|------------|---------|-----------------|
| `theme.TextOf(ctx, content, style)` | `widgets.Text` | Wrap enabled by default |
| `theme.ButtonOf(ctx, label, onTap)` | `widgets.Button` | `ButtonThemeData` |
| `theme.CheckboxOf(ctx, value, onChanged)` | `widgets.Checkbox` | `CheckboxThemeData` |
| `theme.DropdownOf[T](ctx, value, items, onChanged)` | `widgets.Dropdown[T]` | `DropdownThemeData` |
| `theme.TextFieldOf(ctx, controller)` | `widgets.TextField` | `TextFieldThemeData` |
| `theme.TextFormFieldOf(ctx)` | `widgets.TextFormField` | `TextFieldThemeData` |
| `theme.ToggleOf(ctx, value, onChanged)` | `widgets.Toggle` | `SwitchThemeData` |
| `theme.RadioOf[T](ctx, value, groupValue, onChanged)` | `widgets.Radio[T]` | `RadioThemeData` |
| `theme.TabBarOf(ctx, tabs, selectedIndex, onChanged)` | `widgets.TabBar` | `TabBarThemeData` |
| `theme.DatePickerOf(ctx, value, onChanged)` | `widgets.DatePicker` | `ColorScheme` |
| `theme.TimePickerOf(ctx, hour, minute, onChanged)` | `widgets.TimePicker` | `ColorScheme` |
| `theme.IconOf(ctx, glyph)` | `widgets.Icon` | `ColorScheme` |
| `theme.CircularProgressIndicatorOf(ctx, value)` | `widgets.CircularProgressIndicator` | `ColorScheme` |
| `theme.DividerOf(ctx)` | `widgets.Divider` | `DividerThemeData` |
| `theme.VerticalDividerOf(ctx)` | `widgets.VerticalDivider` | `DividerThemeData` |
| `theme.LinearProgressIndicatorOf(ctx, value)` | `widgets.LinearProgressIndicator` | `ColorScheme` |

### Usage

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            // Themed button - reads colors, padding, etc. from theme
            theme.ButtonOf(ctx, "Save", s.onSave),

            widgets.VSpace(16),

            // Themed checkbox
            theme.CheckboxOf(ctx, s.accepted.Get(), func(v bool) {
                s.accepted.Set(v)
            }),

            widgets.VSpace(16),

            // Themed divider between sections
            theme.DividerOf(ctx),

            // Themed with override
            theme.ButtonOf(ctx, "Custom", s.onCustom).
                WithBorderRadius(0),  // zero is honored
        },
    }
}
```

### When to Use What

| Pattern | When to Use |
|---------|-------------|
| `theme.XxxOf(ctx, ...)` | Most apps — consistent theme styling |
| Struct literal | Full control over all properties |
| `.WithX()` | Override specific theme values |

### Explicit Widgets

For widgets without themed constructors (like layout widgets), pull theme values manually:

```go
_, colors, textTheme := theme.UseTheme(ctx)

widgets.Container{
    Color:   colors.Surface,
    Padding: layout.EdgeInsetsAll(16),
    Child: widgets.Text{
        Content: "Hello",
        Style:   textTheme.BodyLarge,
    },
}

widgets.DecoratedBox{
    Color:        colors.SurfaceVariant,
    BorderRadius: 8,
    Child:  content,
}
```

## Disabled Styling

Widgets support **theme-controlled disabled colors**:

- **Themed widgets** (`theme.XxxOf`) automatically use disabled colors from theme data
- **Explicit widgets** without disabled colors fall back to 0.5 opacity

```go
// Themed: uses theme disabled colors
btn := theme.ButtonOf(ctx, "Submit", onSubmit)
btn.Disabled = true  // uses DisabledBackgroundColor, DisabledForegroundColor

// Explicit without disabled colors: falls back to 0.5 opacity
widgets.Button{Label: "Submit", OnTap: onSubmit, Disabled: true}

// Explicit with custom disabled colors
widgets.Button{
    Label:             "Submit",
    Disabled:          true,
    Color:             colors.Primary,
    DisabledColor:     colors.SurfaceVariant,
    DisabledTextColor: colors.OnSurfaceVariant,
}
```

## Next Steps

- [Navigation](/docs/guides/navigation) - Navigate between screens
- [Gestures](/docs/guides/gestures) - Handle touch input
- [API Reference](/docs/api/theme) - Theme API documentation
