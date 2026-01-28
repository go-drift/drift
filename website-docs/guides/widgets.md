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

### Struct Literals

Use struct literals when fields are self-documenting:

```go
button := widgets.Button{
    Label: "Submit",
    OnTap: handleSubmit,
}

text := widgets.Text{
    Content: "Hello, Drift",
    Style:   graphics.TextStyle{Color: colors.OnSurface, FontSize: 16},
}
```

### Helper Functions

Use helpers when sensible defaults improve ergonomics:

```go
// ButtonOf applies defaults (haptic feedback enabled)
button := widgets.ButtonOf("Submit", handleSubmit)

// TextOf is concise for styled text (Wrap defaults to true)
title := widgets.TextOf("Welcome", textTheme.HeadlineLarge)

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

### Builder Pattern

Chain methods when you need to override defaults:

```go
button := widgets.ButtonOf("Submit", onSubmit).
    WithColor(colors.Primary, colors.OnPrimary).
    WithFontSize(18).
    WithPadding(layout.EdgeInsetsSymmetric(32, 16))

container := widgets.Container{
    Color:     colors.Surface,
    Padding:   layout.EdgeInsetsAll(20),
    Alignment: layout.AlignmentCenter,
    ChildWidget: child,
}
```

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
| `ErrorBoundary` | Catch and recover from build errors |

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
    return widgets.TextOf("Hello, "+g.Name, theme.TextThemeOf(ctx).BodyLarge)
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
    return widgets.ButtonOf(
        fmt.Sprintf("Count: %d", s.count),
        func() {
            s.SetState(func() {
                s.count++
            })
        },
    )
}
```

## Error Boundaries

Catch and recover from errors in a widget subtree:

```go
widgets.ErrorBoundary{
    ChildWidget: riskyWidget,
    FallbackBuilder: func(err *errors.BuildError) core.Widget {
        return widgets.Text{Content: "Something went wrong"}
    },
    OnError: func(err *errors.BuildError) {
        log.Printf("Widget error: %v", err)
    },
}
```

Error boundaries prevent a single widget's error from crashing your entire app. They catch panics during the build phase and display a fallback UI instead.

### When to Use Error Boundaries

- Around third-party widgets
- Around complex widget subtrees
- At screen boundaries
- Around widgets that depend on external data

```go
// Wrap each screen in an error boundary
func buildHomeScreen(ctx core.BuildContext) core.Widget {
    return widgets.ErrorBoundary{
        ChildWidget: HomeContent{},
        FallbackBuilder: func(err *errors.BuildError) core.Widget {
            return widgets.Column{
                ChildrenWidgets: []core.Widget{
                    widgets.Text{Content: "Failed to load home screen"},
                    widgets.ButtonOf("Retry", func() {
                        // Trigger rebuild
                    }),
                },
            }
        },
    }
}
```

## Lists and Scrolling

### Basic ListView

For small lists with all items in memory:

```go
widgets.ListView{
    ChildrenWidgets: []core.Widget{
        item1,
        item2,
        item3,
    },
    Padding: layout.EdgeInsetsAll(16),
}
```

### Virtualized Lists

For large lists, use `ListViewBuilder` which only builds visible items:

```go
widgets.ListViewBuilder{
    ItemCount:   1000,
    ItemExtent:  60,  // Fixed item height enables virtualization
    CacheExtent: 100, // Extra pixels to render beyond viewport
    ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
        item := items[index]
        return widgets.Container{
            Padding:     layout.EdgeInsetsAll(16),
            ChildWidget: widgets.Text{Content: item.Title},
        }
    },
}
```

### Scroll Physics

Control scroll behavior:

```go
widgets.ScrollView{
    Physics: widgets.BouncingScrollPhysics{}, // iOS-style bounce
    // or
    Physics: widgets.ClampingScrollPhysics{}, // Android-style clamp
    ChildWidget: content,
}
```

### Scroll Direction

```go
widgets.ListView{
    ScrollDirection: widgets.AxisHorizontal, // Defaults to vertical
    ChildrenWidgets: items,
}
```

## Next Steps

- [Forms](/docs/guides/forms) - Form validation and input controls
- [State Management](/docs/guides/state-management) - Managing widget state
- [Layout](/docs/guides/layout) - Arranging widgets
- [Animation](/docs/guides/animation) - Animating widgets
- [API Reference](/docs/api/widgets) - Full widgets API
