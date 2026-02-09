---
id: divider
title: Divider & VerticalDivider
---

# Divider & VerticalDivider

Thin lines that separate content. `Divider` draws a horizontal line (for use in columns) and `VerticalDivider` draws a vertical line (for use in rows).

## Divider

A horizontal separator that expands to fill available width.

```go
// Themed (recommended)
theme.DividerOf(ctx)

// Explicit (full control)
widgets.Divider{
    Height:    16,
    Thickness: 1,
    Color:     colors.OutlineVariant,
}
```

### Divider Properties

| Property | Type | Description |
|----------|------|-------------|
| `Height` | `float64` | Total vertical space occupied |
| `Thickness` | `float64` | Thickness of the drawn line |
| `Color` | `graphics.Color` | Line color |
| `Indent` | `float64` | Left inset from the leading edge |
| `EndIndent` | `float64` | Right inset from the trailing edge |

## VerticalDivider

A vertical separator that expands to fill available height.

```go
// Themed (recommended)
theme.VerticalDividerOf(ctx)

// Explicit (full control)
widgets.VerticalDivider{
    Width:     16,
    Thickness: 1,
    Color:     colors.OutlineVariant,
}
```

### VerticalDivider Properties

| Property | Type | Description |
|----------|------|-------------|
| `Width` | `float64` | Total horizontal space occupied |
| `Thickness` | `float64` | Thickness of the drawn line |
| `Color` | `graphics.Color` | Line color |
| `Indent` | `float64` | Top inset |
| `EndIndent` | `float64` | Bottom inset |

## Common Patterns

### List with Dividers

```go
widgets.Column{
    CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
    Children: []core.Widget{
        listItem("Account"),
        theme.DividerOf(ctx),
        listItem("Notifications"),
        theme.DividerOf(ctx),
        listItem("Privacy"),
    },
}
```

### Indented Divider

Use `Indent` to align the divider with content that has leading padding:

```go
widgets.Divider{
    Height:    1,
    Thickness: 1,
    Color:     colors.OutlineVariant,
    Indent:    56, // align with text after a 40px avatar + 16px gap
}
```

### Vertical Separator in a Row

Pair `VerticalDivider` with `IntrinsicHeight` so the divider fills the row height:

```go
widgets.IntrinsicHeight{
    Child: widgets.Row{
        Children: []core.Widget{
            leftSection,
            theme.VerticalDividerOf(ctx),
            rightSection,
        },
    },
}
```

## Theme Data

Both widgets share `DividerThemeData`:

| Field | Default | Description |
|-------|---------|-------------|
| `Color` | `OutlineVariant` | Line color |
| `Space` | `16` | Default Height (Divider) or Width (VerticalDivider) |
| `Thickness` | `1` | Line thickness |
| `Indent` | `0` | Leading inset |
| `EndIndent` | `0` | Trailing inset |

Override via `ThemeData.DividerTheme`:

```go
theme.Theme{
    Data: &theme.ThemeData{
        ColorScheme: colors,
        DividerTheme: &theme.DividerThemeData{
            Color:     colors.Outline,
            Space:     8,
            Thickness: 2,
        },
    },
    Child: app,
}
```

## Related

- [Column & Row](/docs/catalog/layout/column-row) for the layouts that typically contain dividers
- [Container & DecoratedBox](/docs/catalog/layout/container-decoratedbox) for bordered sections
- [SizedBox](/docs/catalog/layout/sizedbox) and `VSpace`/`HSpace` helpers for plain spacing
