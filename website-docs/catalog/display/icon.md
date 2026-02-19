---
id: icon
title: Icon
---

# Icon

Displays a Material Design icon.

## Basic Usage

```go
widgets.Icon{
    Icon:  icons.Favorite,
    Size:  24,
    Color: colors.Primary,
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Icon` | `icons.IconData` | The icon to display |
| `Size` | `float64` | Icon size in pixels |
| `Color` | `color.Color` | Icon color |

## Common Patterns

### Icon in a Row

```go
widgets.Row{
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    MainAxisSize:       widgets.MainAxisSizeMin,
    Children: []core.Widget{
        widgets.Icon{Icon: icons.Email, Size: 20, Color: colors.OnSurfaceVariant},
        widgets.HSpace(8),
        widgets.Text{Content: "user@example.com", Style: textTheme.BodyMedium},
    },
}
```

## Related

- [Image & SVG](/docs/catalog/display/image-svg) for custom SVG icons and images
- [Button](/docs/catalog/input/button) for tappable icon buttons
