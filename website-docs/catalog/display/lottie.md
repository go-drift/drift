---
id: lottie
title: Lottie
---

# Lottie

Render Lottie animations with automatic or programmatic playback control.

```go
anim, _ := lottie.Load(f)

widgets.Lottie{
    Source: anim,
    Width:  200,
    Height: 200,
    Repeat: widgets.LottieLoop,
}
```

## Lottie Properties

| Property | Type | Description |
|----------|------|-------------|
| `Source` | `*lottie.Animation` | Pre-loaded Lottie animation. If nil, renders nothing. |
| `Width` | `float64` | Display width. If zero and Height is set, derived from aspect ratio. |
| `Height` | `float64` | Display height. If zero and Width is set, derived from aspect ratio. |
| `Repeat` | `LottieRepeat` | Repeat mode. Ignored when Controller is set. |
| `Controller` | `*animation.AnimationController` | External controller for programmatic playback. Disables auto-play. |
| `OnComplete` | `func()` | Called when play-once finishes. Ignored with Controller or looping modes. |

## LottieRepeat Values

| Value | Description |
|-------|-------------|
| `LottiePlayOnce` | Play once and stop at the last frame (default) |
| `LottieLoop` | Replay from the beginning continuously |
| `LottieBounce` | Play forward then backward continuously (ping-pong) |

## Loading Functions

| Function | Description |
|----------|-------------|
| `lottie.Load(r io.Reader)` | Parse from any reader (asset file, HTTP body) |
| `lottie.LoadBytes(data []byte)` | Parse from raw bytes |
| `lottie.LoadFile(path string)` | Parse from a file path |

## Related

- [Image & SVG](/docs/catalog/display/image-svg) for raster and vector images
- [Animation](/docs/guides/animation) for the animation controller system
