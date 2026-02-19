---
id: center-align
title: Center & Align
---

# Center & Align

`Center` centers a child within available space. `Align` positions a child at any alignment point.

## Center

```go
widgets.Center{
    Child: widgets.Text{Content: "Centered"},
}

// Helper function
widgets.Centered(widgets.Text{Content: "Centered"})
```

## Align

Position a child within available space using any alignment:

```go
widgets.Align{
    Alignment: layout.AlignmentBottomRight,
    Child:     widgets.Text{Content: "Bottom right"},
}
```

Align expands to fill available space, then positions the child. Center is equivalent to `Align{Alignment: layout.AlignmentCenter}`.

## Properties

### Center

| Property | Type | Description |
|----------|------|-------------|
| `Child` | `core.Widget` | Child widget to center |

### Align

| Property | Type | Description |
|----------|------|-------------|
| `Alignment` | `layout.Alignment` | Where to position the child |
| `Child` | `core.Widget` | Child widget |

## Common Patterns

### Centering Content on Screen

```go
widgets.SafeArea{
    Child: widgets.Center{
        Child: widgets.Column{
            MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
            CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
            MainAxisSize:       widgets.MainAxisSizeMin,
            Children: []core.Widget{
                logo,
                widgets.VSpace(16),
                title,
            },
        },
    },
}
```

## Related

- [Stack & Positioned](/docs/catalog/layout/stack-positioned) for overlapping aligned children
- [Container & DecoratedBox](/docs/catalog/layout/container-decoratedbox) for alignment with decoration
