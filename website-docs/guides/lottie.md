---
id: lottie
title: Lottie Animations
sidebar_position: 6
---

# Lottie Animations

Drift can play [Lottie](https://airbnb.io/lottie/) animations natively using Skia's Skottie renderer. Lottie files are JSON-based motion graphics exported from After Effects that render at any resolution without quality loss.

## Loading Animations

Load a Lottie JSON file before passing it to a widget. Three loaders are available:

```go
import (
    "github.com/go-drift/drift/pkg/lottie"
)

// From an io.Reader (asset filesystem, HTTP body, etc.)
f, err := assetFS.Open("assets/bouncing-ball.json")
if err != nil {
    return nil
}
defer f.Close()
anim, err := lottie.Load(f)

// From raw bytes
anim, err := lottie.LoadBytes(jsonBytes)

// From a file path
anim, err := lottie.LoadFile("/path/to/animation.json")
```

All three return a `*lottie.Animation` that holds the parsed animation data and exposes `Duration()` and `Size()` for intrinsic dimensions.

## Basic Playback

The `Lottie` widget plays automatically when mounted. Use the `Repeat` field to control what happens when playback reaches the end:

```go
// Play once and stop at the last frame
widgets.Lottie{
    Source: anim,
    Width:  200,
    Height: 200,
}

// Loop continuously
widgets.Lottie{
    Source: anim,
    Width:  200,
    Height: 200,
    Repeat: widgets.LottieLoop,
}

// Bounce (play forward, then reverse, continuously)
widgets.Lottie{
    Source: anim,
    Width:  200,
    Height: 200,
    Repeat: widgets.LottieBounce,
}
```

## Completion Callback

In play-once mode (the default), use `OnComplete` to run code when the animation finishes:

```go
widgets.Lottie{
    Source: anim,
    Width:  200,
    Height: 200,
    OnComplete: func() {
        s.SetState(func() {
            s.showResult = true
        })
    },
}
```

`OnComplete` is only called in `LottiePlayOnce` mode and is ignored when a `Controller` is set.

## Sizing

The widget supports three sizing strategies:

**Explicit dimensions**: both Width and Height are set.

```go
widgets.Lottie{Source: anim, Width: 300, Height: 200}
```

**One dimension with aspect ratio preserved**: set one dimension and leave the other at zero. The missing dimension is calculated from the animation's intrinsic aspect ratio.

```go
widgets.Lottie{Source: anim, Width: 300} // Height derived from aspect ratio
```

**Intrinsic size**: leave both at zero. The widget uses the animation's native dimensions.

```go
widgets.Lottie{Source: anim, Repeat: widgets.LottieLoop}
```

## Programmatic Control

For play/pause/restart behavior, pass an external `AnimationController`. When a controller is provided, the widget does not auto-play and ignores `Repeat` and `OnComplete`. The controller's `Value` (0.0 to 1.0) maps directly to animation progress.

```go
type playerState struct {
    core.StateBase
    controller *animation.AnimationController
    anim       *lottie.Animation
}

func (s *playerState) InitState() {
    // Load animation (in practice, handle errors)
    f, _ := assetFS.Open("assets/bouncing-ball.json")
    defer f.Close()
    s.anim, _ = lottie.Load(f)

    // Create a controller matching the animation duration
    s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(s.anim.Duration())
    })

    // Rebuild on every frame
    core.UseListenable(&s.StateBase, s.controller)

    // React to completion
    s.controller.AddStatusListener(func(status animation.AnimationStatus) {
        if status == animation.AnimationCompleted {
            // Animation reached the end
        }
    })
}

func (s *playerState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Lottie{
                Source:     s.anim,
                Controller: s.controller,
                Width:      200,
                Height:     200,
            },
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Play", func() {
                        s.controller.Forward()
                    }),
                    theme.ButtonOf(ctx, "Pause", func() {
                        s.controller.Stop()
                    }),
                    theme.ButtonOf(ctx, "Restart", func() {
                        s.controller.Reset()
                        s.controller.Forward()
                    }),
                },
            },
        },
    }
}
```

Lottie animations contain their own easing curves baked into keyframes, so the controller uses linear interpolation by default. There is no need to set a curve.

:::tip
`UseController` is documented in the [State Management](/docs/guides/state-management#usecontroller) guide. For animation curves and the controller API, see [Animation](/docs/guides/animation).
:::

## Next Steps

- [Animation](/docs/guides/animation) for the animation controller and curves system
- [Image & SVG](/docs/catalog/display/image-svg) for raster and vector image widgets
