---
id: gestures
title: Gestures
sidebar_position: 3
---

# Gestures

Drift provides gesture detection for handling touch input including taps, drags, and long presses.

## Tap Gesture

Use the `Tap` helper for simple tap gestures:

```go
widgets.Tap(func() {
    fmt.Println("Tapped!")
}, myButton)
```

For more control, use `GestureDetector` directly:

```go
widgets.GestureDetector{
    OnTap: func() {
        fmt.Println("Tapped!")
    },
    Child: myButton,
}
```

## Pan Gesture (Omnidirectional Drag)

Use the `Drag` helper for simple pan gestures:

```go
widgets.Drag(func(d widgets.DragUpdateDetails) {
    s.SetState(func() {
        s.x += d.Delta.X
        s.y += d.Delta.Y
    })
}, draggableBox)
```

For more control (OnStart, OnEnd, OnCancel), use `GestureDetector`:

```go
widgets.GestureDetector{
    OnPanStart: func(d widgets.DragStartDetails) {
        // Drag started at d.Position
        s.SetState(func() {
            s.isDragging = true
        })
    },
    OnPanUpdate: func(d widgets.DragUpdateDetails) {
        // d.Delta contains movement since last update
        s.SetState(func() {
            s.x += d.Delta.X
            s.y += d.Delta.Y
        })
    },
    OnPanEnd: func(d widgets.DragEndDetails) {
        // d.Velocity contains fling velocity
        s.SetState(func() {
            s.isDragging = false
        })
        if d.Velocity.X > threshold {
            flingRight()
        }
    },
    OnPanCancel: func() {
        s.SetState(func() {
            s.isDragging = false
        })
    },
    Child: draggableBox,
}
```

## Axis-Locked Drags

For gestures that should only respond to one axis, use axis-specific callbacks.

### Horizontal Drag

Use the `HorizontalDrag` helper:

```go
widgets.HorizontalDrag(func(d widgets.DragUpdateDetails) {
    s.SetState(func() {
        s.sliderValue += d.PrimaryDelta
    })
}, slider)
```

Or use `GestureDetector` for full control:

```go
widgets.GestureDetector{
    OnHorizontalDragStart: func(d widgets.DragStartDetails) {
        // Horizontal drag started
    },
    OnHorizontalDragUpdate: func(d widgets.DragUpdateDetails) {
        // d.PrimaryDelta is the X movement
        s.SetState(func() {
            s.offset += d.PrimaryDelta
        })
    },
    OnHorizontalDragEnd: func(d widgets.DragEndDetails) {
        // d.PrimaryVelocity is the X velocity
        if d.PrimaryVelocity > swipeThreshold {
            dismissCard()
        }
    },
    OnHorizontalDragCancel: func() {},
    Child: swipeableCard,
}
```

### Vertical Drag

Use the `VerticalDrag` helper:

```go
widgets.VerticalDrag(func(d widgets.DragUpdateDetails) {
    s.SetState(func() {
        s.pullOffset += d.PrimaryDelta
    })
}, pullToRefresh)
```

Or use `GestureDetector`:

```go
widgets.GestureDetector{
    OnVerticalDragStart: func(d widgets.DragStartDetails) {
        // Vertical drag started
    },
    OnVerticalDragUpdate: func(d widgets.DragUpdateDetails) {
        // d.PrimaryDelta is the Y movement
        s.SetState(func() {
            s.sheetHeight -= d.PrimaryDelta
        })
    },
    OnVerticalDragEnd: func(d widgets.DragEndDetails) {
        // Snap to positions based on velocity
        if d.PrimaryVelocity > 0 {
            collapseSheet()
        } else {
            expandSheet()
        }
    },
    OnVerticalDragCancel: func() {},
    Child: bottomSheet,
}
```

## Gesture Competition

When multiple gesture recognizers compete for the same pointer:

- **Axis-locked drags** win when the primary axis movement exceeds slop and is greater than or equal to the orthogonal movement
- **Tap** loses if movement exceeds the touch slop
- **Pan** wins when total movement exceeds the touch slop
- **Long press** wins when held long enough without movement

This enables patterns like swipe-to-dismiss cards inside a vertical ScrollView:

```go
// Vertical ScrollView with horizontally-swipeable cards
widgets.ScrollView{
    ScrollDirection: widgets.AxisVertical,
    Child: widgets.Column{
        Children: []core.Widget{
            // This card responds to horizontal swipes
            // while the parent ScrollView responds to vertical swipes
            widgets.GestureDetector{
                OnHorizontalDragUpdate: func(d widgets.DragUpdateDetails) {
                    s.SetState(func() {
                        s.cardOffset += d.PrimaryDelta
                    })
                },
                Child: swipeCard,
            },
        },
    },
}
```

## Drag Details

The drag callbacks receive detail structs:

### DragStartDetails

| Field | Type | Description |
|-------|------|-------------|
| `Position` | `graphics.Offset` | Global position where the drag started |

### DragUpdateDetails

