---
id: image-svg
title: Image & SVG
---

# Image & SVG

Display raster images from assets or files, and render SVG content with flexible sizing.

## Image

Display an image from the asset filesystem or a file path:

```go
widgets.Image{
    Source: "assets/photo.png",
    Width:  200,
    Height: 150,
}
```

### Image Properties

| Property | Type | Description |
|----------|------|-------------|
| `Source` | `string` | Asset path or file path |
| `Width` | `float64` | Display width |
| `Height` | `float64` | Display height |

## SvgImage

Renders an SVG with flexible sizing:

```go
widgets.SvgImage{
    Icon:   myIcon,
    Width:  120,
    Height: 80,
    Color:  colors.Primary,  // Optional tint
}
```

### SvgImage Properties

| Property | Type | Description |
|----------|------|-------------|
| `Icon` | `*svg.Icon` | Loaded SVG icon |
| `Width` | `float64` | Display width |
| `Height` | `float64` | Display height |
| `Color` | `color.Color` | Optional tint color |

## SvgIcon

A convenience wrapper around `SvgImage` for square icons:

```go
widgets.SvgIcon{
    Icon:  myIcon,
    Size:  24,
    Color: colors.OnSurface,
}
```

### SvgIcon Properties

| Property | Type | Description |
|----------|------|-------------|
| `Icon` | `*svg.Icon` | Loaded SVG icon |
| `Size` | `float64` | Width and height (square) |
| `Color` | `color.Color` | Optional tint color |

## Caching Static SVGs

For static SVG assets (logos, icons), cache loaded icons so rebuilds reuse the same underlying SVG DOM:

```go
var svgCache = svg.NewIconCache()

func loadIcon(name string) *svg.Icon {
    icon, err := svgCache.Get(name, func() (*svg.Icon, error) {
        f, err := assetFS.Open("assets/" + name)
        if err != nil {
            return nil, err
        }
        defer f.Close()
        return svg.Load(f)
    })
    if err != nil {
        return nil
    }
    return icon
}
```

## Related

- [Icon](/docs/catalog/display/icon) for built-in Material Design icons
- [Widget Architecture](/docs/guides/widgets) for how widgets work
