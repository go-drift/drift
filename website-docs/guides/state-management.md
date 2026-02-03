---
id: state-management
title: State Management
sidebar_position: 6
---

# State Management

Drift provides several patterns for managing state in your application, from simple local state to app-wide shared state.

## The SetState Pattern

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

### Example: Counter

```go
type counterState struct {
    core.StateBase
    count int
}

func (s *counterState) InitState() {
    s.count = 0
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

## Thread Safety

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

### Common Pattern: Async Loading

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

## ManagedState

`ManagedState` holds a value and triggers rebuilds automatically when changed:

```go
type myState struct {
    core.StateBase
    count *core.ManagedState[int]
    name  *core.ManagedState[string]
}

func (s *myState) InitState() {
    s.count = core.NewManagedState(&s.StateBase, 0)
    s.name = core.NewManagedState(&s.StateBase, "")
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: fmt.Sprintf("Count: %d", s.count.Get())},
            theme.ButtonOf(ctx, "Increment", func() {
                s.count.Set(s.count.Get() + 1) // Automatically triggers rebuild
            }),
        },
    }
}
```

Like `SetState`, `ManagedState` is not thread-safe. Use `drift.Dispatch` for background updates.

## Observable

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

### Observable in State

```go
type myState struct {
    core.StateBase
    counter *core.Observable[int]
}

func (s *myState) InitState() {
    s.counter = core.NewObservable(0)
    // UseObservable subscribes and triggers rebuilds on change
    core.UseObservable(&s.StateBase, s.counter)
}

func (s *myState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Text{Content: fmt.Sprintf("Count: %d", s.counter.Value())}
}
```

## Hooks

Hooks help manage subscriptions and controllers with automatic cleanup when the state is disposed.

### UseObservable

Subscribe to an `Observable` and trigger rebuilds on change:

```go
func (s *myState) InitState() {
    s.counter = core.NewObservable(0)
    core.UseObservable(&s.StateBase, s.counter)
}
```

### UseListenable

Subscribe to any `Listenable` (animation controllers, custom notifiers):

```go
func (s *myState) InitState() {
    s.animation = animation.NewAnimationController(300 * time.Millisecond)
    core.UseListenable(&s.StateBase, s.animation)
}
```

### UseController

Create a controller with automatic disposal:

```go
func (s *myState) InitState() {
    // Controller is automatically disposed when state is disposed
    s.animation = core.UseController(&s.StateBase, func() *animation.AnimationController {
        return animation.NewAnimationController(300 * time.Millisecond)
    })
}
```

## InheritedWidget

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

For advanced use cases like aspect-based dependency tracking, implement a custom `InheritedWidget`:

```go
// Define the inherited widget
type UserProvider struct {
    User        *User
    Child       core.Widget
}

func (u UserProvider) CreateElement() core.Element {
    return core.NewInheritedElement(u, nil)
}

func (u UserProvider) Key() any { return nil }

func (u UserProvider) ChildWidget() core.Widget { return u.Child }

// UpdateShouldNotify is called when this widget updates. Return true to
// rebuild all dependents, false to skip rebuilding entirely.
func (u UserProvider) UpdateShouldNotify(old core.InheritedWidget) bool {
    if prev, ok := old.(UserProvider); ok {
        return u.User != prev.User
    }
    return true
}

// UpdateShouldNotifyDependent enables fine-grained control per dependent.
// The aspects map contains the aspects each dependent registered interest in.
// For simple cases, just delegate to UpdateShouldNotify.
func (u UserProvider) UpdateShouldNotifyDependent(old core.InheritedWidget, _ map[any]struct{}) bool {
    return u.UpdateShouldNotify(old)
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
s.controller = core.UseController(&s.StateBase, func() *Controller {
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

## Next Steps

- [Layout](/docs/guides/layout) - Arranging widgets
- [Theming](/docs/guides/theming) - Theming your app
- [API Reference](/docs/api/core) - Core API documentation
