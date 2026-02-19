---
id: safearea
title: SafeArea
---

# SafeArea

Avoids system UI such as notches, status bars, and navigation bars by adding padding to keep content within the safe region.

## Basic Usage

```go
widgets.SafeArea{
    Child: content,
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Top` | `bool` | Avoid top system UI (default true) |
| `Bottom` | `bool` | Avoid bottom system UI (default true) |
| `Left` | `bool` | Avoid left system UI (default true) |
| `Right` | `bool` | Avoid right system UI (default true) |
| `Child` | `core.Widget` | Child widget |

## Selective Sides

Disable safe area padding on specific sides:

```go
widgets.SafeArea{
    Top:    true,
    Bottom: true,
    Left:   false,
    Right:  false,
    Child:  content,
}
```

## Common Patterns

### Full-Screen Layout with Safe Content

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.SafeArea{
        Child: widgets.PaddingAll(20,
            widgets.Column{
                MainAxisSize: widgets.MainAxisSizeMin,
                Children: []core.Widget{
                    header,
                    widgets.VSpace(16),
                    content,
                },
            },
        ),
    }
}
```

## Related

- [Padding](/docs/catalog/layout/padding) for adding custom spacing
- [Layout System](/docs/guides/layout) for how constraints flow through the tree
