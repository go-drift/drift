---
id: theming
title: Theming
sidebar_position: 6
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
        ChildWidget: widgets.TextOf("Hello", textTheme.HeadlineLarge),
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
    ChildWidget: myApp,
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
    ChildWidget: child,
}

widgets.TextOf("Error!", graphics.TextStyle{
    Color: colors.Error,
})
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

widgets.TextOf("Welcome", textTheme.HeadlineLarge)
widgets.TextOf("Body content", textTheme.BodyMedium)
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
    ChildWidget: myApp,
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
        ChildWidget: widgets.Column{
            ChildrenWidgets: []core.Widget{
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
    ChildWidget: widgets.Column{
        ChildrenWidgets: []core.Widget{
            lightContent,
            // Dark section within light app
            theme.Theme{
                Data: theme.DefaultDarkTheme(),
                ChildWidget: darkSection,
            },
        },
    },
}
```

## Next Steps

- [Navigation](/docs/guides/navigation) - Navigate between screens
- [Gestures](/docs/guides/gestures) - Handle touch input
- [API Reference](/docs/api/theme) - Theme API documentation
