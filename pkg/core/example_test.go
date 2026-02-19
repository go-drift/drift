package core_test

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
)

// This example shows how to create an Observable for reactive state.
// Observable is thread-safe and can be shared across goroutines.
func ExampleObservable() {
	// Create an observable with an initial value
	counter := core.NewObservable(0)

	// Add a listener that fires when the value changes
	unsub := counter.AddListener(func(value int) {
		fmt.Printf("Counter changed to: %d\n", value)
	})

	// Update the value - this triggers all listeners
	counter.Set(5)

	// Read the current value
	current := counter.Value()
	fmt.Printf("Current value: %d\n", current)

	// Clean up when done
	unsub()

	// Output:
	// Counter changed to: 5
	// Current value: 5
}

// This example shows how to use Observable with a custom equality function.
// This is useful when you want to avoid unnecessary updates.
func ExampleNewObservableWithEquality() {
	type User struct {
		ID   int
		Name string
	}

	// Only notify listeners when the user ID changes
	user := core.NewObservableWithEquality(User{ID: 1, Name: "Alice"}, func(a, b User) bool {
		return a.ID == b.ID
	})

	user.AddListener(func(u User) {
		fmt.Printf("User changed: %s\n", u.Name)
	})

	// This won't trigger listeners because ID is the same
	user.Set(User{ID: 1, Name: "Alice Updated"})

	// This will trigger listeners because ID changed
	user.Set(User{ID: 2, Name: "Bob"})

	// Output:
	// User changed: Bob
}

// This example shows the Notifier type for event broadcasting.
// Unlike Observable, Notifier doesn't hold a value.
func ExampleNotifier() {
	refresh := core.NewNotifier()

	// Add a listener
	unsub := refresh.AddListener(func() {
		fmt.Println("Refresh triggered!")
	})

	// Trigger the notification
	refresh.Notify()

	// Clean up
	unsub()

	// Output:
	// Refresh triggered!
}

// This example shows the StateBase type for stateful widgets.
// Embed StateBase in your state struct to get automatic lifecycle management.
func ExampleStateBase() {
	// In a real stateful widget, you would define:
	//
	// type counterState struct {
	//     core.StateBase
	//     count int
	// }
	//
	// func (s *counterState) InitState() {
	//     s.count = 0
	// }
	//
	// func (s *counterState) Build(ctx core.BuildContext) core.Widget {
	//     return widgets.GestureDetector{
	//         OnTap: func() {
	//             s.SetState(func() {
	//                 s.count++
	//             })
	//         },
	//         Child: widgets.Text{
	//             Content: fmt.Sprintf("Count: %d", s.count),
	//         },
	//     }
	// }

	// StateBase provides SetState, OnDispose, and IsDisposed methods
	state := &core.StateBase{}
	_ = state
}

// This example shows how to use Managed for automatic rebuilds.
// Managed wraps a value and triggers rebuilds when it changes.
func ExampleManaged() {
	// In a stateful widget's InitState:
	//
	// func (s *myState) InitState() {
	//     s.count = core.NewManaged(s, 0)
	// }
	//
	// In Build:
	//
	// func (s *myState) Build(ctx core.BuildContext) core.Widget {
	//     return widgets.GestureDetector{
	//         OnTap: func() {
	//             // Set automatically triggers a rebuild
	//             s.count.Set(s.count.Value() + 1)
	//         },
	//         Child: widgets.Text{
	//             Content: fmt.Sprintf("Count: %d", s.count.Value()),
	//         },
	//     }
	// }

	// Direct usage for demonstration:
	base := &core.StateBase{}
	count := core.NewManaged(base, 0)

	// Get the current value
	fmt.Printf("Initial: %d\n", count.Value())

	// Update using transform function
	count.Update(func(v int) int { return v + 10 })
	fmt.Printf("After update: %d\n", count.Value())

	// Output:
	// Initial: 0
	// After update: 10
}

// This example shows how to create a stateless widget.
func ExampleStatelessWidget() {
	// A stateless widget is a struct that implements StatelessWidget.
	// Embed StatelessBase to get CreateElement and Key for free.
	//
	// type Greeting struct {
	//     core.StatelessBase
	//     Name string
	// }
	//
	// func (g Greeting) Build(ctx core.BuildContext) core.Widget {
	//     return widgets.Text{Content: "Hello, " + g.Name}
	// }
	//
	// Usage:
	//     Greeting{Name: "World"}
}

// This example shows how to create a stateful widget.
func ExampleStatefulWidget() {
	// A stateful widget maintains mutable state across rebuilds.
	// Embed StatefulBase to get CreateElement and Key for free.
	//
	// type Counter struct {
	//     core.StatefulBase
	// }
	//
	// func (Counter) CreateState() core.State {
	//     return &counterState{}
	// }
	//
	// type counterState struct {
	//     core.StateBase
	//     count int
	// }
	//
	// func (s *counterState) Build(ctx core.BuildContext) core.Widget {
	//     return widgets.Button{
	//         Label: fmt.Sprintf("Count: %d", s.count),
	//         OnTap: func() {
	//             s.SetState(func() { s.count++ })
	//         },
	//     }
	// }
}

