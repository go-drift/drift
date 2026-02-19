---
id: widgets
title: Widget Architecture
sidebar_position: 2
---

# Widget Architecture

Widgets are the building blocks of Drift applications. Everything you see on screen is a widget.

## How Widgets Work

Drift uses a layered architecture:

- **Widgets** are immutable configuration objects. They describe what the UI should look like but don't hold state or do graphics. Widgets are cheap to create and are rebuilt frequently.

- **Elements** are the mutable objects that manage the widget lifecycle. When you call `CreateElement()`, Drift creates an Element that persists across rebuilds. Elements hold references to State (for StatefulWidgets) and handle the work of updating the tree.

- **RenderObjects** handle layout and painting. Most of the time you won't interact with these directly.

When a rebuild happens, Drift compares the new widget tree with the existing elements to determine what changed. This is called *reconciliation*.

### Keys

The `Key()` method helps Drift identify widgets during reconciliation. Drift can reuse an existing element when the new widget has the same type and key as the old one.

By default, widgets return `nil` for their key, and widgets of the same type are matched in order among their siblings. Provide a key when you have a dynamic collection of stateful widgets—in a `Column`, `Row`, `ListView`, or any parent with multiple children. Without keys, if you remove the first child, Drift thinks the second child became the first, the third became the second, and so on. This causes state to be associated with the wrong widgets.

```go
// A todo item with its own state (e.g., expanded/collapsed, text field content)
type TodoItem struct {
    ID   string
    Text string
}

func (t TodoItem) Key() any { return t.ID }  // Track by ID, not position
```

When to use keys:
- Dynamic children that can be added, removed, or reordered
- Stateful widgets whose position among siblings may change

When `nil` is fine:
- Static children that never change
- Stateless widgets (no state to preserve)

## Widget Types

Drift has three types of widgets:

1. **StatelessWidget** - For UI that depends only on configuration
2. **StatefulWidget** - For UI that manages mutable state
3. **InheritedWidget** - For sharing data down the widget tree

## Creating Widgets

Most Drift widgets are **explicit by default** — zero values mean zero, not "use theme default."
There are three patterns for creating widgets:

### 1. Struct Literals (Full Control)

Use struct literals when you want complete control over all properties:

```go
colors := theme.ColorsOf(ctx)

button := widgets.Button{
    Label:        "Submit",
    OnTap:        handleSubmit,
    Color:        colors.Primary,
    TextColor:    colors.OnPrimary,
    Padding:      layout.EdgeInsetsSymmetric(24, 14),
    BorderRadius: 8,
}

text := widgets.Text{
    Content: "Hello, Drift",
    Style:   graphics.TextStyle{Color: colors.OnSurface, FontSize: 16},
}

// Centered text (wraps by default)
description := widgets.Text{
    Content: "A cross-platform UI framework for Go",
    Style:   graphics.TextStyle{Color: colors.OnSurface, FontSize: 14},
    Align:   graphics.TextAlignCenter,
}
```

### 2. Layout Helpers

Convenience helpers like `Centered`, `VSpace`, and `HSpace` remain for ergonomics:

```go
// Centered wraps a child in a Center widget
centered := widgets.Centered(child)

// Row and Column use struct literals
col := widgets.Column{
    MainAxisSize: widgets.MainAxisSizeMin,
    Children:     []core.Widget{child1, child2},
}
```

### 3. `theme.XxxOf(ctx, ...)` (Themed)

Use themed constructors to read visual properties from the current theme.
This is the recommended approach for apps that want consistent theming:

```go
// Reads colors, padding, font size, border radius from ButtonThemeData
button := theme.ButtonOf(ctx, "Submit", handleSubmit)

// Themed checkbox with theme colors
checkbox := theme.CheckboxOf(ctx, isChecked, func(v bool) {
    s.SetState(func() { isChecked = v })
})

// Themed dropdown
dropdown := theme.DropdownOf(ctx, selected, items, func(v string) {
    s.SetState(func() { selected = v })
})
```

### Builder Pattern (Overrides)

