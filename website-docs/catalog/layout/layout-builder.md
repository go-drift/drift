---
id: layout-builder
title: LayoutBuilder
---

# LayoutBuilder

Builds its child using a callback that receives the parent's layout constraints. This lets you create responsive layouts that adapt to available space.

## Basic Usage

```go
widgets.LayoutBuilder{
    Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
        if constraints.MaxWidth > 600 {
            return wideLayout()
        }
        return narrowLayout()
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Builder` | `func(core.BuildContext, layout.Constraints) core.Widget` | Called during layout with the resolved constraints. Returns the child widget tree. May be nil, which produces a zero-size box. |

## How It Works

Drift's pipeline normally runs Build before Layout, so widgets cannot observe constraints at build time. LayoutBuilder bridges this gap by deferring its child building to the layout phase:

1. During the build phase, LayoutBuilder mounts without building any children.
2. During layout, the render object receives constraints from its parent and invokes the builder callback.
3. The builder returns a widget tree that is reconciled into the element tree.
4. The resulting child is laid out with the same constraints.

The builder is re-invoked when constraints change or when the widget is updated (for example, because an inherited dependency changed).

## Common Patterns

### Responsive Breakpoints

Switch between layouts based on available width:

```go
widgets.LayoutBuilder{
    Builder: func(ctx core.BuildContext, c layout.Constraints) core.Widget {
        if c.MaxWidth >= 900 {
            return desktopLayout(ctx)
        }
        if c.MaxWidth >= 600 {
            return tabletLayout(ctx)
        }
        return phoneLayout(ctx)
    },
}
```

### Adaptive Grid Columns

Choose a column count based on the container width:

```go
widgets.LayoutBuilder{
    Builder: func(ctx core.BuildContext, c layout.Constraints) core.Widget {
        columns := int(c.MaxWidth) / 200
        if columns < 1 {
            columns = 1
        }
        return buildGrid(columns, items)
    },
}
```

### Nested Inside a SizedBox

LayoutBuilder receives whatever constraints its parent provides. Wrapping it in a constraining widget narrows the constraints the builder sees:

```go
widgets.Center{
    Child: widgets.SizedBox{
        Width:  300,
        Height: 200,
        Child: widgets.LayoutBuilder{
            Builder: func(ctx core.BuildContext, c layout.Constraints) core.Widget {
                // c.MaxWidth == 300, c.MaxHeight == 200
                return buildContent(c)
            },
        },
    },
}
```

## When to Use LayoutBuilder

- Choosing between different widget trees based on available width or height
- Computing sizes or column counts from constraints
- Building content that must know its maximum dimensions before rendering

When you only need to constrain a child to a fixed size, prefer [SizedBox](/docs/catalog/layout/sizedbox) instead.

## Related

- [SizedBox](/docs/catalog/layout/sizedbox) for fixed dimensions
- [Expanded & Flexible](/docs/catalog/layout/expanded-flexible) for proportional sizing in flex containers
- [Layout System](/docs/guides/layout) for how constraints flow through the tree
