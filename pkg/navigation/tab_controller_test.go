package navigation

import (
	"testing"
)

func TestNewTabController(t *testing.T) {
	c := NewTabController(2)
	if c.Index() != 2 {
		t.Errorf("Index() = %d, want 2", c.Index())
	}
}

func TestTabController_Index_Nil(t *testing.T) {
	var c *TabController
	if c.Index() != 0 {
		t.Errorf("nil.Index() = %d, want 0", c.Index())
	}
}

func TestTabController_SetIndex(t *testing.T) {
	c := NewTabController(0)

	c.SetIndex(2)
	if c.Index() != 2 {
		t.Errorf("Index() = %d, want 2", c.Index())
	}

	c.SetIndex(5)
	if c.Index() != 5 {
		t.Errorf("Index() = %d, want 5", c.Index())
	}
}

func TestTabController_SetIndex_Nil(t *testing.T) {
	var c *TabController
	// Should not panic
	c.SetIndex(1)
}

func TestTabController_SetIndex_SameValue(t *testing.T) {
	c := NewTabController(2)

	called := false
	c.AddListener(func(index int) {
		called = true
	})

	c.SetIndex(2) // Same value

	if called {
		t.Error("Listener should not be called when setting same index")
	}
}

func TestTabController_AddListener(t *testing.T) {
	c := NewTabController(0)

	var receivedIndex int
	c.AddListener(func(index int) {
		receivedIndex = index
	})

	c.SetIndex(3)

	if receivedIndex != 3 {
		t.Errorf("Listener received index = %d, want 3", receivedIndex)
	}
}

func TestTabController_AddListener_Multiple(t *testing.T) {
	c := NewTabController(0)

	var count1, count2 int
	c.AddListener(func(index int) { count1++ })
	c.AddListener(func(index int) { count2++ })

	c.SetIndex(1)
	c.SetIndex(2)

	if count1 != 2 || count2 != 2 {
		t.Errorf("Listeners called %d and %d times, want 2 each", count1, count2)
	}
}

func TestTabController_AddListener_Unsubscribe(t *testing.T) {
	c := NewTabController(0)

	callCount := 0
	unsub := c.AddListener(func(index int) {
		callCount++
	})

	c.SetIndex(1)
	unsub()
	c.SetIndex(2)

	if callCount != 1 {
		t.Errorf("Listener called %d times, want 1 (before unsubscribe)", callCount)
	}
}
