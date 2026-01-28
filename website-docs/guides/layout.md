---
id: layout
title: Layout
sidebar_position: 4
---

# Layout

Drift uses a constraint-based layout system. Parent widgets pass constraints to children, and children return their size.

## The Composition Pattern

Build complex layouts by nesting simple widgets:

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    _, colors, textTheme := theme.UseTheme(ctx)

    return widgets.SafeArea{
        ChildWidget: widgets.PaddingAll(20,
            widgets.ColumnOf(
                widgets.MainAxisAlignmentStart,
                widgets.CrossAxisAlignmentStart,
                widgets.MainAxisSizeMin,
                // Header
                widgets.TextOf("Settings", textTheme.HeadlineLarge),
                widgets.VSpace(24),
                // Content
                widgets.RowOf(
                    widgets.MainAxisAlignmentSpaceBetween,
                    widgets.CrossAxisAlignmentCenter,
                    widgets.MainAxisSizeMax,
                    widgets.TextOf("Dark Mode", textTheme.BodyLarge),
                    widgets.Switch{Value: s.isDark, OnChanged: s.setDarkMode},
                ),
                widgets.VSpace(16),
                // Action
                widgets.ButtonOf("Save", s.handleSave).
                    WithColor(colors.Primary, colors.OnPrimary),
            ),
        ),
    }
}
```

## Flex Layout

`Row` and `Column` are flex containers that arrange children along an axis.

### Row

Arrange children horizontally:

```go
widgets.Row{
    MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    MainAxisSize:       widgets.MainAxisSizeMax,
    ChildrenWidgets: []core.Widget{
        avatar,
        widgets.Expanded{ChildWidget: title},
        menuButton,
    },
}
```

### Column

Arrange children vertically:

```go
widgets.Column{
    MainAxisAlignment:  widgets.MainAxisAlignmentStart,
    CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
    MainAxisSize:       widgets.MainAxisSizeMin,
    ChildrenWidgets: []core.Widget{
        header,
        content,
        footer,
    },
}
```

### Helper Functions

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
// Example: Evenly distribute buttons
widgets.RowOf(
    widgets.MainAxisAlignmentSpaceEvenly,
    widgets.CrossAxisAlignmentCenter,
    widgets.MainAxisSizeMax,
    cancelButton,
    saveButton,
)
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
// Example: Stretch children to full width
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStretch,
    widgets.MainAxisSizeMax,
    cardOne,
    cardTwo,
)
```

## Main Axis Size

Controls how much space the flex container takes:

| Size | Effect |
|------|--------|
| `MainAxisSizeMin` | Take minimum needed space |
| `MainAxisSizeMax` | Take all available space |

```go
// Example: Column that takes minimum height
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,  // Only as tall as content
    items...,
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

## Stack Layout

`Stack` overlays children on top of each other:

```go
widgets.Stack{
    Alignment: layout.AlignmentCenter,
    Fit:       widgets.StackFitLoose,
    ChildrenWidgets: []core.Widget{
        backgroundImage,
        gradientOverlay,
        widgets.Positioned{
            Bottom: widgets.Ptr(16),
            Left:   widgets.Ptr(16),
            Right:  widgets.Ptr(16),
            ChildWidget: titleText,
        },
    },
}
```

### Positioned

Use `Positioned` within a Stack for absolute positioning:

```go
widgets.Stack{
    ChildrenWidgets: []core.Widget{
        mainContent,
        // Badge in top-right corner
        widgets.Positioned{
            Top:   widgets.Ptr(8),
            Right: widgets.Ptr(8),
            ChildWidget: badge,
        },
    },
}
```

### Stack Fit

| Fit | Effect |
|-----|--------|
| `StackFitLoose` | Children can be smaller than stack |
| `StackFitExpand` | Non-positioned children expand to fill |

## Sizing Widgets

### SizedBox

Give a widget fixed dimensions:

```go
// Fixed size
widgets.SizedBox{
    Width:       100,
    Height:      50,
    ChildWidget: content,
}

// Width only
widgets.SizedBox{
    Width:       200,
    ChildWidget: content,
}

// Spacer (no child)
widgets.SizedBox{Height: 16}
```

### Expanded

Fill remaining space in a flex container:

```go
widgets.RowOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentCenter,
    widgets.MainAxisSizeMax,
    avatar,
    widgets.HSpace(12),
    widgets.Expanded{ChildWidget: nameAndStatus},  // Takes remaining width
    menuButton,
)
```

### Expanded with Flex

Control how space is distributed:

```go
widgets.RowOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentCenter,
    widgets.MainAxisSizeMax,
    widgets.Expanded{Flex: 2, ChildWidget: leftPanel},  // 2/3 of space
    widgets.Expanded{Flex: 1, ChildWidget: rightPanel}, // 1/3 of space
)
```

## Padding

Add spacing around a widget:

```go
// All sides
widgets.PaddingAll(16, child)

// Symmetric
widgets.Padding{
    EdgeInsets:  layout.EdgeInsetsSymmetric(16, 8), // horizontal, vertical
    ChildWidget: child,
}

// Individual sides
widgets.Padding{
    EdgeInsets: layout.EdgeInsets{
        Left:   16,
        Top:    8,
        Right:  16,
        Bottom: 8,
    },
    ChildWidget: child,
}

// Only specific sides
widgets.Padding{
    EdgeInsets:  layout.EdgeInsetsOnly(16, 0, 16, 0), // left, top, right, bottom
    ChildWidget: child,
}
```

## Container

Combines decoration, sizing, padding, and alignment:

```go
// Struct literal (preferred)
widgets.Container{
    Color:       colors.Surface,
    Padding:     layout.EdgeInsetsAll(20),
    Alignment:   layout.AlignmentCenter,
    ChildWidget: child,
}

