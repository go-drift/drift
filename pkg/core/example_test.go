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
	//         ChildWidget: widgets.Text{
	//             Content: fmt.Sprintf("Count: %d", s.count),
	//         },
	//     }
	// }

	// StateBase provides SetState, OnDispose, and IsDisposed methods
	state := &core.StateBase{}
	_ = state
}

// This example shows how to use ManagedState for automatic rebuilds.
// ManagedState wraps a value and triggers rebuilds when it changes.
func ExampleManagedState() {
	// In a stateful widget's InitState:
	//
	// func (s *myState) InitState() {
	//     s.count = core.NewManagedState(&s.StateBase, 0)
	// }
	//
	// In Build:
	//
	// func (s *myState) Build(ctx core.BuildContext) core.Widget {
	//     return widgets.GestureDetector{
	//         OnTap: func() {
	//             // Set automatically triggers a rebuild
	//             s.count.Set(s.count.Get() + 1)
	//         },
	//         ChildWidget: widgets.Text{
	//             Content: fmt.Sprintf("Count: %d", s.count.Get()),
	//         },
	//     }
	// }

	// Direct usage for demonstration:
	base := &core.StateBase{}
	count := core.NewManagedState(base, 0)

	// Get the current value
	fmt.Printf("Initial: %d\n", count.Get())

	// Update using transform function
	count.Update(func(v int) int { return v + 10 })
	fmt.Printf("After update: %d\n", count.Value())

	// Output:
	// Initial: 0
	// After update: 10
}
