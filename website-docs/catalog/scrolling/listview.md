---
id: listview
title: ListView
---

# ListView

Scrollable list of widgets. For small lists, use `ListView` with all items in memory. For large lists, use `ListViewBuilder` with `ItemExtent` for virtualization.

## Basic ListView

```go
widgets.ListView{
    Children: []core.Widget{
        item1,
        item2,
        item3,
    },
    Padding: layout.EdgeInsetsAll(16),
}
```

## ListViewBuilder (Virtualized)

For large lists, `ListViewBuilder` with `ItemExtent` only builds visible items:

```go
widgets.ListViewBuilder{
    ItemCount:   1000,
    ItemExtent:  60,  // Required for virtualization
    CacheExtent: 100, // Extra pixels to render beyond viewport
    ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
        item := items[index]
        return widgets.Container{
            Padding: layout.EdgeInsetsAll(16),
            Child:   widgets.Text{Content: item.Title},
        }
    },
}
```

## Properties

### ListView

| Property | Type | Description |
|----------|------|-------------|
| `Children` | `[]core.Widget` | List items |
| `Padding` | `layout.EdgeInsets` | Padding around the list |
| `ScrollDirection` | `Axis` | `AxisVertical` (default) or `AxisHorizontal` |

### ListViewBuilder

| Property | Type | Description |
|----------|------|-------------|
| `ItemCount` | `int` | Total number of items |
| `ItemExtent` | `float64` | Fixed item height (enables virtualization) |
| `CacheExtent` | `float64` | Extra pixels to render beyond the viewport |
| `ItemBuilder` | `func(BuildContext, int) Widget` | Builds a widget for the given index |
| `Padding` | `layout.EdgeInsets` | Padding around the list |
| `ScrollDirection` | `Axis` | `AxisVertical` (default) or `AxisHorizontal` |

## ItemExtent is Required for Virtualization

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

## When to Use ListViewBuilder

| Scenario | Recommendation |
|----------|----------------|
| < 50 items | `ListView` is fine |
| 50+ fixed-height items | `ListViewBuilder` with `ItemExtent` |
| Variable-height items | `ListView` or accept no virtualization |

## Scroll Direction

```go
widgets.ListView{
    ScrollDirection: widgets.AxisHorizontal, // Defaults to vertical
    Children: items,
}
```

## Related

- [ScrollView](/docs/catalog/scrolling/scrollview) for scrollable non-list content
- [Column & Row](/docs/catalog/layout/column-row) for non-scrollable lists
