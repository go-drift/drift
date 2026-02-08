---
id: expanded-flexible
title: Expanded & Flexible
---

# Expanded & Flexible

`Expanded` fills remaining space in a Row or Column. `Flexible` allows a child to participate in flex space distribution without requiring it to fill all allocated space.

## Expanded

Fill remaining space in a flex container:

```go
widgets.Row{
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    Children: []core.Widget{
        avatar,
        widgets.HSpace(12),
        widgets.Expanded{Child: nameAndStatus},  // Takes remaining width
        menuButton,
    },
}
```

### Expanded with Flex

Control how space is distributed:

```go
widgets.Row{
    Children: []core.Widget{
        widgets.Expanded{Flex: 2, Child: leftPanel},  // 2/3 of space
        widgets.Expanded{Flex: 1, Child: rightPanel}, // 1/3 of space
    },
}
```

## Flexible

`Flexible` allows a child to participate in flex space distribution without requiring it to fill all allocated space. Useful when you want proportional space allocation but the child may not need all of it.

```go
widgets.Row{
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    Children: []core.Widget{
        widgets.Flexible{Child: widgets.Text{Content: "Short"}},  // Uses only needed width
        widgets.Expanded{Child: panel},                           // Fills remaining space
    },
}
```

### With Flex Factors

Distribute space proportionally while allowing children to be smaller than allocated:

```go
widgets.Row{
    Children: []core.Widget{
        widgets.Flexible{Flex: 1, Child: labelA},  // Gets up to 1/3 of space
        widgets.Flexible{Flex: 2, Child: labelB},  // Gets up to 2/3 of space
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Flex` | `int` | Flex factor for space distribution (default 1) |
| `Fit` | `FlexFit` | How the child fills allocated space |
| `Child` | `core.Widget` | Child widget |

## Expanded vs Flexible

| Widget | Default Fit | Constraints | Use Case |
|--------|-------------|-------------|----------|
| `Expanded` | Tight | Min = Max = allocated | Child must fill space (panels, containers) |
| `Flexible` | Loose | Min = 0, Max = allocated | Child can be smaller (text, icons) |

## FlexFit Options

| Fit | Behavior |
|-----|----------|
| `FlexFitLoose` (default for Flexible) | Child can be smaller than allocated space |
| `FlexFitTight` (default for Expanded) | Child must fill allocated space |

```go
// Equivalent to Expanded
widgets.Flexible{
    Flex:  1,
    Fit:   widgets.FlexFitTight,
    Child: content,
}
```

## When to Use Flexible

- Text labels that may vary in length
- Icons or badges with fixed intrinsic size
- Any widget where you want proportional space allocation but the widget should not stretch

## Related

- [Column & Row](/docs/catalog/layout/column-row) for the flex containers these work within
- [SizedBox](/docs/catalog/layout/sizedbox) for fixed dimensions