// For rounded corners, wrap with DecoratedBox or ClipRRect
widgets.DecoratedBox{
    Color:        colors.Surface,
    BorderRadius: 8,
    ChildWidget: widgets.Padding{
        EdgeInsets:  layout.EdgeInsetsAll(20),
        ChildWidget: child,
    },
}

// Struct literal
widgets.Container{
    Color:       colors.Surface,
    Padding:     layout.EdgeInsetsAll(20),
    Alignment:   layout.AlignmentCenter,
    Width:       200,
    Height:      100,
    ChildWidget: child,
}
```

## Center

Center a child within available space:

```go
widgets.Center{
    ChildWidget: widgets.Text{Content: "Centered"},
}

// Helper function
widgets.Centered(widgets.Text{Content: "Centered"})
```

## SafeArea

Avoid system UI (notches, status bars, navigation bars):

```go
widgets.SafeArea{
    ChildWidget: content,
}

// Selective sides
widgets.SafeArea{
    Top:         true,
    Bottom:      true,
    Left:        false,
    Right:       false,
    ChildWidget: content,
}
```

## Constraints

Every widget receives `BoxConstraints` from its parent:

```go
type BoxConstraints struct {
    MinWidth  float64
    MaxWidth  float64
    MinHeight float64
    MaxHeight float64
}
```

Widgets must return a size that satisfies these constraints.

### Constraint Types

- **Tight**: MinWidth == MaxWidth and MinHeight == MaxHeight (exact size)
- **Loose**: Min values are 0 (size can be smaller than max)
- **Unbounded**: Max value is infinity (content determines size)

### Example: How Constraints Flow

```go
// Parent passes tight constraints (100x100)
Container{Width: 100, Height: 100, ChildWidget: child}
// Child receives: MinWidth=100, MaxWidth=100, MinHeight=100, MaxHeight=100

// Parent passes loose constraints
Column{ChildrenWidgets: []Widget{child}}
// Child receives: MinWidth=0, MaxWidth=parentWidth, MinHeight=0, MaxHeight=infinity
```

## DecoratedBox

Add visual styling without layout:

```go
widgets.DecoratedBox{
    Color:        colors.Surface,
    BorderRadius: 8,
    Gradient: graphics.NewLinearGradient(
        graphics.Offset{X: 0, Y: 0},    // start
        graphics.Offset{X: 0, Y: 100},  // end
        []graphics.GradientStop{
            {Position: 0, Color: color1},
            {Position: 1, Color: color2},
        },
    ),
    Shadow: &graphics.BoxShadow{
        Color:      shadowColor,
        BlurRadius: 8,
        Offset:     graphics.Offset{X: 0, Y: 2},
    },
    ChildWidget: content,
}
```

## ClipRRect

Clip with rounded corners:

```go
widgets.ClipRRect{
    BorderRadius: 16,
    ChildWidget:  image,
}
```

## Layout Boundaries

For performance, use `RepaintBoundary` to prevent repaint propagation:

```go
widgets.RepaintBoundary{
    ChildWidget: expensiveContent,
}
```

Use when:
- A subtree repaints frequently but ancestors don't change
- Animating a small part of a complex layout
- Complex custom painting

## Common Patterns

### Card Layout

```go
widgets.DecoratedBox{
    Color:        colors.Surface,
    BorderRadius: 8,
    ChildWidget: widgets.Column{
        MainAxisAlignment:  widgets.MainAxisAlignmentStart,
        CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
        MainAxisSize:       widgets.MainAxisSizeMin,
        ChildrenWidgets: []core.Widget{
            widgets.ClipRRect{
                BorderRadius: 8,
                ChildWidget:  image,
            },
            widgets.PaddingAll(16,
                widgets.ColumnOf(
                    widgets.MainAxisAlignmentStart,
                    widgets.CrossAxisAlignmentStart,
                    widgets.MainAxisSizeMin,
                    widgets.TextOf(title, textTheme.TitleMedium),
                    widgets.VSpace(4),
                    widgets.TextOf(subtitle, textTheme.BodySmall),
                ),
            ),
        },
    },
}
```

### List Item

```go
widgets.PaddingAll(16,
    widgets.RowOf(
        widgets.MainAxisAlignmentStart,
        widgets.CrossAxisAlignmentCenter,
        widgets.MainAxisSizeMax,
        avatar,
        widgets.HSpace(16),
        widgets.Expanded{
            ChildWidget: widgets.ColumnOf(
                widgets.MainAxisAlignmentCenter,
                widgets.CrossAxisAlignmentStart,
                widgets.MainAxisSizeMin,
                widgets.TextOf(name, textTheme.TitleMedium),
                widgets.TextOf(subtitle, textTheme.BodySmall),
            ),
        },
        chevronIcon,
    ),
)
```

### App Bar

```go
widgets.Container{
    Color:   colors.Surface,
    Padding: layout.EdgeInsetsSymmetric(16, 12),
    ChildWidget: widgets.RowOf(
        widgets.MainAxisAlignmentSpaceBetween,
        widgets.CrossAxisAlignmentCenter,
        widgets.MainAxisSizeMax,
        backButton,
        widgets.TextOf(title, textTheme.TitleLarge),
        menuButton,
    ),
}
```

## Next Steps

- [Animation](/docs/guides/animation) - Animate layout changes
- [Theming](/docs/guides/theming) - Style your app
- [API Reference](/docs/api/layout) - Layout API documentation
