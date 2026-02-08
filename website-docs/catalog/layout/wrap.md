---
id: wrap
title: Wrap
---

# Wrap

Lays out children in runs, automatically wrapping to the next line when space runs out. Similar to CSS flexbox with `flex-wrap: wrap`.

## Basic Usage

```go
widgets.Wrap{
    Direction:  widgets.WrapAxisHorizontal,
    Spacing:    8,
    RunSpacing: 8,
    Children: []core.Widget{
        chip("Go"),
        chip("Rust"),
        chip("TypeScript"),
        chip("Python"),
        chip("JavaScript"),
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Direction` | `WrapAxis` | Flow direction (`WrapAxisHorizontal` or `WrapAxisVertical`) |
| `Spacing` | `float64` | Space between children within a run |
| `RunSpacing` | `float64` | Space between runs |
| `Alignment` | `WrapAlignment` | Main axis positioning within each run |
| `CrossAxisAlignment` | `WrapCrossAlignment` | Cross axis positioning within each run |
| `RunAlignment` | `RunAlignment` | Distribution of runs in cross axis |
| `Children` | `[]core.Widget` | Child widgets |

## Direction

Set `Direction` to control the flow direction:

- `WrapAxisHorizontal` (default): Children flow left-to-right, wrapping to new rows below
- `WrapAxisVertical`: Children flow top-to-bottom, wrapping to new columns to the right

```go
// Vertical wrap: items flow down, then wrap to the next column
widgets.Wrap{
    Direction:  widgets.WrapAxisVertical,
    Spacing:    8,
    RunSpacing: 12,
    Children: tags,
}
```

## Alignment

Wrap provides three alignment properties:

| Property | Purpose | Values |
|----------|---------|--------|
| `Alignment` | Main axis positioning within each run | Start, End, Center, SpaceBetween, SpaceAround, SpaceEvenly |
| `CrossAxisAlignment` | Cross axis positioning within each run | Start, End, Center |
| `RunAlignment` | Distribution of runs in cross axis | Start, End, Center, SpaceBetween, SpaceAround, SpaceEvenly |

```go
widgets.Wrap{
    Alignment:          widgets.WrapAlignmentCenter,
    CrossAxisAlignment: widgets.WrapCrossAlignmentCenter,
    RunAlignment:       widgets.RunAlignmentSpaceEvenly,
    Spacing:            8,
    RunSpacing:         8,
    Children:    chips,
}
```

## WrapOf Helper

Use `WrapOf` for concise creation with spacing:

```go
widgets.WrapOf(8, 12, // spacing, runSpacing
    chip("Tag 1"),
    chip("Tag 2"),
    chip("Tag 3"),
)
```

## When to Use Wrap vs Row/Column

| Use Case | Widget |
|----------|--------|
| Fixed number of items in a line | Row or Column |
| Items should wrap when they don't fit | Wrap |
| Need flexible children (Expanded) | Row or Column |
| Dynamic tags, chips, or badges | Wrap |

## Related

- [Column & Row](/docs/catalog/layout/column-row) for single-line flex layout
- [Layout System](/docs/guides/layout) for how constraints flow through the tree
