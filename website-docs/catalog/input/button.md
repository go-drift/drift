---
id: button
title: Button
---

# Button

A tappable button with optional haptic feedback.

## Basic Usage

```go
// Themed (recommended)
theme.ButtonOf(ctx, "Submit", handleSubmit)

// Explicit
widgets.Button{
    Label:        "Submit",
    OnTap:        handleSubmit,
    Color:        colors.Primary,
    TextColor:    colors.OnPrimary,
    Padding:      layout.EdgeInsetsSymmetric(24, 14),
    BorderRadius: 8,
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Label` | `string` | Button text |
| `OnTap` | `func()` | Tap callback |
| `Color` | `color.Color` | Background color |
| `TextColor` | `color.Color` | Label color |
| `Padding` | `layout.EdgeInsets` | Inner padding |
| `BorderRadius` | `float64` | Corner radius |
| `Haptic` | `bool` | Enable haptic feedback on tap |
| `Disabled` | `bool` | Disable the button |

## Themed vs Explicit

```go
// Themed: reads colors, padding, font size, border radius from ButtonThemeData
button := theme.ButtonOf(ctx, "Submit", handleSubmit)

// Override specific theme values with builder methods
button := theme.ButtonOf(ctx, "Submit", onSubmit).
    WithBorderRadius(0).
    WithPadding(layout.EdgeInsetsSymmetric(32, 16))
```

## Common Patterns

### Destructive Action

```go
widgets.Button{
    Label:        "Delete",
    OnTap:        handleDelete,
    Color:        colors.Error,
    TextColor:    colors.OnError,
    BorderRadius: 8,
    Haptic:       true,
}
```

### Button Row

```go
widgets.RowOf(
    widgets.MainAxisAlignmentSpaceEvenly,
    widgets.CrossAxisAlignmentCenter,
    widgets.MainAxisSizeMax,
    theme.ButtonOf(ctx, "Cancel", handleCancel),
    theme.ButtonOf(ctx, "Save", handleSave),
)
```

## Related

- [TextField](/docs/catalog/input/textfield) for text input
- [Forms & Validation](/docs/guides/forms) for form submission
