---
id: state-management
title: State Management
sidebar_position: 3
---

# State Management

Drift provides several patterns for managing state, organized from local widget state to shared reactive patterns.

For how to define stateful and stateless widget types, see [Widget Architecture](/docs/guides/widgets#stateful-widgets).

## SetState

The simplest state pattern. Mutate fields directly and trigger a rebuild.

Always mutate state inside a `SetState` call to trigger a rebuild:

```go
// Good - explicit mutation, triggers rebuild
s.SetState(func() {
    s.count++
    s.label = "Updated"
})

// Bad - mutation without rebuild
s.count++  // UI won't update!
```

#### Example: Counter

```go
type counter struct {
    core.StatefulBase
}

func (counter) CreateState() core.State { return &counterState{} }

type counterState struct {
    core.StateBase
    count int
}

func (s *counterState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: fmt.Sprintf("Count: %d", s.count)},
            theme.ButtonOf(ctx, "Increment", func() {
                s.SetState(func() {
                    s.count++
                })
            }),
        },
    }
}
```

### Thread Safety

`SetState` is **not thread-safe**. It must only be called from the UI thread. To update state from a background goroutine, use `drift.Dispatch`:

```go
go func() {
    // Expensive work on background thread
    result := fetchDataFromNetwork()

    // Schedule UI update on main thread
    drift.Dispatch(func() {
        s.SetState(func() {
            s.data = result
            s.loading = false
        })
    })
}()
```

#### Common Pattern: Async Loading

```go
type dataState struct {
    core.StateBase
    data    []Item
    loading bool
    error   error
}

func (s *dataState) InitState() {
    s.loading = true
    go s.loadData()
}

func (s *dataState) loadData() {
    data, err := api.FetchItems()

    drift.Dispatch(func() {
        s.SetState(func() {
            s.data = data
            s.error = err
            s.loading = false
        })
    })
}

func (s *dataState) Build(ctx core.BuildContext) core.Widget {
    if s.loading {
        return widgets.Text{Content: "Loading..."}
    }
    if s.error != nil {
        return widgets.Text{Content: "Error: " + s.error.Error()}
    }
    return buildList(s.data)
}
```

:::tip
For loading images from URLs, use [NetworkImage](/docs/catalog/display/image-svg#networkimage) instead of writing your own fetch/decode/cache pipeline. It handles loading states, error display, caching, and fade-in transitions automatically.
:::

## Sharing State with InheritedWidget

Share data down the widget tree without passing it through every level.

### Simple Provider (Recommended)

For most cases, use `InheritedProvider[T]` to eliminate boilerplate:

```go
// Provide at top of tree
func App() core.Widget {
    return core.InheritedProvider[*User]{
        Value:       currentUser,
        Child: MainContent{},
    }
}

// Consume anywhere below
func (s *profileState) Build(ctx core.BuildContext) core.Widget {
    user, ok := core.Provide[*User](ctx)
    if !ok {
        return widgets.Text{Content: "Not logged in"}
    }
    return widgets.Text{Content: "Hello, " + user.Name}
}

// Or use MustProvide when you're certain the provider exists
func (s *profileState) Build(ctx core.BuildContext) core.Widget {
    user := core.MustProvide[*User](ctx) // panics if not found
    return widgets.Text{Content: "Hello, " + user.Name}
}
```

By default, dependents rebuild when the value changes (pointer equality for pointers, value equality for value types).

**When to use `Provide` vs `MustProvide`:** Use `MustProvide` when the provider is structurally guaranteed to exist (e.g. theme data provided at the root). The panic gives a clear error during development if the tree is wired incorrectly. Use `Provide` with the `ok` check when absence is a valid runtime state (e.g. optional user session) and the widget should degrade gracefully.

### Custom Comparison

Use `ShouldRebuild` when you need custom comparison logic:

```go
core.InheritedProvider[*User]{
    Value:       currentUser,
    Child: MainContent{},
    ShouldRebuild: func(old, new *User) bool {
        // Only rebuild when ID changes, ignore name updates
        return old.ID != new.ID
    },
}
```

### Custom InheritedWidget

For advanced use cases, implement a custom `InheritedWidget`. Embed `core.InheritedBase`
to get `CreateElement` and `Key` for free, then implement `ChildWidget` and
`ShouldRebuildDependents`:

```go
type UserProvider struct {
    core.InheritedBase
    User  *User
    Child core.Widget
}

func (u UserProvider) ChildWidget() core.Widget { return u.Child }

// ShouldRebuildDependents is called when this widget updates. Return true to
// rebuild all dependents, false to skip rebuilding entirely.
func (u UserProvider) ShouldRebuildDependents(old core.InheritedWidget) bool {
    if prev, ok := old.(UserProvider); ok {
        return u.User != prev.User
    }
    return true
}

// Access from anywhere in the subtree
var userProviderType = reflect.TypeOf(UserProvider{})

func UserOf(ctx core.BuildContext) *User {
    if p, ok := ctx.DependOnInherited(userProviderType, nil).(UserProvider); ok {
        return p.User
    }
    return nil
}
```

## Custom State Holders

When state lives outside a single widget, or multiple widgets need to react to changes, build a custom state holder.
Embed `Notifier` to get listener management and disposal for free. A state holder is any type that implements the `Listenable` interface:

```go
type Listenable interface {
    AddListener(listener func()) func()
}
```

### Notifier

Embed `Notifier` to turn any struct into a listenable state holder with built-in disposal:

```go
type CartNotifier struct {
    core.Notifier
    items []Item
}

func (c *CartNotifier) Add(item Item) {
    c.items = append(c.items, item)
    c.Notify() // Triggers all listeners
}

func (c *CartNotifier) Items() []Item { return c.items }
```

If your notifier holds resources, override `Dispose` and call the embedded one:

```go
func (c *CartNotifier) Dispose() {
    c.cleanup()
    c.Notifier.Dispose()
}
```

### Valueless Event Broadcasting

For a standalone event broadcaster with no value attached, use a `Notifier` directly. `Notifier` is thread-safe, so it can be shared as a package-level variable. This is useful when you need to say "something happened" without carrying data:

```go
var refreshNotifier = &core.Notifier{}

// Producer
refreshNotifier.Notify()

// Consumer
unsub := refreshNotifier.AddListener(func() {
    reloadData()
})
```

`Notifier` implements `Listenable`, so it works with `UseListenable` and can be passed as a `RefreshListenable` to routers.

### Connecting Notifiers to Widgets

Use `UseListenable` to subscribe a widget to any `Listenable` and trigger rebuilds on notification. The subscription is cleaned up automatically when the widget is removed from the tree:

```go
type cartViewState struct {
    core.StateBase
    cart *CartNotifier
}

func (s *cartViewState) InitState() {
    s.cart = globalCart
    core.UseListenable(s, s.cart) // Rebuild when cart changes
}

func (s *cartViewState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{
        Content: fmt.Sprintf("%d items in cart", len(s.cart.Items())),
    }
}
```

### ListenableBuilder

When a widget only needs to rebuild when a `Listenable` changes and has no other state, `ListenableBuilder` avoids the ceremony of a full StatefulWidget:

```go
core.ListenableBuilder{
    Listenable: cart,
    Builder: func(ctx core.BuildContext) core.Widget {
        return widgets.Text{
            Content: fmt.Sprintf("%d items in cart", len(cart.Items())),
        }
    },
}
```

`ListenableBuilder` accepts a single `Listenable`. For multiple sources, merge them with `NewDerived` or use a StatefulWidget with `UseListenable`.

**When to use what:**

| Pattern | Best for |
|---------|----------|
| `ListenableBuilder` | Leaf widgets that just display a listenable's current value |
| `UseListenable` in a StatefulWidget | Widgets that combine listenable subscriptions with local state, lifecycle hooks, or multiple listenables |

## Reactive State

`Signal` and `Derived` provide thread-safe, typed reactive values with change notification and computed derivations.

### Choosing Between Signal and Notifier

`Signal` is a ready-to-use reactive variable. `Notifier` is a building block you embed in your own types to make them listenable. They operate at different levels:

- **Use `Signal[T]`** when you have a single value that changes over time. It holds a typed value, skips notifications when the value is unchanged, and works out of the box.
- **Embed `Notifier`** when you need a custom state holder with multiple fields and manual control over when listeners fire. It provides `AddListener`, `Notify`, and `Dispose` so you don't have to implement listener management yourself.

If you find yourself reaching for `Signal[MyBigStruct]` and calling `Set` with a copy of the whole struct after changing one field, that's a sign you want a custom type with an embedded `Notifier` instead.

### Signal

`Signal` is a thread-safe reactive value. It satisfies `Listenable`, so it works with `UseListenable` and can serve as a dependency for `NewDerived`. Setting the same value is a no-op (compared via `==`). `NewSignal` requires a `comparable` type constraint, so passing a slice or map will fail at compile time. For non-comparable types, use `NewSignalWithEquality` to provide a custom comparison.

**Thread safety note:** `Signal` itself is safe to read and write from any goroutine. However, listener callbacks fire on the caller's goroutine. Since hooks like `UseListenable` and `UseDerived` call `SetState` inside those callbacks, you must call `Set` on the UI thread. From a background goroutine, wrap the call with `drift.Dispatch`.

```go
// Create a signal
counter := core.NewSignal(0)

// Listen for changes
unsub := counter.AddListener(func() {
    fmt.Println("Count changed to:", counter.Value())
})

// Update value (notifies all listeners)
counter.Set(5)

// Read-modify-write
counter.Update(func(v int) int { return v + 1 })

// Read value
current := counter.Value()

// Unsubscribe when done
unsub()
```

#### Signal in State

Use `UseListenable` to subscribe a widget to a `Signal` and trigger rebuilds on change:

```go
type myState struct {
    core.StateBase
    counter *core.Signal[int]
}

func (s *myState) InitState() {
    s.counter = core.NewSignal(0)
    core.UseListenable(s, s.counter)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: fmt.Sprintf("Count: %d", s.counter.Value())}
}
```

#### Shared Signals

A `Signal` can live outside any widget. Multiple widgets subscribe independently:

```go
// Package-level signal, outlives any single widget
var currentUser = core.NewSignal[*User](nil)

// Any widget can subscribe
func (s *profileState) InitState() {
    core.UseListenable(s, currentUser)
}

func (s *headerState) InitState() {
    core.UseListenable(s, currentUser)
}

// Update from anywhere (on the UI thread)
currentUser.Set(loggedInUser)
```

#### InheritedProvider vs Shared Signal

Both let multiple widgets access the same data. The difference is scope and rebuild semantics:

- **`InheritedProvider`** is tree-scoped. Only descendants can access the value, and only dependents rebuild when it changes. Use it for data that flows top-down (theme, locale, auth session) and where you want the tree structure to control visibility.
- **Shared `Signal`** is global. Any widget anywhere can subscribe. Use it for app-wide state that lives outside the widget tree (current user, feature flags, a shopping cart).

A rule of thumb: if you're wrapping the value in a widget and passing `Child`, use `InheritedProvider`. If the value outlives any particular subtree, use a shared `Signal`.

### Derived

`Derived` is a read-only signal that recomputes its value automatically when any of its dependencies change. Use it when you have a value that is always a function of one or more other signals.

```go
firstName := core.NewSignal("John")
lastName := core.NewSignal("Doe")

// fullName recomputes whenever firstName or lastName changes
fullName := core.NewDerived(func() string {
    return firstName.Value() + " " + lastName.Value()
}, firstName, lastName)
defer fullName.Dispose()

fmt.Println(fullName.Value()) // "John Doe"
lastName.Set("Smith")
fmt.Println(fullName.Value()) // "John Smith"
```

`Derived` only notifies listeners when the computed value actually changes, so setting `firstName` to the same string twice will not fire listeners a second time.

#### Custom Equality

Like `NewSignal`, `NewDerived` requires a `comparable` type constraint. For non-comparable types (slices, maps), use `NewDerivedWithEquality`:

```go
tags := core.NewDerivedWithEquality(
    func() []string { return buildTagList(source.Value()) },
    slices.Equal,
    source,
)
```

#### Chaining

A `Derived` satisfies `Listenable`, so it can serve as a dependency for another derived value:

```go
doubled := core.NewDerived(func() int { return src.Value() * 2 }, src)
quadrupled := core.NewDerived(func() int { return doubled.Value() * 2 }, doubled)
```

#### Lifecycle

**Every `NewDerived` must be paired with a `Dispose()` call.** A `Derived` subscribes to its dependencies on creation. Without `Dispose()`, it keeps listening indefinitely, recomputing on every change and preventing garbage collection of both itself and its dependencies. Use `defer` for short-lived values, or `UseDerived` inside widgets (which handles disposal automatically).

### UseDerived

Create a `Derived`, subscribe to it for rebuilds, and auto-dispose it when the state is disposed. This combines `NewDerived` + `UseListenable` + `OnDispose` in one call:

```go
func (s *myState) InitState() {
    s.firstName = core.NewSignal("John")
    s.lastName = core.NewSignal("Doe")

    s.fullName = core.UseDerived(s, func() string {
        return s.firstName.Value() + " " + s.lastName.Value()
    }, s.firstName, s.lastName)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: s.fullName.Value()}
}
```

For non-comparable derived types (slices, maps), use `UseDerivedWithEquality`:

```go
s.tags = core.UseDerivedWithEquality(s, func() []string {
    return buildTagList(s.source.Value())
}, slices.Equal, s.source)
```

### UseSelector

Subscribe to any `Listenable` but only trigger rebuilds when a *selected portion* of state changes. The selector closure reads the current value and extracts the part you care about:

```go
func (s *myState) InitState() {
    // Only rebuilds when user.Name changes, ignoring other field updates
    core.UseSelector(s, s.user, func() string {
        return s.user.Value().Name
    })
}
```

For non-comparable selected types (slices, maps), use `UseSelectorWithEquality`:

```go
core.UseSelectorWithEquality(s, s.store, func() []string {
    return s.store.Value().Tags
}, slices.Equal)
```

### UseDerived vs UseSelector

Both prevent unnecessary rebuilds, but they serve different purposes:

- **UseDerived**: Creates a new reactive node with its own value and listeners. Other widgets (or other `Derived` values) can depend on it. Use it when the computed value is reused or shared.
- **UseSelector**: Widget-local optimization with no new reactive node. It filters an existing `Listenable`'s notifications so the widget only rebuilds when the selected portion changes. Use it when a single widget depends on one field of a large signal.

Rule of thumb: if you need the computed value as a `Listenable` dependency elsewhere, use `UseDerived`. If you just want to skip rebuilds for one widget, use `UseSelector`.

## Resource Hooks

These hooks manage resource cleanup tied to the widget lifecycle. Call them once in `InitState()`, not in `Build()`.

### UseDisposable

Register any `Disposable` resource for automatic cleanup when the widget is removed from the tree:

```go
func (s *myState) InitState() {
    s.animation = animation.NewAnimationController(300 * time.Millisecond)
    core.UseDisposable(s, s.animation)
    core.UseListenable(s, s.animation) // Rebuild on each animation tick
}
```

For subscribe/unsubscribe patterns, use `OnDispose` directly:

```go
func (s *myState) InitState() {
    s.OnDispose(dataStream.Subscribe(s.onData))
}
```

## State Lifecycle

Stateful widgets have lifecycle methods:

```go
type myState struct {
    core.StateBase
    subscription func()
}

// Called once when state is first created
func (s *myState) InitState() {
    s.subscription = dataService.Subscribe(s.onDataChange)
}

// Called when the widget configuration changes
func (s *myState) DidUpdateWidget(oldWidget core.StatefulWidget) {
    old := oldWidget.(MyWidget)
    new := s.Element().Widget().(MyWidget)
    if old.ID != new.ID {
        s.reloadData()
    }
}

// Called when InheritedWidget dependencies change
func (s *myState) DidChangeDependencies() {
    // React to inherited widget changes (e.g. theme, locale)
}

// Called when the state is removed from the tree
func (s *myState) Dispose() {
    if s.subscription != nil {
        s.subscription() // Unsubscribe
    }
}
```

### Lifecycle Order

1. `InitState()` - Called once when state is created
2. `DidChangeDependencies()` - Called after `InitState` and whenever dependencies change
3. `Build()` - Called to build the widget tree
4. `DidUpdateWidget()` - Called when parent rebuilds with new widget configuration
5. `Dispose()` - Called when state is removed from tree

To respond to app-level lifecycle events (pause, resume, detach), see [`UseLifecycleObserver`](/docs/guides/platform#widget-level-lifecycle-observation) in the Platform guide.

## Best Practices

### 1. Keep State Local

Only lift state up when multiple widgets need it:

```go
// Good - local state
type toggleState struct {
    core.StateBase
    isOn bool
}

// Only lift up when needed
type parentState struct {
    core.StateBase
    sharedValue string  // Multiple children need this
}
```

### 2. Use Hooks for Resources

`UseDisposable` and `UseListenable` ensure proper cleanup:

```go
// Good - automatic cleanup
s.controller = NewController()
core.UseDisposable(s, s.controller)

// Manual cleanup required
s.controller = NewController()
// Must remember to call s.controller.Dispose() in Dispose()
```

### 3. Dispatch from Goroutines

Always use `drift.Dispatch` for background work:

```go
go func() {
    result := expensiveOperation()
    drift.Dispatch(func() {
        s.SetState(func() {
            s.result = result
        })
    })
}()
```

### 4. Minimize Rebuilds

Only call `SetState` when state actually changes:

```go
// Good - check before setting
func (s *myState) updateValue(newValue int) {
    if s.value != newValue {
        s.SetState(func() {
            s.value = newValue
        })
    }
}

// Bad - unnecessary rebuilds
func (s *myState) updateValue(newValue int) {
    s.SetState(func() {
        s.value = newValue  // Rebuilds even if value is the same
    })
}
```

### 5. Batch State Updates

Combine multiple changes in a single `SetState`:

```go
// Good - single rebuild
s.SetState(func() {
    s.name = newName
    s.email = newEmail
    s.isValid = true
})

// Bad - three rebuilds
s.SetState(func() { s.name = newName })
s.SetState(func() { s.email = newEmail })
s.SetState(func() { s.isValid = true })
```

## Quick Reference

### State Patterns

| Tool | Thread-safe | Use case |
|------|:-----------:|----------|
| `SetState` | No | Local widget state mutations |
| `InheritedProvider[T]` | - | Share data down the widget tree |
| `Signal[T]` | Yes | Reactive value with equality-based notification |
| `Derived[T]` | Yes | Computed value that tracks source signals |
| `Notifier` | Yes | Embed in custom state holders for listener management |

### Convenience Widgets

| Widget | Use case |
|--------|----------|
| `ListenableBuilder` | Rebuild a subtree when a single `Listenable` changes, without a full StatefulWidget |

### Hooks

| Hook | Use case |
|------|----------|
| `UseListenable` | Subscribe to any `Listenable` for rebuilds (Signal, Derived, Notifier, etc.) |
| `UseDerived` / `UseDerivedWithEquality` | Create, subscribe, and auto-dispose a Derived |
| `UseSelector` / `UseSelectorWithEquality` | Subscribe but only rebuild when a selected portion changes |
| `UseDisposable` | Register a Disposable resource for automatic cleanup |

## Next Steps

- [Layout](/docs/guides/layout) - Arranging widgets
- [Theming](/docs/guides/theming) - Theming your app
- [Widget Architecture](/docs/guides/widgets) - Keys, GlobalKey, and widget types
- [API Reference](/docs/api/core) - Core API documentation