| Field | Type | Description |
|-------|------|-------------|
| `Position` | `graphics.Offset` | Current global position |
| `Delta` | `graphics.Offset` | Movement since last update |
| `PrimaryDelta` | `float64` | Axis-specific delta (only for axis-locked drags) |

### DragEndDetails

| Field | Type | Description |
|-------|------|-------------|
| `Position` | `graphics.Offset` | Final global position |
| `Velocity` | `graphics.Offset` | Fling velocity in pixels/second |
| `PrimaryVelocity` | `float64` | Axis-specific velocity (only for axis-locked drags) |

Note: `PrimaryDelta` and `PrimaryVelocity` are only meaningful for axis-locked recognizers.

## Clamp Helper

The `Clamp` helper constrains a value between min and max bounds:

```go
widgets.Drag(func(d widgets.DragUpdateDetails) {
    s.SetState(func() {
        s.x = widgets.Clamp(s.x+d.Delta.X, 0, s.maxX)
        s.y = widgets.Clamp(s.y+d.Delta.Y, 0, s.maxY)
    })
}, draggableBox)
```

## Common Patterns

### Swipe to Dismiss

```go
type swipeState struct {
    core.StateBase
    offset float64
}

func (s *swipeState) Build(ctx core.BuildContext) core.Widget {
    return widgets.GestureDetector{
        OnHorizontalDragUpdate: func(d widgets.DragUpdateDetails) {
            s.SetState(func() {
                s.offset += d.PrimaryDelta
            })
        },
        OnHorizontalDragEnd: func(d widgets.DragEndDetails) {
            if math.Abs(s.offset) > dismissThreshold || math.Abs(d.PrimaryVelocity) > velocityThreshold {
                onDismiss()
            } else {
                // Snap back
                s.SetState(func() {
                    s.offset = 0
                })
            }
        },
        Child: widgets.Stack{
            Children: []core.Widget{
                widgets.Positioned(card).Left(s.offset).Top(0),
            },
        },
    }
}
```

### Pull to Refresh

```go
type refreshState struct {
    core.StateBase
    pullDistance float64
    isRefreshing bool
}

func (s *refreshState) Build(ctx core.BuildContext) core.Widget {
    return widgets.GestureDetector{
        OnVerticalDragUpdate: func(d widgets.DragUpdateDetails) {
            if d.PrimaryDelta > 0 && !s.isRefreshing {
                s.SetState(func() {
                    s.pullDistance += d.PrimaryDelta
                })
            }
        },
        OnVerticalDragEnd: func(d widgets.DragEndDetails) {
            if s.pullDistance > refreshThreshold {
                s.SetState(func() {
                    s.isRefreshing = true
                })
                go s.doRefresh()
            } else {
                s.SetState(func() {
                    s.pullDistance = 0
                })
            }
        },
        Child: content,
    }
}
```

### Draggable Position

```go
type draggableState struct {
    core.StateBase
    x, y float64
}

func (s *draggableState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Stack{
        Children: []core.Widget{
            widgets.Positioned(widgets.GestureDetector{
                OnPanUpdate: func(d widgets.DragUpdateDetails) {
                    s.SetState(func() {
                        s.x += d.Delta.X
                        s.y += d.Delta.Y
                    })
                },
                Child: draggableHandle,
            }).Left(s.x).Top(s.y),
        },
    }
}
```

## Text Input

For text input, use a controller pattern:

```go
type formState struct {
    core.StateBase
    controller *platform.TextEditingController
}

func (s *formState) InitState() {
    s.controller = platform.NewTextEditingController("")
}

func (s *formState) Build(ctx core.BuildContext) core.Widget {
    return widgets.TextInput{
        Controller:   s.controller,
        Placeholder:  "Enter text",
        KeyboardType: platform.KeyboardTypeText,
        OnSubmitted:  s.handleSubmit,
    }
}

func (s *formState) handleSubmit(text string) {
    value := s.controller.Text()  // Read current value
    s.controller.Clear()          // Clear programmatically
}
```

### Keyboard Types

| Type | Use |
|------|-----|
| `KeyboardTypeText` | General text input |
| `KeyboardTypeNumber` | Numeric input |
| `KeyboardTypeEmail` | Email address |
| `KeyboardTypePhone` | Phone number |
| `KeyboardTypeURL` | URL input |

## Haptic Feedback

Add tactile feedback to gestures:

```go
widgets.GestureDetector{
    OnTap: func() {
        platform.Haptics.LightImpact()
        handleTap()
    },
    Child: button,
}
```

| Method | Use |
|--------|-----|
| `LightImpact()` | Subtle feedback (selections) |
| `MediumImpact()` | Standard feedback (toggles) |
| `HeavyImpact()` | Strong feedback (errors) |
| `SelectionClick()` | Selection change |

## Next Steps

- [Accessibility](/docs/guides/accessibility) - Make your app accessible
- [Platform Services](/docs/guides/platform) - Clipboard, haptics, and more
- [API Reference](/docs/api/gestures) - Gestures API documentation
