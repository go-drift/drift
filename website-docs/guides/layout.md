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
        Child: widgets.PaddingAll(20,
            widgets.ColumnOf(
                widgets.MainAxisAlignmentStart,
                widgets.CrossAxisAlignmentStart,
                widgets.MainAxisSizeMin,
                // Header
                widgets.Text{Content: "Settings", Style: textTheme.HeadlineLarge},
                widgets.VSpace(24),
                // Content
                widgets.RowOf(
                    widgets.MainAxisAlignmentSpaceBetween,
                    widgets.CrossAxisAlignmentCenter,
                    widgets.MainAxisSizeMax,
                    widgets.Text{Content: "Dark Mode", Style: textTheme.BodyLarge},
                    widgets.Switch{Value: s.isDark, OnChanged: s.setDarkMode},
                ),
                widgets.VSpace(16),
                // Action
                widgets.Button{
                    Label:     "Save",
                    OnTap:     s.handleSave,
                    Color:     colors.Primary,
                    TextColor: colors.OnPrimary,
                    Haptic:    true,
                },
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
    Children: []core.Widget{
        avatar,
        widgets.Expanded{Child: title},
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
    Children: []core.Widget{
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
// Example: Stretch children to full width
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
// Example: Column that shrink-wraps its children
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

## Wrap Layout

`Wrap` lays out children in runs, automatically wrapping to the next line when space runs out. Similar to CSS flexbox with `flex-wrap: wrap`.

### Basic Usage

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

### Direction

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

### Alignment

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

### WrapOf Helper

Use `WrapOf` for concise creation with spacing:

```go
widgets.WrapOf(8, 12, // spacing, runSpacing
    chip("Tag 1"),
    chip("Tag 2"),
    chip("Tag 3"),
)
```

### When to Use Wrap vs Row/Column

| Use Case | Widget |
|----------|--------|
| Fixed number of items in a line | Row or Column |
| Items should wrap when they don't fit | Wrap |
| Need flexible children (Expanded) | Row or Column |
| Dynamic tags, chips, or badges | Wrap |

## Stack Layout

`Stack` overlays children on top of each other:

```go
widgets.Stack{
    Alignment: layout.AlignmentCenter,
    Fit:       widgets.StackFitLoose,
    Children: []core.Widget{
        backgroundImage,
        gradientOverlay,
        widgets.Positioned{
            Bottom: widgets.Ptr(16),
            Left:   widgets.Ptr(16),
            Right:  widgets.Ptr(16),
            Child: titleText,
        },
    },
}
```

### Positioned

Use `Positioned` within a Stack for absolute positioning:

```go
widgets.Stack{
    Children: []core.Widget{
        mainContent,
        // Badge in top-right corner
        widgets.Positioned{
            Top:   widgets.Ptr(8),
            Right: widgets.Ptr(8),
            Child: badge,
        },
    },
}
```

### Positioned with Alignment

`Positioned` also supports relative positioning via `Alignment`. When set, the child is centered on the alignment point within the Stack bounds.

The `Alignment` type uses coordinates from -1 to 1, where (-1, -1) is top-left, (0, 0) is center, and (1, 1) is bottom-right. Use the named constants like `graphics.AlignCenter`, `graphics.AlignBottomRight`, etc.

When `Alignment` is set, `Left`/`Top`/`Right`/`Bottom` become pixel offsets from that centered position:
- `Left`/`Top` shift the child in the positive direction (right/down)
- `Right`/`Bottom` shift the child in the negative direction (left/up, i.e., inward from edges)

```go
widgets.Stack{
    Children: []core.Widget{
        background,
        // Centered dialog (no offsets needed)
        widgets.Positioned{
            Alignment: &graphics.AlignCenter,
            Child: dialog,
        },
        // Floating action button: starts at bottom-right corner,
        // then shifts 16px left and 16px up (inward from corner)
        widgets.Positioned{
            Alignment: &graphics.AlignBottomRight,
            Right:     widgets.Ptr(16),
            Bottom:    widgets.Ptr(16),
            Child: fab,
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
    Child: content,
}

// Width only
widgets.SizedBox{
    Width:       200,
    Child: content,
}

// Spacer (no child)
widgets.SizedBox{Height: 16}
```

### Expanded

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

### Flexible

`Flexible` allows a child to participate in flex space distribution without requiring it to fill all allocated space. This is useful when you want proportional space allocation but the child may not need all of it.

#### Flexible vs Expanded

| Widget | Default Fit | Constraints | Use Case |
|--------|-------------|-------------|----------|
| `Expanded` | Tight | Min = Max = allocated | Child must fill space (panels, containers) |
| `Flexible` | Loose | Min = 0, Max = allocated | Child can be smaller (text, icons) |

#### Basic Usage

Text takes only the width it needs, while the panel fills the rest:

```go
widgets.Row{
    CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
    Children: []core.Widget{
        widgets.Flexible{Child: widgets.Text{Content: "Short"}},  // Uses only needed width
        widgets.Expanded{Child: panel},                           // Fills remaining space
    },
}
```

#### With Flex Factors

Distribute space proportionally while allowing children to be smaller than allocated:

```go
widgets.Row{
    Children: []core.Widget{
        widgets.Flexible{Flex: 1, Child: labelA},  // Gets up to 1/3 of space
        widgets.Flexible{Flex: 2, Child: labelB},  // Gets up to 2/3 of space
    },
}
```

#### FlexFit Options

Control fit behavior explicitly with the `Fit` field:

| Fit | Behavior |
|-----|----------|
| `FlexFitLoose` (default) | Child can be smaller than allocated space |
| `FlexFitTight` | Child must fill allocated space (same as Expanded) |

```go
// Equivalent to Expanded
widgets.Flexible{
    Flex:  1,
    Fit:   widgets.FlexFitTight,
    Child: content,
}
```

#### When to Use Flexible

- Text labels that may vary in length
- Icons or badges with fixed intrinsic size
- Any widget where you want proportional space allocation but the widget shouldn't stretch

## Padding

Add spacing around a widget:

```go
// All sides
widgets.PaddingAll(16, child)

// Symmetric
widgets.Padding{
    EdgeInsets:  layout.EdgeInsetsSymmetric(16, 8), // horizontal, vertical
    Child: child,
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
    EdgeInsets:  layout.EdgeInsetsOnly(16, 0, 16, 0), // left, top, right, bottom
    Child: child,
}
```

## Container

Combines decoration, sizing, padding, and alignment:

```go
// Rounded card with padding (shrink-wraps to content)
widgets.Container{
    Color:        colors.Surface,
    BorderRadius: 12,
    Padding:      layout.EdgeInsetsAll(20),
    Child:  content,
}

// Bordered box
widgets.Container{
    BorderColor:  colors.Outline,
    BorderWidth:  1,
    BorderRadius: 8,
    Padding:      layout.EdgeInsetsAll(12),
    Child:  child,
}

// Fixed-size with centered child
widgets.Container{
    Color:       colors.Surface,
    Width:       200,
    Height:      100,
    Alignment:   layout.AlignmentCenter,  // Centers child within 200x100
    Child: child,
}
```

Alignment positions the child within the content area (after padding). When Container shrink-wraps to child+padding, alignment has no visible effect. When the container is larger than its content (via Width/Height or parent constraints), alignment controls child positioning.

## Center

Center a child within available space:

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
    Alignment:   layout.AlignmentBottomRight,
    Child: widgets.Text{Content: "Bottom right"},
}
```

Align expands to fill available space, then positions the child. Center is equivalent to `Align{Alignment: layout.AlignmentCenter}`.

## SafeArea

Avoid system UI (notches, status bars, navigation bars):

```go
widgets.SafeArea{
    Child: content,
}

// Selective sides
widgets.SafeArea{
    Top:         true,
    Bottom:      true,
    Left:        false,
    Right:       false,
    Child: content,
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
// Container with explicit size passes loose constraints to child
Container{Width: 100, Height: 100, Child: child}
// Child receives: MinWidth=0, MaxWidth=100, MinHeight=0, MaxHeight=100
// Child can be smaller than container; Alignment positions it within

// Column passes loose/unbounded constraints
Column{Children: []Widget{child}}
// Child receives: MinWidth=0, MaxWidth=parentWidth, MinHeight=0, MaxHeight=infinity
```

## DecoratedBox

Add visual styling without layout:

```go
widgets.DecoratedBox{
    Color:        colors.Surface,
    BorderRadius: 8,
    Gradient: graphics.NewLinearGradient(
        graphics.AlignTopCenter,    // start (top center)
        graphics.AlignBottomCenter, // end (bottom center)
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
    Child: content,
}
```

### Gradient Alignment

Gradients use relative coordinates via `Alignment` where (-1, -1) is top-left, (0, 0) is center, and (1, 1) is bottom-right. This allows gradients to scale with widget dimensions:

```go
// Horizontal gradient (left to right)
graphics.NewLinearGradient(
    graphics.AlignCenterLeft,
    graphics.AlignCenterRight,
    stops,
)

// Vertical gradient (top to bottom)
graphics.NewLinearGradient(
    graphics.AlignTopCenter,
    graphics.AlignBottomCenter,
    stops,
)

// Diagonal gradient
graphics.NewLinearGradient(
    graphics.AlignTopLeft,
    graphics.AlignBottomRight,
    stops,
)

// Radial gradient centered in widget
graphics.NewRadialGradient(
    graphics.AlignCenter,
    1.0, // radius = half the min dimension (touches nearest edge)
    stops,
)
```

### Gradient Borders

Apply gradients to borders using `BorderGradient`:

```go
widgets.DecoratedBox{
    BorderWidth:  3,
    BorderRadius: 12,
    BorderGradient: graphics.NewLinearGradient(
        graphics.AlignTopLeft,
        graphics.AlignBottomRight,
        []graphics.GradientStop{
            {Position: 0, Color: colors.Primary},
            {Position: 1, Color: colors.Secondary},
        },
    ),
    Child: content,
}

// Dashed gradient border
widgets.Container{
    BorderWidth:  2,
    BorderRadius: 8,
    BorderDash:   &graphics.DashPattern{Intervals: []float64{8, 4}},
    BorderGradient: graphics.NewLinearGradient(
        graphics.AlignCenterLeft,
        graphics.AlignCenterRight,
        stops,
    ),
    Padding:     layout.EdgeInsetsAll(16),
    Child: child,
}
```

When both `BorderColor` and `BorderGradient` are set, the gradient takes precedence.

### Container vs DecoratedBox

| Feature | Container | DecoratedBox |
|---------|-----------|--------------|
| **Purpose** | Layout + decoration | Decoration only |
| Padding | ✓ | ✗ |
| Width/Height | ✓ | ✗ |
| Alignment | ✓ | ✗ |
| Color/Gradient | ✓ | ✓ |
| Shadow | ✓ | ✓ |
| BorderRadius | ✓ | ✓ |
| BorderColor/Width | ✓ | ✓ |
| BorderGradient | ✓ | ✓ |
| BorderDash | ✓ | ✓ |

Container and DecoratedBox share the same painting implementation internally.

**Use Container** for most cases — it handles layout and decoration together.

**Use DecoratedBox** when you want decoration without any layout behavior (child sizes to parent constraints).

### Overflow and Clipping

The `Overflow` field controls whether children are clipped to the container bounds:

```go
// Children clipped to bounds (default)
widgets.Container{
    BorderRadius: 12,
    Overflow:     widgets.OverflowClip,  // default
    Child: widgets.Column{
        Children: []core.Widget{
            // This gradient bar will have rounded top corners
            widgets.Container{
                Height:   4,
                Gradient: accentGradient,
            },
            content,
        },
    },
}
```

| Overflow | Gradient | Children |
|----------|----------|----------|
| `OverflowClip` (default) | Clipped to bounds | Clipped to bounds |
| `OverflowVisible` | Can extend beyond bounds | Not clipped |

With `OverflowClip`, children are clipped to the widget bounds. When `BorderRadius > 0`, content is clipped to the rounded shape. This is useful for cards with accent bars, images, or other content at the edges that should conform to the container's shape.

Shadows are always drawn behind the decoration and naturally overflow bounds regardless of the `Overflow` setting.

**Note:** Platform views (native text fields, switches, etc.) are clipped to rectangular bounds only, not rounded corners. This is a platform limitation.

## ClipRRect

Clip with rounded corners:

```go
widgets.ClipRRect{
    BorderRadius: 16,
    Child:  image,
}
```

## Repaint Boundaries and the Layer Tree

Drift uses a **layer tree** for efficient incremental repainting. Each repaint boundary gets its own cached layer. When a widget marks itself as needing paint, only its boundary's layer is re-recorded -- parent layers reference children via stable pointers and don't need to re-record.

### How It Works

The rendering pipeline has three phases:

1. **Build & Layout** -- widgets rebuild and render objects compute sizes/positions.
2. **Record** -- dirty repaint boundaries re-record their content into display lists. Child boundaries are recorded as `DrawChildLayer` references, not embedded content.
3. **Composite** -- the layer tree is walked top-down, replaying each layer's display list onto the canvas.

This means changing a deeply nested widget only re-records the nearest boundary's layer, not the entire tree.

### Using RepaintBoundary

Wrap subtrees that repaint independently:

```go
widgets.RepaintBoundary{
    Child: expensiveContent,
}
```

Use when:
- A subtree repaints frequently but ancestors don't change
- Animating a small part of a complex layout
- Complex custom painting

The root `View` widget is always a repaint boundary. You don't need to add one yourself unless you want to isolate a specific subtree.

### Platform Views and Culling

Platform views (native text fields, switches, etc.) call `ctx.EmbedPlatformView()` during paint. The compositing phase resolves each view's position and clip bounds in global coordinates and sends them to the native side.

When a platform view is culled (scrolled off-screen), no `EmbedPlatformView` op is recorded. The framework detects unseen views after compositing and tells the native side to hide them. When the view scrolls back into view, it receives updated geometry and becomes visible again.

## Common Patterns

### Card Layout

```go
// Image at top is automatically clipped to rounded corners
widgets.DecoratedBox{
    Color:        colors.Surface,
    BorderRadius: 8,
    Overflow:     widgets.OverflowClip,  // default, clips children to rounded shape
    Child: widgets.Column{
        MainAxisAlignment:  widgets.MainAxisAlignmentStart,
        CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
        MainAxisSize:       widgets.MainAxisSizeMin,
        Children: []core.Widget{
            image,  // clipped to parent's rounded corners
            widgets.PaddingAll(16,
                widgets.ColumnOf(
                    widgets.MainAxisAlignmentStart,
                    widgets.CrossAxisAlignmentStart,
                    widgets.MainAxisSizeMin,
                    widgets.Text{Content: title, Style: textTheme.TitleMedium},
                    widgets.VSpace(4),
                    widgets.Text{Content: subtitle, Style: textTheme.BodySmall},
                ),
            ),
        },
    },
}
```

### List Item

```go
widgets.PaddingAll(16,
    widgets.Row{
        CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
        Children: []core.Widget{
            avatar,
            widgets.HSpace(16),
            widgets.Expanded{
                Child: widgets.Column{
                    MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
                    CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
                    MainAxisSize:       widgets.MainAxisSizeMin,
                    Children: []core.Widget{
                        widgets.Text{Content: name, Style: textTheme.TitleMedium},
                        widgets.Text{Content: subtitle, Style: textTheme.BodySmall},
                    },
                },
            },
            chevronIcon,
        },
    },
)
```

### App Bar

```go
widgets.Container{
    Color:   colors.Surface,
    Padding: layout.EdgeInsetsSymmetric(16, 12),
    Child: widgets.Row{
        MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
        CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
        Children: []core.Widget{
            backButton,
            widgets.Text{Content: title, Style: textTheme.TitleLarge},
            menuButton,
        },
    },
}
```

## Next Steps

- [Animation](/docs/guides/animation) - Animate layout changes
- [Theming](/docs/guides/theming) - Style your app
- [API Reference](/docs/api/layout) - Layout API documentation
