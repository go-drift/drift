---
id: state-management
title: State Management
sidebar_position: 3
---

# State Management

Drift provides several patterns for managing state, organized here from simplest to most advanced. Most apps only need the first two sections.

For how to define stateful and stateless widget types, see [Widget Architecture](/docs/guides/widgets#stateful-widgets).

## SetState and Managed

These two tools cover the vast majority of state management needs. `SetState` handles direct mutations; `Managed` wraps a single value with automatic rebuilds.

### The SetState Pattern

The most fundamental pattern is `SetState`. Always mutate state inside a `SetState` call to trigger a rebuild:

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

### Managed

`Managed` holds a value and triggers rebuilds automatically when changed:

```go
type myState struct {
    core.StateBase
    count *core.Managed[int]
    name  *core.Managed[string]
}

func (s *myState) InitState() {
    s.count = core.NewManaged(s, 0)
    s.name = core.NewManaged(s, "")
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: fmt.Sprintf("Count: %d", s.count.Value())},
            theme.ButtonOf(ctx, "Increment", func() {
                s.count.Set(s.count.Value() + 1) // Automatically triggers rebuild
            }),
        },
    }
}
```

Like `SetState`, `Managed` is not thread-safe. Use `drift.Dispatch` for background updates.

:::tip You can stop here
`SetState` and `Managed` are all you need for most applications. The sections below cover sharing state across widgets, working with controllers, and reactive patterns for more complex apps. Skip ahead to [Best Practices](#best-practices) if these two tools are sufficient for your needs.
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
    user, ok := core.ProviderOf[*User](ctx)
    if !ok {
        return widgets.Text{Content: "Not logged in"}
    }
    return widgets.Text{Content: "Hello, " + user.Name}
}

// Or use MustProviderOf when you're certain the provider exists
func (s *profileState) Build(ctx core.BuildContext) core.Widget {
    user := core.MustProviderOf[*User](ctx) // panics if not found
    return widgets.Text{Content: "Hello, " + user.Name}
}
```

By default, dependents rebuild when the value changes (pointer equality for pointers, value equality for value types).

### Custom Comparison

Use `ShouldNotify` when you need custom comparison logic:

```go
core.InheritedProvider[*User]{
    Value:       currentUser,
    Child: MainContent{},
    ShouldNotify: func(old, new *User) bool {
        // Only rebuild when ID changes, ignore name updates
        return old.ID != new.ID
    },
}
```

### Custom InheritedWidget

For advanced use cases, implement a custom `InheritedWidget`. Embed `core.InheritedBase`
to get `CreateElement` and `Key` for free, then implement `ChildWidget` and
`UpdateShouldNotify`:

```go
type UserProvider struct {
    core.InheritedBase
    User  *User
    Child core.Widget
}

func (u UserProvider) ChildWidget() core.Widget { return u.Child }

// UpdateShouldNotify is called when this widget updates. Return true to
// rebuild all dependents, false to skip rebuilding entirely.
func (u UserProvider) UpdateShouldNotify(old core.InheritedWidget) bool {
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

## Controllers and Listeners

When working with animation controllers or other resources that need lifecycle management, use `UseController` and `UseListenable`. These hooks handle subscription and cleanup automatically. Call them once in `InitState()`, not in `Build()`.

### UseController

Create a controller with automatic disposal:

```go
func (s *myState) InitState() {
    // Controller is automatically disposed when state is disposed
    s.animation = core.UseController(s, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })
}
```

### UseListenable

Subscribe to any `Listenable` (animation controllers, custom notifiers) and trigger rebuilds on notification:

```go
func (s *myState) InitState() {
    s.animation = animation.NewAnimationController(300 * time.Millisecond)
    core.UseListenable(s, s.animation)
}
```

These two hooks are often used together:

```go
func (s *myState) InitState() {
    s.animation = core.UseController(s, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })
    core.UseListenable(s, s.animation) // Rebuild on each animation tick
}
```

:::tip You can stop here
`SetState`, `Managed`, `InheritedProvider`, `UseController`, and `UseListenable` cover the needs of most applications. The next section covers reactive state patterns for apps that need cross-widget reactive state, computed values, or fine-grained subscription control.
:::

## Reactive State

For apps that need thread-safe, shareable state with listener support and computed values, Drift provides `Observable`, `DerivedObservable`, and associated hooks.

### Observable

`Observable` is a thread-safe reactive value with listener support:

```go
// Create an observable
counter := core.NewObservable(0)

// Add a listener
unsub := counter.AddListener(func(value int) {
    fmt.Println("Count changed to:", value)
})

// Update value (notifies all listeners)
counter.Set(5)

// Read value
current := counter.Value()

// Unsubscribe when done
unsub()
```

#### Observable in State

```go
type myState struct {
    core.StateBase
    counter *core.Observable[int]
}

func (s *myState) InitState() {
    s.counter = core.NewObservable(0)
    // UseObservable subscribes and triggers rebuilds on change
    core.UseObservable(s, s.counter)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: fmt.Sprintf("Count: %d", s.counter.Value())}
}
```

### Derived Observable

`DerivedObservable` is a read-only observable that recomputes its value automatically when any of its dependencies change. Use it when you have a value that is always a function of one or more other observables.

```go
type myState struct {
    core.StateBase
    firstName *core.Observable[string]
    lastName  *core.Observable[string]
    fullName  *core.DerivedObservable[string]
}

func (s *myState) InitState() {
    s.firstName = core.NewObservable("John")
    s.lastName = core.NewObservable("Doe")

    // fullName recomputes whenever firstName or lastName changes
    s.fullName = core.Derive(func() string {
        return s.firstName.Value() + " " + s.lastName.Value()
    }, s.firstName, s.lastName)
}
```

`DerivedObservable` only notifies listeners when the computed value actually changes, so setting `firstName` to the same string twice will not fire listeners a second time.

#### Custom Equality

By default, values are compared with Go's `==` operator. For non-comparable types (slices, maps), use `DeriveWithEquality`:

```go
tags := core.DeriveWithEquality(
    func() []string { return buildTagList(source.Value()) },
    slices.Equal,
    source,
)
```

#### Chaining

A `DerivedObservable` satisfies `Subscribable`, so it can serve as a dependency for another derived value:

```go
doubled := core.Derive(func() int { return src.Value() * 2 }, src)
quadrupled := core.Derive(func() int { return doubled.Value() * 2 }, doubled)
```

#### Lifecycle

A `DerivedObservable` subscribes to its dependencies on creation. Call `Dispose()` to unsubscribe when you no longer need it. Inside a stateful widget, prefer `UseDerived` (below) which handles disposal automatically.

### Reactive Hooks

These hooks connect observables to the widget rebuild cycle. Like `UseController` and `UseListenable`, call them once in `InitState()`.

#### UseObservable

Subscribe to an `Observable` (or `DerivedObservable`) and trigger rebuilds on change:

```go
func (s *myState) InitState() {
    s.counter = core.NewObservable(0)
    core.UseObservable(s, s.counter)
}
```

#### UseDerived

Create a `DerivedObservable`, subscribe to it for rebuilds, and auto-dispose it when the state is disposed. This combines `Derive` + `UseObservable` + `OnDispose` in one call:

```go
func (s *myState) InitState() {
    s.firstName = core.NewObservable("John")
    s.lastName = core.NewObservable("Doe")

    s.fullName = core.UseDerived(s, func() string {
        return s.firstName.Value() + " " + s.lastName.Value()
    }, s.firstName, s.lastName)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: s.fullName.Value()}
}
```

#### UseObservableSelector

Subscribe to an observable but only trigger rebuilds when a *selected portion* of the value changes. This is useful when the observable holds a large struct but the widget only cares about one field:

```go
func (s *myState) InitState() {
    // Only rebuilds when user.Name changes, ignoring other field updates
    core.UseObservableSelector(s, s.user, func(u User) string {
        return u.Name
    })
}
```

For non-comparable selected types, use `UseObservableSelectorWithEquality`:

```go
core.UseObservableSelectorWithEquality(s, s.store, func(st Store) []string {
    return st.Tags
}, slices.Equal)
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
    new := s.Widget().(MyWidget)
    if old.ID != new.ID {
        s.reloadData()
    }
}

// Called when InheritedWidget dependencies change
func (s *myState) DidChangeDependencies() {
    theme := theme.ThemeOf(s.Context())
    // React to theme changes
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

`UseController` and `UseListenable` ensure proper cleanup:

```go
// Good - automatic cleanup
s.controller = core.UseController(s, func() *Controller {
    return NewController()
})

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

### Core (most apps)

| Tool | Thread-safe | Use case |
|------|:-----------:|----------|
| `SetState` | No | Simple local mutations |
| `Managed[T]` | No | Single value with automatic rebuild |
| `InheritedProvider[T]` | - | Share data down the widget tree |

### Controllers and listeners

| Tool | Use case |
|------|----------|
| `UseController` | Create a controller with automatic disposal |
| `UseListenable` | Subscribe to any Listenable for rebuilds |
| `UseSubscription` | Auto-cleanup any subscribe/unsubscribe pair |

### Reactive state

| Tool | Thread-safe | Use case |
|------|:-----------:|----------|
| `Observable[T]` | Yes | Shared reactive value with listener support |
| `DerivedObservable[T]` | Yes | Computed value that tracks source observables |
| `UseObservable` | - | Subscribe to Observable or DerivedObservable for rebuilds |
| `UseDerived` | - | Create, subscribe, and auto-dispose a DerivedObservable |
| `UseObservableSelector` | - | Subscribe but only rebuild when a selected slice changes |

## Next Steps

- [Layout](/docs/guides/layout) - Arranging widgets
- [Theming](/docs/guides/theming) - Theming your app
- [Widget Architecture](/docs/guides/widgets) - Keys, GlobalKey, and widget types
- [API Reference](/docs/api/core) - Core API documentation
