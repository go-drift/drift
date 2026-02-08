---
id: container-decoratedbox
title: Container & DecoratedBox
---

# Container & DecoratedBox

`Container` combines decoration, sizing, padding, and alignment. `DecoratedBox` provides decoration without any layout behavior.

## Basic Usage

```go
// Rounded card with padding (shrink-wraps to content)
widgets.Container{
    Color:        colors.Surface,
    BorderRadius: 12,
    Padding:      layout.EdgeInsetsAll(20),
    Child:  content,
}

// Bordered box
widgets.Container{
    BorderColor:  colors.Outline,
    BorderWidth:  1,
    BorderRadius: 8,
    Padding:      layout.EdgeInsetsAll(12),
    Child:  child,
}

// Fixed-size with centered child
widgets.Container{
    Color:       colors.Surface,
    Width:       200,
    Height:      100,
    Alignment:   layout.AlignmentCenter,  // Centers child within 200x100
    Child: child,
}
```

Alignment positions the child within the content area (after padding). When Container shrink-wraps to child+padding, alignment has no visible effect. When the container is larger than its content (via Width/Height or parent constraints), alignment controls child positioning.

## Container Properties

| Property | Type | Description |
|----------|------|-------------|
| `Color` | `color.Color` | Background color |
| `Width` | `float64` | Fixed width |
| `Height` | `float64` | Fixed height |
| `Padding` | `layout.EdgeInsets` | Inner padding |
| `Alignment` | `layout.Alignment` | Child alignment within the container |
| `BorderColor` | `color.Color` | Border color |
| `BorderWidth` | `float64` | Border width |
| `BorderRadius` | `float64` | Corner radius |
| `BorderDash` | `*graphics.DashPattern` | Dashed border pattern |
| `BorderGradient` | `graphics.Gradient` | Gradient applied to the border |
| `Gradient` | `graphics.Gradient` | Background gradient |
| `Shadow` | `*graphics.BoxShadow` | Drop shadow |
| `Overflow` | `Overflow` | Clipping behavior |
| `Child` | `core.Widget` | Child widget |

## DecoratedBox

Add visual styling without layout:

```go
widgets.DecoratedBox{
    Color:        colors.Surface,
    BorderRadius: 8,
    Gradient: graphics.NewLinearGradient(
        graphics.AlignTopCenter,    // start (top center)
        graphics.AlignBottomCenter, // end (bottom center)
        []graphics.GradientStop{
            {Position: 0, Color: color1},
            {Position: 1, Color: color2},
        },
    ),
    Shadow: &graphics.BoxShadow{
        Color:      shadowColor,
        BlurRadius: 8,
        Offset:     graphics.Offset{X: 0, Y: 2},
    },
    Child: content,
}
```

## Gradients

Gradients use relative coordinates via `Alignment` where (-1, -1) is top-left, (0, 0) is center, and (1, 1) is bottom-right. This allows gradients to scale with widget dimensions:

```go
// Horizontal gradient (left to right)
graphics.NewLinearGradient(
    graphics.AlignCenterLeft,
    graphics.AlignCenterRight,
    stops,
)

// Vertical gradient (top to bottom)
graphics.NewLinearGradient(
    graphics.AlignTopCenter,
    graphics.AlignBottomCenter,
    stops,
)

// Diagonal gradient
graphics.NewLinearGradient(
    graphics.AlignTopLeft,
    graphics.AlignBottomRight,
    stops,
)

// Radial gradient centered in widget
graphics.NewRadialGradient(
    graphics.AlignCenter,
    1.0, // radius = half the min dimension (touches nearest edge)
    stops,
)
```

## Gradient Borders

Apply gradients to borders using `BorderGradient`:

```go
widgets.DecoratedBox{
    BorderWidth:  3,
    BorderRadius: 12,
    BorderGradient: graphics.NewLinearGradient(
        graphics.AlignTopLeft,
        graphics.AlignBottomRight,
        []graphics.GradientStop{
            {Position: 0, Color: colors.Primary},
            {Position: 1, Color: colors.Secondary},
        },
    ),
    Child: content,
}

// Dashed gradient border
widgets.Container{
    BorderWidth:  2,
    BorderRadius: 8,
    BorderDash:   &graphics.DashPattern{Intervals: []float64{8, 4}},
    BorderGradient: graphics.NewLinearGradient(
        graphics.AlignCenterLeft,
        graphics.AlignCenterRight,
        stops,
    ),
    Padding:     layout.EdgeInsetsAll(16),
    Child: child,
}
```

When both `BorderColor` and `BorderGradient` are set, the gradient takes precedence.

## Container vs DecoratedBox

| Feature | Container | DecoratedBox |
|---------|-----------|--------------|
| **Purpose** | Layout + decoration | Decoration only |
| Padding | Yes | No |
| Width/Height | Yes | No |
| Alignment | Yes | No |
| Color/Gradient | Yes | Yes |
| Shadow | Yes | Yes |
| BorderRadius | Yes | Yes |
| BorderColor/Width | Yes | Yes |
| BorderGradient | Yes | Yes |
| BorderDash | Yes | Yes |

Container and DecoratedBox share the same painting implementation internally.

**Use Container** for most cases, as it handles layout and decoration together.

**Use DecoratedBox** when you want decoration without any layout behavior (child sizes to parent constraints).

## Overflow and Clipping

The `Overflow` field controls whether children are clipped to the container bounds:

```go
// Children clipped to bounds (default)
widgets.Container{
    BorderRadius: 12,
    Overflow:     widgets.OverflowClip,  // default
    Child: widgets.Column{
        Children: []core.Widget{
            // This gradient bar will have rounded top corners
            widgets.Container{
                Height:   4,
                Gradient: accentGradient,
            },
            content,
        },
    },
}
```

| Overflow | Gradient | Children |
|----------|----------|----------|
| `OverflowClip` (default) | Clipped to bounds | Clipped to bounds |
| `OverflowVisible` | Can extend beyond bounds | Not clipped |

With `OverflowClip`, children are clipped to the widget bounds. When `BorderRadius > 0`, content is clipped to the rounded shape. This is useful for cards with accent bars, images, or other content at the edges that should conform to the container's shape.

Shadows are always drawn behind the decoration and naturally overflow bounds regardless of the `Overflow` setting.

**Note:** Platform views (native text fields, switches, etc.) are clipped to rectangular bounds only, not rounded corners. This is a platform limitation.

## Related

- [Padding](/docs/catalog/layout/padding) for spacing without decoration
- [SizedBox](/docs/catalog/layout/sizedbox) for fixed dimensions without decoration
- [Layout System](/docs/guides/layout) for how constraints flow through the tree
