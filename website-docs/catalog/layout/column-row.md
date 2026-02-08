---
id: column-row
title: Column & Row
---

# Column & Row

Flex containers that arrange children along a single axis. `Column` lays out children vertically, `Row` lays out children horizontally.

## Basic Usage

```go
// Vertical layout
widgets.Column{
    MainAxisAlignment:  widgets.MainAxisAlignmentStart,
    CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
    MainAxisSize:       widgets.MainAxisSizeMin,
    Children: []core.Widget{
        header,
        content,
        footer,
    },
}

// Horizontal layout
widgets.Row{
    MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    Children: []core.Widget{
        avatar,
        widgets.Expanded{Child: title},
        menuButton,
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `MainAxisAlignment` | `MainAxisAlignment` | How children are positioned along the main axis |
| `CrossAxisAlignment` | `CrossAxisAlignment` | How children are positioned along the cross axis |
| `MainAxisSize` | `MainAxisSize` | How much space the container takes along the main axis |
| `Children` | `[]core.Widget` | Child widgets |

## Main Axis Alignment

Controls how children are positioned along the main axis:

| Alignment | Effect |
|-----------|--------|
| `MainAxisAlignmentStart` | Pack at start |
| `MainAxisAlignmentEnd` | Pack at end |
| `MainAxisAlignmentCenter` | Center children |
| `MainAxisAlignmentSpaceBetween` | Equal space between, none at edges |
| `MainAxisAlignmentSpaceAround` | Equal space around each child |
| `MainAxisAlignmentSpaceEvenly` | Equal space everywhere |

```go
widgets.Row{
    MainAxisAlignment:  widgets.MainAxisAlignmentSpaceEvenly,
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    Children: []core.Widget{cancelButton, saveButton},
}
```

## Cross Axis Alignment

Controls how children are positioned along the cross axis:

| Alignment | Effect |
|-----------|--------|
| `CrossAxisAlignmentStart` | Align to start edge |
| `CrossAxisAlignmentEnd` | Align to end edge |
| `CrossAxisAlignmentCenter` | Center children |
| `CrossAxisAlignmentStretch` | Stretch to fill cross axis |

```go
widgets.Column{
    CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
    Children: []core.Widget{cardOne, cardTwo},
}
```

## Main Axis Size

Controls how much space the flex container takes:

| Size | Effect |
|------|--------|
| `MainAxisSizeMax` (default) | Take all available space |
| `MainAxisSizeMin` | Take minimum needed space |

`MainAxisSizeMax` is the zero value, so `Row{}` and `Column{}` expand to fill their parent by default. Set `MainAxisSizeMin` explicitly when you want shrink-wrap behavior.

```go
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,  // Only as tall as content
    items...,
)
```

## Helper Functions

Use `RowOf` and `ColumnOf` for concise layout:

```go
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    title,
    subtitle,
    description,
)
```

## Spacing

Use `VSpace` and `HSpace` for consistent gaps:

```go
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    header,
    widgets.VSpace(16),  // 16px vertical gap
    body,
    widgets.VSpace(24),
    footer,
)

widgets.RowOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentCenter,
    widgets.MainAxisSizeMin,
    icon,
    widgets.HSpace(8),   // 8px horizontal gap
    label,
)
```

## Related

- [Expanded & Flexible](/docs/catalog/layout/expanded-flexible) for distributing space among children
- [Wrap](/docs/catalog/layout/wrap) for flow layout that wraps to new lines
- [Layout System](/docs/guides/layout) for how constraints flow through the tree