Chain `.WithX()` methods to override specific theme values. Zero values are honored:

```go
// Themed button with custom border radius (zero = sharp corners)
button := theme.ButtonOf(ctx, "Submit", onSubmit).
    WithBorderRadius(0).
    WithPadding(layout.EdgeInsetsSymmetric(32, 16))
```

### When to Use What

| Pattern | When to Use |
|---------|-------------|
| `theme.XxxOf(ctx, ...)` | Most apps — consistent theme styling |
| Struct literal | Full control, or widgets without themed constructors |

### Explicit Styling Requirements

Explicit widgets only render what you set. If colors, sizes, or text styles are zero,
the widget can be invisible or collapsed. Common cases that require full styling:

- `TextField` / `TextInput`: set `Height`, `Padding`, `BackgroundColor`, `BorderColor`,
  `FocusColor`, `BorderWidth`, `Style` (FontSize + Color), and `PlaceholderColor`.
- `Dropdown`: set `BackgroundColor`, `BorderColor`, `TextStyle.Color`, and
  `SelectedItemColor` if you want a visible selected-row highlight.
- `DatePicker` / `TimePicker`: set `TextStyle` and `Decoration` colors
  (`BorderColor`, `BackgroundColor`, hint/label styles) for explicit usage.

If you want defaults from the theme, prefer `theme.XxxOf(ctx, ...)`.

### Disabled Styling

Themed widgets use disabled colors from theme data. Explicit widgets without
`DisabledXxxColor` fields fall back to a 0.5 opacity wrapper when disabled.

## Custom Stateless Widgets

Create a stateless widget for UI that depends only on its configuration.
Embed `core.StatelessBase` to get `CreateElement` and `Key` for free:

```go
type Greeting struct {
    core.StatelessBase
    Name string
}

func (g Greeting) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: "Hello, " + g.Name, Style: theme.TextThemeOf(ctx).BodyLarge}
}
```

## Custom Stateful Widgets

Create a stateful widget when you need to manage mutable state. There are two
patterns, depending on complexity:

| Pattern | Best for |
|---------|----------|
| `Stateful[S]` | Quick inline fragments (no lifecycle, no `StateBase`) |
| `StatefulBase` embedding | All other stateful widgets (full lifecycle, `StateBase`) |

### Inline: `Stateful`

Use `Stateful` for small, self-contained pieces of state where you don't need
lifecycle hooks, `Managed`, or `UseController`:

```go
core.Stateful(
    func() int { return 0 },
    func(count int, ctx core.BuildContext, setState func(func(int) int)) core.Widget {
        return theme.ButtonOf(ctx, fmt.Sprintf("Count: %d", count), func() {
            setState(func(c int) int { return c + 1 })
        })
    },
)
```

The `init` function runs once; `build` is called on every rebuild with the
current state, a `BuildContext`, and a `setState` callback that applies
a transform function to the state.

### Struct-based: `StatefulBase`

Embed `core.StatefulBase` in your widget struct to get `CreateElement` and `Key`
for free. This works whether or not your widget carries configuration fields:

```go
type Counter struct {
    core.StatefulBase
    InitialValue int
}

func (c Counter) CreateState() core.State {
    return &counterState{initial: c.InitialValue}
}

type counterState struct {
    core.StateBase
    initial int
    count   int
}

func (s *counterState) InitState() {
    s.count = s.initial
}

func (s *counterState) Build(ctx core.BuildContext) core.Widget {
    return theme.ButtonOf(ctx, fmt.Sprintf("Count: %d", s.count), func() {
        s.SetState(func() {
            s.count++
        })
    })
}
```

## Next Steps

- [Layout System](/docs/guides/layout) - Constraints, composition, and layout concepts
- [Widget Catalog](/docs/category/widget-catalog) - Detailed usage for every Drift widget
- [Theming](/docs/guides/theming) - Colors, typography, and styling
- [State Management](/docs/guides/state-management) - Managing widget state
- [API Reference](/docs/api/widgets) - Full widgets API
