---
id: progress-indicators
title: Progress Indicators
---

# Progress Indicators

Show loading or progress state with native and Drift-rendered indicators.

## ActivityIndicator

Native platform spinner (UIActivityIndicatorView on iOS, ProgressBar on Android).

```go
widgets.ActivityIndicator{
    Animating: true,
    Size:      widgets.ActivityIndicatorSizeMedium, // Small, Medium, Large
    Color:     colors.Primary, // Optional
}
```

### ActivityIndicator Properties

| Property | Type | Description |
|----------|------|-------------|
| `Animating` | `bool` | Whether the spinner is active |
| `Size` | `ActivityIndicatorSize` | `Small`, `Medium`, or `Large` |
| `Color` | `color.Color` | Optional tint color |

## CircularProgressIndicator

Drift-rendered circular progress. Set `Value` to `nil` for indeterminate animation.

```go
// Themed (recommended)
theme.CircularProgressIndicatorOf(ctx, nil)       // indeterminate
theme.CircularProgressIndicatorOf(ctx, &progress)  // determinate

// Explicit (full control)
widgets.CircularProgressIndicator{
    Value:       nil,
    Size:        36,
    Color:       colors.Primary,
    TrackColor:  colors.SurfaceVariant,
    StrokeWidth: 4,
}
```

### CircularProgressIndicator Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `*float64` | Progress 0.0 to 1.0, nil for indeterminate |
| `Size` | `float64` | Diameter in pixels |
| `Color` | `color.Color` | Progress arc color |
| `TrackColor` | `color.Color` | Background track color |
| `StrokeWidth` | `float64` | Arc thickness |

## LinearProgressIndicator

Drift-rendered linear progress bar. Set `Value` to `nil` for indeterminate animation.

```go
// Themed (recommended)
theme.LinearProgressIndicatorOf(ctx, nil)       // indeterminate
theme.LinearProgressIndicatorOf(ctx, &progress)  // determinate

// Explicit (full control)
widgets.LinearProgressIndicator{
    Value:        nil,
    Color:        colors.Primary,
    TrackColor:   colors.SurfaceVariant,
    Height:       4,
    BorderRadius: 2,
}
```

### LinearProgressIndicator Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `*float64` | Progress 0.0 to 1.0, nil for indeterminate |
| `Color` | `color.Color` | Progress bar color |
| `TrackColor` | `color.Color` | Background track color |
| `Height` | `float64` | Bar height |
| `BorderRadius` | `float64` | Corner radius |

## Related

- [Button](/docs/catalog/input/button) for triggering actions that show progress
- [Error Boundary](/docs/catalog/feedback/error-boundary) for error states
