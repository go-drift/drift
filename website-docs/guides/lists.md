---
id: lists
title: Lists & Scrolling
sidebar_position: 3
---

# Lists & Scrolling

Drift provides several widgets for displaying scrollable content, from simple scroll containers to virtualized lists that efficiently handle thousands of items.

## Basic ListView

For small lists with all items in memory:

```go
widgets.ListView{
    ChildrenWidgets: []core.Widget{
        item1,
        item2,
        item3,
    },
    Padding: layout.EdgeInsetsAll(16),
}
```

## Virtualized Lists

For large lists, use `ListViewBuilder` with `ItemExtent` to only build visible items:

```go
widgets.ListViewBuilder{
    ItemCount:   1000,
    ItemExtent:  60,  // Required for virtualization
    CacheExtent: 100, // Extra pixels to render beyond viewport
    ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
        item := items[index]
        return widgets.Container{
            Padding:     layout.EdgeInsetsAll(16),
            ChildWidget: widgets.Text{Content: item.Title},
        },
    },
}
```

### ItemExtent is Required for Virtualization

`ItemExtent` (fixed item height) enables virtualization. Without it, all items are built upfront:

```go
// Virtualized: only visible items are built
widgets.ListViewBuilder{
    ItemCount:   1000,
    ItemExtent:  60,  // All items are 60 pixels tall
    ItemBuilder: buildItem,
}

// NOT virtualized: all items built upfront
widgets.ListViewBuilder{
    ItemCount:   1000,
    // ItemExtent omitted - no virtualization
    ItemBuilder: buildItem,
}
```

### When to Use ListViewBuilder

| Scenario | Recommendation |
|----------|----------------|
| < 50 items | `ListView` is fine |
| 50+ fixed-height items | `ListViewBuilder` with `ItemExtent` |
| Variable-height items | `ListView` or accept no virtualization |

## ScrollView

For scrollable content that isn't a list:

```go
widgets.ScrollView{
    ChildWidget: widgets.Column{
        ChildrenWidgets: []core.Widget{
            header,
            content,
            footer,
        },
    },
}
```

## Scroll Physics

Control scroll behavior:

```go
widgets.ScrollView{
    Physics: widgets.BouncingScrollPhysics{}, // iOS-style bounce
    // or
    Physics: widgets.ClampingScrollPhysics{}, // Android-style clamp
    ChildWidget: content,
}
```

| Physics | Behavior |
|---------|----------|
| `BouncingScrollPhysics` | Bounces when reaching edges (iOS style) |
| `ClampingScrollPhysics` | Clamps at edges (Android style, default) |

## Scroll Direction

```go
widgets.ListView{
    ScrollDirection: widgets.AxisHorizontal, // Defaults to vertical
    ChildrenWidgets: items,
}
```

## Next Steps

- [Layout](/docs/guides/layout) - Arranging widgets with Flex, Stack, and containers
- [Widgets](/docs/guides/widgets) - Available widget types
- [State Management](/docs/guides/state-management) - Managing list data
