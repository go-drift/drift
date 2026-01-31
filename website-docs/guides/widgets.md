---
id: widgets
title: Widgets
sidebar_position: 2
---

# Widgets

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
_, colors, _ := theme.UseTheme(ctx)

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
```

### 2. Layout Helpers

Layout helpers like `ColumnOf`, `RowOf`, and `Centered` remain for ergonomics:

```go
// Centered wraps a child in a Center widget
centered := widgets.Centered(child)

// ColumnOf/RowOf avoid verbose struct initialization
col := widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    child1,
    child2,
)
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

## Available Widgets

### Layout Widgets

| Widget | Purpose |
|--------|---------|
| `Row` | Horizontal arrangement |
| `Column` | Vertical arrangement |
| `Stack` | Overlay children |
| `IndexedStack` | Show one child at a time by index |
| `Center` | Center child in available space |
| `Padding` | Add spacing around child |
| `Container` | Decoration, sizing, alignment |
| `SizedBox` | Fixed dimensions |
| `Expanded` | Fill remaining flex space |
| `SafeArea` | Avoid system UI (notches, nav bars) |
| `Positioned` | Absolute positioning within Stack |

### Display Widgets

| Widget | Purpose |
|--------|---------|
| `Text` | Display text with styling |
| `Icon` | Material icons |
| `SvgImage` | SVG rendering with flexible sizing |
| `SvgIcon` | Square SVG icons (convenience wrapper) |
| `Image` | Display images from assets/files |

### Progress Indicators

| Widget | Purpose |
|--------|---------|
| `ActivityIndicator` | Native platform spinner |
| `CircularProgressIndicator` | Circular progress (determinate/indeterminate) |
| `LinearProgressIndicator` | Linear progress bar (determinate/indeterminate) |

### Input Widgets

| Widget | Purpose |
|--------|---------|
| `Button` | Tappable button with haptic feedback |
| `TextInput` | Base native text input |
| `TextField` | Decorated native text input with label and helper text |
| `TextFormField` | TextField with form validation |
| `Checkbox` | Boolean toggle |
| `Radio` | Single selection from group |
| `Switch` | Native on/off toggle (UISwitch/SwitchCompat) |
| `Toggle` | Drift-rendered on/off toggle |
| `Dropdown` | Selection menu |
| `DatePicker` | Native date picker modal |
| `TimePicker` | Native time picker modal |
| `Form` | Form container with validation |

### Scrolling Widgets

| Widget | Purpose |
|--------|---------|
| `ScrollView` | Scrollable content |
| `ListView` | Scrollable list of widgets |
| `ListViewBuilder` | Virtualized lazy-loading list |

### Decorative Widgets

| Widget | Purpose |
|--------|---------|
| `ClipRRect` | Rounded rectangle clipping |
| `DecoratedBox` | Background, border, gradient, shadow |
| `Opacity` | Static transparency |
| `AnimatedOpacity` | Animated transparency |
| `AnimatedContainer` | Animated layout/decoration changes |
| `RepaintBoundary` | Isolate paint for performance |

### Navigation Widgets

| Widget | Purpose |
|--------|---------|
| `Navigator` | Stack-based route management |
| `TabBar` | Tab navigation bar |
| `TabScaffold` | Tab layout with content |

### Error Handling

| Widget | Purpose |
|--------|---------|
| `ErrorBoundary` | Catch panics and display fallback UI |
| `ErrorWidget` | Inline error display (default fallback) |
| `DebugErrorScreen` | Full-screen error display (debug mode) |

## Custom Stateless Widgets

Create a stateless widget for UI that depends only on its configuration:

```go
type Greeting struct {
    Name string
}

func (g Greeting) CreateElement() core.Element {
    return core.NewStatelessElement(g, nil)
}

func (g Greeting) Key() any { return nil }

func (g Greeting) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: "Hello, " + g.Name, Style: theme.TextThemeOf(ctx).BodyLarge}
}
```

## Custom Stateful Widgets

Create a stateful widget when you need to manage mutable state:

```go
type Counter struct{}

func (c Counter) CreateElement() core.Element {
    return core.NewStatefulElement(c, nil)
}

func (c Counter) Key() any { return nil }

func (c Counter) CreateState() core.State {
    return &counterState{}
}

type counterState struct {
    core.StateBase
    count int
}

func (s *counterState) InitState() {
    s.count = 0
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

- [Lists & Scrolling](/docs/guides/lists) - ListView, virtualized lists, scroll physics
- [Layout](/docs/guides/layout) - Arranging widgets with Flex, Stack, and containers
- [Theming](/docs/guides/theming) - Colors, typography, and styling
- [State Management](/docs/guides/state-management) - Managing widget state
- [Error Handling](/docs/guides/error-handling) - Error boundaries and fallback UI
- [API Reference](/docs/api/widgets) - Full widgets API
