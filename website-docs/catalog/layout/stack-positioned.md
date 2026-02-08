---
id: stack-positioned
title: Stack & Positioned
---

# Stack & Positioned

`Stack` overlays children on top of each other. `Positioned` places a child at absolute coordinates within a Stack.

## Basic Usage

```go
widgets.Stack{
    Alignment: layout.AlignmentCenter,
    Fit:       widgets.StackFitLoose,
    Children: []core.Widget{
        backgroundImage,
        gradientOverlay,
        widgets.Positioned{
            Bottom: widgets.Ptr(16),
            Left:   widgets.Ptr(16),
            Right:  widgets.Ptr(16),
            Child: titleText,
        },
    },
}
```

## Stack Properties

| Property | Type | Description |
|----------|------|-------------|
| `Alignment` | `layout.Alignment` | Default alignment for non-positioned children |
| `Fit` | `StackFit` | How non-positioned children are sized |
| `Children` | `[]core.Widget` | Child widgets (can include `Positioned` children) |

## Positioned

Use `Positioned` within a Stack for absolute positioning:

```go
widgets.Stack{
    Children: []core.Widget{
        mainContent,
        // Badge in top-right corner
        widgets.Positioned{
            Top:   widgets.Ptr(8),
            Right: widgets.Ptr(8),
            Child: badge,
        },
    },
}
```

### Positioned Properties

| Property | Type | Description |
|----------|------|-------------|
| `Left` | `*float64` | Distance from the left edge |
| `Top` | `*float64` | Distance from the top edge |
| `Right` | `*float64` | Distance from the right edge |
| `Bottom` | `*float64` | Distance from the bottom edge |
| `Alignment` | `*graphics.Alignment` | Relative positioning via alignment coordinates |
| `Child` | `core.Widget` | Child widget |

## Positioned with Alignment

`Positioned` also supports relative positioning via `Alignment`. When set, the child is centered on the alignment point within the Stack bounds.

The `Alignment` type uses coordinates from -1 to 1, where (-1, -1) is top-left, (0, 0) is center, and (1, 1) is bottom-right. Use the named constants like `graphics.AlignCenter`, `graphics.AlignBottomRight`, etc.

When `Alignment` is set, `Left`/`Top`/`Right`/`Bottom` become pixel offsets from that centered position:
- `Left`/`Top` shift the child in the positive direction (right/down)
- `Right`/`Bottom` shift the child in the negative direction (left/up, i.e., inward from edges)

```go
widgets.Stack{
    Children: []core.Widget{
        background,
        // Centered dialog (no offsets needed)
        widgets.Positioned{
            Alignment: &graphics.AlignCenter,
            Child: dialog,
        },
        // Floating action button: starts at bottom-right corner,
        // then shifts 16px left and 16px up (inward from corner)
        widgets.Positioned{
            Alignment: &graphics.AlignBottomRight,
            Right:     widgets.Ptr(16),
            Bottom:    widgets.Ptr(16),
            Child: fab,
        },
    },
}
```

## Stack Fit

| Fit | Effect |
|-----|--------|
| `StackFitLoose` | Children can be smaller than stack |
| `StackFitExpand` | Non-positioned children expand to fill |

## Related

- [Center & Align](/docs/catalog/layout/center-align) for simpler alignment without overlapping
- [Layout System](/docs/guides/layout) for how constraints flow through the tree
