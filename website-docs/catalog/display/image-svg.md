---
id: image-svg
title: Image & SVG
---

# Image & SVG

Display raster images from assets, files, or the network, and render SVG content with flexible sizing.

## Image

Display a decoded image:

```go
widgets.Image{
    Source:    myImage,  // image.Image
    Width:    200,
    Height:   150,
    Fit:      widgets.ImageFitCover,
}
```

### Image Properties

| Property | Type | Description |
|----------|------|-------------|
| `Source` | `image.Image` | Decoded image to render |
| `Width` | `float64` | Display width (zero uses intrinsic width) |
| `Height` | `float64` | Display height (zero uses intrinsic height) |
| `Fit` | `ImageFit` | Scaling mode: `Contain` (default), `Fill`, `Cover`, `None`, `ScaleDown` |
| `Alignment` | `layout.Alignment` | Position within bounds (default: center) |
| `SemanticLabel` | `string` | Accessibility description |
| `ExcludeFromSemantics` | `bool` | Hide from screen readers (for decorative images) |

## NetworkImage

Load and display an image from a URL with automatic caching and fade-in:

```go
widgets.NetworkImage{
    URL:    "https://example.com/avatar.jpg",
    Width:  200,
    Height: 200,
    Fit:    widgets.ImageFitCover,
}
```

While loading, a [CircularProgressIndicator](/docs/catalog/feedback/progress-indicators) is shown. On failure, a fallback error message is displayed. Both are customizable:

```go
widgets.NetworkImage{
    URL:         "https://example.com/photo.jpg",
    Width:       300,
    Height:      200,
    Placeholder: widgets.Center{Child: widgets.Text{Content: "Loading..."}},
    ErrorBuilder: func(err error) core.Widget {
        return widgets.Center{Child: widgets.Text{Content: "Failed to load"}}
    },
}
```

### Authenticated Requests

Pass headers for APIs that require authorization. Requests with different headers are cached independently:

```go
widgets.NetworkImage{
    URL:     "https://api.example.com/user/avatar",
    Width:   48,
    Height:  48,
    Fit:     widgets.ImageFitCover,
    Headers: map[string]string{"Authorization": "Bearer " + token},
}
```

### NetworkImage Properties

| Property | Type | Description |
|----------|------|-------------|
| `URL` | `string` | HTTP(S) image URL |
| `Headers` | `map[string]string` | Optional HTTP headers (e.g., authorization) |
| `Width` | `float64` | Display width (zero uses intrinsic width) |
| `Height` | `float64` | Display height (zero uses intrinsic height) |
| `Fit` | `ImageFit` | Scaling mode (default: `Cover`) |
| `Alignment` | `layout.Alignment` | Position within bounds |
| `Placeholder` | `core.Widget` | Widget shown while loading (default: spinner) |
| `ErrorBuilder` | `func(error) core.Widget` | Widget builder for load failures |
| `FadeDuration` | `*time.Duration` | Fade-in duration (default: 300ms, nil pointer for default, zero to disable) |
| `Loader` | `*image.Loader` | Custom loader (default: shared global loader) |
| `SemanticLabel` | `string` | Accessibility description |

### Caching

NetworkImage uses a two-tier cache by default:

- **Memory**: LRU cache (100 entries, 100 MB) for instant display of recently viewed images
- **Disk**: Filesystem cache (500 MB) under the platform cache directory for persistence across app restarts

Images are decoded from PNG, JPEG, GIF, and WebP formats. Concurrent requests for the same URL and headers are deduplicated into a single HTTP request.

To evict or clear the cache programmatically:

```go
loader := image.DefaultLoader()
loader.Evict("https://example.com/avatar.jpg")  // Remove one URL
loader.ClearCache()                              // Clear everything
```

## SvgImage

Renders an SVG with flexible sizing:

```go
widgets.SvgImage{
    Source:    myIcon,
    Width:     120,
    Height:    80,
    TintColor: colors.Primary,  // Optional tint
}
```

### SvgImage Properties

| Property | Type | Description |
|----------|------|-------------|
| `Source` | `*svg.Icon` | Loaded SVG icon |
| `Width` | `float64` | Display width |
| `Height` | `float64` | Display height |
| `TintColor` | `graphics.Color` | Optional tint color |

## SvgIcon

A convenience wrapper around `SvgImage` for square icons:

```go
widgets.SvgIcon{
    Source:    myIcon,
    Size:      24,
    TintColor: colors.OnSurface,
}
```

### SvgIcon Properties

| Property | Type | Description |
|----------|------|-------------|
| `Source` | `*svg.Icon` | Loaded SVG icon |
| `Size` | `float64` | Width and height (square) |
| `TintColor` | `graphics.Color` | Optional tint color |

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

- [Icon](/docs/catalog/display/icon) for rendering text glyphs as icons
- [Widget Architecture](/docs/guides/widgets) for how widgets work
