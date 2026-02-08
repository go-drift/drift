---
id: padding
title: Padding
---

# Padding

Adds spacing around a child widget using `EdgeInsets`.

## Basic Usage

```go
// All sides equal
widgets.PaddingAll(16, child)

// Symmetric (horizontal, vertical)
widgets.Padding{
    EdgeInsets: layout.EdgeInsetsSymmetric(16, 8),
    Child:     child,
}

// Individual sides
widgets.Padding{
    EdgeInsets: layout.EdgeInsets{
        Left:   16,
        Top:    8,
        Right:  16,
        Bottom: 8,
    },
    Child: child,
}

// Only specific sides
widgets.Padding{
    EdgeInsets: layout.EdgeInsetsOnly(16, 0, 16, 0), // left, top, right, bottom
    Child:     child,
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `EdgeInsets` | `layout.EdgeInsets` | Padding on each side |
| `Child` | `core.Widget` | Child widget |

## EdgeInsets Helpers

| Helper | Description |
|--------|-------------|
| `layout.EdgeInsetsAll(v)` | Equal padding on all sides |
| `layout.EdgeInsetsSymmetric(h, v)` | Horizontal and vertical padding |
| `layout.EdgeInsetsOnly(l, t, r, b)` | Specific per-side padding |
| `widgets.PaddingAll(v, child)` | Shorthand that wraps a child in equal padding |

## Related

- [Container & DecoratedBox](/docs/catalog/layout/container-decoratedbox) for padding combined with decoration
- [SizedBox](/docs/catalog/layout/sizedbox) for fixed-dimension spacing
