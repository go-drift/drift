---
id: scrollview
title: ScrollView
---

# ScrollView

A scrollable container for content that isn't a list. Wraps a single child and provides scroll behavior.

## Basic Usage

```go
widgets.ScrollView{
    Child: widgets.Column{
        Children: []core.Widget{
            header,
            content,
            footer,
        },
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Child` | `core.Widget` | Scrollable content |
| `Physics` | `ScrollPhysics` | Scroll behavior (bounce or clamp) |

## Scroll Physics

Control scroll behavior when reaching the edges:

```go
widgets.ScrollView{
    Physics: widgets.BouncingScrollPhysics{}, // iOS-style bounce
    Child:   content,
}

widgets.ScrollView{
    Physics: widgets.ClampingScrollPhysics{}, // Android-style clamp
    Child:   content,
}
```

| Physics | Behavior |
|---------|----------|
| `BouncingScrollPhysics` | Bounces when reaching edges (iOS style) |
| `ClampingScrollPhysics` | Clamps at edges (Android style, default) |

## Related

- [ListView](/docs/catalog/scrolling/listview) for scrollable lists of items
- [SafeArea](/docs/catalog/layout/safearea) for avoiding system UI in scrollable content
