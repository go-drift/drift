# Drift Usage Guide

Guidelines for building UI in Drift.

## Widget Construction

### Struct Literals

Use struct literals when fields are self-documenting:

```go
// Clear example
button := widgets.Button{
    Label: "Submit",
    OnTap: handleSubmit,
}

text := widgets.Text{
    Content: "Hello, Drift",
    Style:   rendering.TextStyle{Color: colors.OnSurface, FontSize: 16},
}
```

### Helper Functions

Use helpers when sensible defaults improve ergonomics:

```go
// NewButton applies defaults (haptic feedback, etc.)
button := widgets.NewButton("Submit", handleSubmit)

// TextOf is concise for styled text
title := widgets.TextOf("Welcome", textTheme.HeadlineLarge)

// ColumnOf avoids verbose struct initialization
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
import (
    "github.com/go-drift/drift/pkg/layout"
    "github.com/go-drift/drift/pkg/widgets"
)

button := widgets.NewButton("Submit", onSubmit).
    WithColor(colors.Primary, colors.OnPrimary).
    WithFontSize(18).
    WithPadding(layout.EdgeInsetsSymmetric(32, 16))

container := widgets.NewContainer(child).
    WithColor(colors.Surface).
    WithPaddingAll(20).
    WithAlignment(layout.AlignmentCenter).
    Build()  // ContainerBuilder requires Build() at the end
```

---

## Layout Composition

### The Composition Pattern

Build complex layouts by nesting simple widgets.

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    _, colors, textTheme := theme.UseTheme(ctx)

    return widgets.PaddingAll(20,
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
                widgets.CrossAxisAlignmentStart,
                widgets.MainAxisSizeMax,
                widgets.TextOf("Dark Mode", textTheme.BodyLarge),
                toggleSwitch,
            ),
            widgets.VSpace(16),
            // Action
            widgets.NewButton("Save", s.handleSave).
                WithColor(colors.Primary, colors.OnPrimary),
        ),
    )
}
```

### Spacing

Use `VSpace` and `HSpace` for consistent gaps:

```go
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    title,
    widgets.VSpace(16),  // Vertical gap
    body,
    widgets.VSpace(24),
    button,
)

widgets.RowOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    icon,
    widgets.HSpace(8),   // Horizontal gap
    label,
)
```

### Available Layout Widgets

| Widget | Purpose |
|--------|---------|
| `Row` | Horizontal arrangement |
| `Column` | Vertical arrangement |
| `Stack` | Overlay children |
| `Center` | Center child in available space |
| `Padding` | Add spacing around child |
| `Container` | Decoration, sizing, alignment |
| `SizedBox` | Fixed dimensions |
| `Expanded` | Fill remaining flex space |

---

## State Management

### Stateless Widgets

Use for UI that depends only on configuration:

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

### Stateful Widgets

Use when the widget manages mutable state. Embed `core.StateBase`:

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
    return widgets.NewButton(
        "Count: "+strconv.Itoa(s.count),
        func() {
            s.SetState(func() {
                s.count++
            })
        },
    )
}
```

### The SetState Pattern

Always mutate state inside `SetState`:

```go
// Good - explicit mutation, triggers rebuild
s.SetState(func() {
    s.count++
    s.label = "Updated"
})

// Bad - mutation without rebuild
s.count++  // UI won't update!
```

**Thread Safety**: `SetState` is not thread-safe. It must only be called from the UI thread. To update state from a background goroutine, use `drift.Dispatch`:

```go
go func() {
    result := doExpensiveWork()
    drift.Dispatch(func() {
        s.SetState(func() {
            s.data = result
        })
    })
}()
```

### ManagedState

`ManagedState` holds a value and triggers rebuilds automatically when changed:

```go
type myState struct {
    core.StateBase
    count *core.ManagedState[int]
}

func (s *myState) InitState() {
    s.count = core.NewManagedState(&s.StateBase, 0)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.GestureDetector{
        OnTap: func() { s.count.Set(s.count.Get() + 1) },
        Child: widgets.TextOf(fmt.Sprintf("Count: %d", s.count.Get()), ...),
    }
}
```

Like `SetState`, `ManagedState` is not thread-safe. Use `drift.Dispatch` for background updates.

### Hooks

Hooks help manage subscriptions and controllers with automatic cleanup.

#### UseObservable

Subscribe to an `Observable` and trigger rebuilds when it changes. Call in `InitState()`, read with `.Value()` in `Build()`:

```go
func (s *myState) InitState() {
    s.counter = core.NewObservable(0)
    core.UseObservable(&s.StateBase, s.counter)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.TextOf(fmt.Sprintf("Count: %d", s.counter.Value()), ...)
}
```

#### UseListenable

Subscribe to any `Listenable` (e.g., animation controllers, notifiers):

```go
func (s *myState) InitState() {
    s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })
    core.UseListenable(&s.StateBase, s.controller)
}
```

#### UseController

Create a controller with automatic disposal:

```go
func (s *myState) InitState() {
    s.animation = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })
}
```

---

## Theme Access

### UseTheme

Get all theme parts in one call:

```go
func (s *myState) Build(ctx core.BuildContext) core.Widget {
    _, colors, textTheme := theme.UseTheme(ctx)

    return widgets.NewContainer(
        widgets.TextOf("Hello", textTheme.HeadlineLarge),
    ).WithColor(colors.Surface).Build()
}
```

### Individual Accessors

When you only need one part:

```go
colors := theme.ColorsOf(ctx)
textTheme := theme.TextThemeOf(ctx)
themeData := theme.ThemeOf(ctx)
```

### Providing Theme

Wrap your app with a Theme widget:

```go
theme.Theme{
    Data: theme.DefaultDarkTheme(),  // or DefaultLightTheme()
    ChildWidget: myApp,
}
```

---

## Navigation

### Setting Up Routes

```go
navigation.Navigator{
    InitialRoute: "/",
    OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
        switch settings.Name {
        case "/":
            return navigation.NewMaterialPageRoute(buildHome, settings)
        case "/details":
            return navigation.NewMaterialPageRoute(buildDetails, settings)
        }
        return nil
    },
}
```

### Navigating

```go
func handleTap(ctx core.BuildContext) {
    nav := navigation.NavigatorOf(ctx)
    if nav == nil {
        return
    }

    // Push a named route
    nav.PushNamed("/details", nil)

    // Go back
    nav.Pop(nil)

    // Check if can go back
    if nav.CanPop() {
        nav.Pop(nil)
    }
}
```

---

## Text Input

### Controller Pattern

```go
type formState struct {
    element    *core.StatefulElement
    controller *platform.TextEditingController
}

func (s *formState) InitState() {
    s.controller = platform.NewTextEditingController("")
}

func (s *formState) Build(ctx core.BuildContext) core.Widget {
    return widgets.NativeTextField{
        Controller:   s.controller,
        Placeholder:  "Enter text",
        KeyboardType: platform.KeyboardTypeText,
        OnSubmitted:  s.handleSubmit,
    }
}

func (s *formState) handleSubmit(text string) {
    value := s.controller.Text()  // Read current value
    s.controller.Clear()          // Clear programmatically
}
```
