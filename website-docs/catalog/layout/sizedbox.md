---
id: sizedbox
title: SizedBox
---

# SizedBox

Gives a widget fixed dimensions, or acts as a spacer when used without a child.

## Basic Usage

```go
// Fixed size
widgets.SizedBox{
    Width:  100,
    Height: 50,
    Child:  content,
}

// Width only
widgets.SizedBox{
    Width: 200,
    Child: content,
}

// Fixed-size spacer (no child)
widgets.SizedBox{Height: 16}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Width` | `float64` | Fixed width in pixels (0 means unconstrained) |
| `Height` | `float64` | Fixed height in pixels (0 means unconstrained) |
| `Child` | `core.Widget` | Optional child widget |

## Common Patterns

### Fixed-Size Spacer

```go
widgets.Column{
    MainAxisSize: widgets.MainAxisSizeMin,
    Children: []core.Widget{
        header,
        widgets.SizedBox{Height: 16},
        body,
    },
}
```

The `VSpace` and `HSpace` helpers are shorthand for this pattern:

```go
widgets.VSpace(16)  // equivalent to SizedBox{Height: 16}
widgets.HSpace(8)   // equivalent to SizedBox{Width: 8}
```

To fill *remaining* space in a Row or Column, use `Spacer()` instead:

```go
widgets.Row{
    Children: []core.Widget{
        label,
        widgets.Spacer(),  // fills remaining horizontal space
        button,
    },
}
```

See [Expanded & Flexible](/docs/catalog/layout/expanded-flexible) for more on flexible space distribution.

### Constraining Child Size

```go
// Limit an image to 200x200
widgets.SizedBox{
    Width:  200,
    Height: 200,
    Child:  profileImage,
}
```

## Related

- [Container & DecoratedBox](/docs/catalog/layout/container-decoratedbox) for sizing with decoration
- [Expanded & Flexible](/docs/catalog/layout/expanded-flexible) for proportional sizing in flex containers
