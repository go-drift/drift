---
id: text
title: Text
---

# Text

Displays a string of text with a single style.

## Basic Usage

```go
widgets.Text{
    Content: "Hello, Drift",
    Style:   graphics.TextStyle{Color: colors.OnSurface, FontSize: 16},
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Content` | `string` | Text to display |
| `Style` | `graphics.TextStyle` | Font size, color, weight, and other styling |

## Using Text Themes

Read typography styles from the current theme for consistent sizing and weight:

```go
textTheme := theme.TextThemeOf(ctx)

widgets.Text{Content: "Headline", Style: textTheme.HeadlineLarge}
widgets.Text{Content: "Body text", Style: textTheme.BodyLarge}
widgets.Text{Content: "Caption", Style: textTheme.BodySmall}
```

## Common Patterns

### Styled Text

```go
widgets.Text{
    Content: "Important",
    Style: graphics.TextStyle{
        Color:    colors.Primary,
        FontSize: 18,
        Weight:   graphics.FontWeightBold,
    },
}
```

### Text in a Layout

```go
widgets.Column{
    MainAxisSize: widgets.MainAxisSizeMin,
    Children: []core.Widget{
        widgets.Text{Content: title, Style: textTheme.TitleMedium},
        widgets.VSpace(4),
        widgets.Text{Content: subtitle, Style: textTheme.BodySmall},
    },
}
```

## Related

- [Icon](/docs/catalog/display/icon) for material icons
- [Theming](/docs/guides/theming) for typography configuration
