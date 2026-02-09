---
id: layout
title: Layout System
sidebar_position: 4
---

# Layout System

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

## Repaint Boundaries and the Layer Tree

Drift uses a **layer tree** for efficient incremental repainting. Each repaint boundary gets its own cached layer. When a widget marks itself as needing paint, only its boundary's layer is re-recorded; parent layers reference children via stable pointers and don't need to re-record.

### How It Works

The rendering pipeline has three phases:

1. **Build & Layout**: widgets rebuild and render objects compute sizes/positions.
2. **Record**: dirty repaint boundaries re-record their content into display lists. Child boundaries are recorded as `DrawChildLayer` references, not embedded content.
3. **Composite**: the layer tree is walked top-down, replaying each layer's display list onto the canvas.

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

## Responsive Layouts with LayoutBuilder

Normally, widgets are built before layout runs, so they cannot observe constraints. `LayoutBuilder` defers child building to the layout phase, giving the builder function access to the resolved constraints:

```go
widgets.LayoutBuilder{
    Builder: func(ctx core.BuildContext, c layout.Constraints) core.Widget {
        if c.MaxWidth >= 600 {
            return twoColumnLayout(ctx)
        }
        return singleColumnLayout(ctx)
    },
}
```

The builder is re-invoked whenever the constraints change (for example, when the window resizes) or when the widget is otherwise invalidated (inherited dependency update, widget replacement).

See the [LayoutBuilder catalog page](/docs/catalog/layout/layout-builder) for more examples.

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

### Settings List with Dividers

```go
widgets.Column{
    CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
    Children: []core.Widget{
        settingsRow("Dark Mode", darkModeSwitch),
        theme.DividerOf(ctx),
        settingsRow("Notifications", notificationsSwitch),
        theme.DividerOf(ctx),
        settingsRow("Language", languageDropdown),
    },
}
```

### Horizontal Sections with Vertical Divider

```go
widgets.IntrinsicHeight{
    Child: widgets.Row{
        Children: []core.Widget{
            leftSection,
            theme.VerticalDividerOf(ctx),
            rightSection,
        },
    },
}
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

- [Widget Catalog](/docs/category/widget-catalog) - Detailed usage for every layout widget
- [Animation](/docs/guides/animation) - Animate layout changes
- [Theming](/docs/guides/theming) - Style your app
- [API Reference](/docs/api/layout) - Layout API documentation