// This example shows how to create and use an inherited widget.
func ExampleInheritedWidget() {
	// InheritedWidget provides data to descendants without prop drilling.
	// For simple cases, use InheritedProvider instead of implementing directly.
	//
	// Using InheritedProvider (recommended for simple cases):
	//
	//     type UserState struct {
	//         Name  string
	//         Email string
	//     }
	//
	//     // Provide data to descendants
	//     core.InheritedProvider[*UserState]{
	//         Value: &UserState{Name: "Alice", Email: "alice@example.com"},
	//         Child: MyApp{},
	//     }
	//
	//     // Access data in a descendant's Build method
	//     func (w MyWidget) Build(ctx core.BuildContext) core.Widget {
	//         if user, ok := core.ProviderOf[*UserState](ctx); ok {
	//             return widgets.Text{Content: "Hello, " + user.Name}
	//         }
	//         return widgets.Text{Content: "Not logged in"}
	//     }
}

// This example shows how to use UseController for automatic disposal.
func ExampleUseController() {
	// UseController creates a controller and registers it for automatic disposal.
	// Call it in InitState, not Build.
	//
	// func (s *myState) InitState() {
	//     s.animation = core.UseController(s, func() *animation.AnimationController {
	//         return animation.NewAnimationController(300 * time.Millisecond)
	//     })
	//     // No need to manually dispose - it's cleaned up automatically
	// }
}

// This example shows how to use UseListenable for reactive updates.
func ExampleUseListenable() {
	// UseListenable subscribes to a Listenable and triggers rebuilds.
	// The subscription is automatically cleaned up on dispose.
	//
	// func (s *myState) InitState() {
	//     s.controller = core.UseController(s, func() *MyController {
	//         return NewMyController()
	//     })
	//     core.UseListenable(s, s.controller)
	// }
	//
	// func (s *myState) Build(ctx core.BuildContext) core.Widget {
	//     // This rebuilds whenever controller.NotifyListeners() is called
	//     return widgets.Text{Content: s.controller.DisplayValue()}
	// }
}

// This example shows how to use UseObservable for reactive state.
func ExampleUseObservable() {
	// UseObservable subscribes to an Observable and triggers rebuilds.
	// Call it once in InitState, not in Build.
	//
	// func (s *myState) InitState() {
	//     s.counter = core.NewObservable(0)
	//     core.UseObservable(s, s.counter)
	// }
	//
	// func (s *myState) Build(ctx core.BuildContext) core.Widget {
	//     // Use .Value() in Build to read the current value
	//     return widgets.Text{Content: fmt.Sprintf("Count: %d", s.counter.Value())}
	// }
	//
	// // Update from anywhere - triggers rebuild automatically
	// s.counter.Set(s.counter.Value() + 1)
}

// This example shows how to create a custom controller.
func ExampleControllerBase() {
	// Embed ControllerBase to get listener management for free.
	//
	// type ScrollController struct {
	//     core.ControllerBase
	//     offset float64
	// }
	//
	// func NewScrollController() *ScrollController {
	//     return &ScrollController{offset: 0}
	// }
	//
	// func (c *ScrollController) SetOffset(offset float64) {
	//     c.offset = offset
	//     c.NotifyListeners() // Triggers all listeners
	// }
	//
	// func (c *ScrollController) Offset() float64 {
	//     return c.offset
	// }
	//
	// Usage in InitState:
	//     s.scroll = core.UseController(s, NewScrollController)
	//     core.UseListenable(s, s.scroll)

	controller := &core.ControllerBase{}
	unsub := controller.AddListener(func() {
		fmt.Println("Controller notified")
	})
	controller.NotifyListeners()
	unsub()
	controller.Dispose()

	// Output:
	// Controller notified
}

// This example shows how to use Stateful for inline stateful widgets.
func ExampleStateful() {
	// Stateful creates an inline stateful widget using closures.
	// Use it for quick, self-contained UI fragments that don't need
	// lifecycle hooks or StateBase features.
	//
	// widget := core.Stateful(
	//     func() int { return 0 },
	//     func(count int, ctx core.BuildContext, setState func(func(int) int)) core.Widget {
	//         return widgets.GestureDetector{
	//             OnTap: func() {
	//                 setState(func(c int) int { return c + 1 })
	//             },
	//             Child: widgets.Text{Content: fmt.Sprintf("Count: %d", count)},
	//         }
	//     },
	// )
	//
	// The generic parameter [int] is the state type. setState takes a
	// function that transforms the current state to a new state.
	//
	// For complex widgets with lifecycle methods, Managed,
	// or UseController, embed StatefulBase in a named struct instead.
}
