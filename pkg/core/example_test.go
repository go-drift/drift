package core_test

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
)

// This example shows how to create a Signal for reactive state.
// Signal is thread-safe and can be shared across goroutines.
func ExampleSignal() {
	// Create a signal with an initial value
	counter := core.NewSignal(0)

	// Add a listener that fires when the value changes
	unsub := counter.AddListener(func() {
		fmt.Printf("Counter changed to: %d\n", counter.Value())
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

// This example shows how to use Signal with a custom equality function.
// This is useful when you want to avoid unnecessary updates.
func ExampleNewSignalWithEquality() {
	type User struct {
		ID   int
		Name string
	}

	// Only notify listeners when the user ID changes
	user := core.NewSignalWithEquality(User{ID: 1, Name: "Alice"}, func(a, b User) bool {
		return a.ID == b.ID
	})

	user.AddListener(func() {
		fmt.Printf("User changed: %s\n", user.Value().Name)
	})

	// This won't trigger listeners because ID is the same
	user.Set(User{ID: 1, Name: "Alice Updated"})

	// This will trigger listeners because ID changed
	user.Set(User{ID: 2, Name: "Bob"})

	// Output:
	// User changed: Bob
}

// This example shows how to create a custom notifier.
func ExampleNotifier() {
	notifier := &core.Notifier{}
	unsub := notifier.AddListener(func() {
		fmt.Println("Notifier triggered")
	})
	notifier.Notify()
	unsub()
	notifier.Dispose()

	// Output:
	// Notifier triggered
}
