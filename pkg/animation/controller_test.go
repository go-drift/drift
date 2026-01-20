package animation

import (
	"testing"
	"time"
)

func TestAnimationController_AddListener(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)
	defer c.Dispose()

	callCount := 0
	c.AddListener(func() {
		callCount++
	})

	// Manually trigger notification (simulating tick)
	c.notifyListeners()

	if callCount != 1 {
		t.Errorf("Expected listener to be called once, called %d times", callCount)
	}
}

func TestAnimationController_AddListener_Unsubscribe(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)
	defer c.Dispose()

	callCount := 0
	unsub := c.AddListener(func() {
		callCount++
	})

	c.notifyListeners()
	if callCount != 1 {
		t.Errorf("Expected 1 call before unsubscribe, got %d", callCount)
	}

	unsub()
	c.notifyListeners()

	if callCount != 1 {
		t.Errorf("Expected no more calls after unsubscribe, got %d", callCount)
	}
}

func TestAnimationController_MultipleListeners(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)
	defer c.Dispose()

	var count1, count2 int
	c.AddListener(func() { count1++ })
	c.AddListener(func() { count2++ })

	c.notifyListeners()

	if count1 != 1 || count2 != 1 {
		t.Errorf("Both listeners should be called, got %d and %d", count1, count2)
	}
}

func TestAnimationController_ListenerRemovalOrder(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)
	defer c.Dispose()

	var count1, count2, count3 int
	unsub1 := c.AddListener(func() { count1++ })
	unsub2 := c.AddListener(func() { count2++ })
	unsub3 := c.AddListener(func() { count3++ })

	c.notifyListeners()

	// Remove middle listener
	unsub2()
	c.notifyListeners()

	if count1 != 2 || count2 != 1 || count3 != 2 {
		t.Errorf("Expected [2,1,2], got [%d,%d,%d]", count1, count2, count3)
	}

	// Remove first listener
	unsub1()
	c.notifyListeners()

	if count1 != 2 || count2 != 1 || count3 != 3 {
		t.Errorf("Expected [2,1,3], got [%d,%d,%d]", count1, count2, count3)
	}

	// Remove last listener
	unsub3()
	c.notifyListeners()

	if count1 != 2 || count2 != 1 || count3 != 3 {
		t.Errorf("Expected [2,1,3], got [%d,%d,%d]", count1, count2, count3)
	}
}

func TestAnimationController_AddStatusListener(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)
	defer c.Dispose()

	var received AnimationStatus
	c.AddStatusListener(func(status AnimationStatus) {
		received = status
	})

	c.setStatus(AnimationForward)

	if received != AnimationForward {
		t.Errorf("Expected AnimationForward, got %v", received)
	}
}

func TestAnimationController_AddStatusListener_Unsubscribe(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)
	defer c.Dispose()

	callCount := 0
	unsub := c.AddStatusListener(func(status AnimationStatus) {
		callCount++
	})

	c.setStatus(AnimationForward)
	if callCount != 1 {
		t.Errorf("Expected 1 call before unsubscribe, got %d", callCount)
	}

	unsub()
	c.setStatus(AnimationCompleted)

	if callCount != 1 {
		t.Errorf("Expected no more calls after unsubscribe, got %d", callCount)
	}
}

func TestAnimationController_Dispose_ClearsListeners(t *testing.T) {
	c := NewAnimationController(100 * time.Millisecond)

	callCount := 0
	c.AddListener(func() { callCount++ })
	c.AddStatusListener(func(AnimationStatus) { callCount++ })

	c.notifyListeners()
	c.setStatus(AnimationForward)

	if callCount != 2 {
		t.Errorf("Expected 2 calls before dispose, got %d", callCount)
	}

	c.Dispose()
	c.notifyListeners()
	// Can't easily test setStatus since maps are nil after dispose

	if callCount != 2 {
		t.Errorf("Expected no more calls after dispose, got %d", callCount)
	}
}

func TestAnimationController_NewControllerInitialization(t *testing.T) {
	c := NewAnimationController(200 * time.Millisecond)
	defer c.Dispose()

	if c.Value != 0 {
		t.Errorf("Expected initial Value 0, got %f", c.Value)
	}
	if c.Duration != 200*time.Millisecond {
		t.Errorf("Expected Duration 200ms, got %v", c.Duration)
	}
	if c.LowerBound != 0 {
		t.Errorf("Expected LowerBound 0, got %f", c.LowerBound)
	}
	if c.UpperBound != 1 {
		t.Errorf("Expected UpperBound 1, got %f", c.UpperBound)
	}
	if c.Status() != AnimationDismissed {
		t.Errorf("Expected AnimationDismissed, got %v", c.Status())
	}
	if c.listeners == nil {
		t.Error("listeners map should be initialized")
	}
	if c.statusListeners == nil {
		t.Error("statusListeners map should be initialized")
	}
}

// Compile-time interface checks
type listenable interface {
	AddListener(listener func()) func()
}

type disposable interface {
	Dispose()
}

var _ listenable = &AnimationController{}
var _ disposable = &AnimationController{}
