---
id: animation
title: Animation
sidebar_position: 5
---

# Animation

Drift provides a flexible animation system with controllers, curves, and animated widgets.

## Animated Widgets

The simplest way to animate is with implicit animation widgets that animate automatically when their properties change.

### AnimatedContainer

Animates size, color, padding, and decoration changes:

```go
type myState struct {
    core.StateBase
    expanded bool
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    colors := theme.ColorsOf(ctx)

    width := 100.0
    if s.expanded {
        width = 200.0
    }

    return widgets.GestureDetector{
        OnTap: func() {
            s.SetState(func() {
                s.expanded = !s.expanded
            })
        },
        Child: widgets.AnimatedContainer{
            Duration:    300 * time.Millisecond,
            Curve:       animation.EaseInOut,
            Width:       width,
            Height:      100,
            Color:       colors.Primary,
            Child: widgets.Text{Content: "Tap me"},
        },
    }
}
```

### AnimatedOpacity

Fades widgets in and out:

```go
widgets.AnimatedOpacity{
    Opacity:     isVisible ? 1.0 : 0.0,
    Duration:    200 * time.Millisecond,
    Child: content,
}
```

## Animation Controller

For more control, use `AnimationController` to drive animations explicitly.

### Basic Usage

```go
type myState struct {
    core.StateBase
    controller *animation.AnimationController
}

func (s *myState) InitState() {
    // Create controller with duration
    s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })

    // Subscribe to value changes
    core.UseListenable(&s.StateBase, s.controller)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    // controller.Value ranges from 0.0 to 1.0
    opacity := s.controller.Value

    return widgets.Opacity{
        Opacity:     opacity,
        Child: content,
    }
}
```

### Controlling Animation

```go
// Animate forward (0 -> 1)
s.controller.Forward()

// Animate in reverse (1 -> 0)
s.controller.Reverse()

// Animate to specific value
s.controller.AnimateTo(0.5)

// Reset without animation
s.controller.Reset()

// Check status
if s.controller.Status() == animation.AnimationCompleted {
    // Animation finished
}
```

### Animation Status

| Status | Description |
|--------|-------------|
| `AnimationDismissed` | At beginning (value = 0) |
| `AnimationForward` | Animating toward end |
| `AnimationReverse` | Animating toward beginning |
| `AnimationCompleted` | At end (value = 1) |

## Curves

Curves control the rate of change over time.

### Built-in Curves

```go
animation.LinearCurve  // Constant speed
animation.EaseIn       // Slow start, fast end
animation.EaseOut      // Fast start, slow end
animation.EaseInOut    // Slow start and end
```

### Using Curves

```go
// Set curve on controller
s.controller.Curve = animation.EaseInOut

// Or in AnimatedContainer
widgets.AnimatedContainer{
    Duration: 300 * time.Millisecond,
    Curve:    animation.EaseOut,
    // ...
}
```

### Custom Curves

Create a cubic bezier curve:

```go
customCurve := animation.CubicBezier(0.68, -0.55, 0.27, 1.55)
s.controller.Curve = customCurve
```

## Spring Animations

For physics-based animations that feel natural, use `SpringSimulation`:

```go
// Create a spring simulation from current position/velocity to target
// Parameters: spring description, start position, initial velocity, target position
spring := animation.NewSpringSimulation(
    animation.IOSSpring(),  // iOS-style spring (snappy)
    0,                      // start position
    0,                      // initial velocity
    100,                    // target position
)

// Or use a bouncy spring
spring := animation.NewSpringSimulation(
    animation.BouncySpring(),  // Playful bounce effect
    0, 0, 100,
)

// Step the simulation each frame
done := spring.Step(deltaTime)
currentPosition := spring.Position()
currentVelocity := spring.Velocity()
```

### Spring Descriptions

| Function | Behavior |
|----------|----------|
| `IOSSpring()` | Critically damped, snappy with minimal overshoot |
| `BouncySpring()` | Underdamped, playful bounce effect |

## Staggered Animations

Run multiple animations in sequence:

```go
type staggeredState struct {
    core.StateBase
    controller1 *animation.AnimationController
    controller2 *animation.AnimationController
    controller3 *animation.AnimationController
}

func (s *staggeredState) InitState() {
    s.controller1 = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(200 * time.Millisecond)
    })
    s.controller2 = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(200 * time.Millisecond)
    })
    s.controller3 = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(200 * time.Millisecond)
    })

    core.UseListenable(&s.StateBase, s.controller1)
    core.UseListenable(&s.StateBase, s.controller2)
    core.UseListenable(&s.StateBase, s.controller3)
}

func (s *staggeredState) startAnimation() {
    s.controller1.Forward()

    // Start second after first completes
    s.controller1.AddStatusListener(func(status animation.AnimationStatus) {
        if status == animation.AnimationCompleted {
            s.controller2.Forward()
        }
    })

    s.controller2.AddStatusListener(func(status animation.AnimationStatus) {
        if status == animation.AnimationCompleted {
            s.controller3.Forward()
        }
    })
}
```

## Ticker

For frame-by-frame updates, use a `Ticker`:

```go
type gameState struct {
    core.StateBase
    ticker   *animation.Ticker
    position float64
}

func (s *gameState) InitState() {
    s.ticker = animation.NewTicker(func(elapsed time.Duration) {
        drift.Dispatch(func() {
            s.SetState(func() {
                s.position += 0.1
            })
        })
    })
    s.ticker.Start()
}

func (s *gameState) Dispose() {
    s.ticker.Stop()
}
```

## Common Patterns

### Fade In on Mount

```go
type fadeInState struct {
    core.StateBase
    controller *animation.AnimationController
}

func (s *fadeInState) InitState() {
    s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })
    core.UseListenable(&s.StateBase, s.controller)

    // Start animation immediately
    s.controller.Forward()
}

func (s *fadeInState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Opacity{
        Opacity:     s.controller.Value,
        Child: content,
    }
}
```

### Toggle Animation

```go
func (s *myState) toggle() {
    if s.controller.Status() == animation.AnimationCompleted {
        s.controller.Reverse()
    } else {
        s.controller.Forward()
    }
}
```

### Interpolating Values

Use the controller's value (0-1) to interpolate between any two values:

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    // Interpolate position
    startX := 0.0
    endX := 100.0
    currentX := startX + (endX-startX)*s.controller.Value

    // Interpolate color
    startColor := colors.Primary
    endColor := colors.Secondary
    // Use graphics.LerpColor for color interpolation

    return widgets.Container{
        // Use interpolated values
    }
}
```

:::tip
`UseController` and `UseListenable` are documented in the [State Management](/docs/guides/state-management#hooks) guide.
:::

## Next Steps

- [Lottie Animations](/docs/guides/lottie) - Play Lottie animations with Skia
- [Gestures](/docs/guides/gestures) - Handle touch input
- [Theming](/docs/guides/theming) - Style your app
- [API Reference](/docs/api/animation) - Animation API documentation
