package core

import (
	"sync"
	"testing"
)

func TestNewDerived_InitialValue(t *testing.T) {
	src := NewSignal(10)
	d := NewDerived(func() int { return src.Value() * 2 }, src)
	defer d.Dispose()

	if d.Value() != 20 {
		t.Errorf("expected 20, got %d", d.Value())
	}
}

func TestNewDerived_RecomputesOnChange(t *testing.T) {
	src := NewSignal(5)
	d := NewDerived(func() int { return src.Value() + 1 }, src)
	defer d.Dispose()

	src.Set(10)

	if d.Value() != 11 {
		t.Errorf("expected 11, got %d", d.Value())
	}
}

func TestNewDerived_MultipleDeps(t *testing.T) {
	a := NewSignal("Hello")
	b := NewSignal("World")
	d := NewDerived(func() string { return a.Value() + " " + b.Value() }, a, b)
	defer d.Dispose()

	if d.Value() != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", d.Value())
	}

	b.Set("Go")
	if d.Value() != "Hello Go" {
		t.Errorf("expected 'Hello Go', got %q", d.Value())
	}

	a.Set("Hi")
	if d.Value() != "Hi Go" {
		t.Errorf("expected 'Hi Go', got %q", d.Value())
	}
}

func TestNewDerived_SkipsNotificationOnEqualValue(t *testing.T) {
	src := NewSignal(3)
	d := NewDerived(func() int { return src.Value() / 2 }, src) // integer division
	defer d.Dispose()

	notified := 0
	d.AddListener(func() { notified++ })

	// 3/2 = 1, 4/2 = 2 (different, should notify)
	src.Set(4)
	if notified != 1 {
		t.Errorf("expected 1 notification, got %d", notified)
	}

	// 4/2 = 2, 5/2 = 2 (same, should skip)
	src.Set(5)
	if notified != 1 {
		t.Errorf("expected 1 notification (unchanged), got %d", notified)
	}
}

func TestNewDerivedWithEquality_CustomEqual(t *testing.T) {
	type pair struct{ x, y int }
	src := NewSignal(pair{1, 2})

	// Only care about x
	d := NewDerivedWithEquality(
		func() pair { return src.Value() },
		func(a, b pair) bool { return a.x == b.x },
		src,
	)
	defer d.Dispose()

	notified := 0
	d.AddListener(func() { notified++ })

	// Change y only: should not notify
	src.Set(pair{1, 99})
	if notified != 0 {
		t.Errorf("expected 0 notifications (x unchanged), got %d", notified)
	}

	// Change x: should notify
	src.Set(pair{2, 99})
	if notified != 1 {
		t.Errorf("expected 1 notification (x changed), got %d", notified)
	}
}

func TestNewDerived_AddListenerAndUnsubscribe(t *testing.T) {
	src := NewSignal(0)
	d := NewDerived(func() int { return src.Value() }, src)
	defer d.Dispose()

	count := 0
	unsub := d.AddListener(func() { count++ })

	src.Set(1)
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	unsub()
	src.Set(2)
	if count != 1 {
		t.Errorf("expected 1 after unsub, got %d", count)
	}
}

func TestNewDerived_Dispose(t *testing.T) {
	src := NewSignal(0)
	d := NewDerived(func() int { return src.Value() }, src)

	if src.ListenerCount() != 1 {
		t.Errorf("expected 1 listener on src, got %d", src.ListenerCount())
	}

	d.Dispose()

	if src.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners after dispose, got %d", src.ListenerCount())
	}

	// Setting src after dispose should not panic
	src.Set(99)
}

func TestNewDerived_Chained(t *testing.T) {
	src := NewSignal(2)
	doubled := NewDerived(func() int { return src.Value() * 2 }, src)
	quadrupled := NewDerived(func() int { return doubled.Value() * 2 }, doubled)
	defer doubled.Dispose()
	defer quadrupled.Dispose()

	if quadrupled.Value() != 8 {
		t.Errorf("expected 8, got %d", quadrupled.Value())
	}

	src.Set(3)
	if doubled.Value() != 6 {
		t.Errorf("expected doubled 6, got %d", doubled.Value())
	}
	if quadrupled.Value() != 12 {
		t.Errorf("expected quadrupled 12, got %d", quadrupled.Value())
	}
}

func TestNewDerived_ConcurrentAccess(t *testing.T) {
	src := NewSignal(0)
	d := NewDerived(func() int { return src.Value() }, src)
	defer d.Dispose()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			src.Set(val)
			_ = d.Value()
		}(i)
	}
	wg.Wait()
}

func TestNewDerived_ListenerCount(t *testing.T) {
	src := NewSignal(0)
	d := NewDerived(func() int { return src.Value() }, src)
	defer d.Dispose()

	if d.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners initially, got %d", d.ListenerCount())
	}

	unsub1 := d.AddListener(func() {})
	unsub2 := d.AddListener(func() {})

	if d.ListenerCount() != 2 {
		t.Errorf("expected 2 listeners, got %d", d.ListenerCount())
	}

	unsub1()
	if d.ListenerCount() != 1 {
		t.Errorf("expected 1 listener after unsub, got %d", d.ListenerCount())
	}

	unsub2()
	if d.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners after both unsub, got %d", d.ListenerCount())
	}
}

func TestSignal_ImplementsListenable(t *testing.T) {
	sig := NewSignal(0)
	var _ Listenable = sig
}

func TestDerived_ImplementsListenable(t *testing.T) {
	src := NewSignal(0)
	d := NewDerived(func() int { return src.Value() }, src)
	defer d.Dispose()
	var _ Listenable = d
}

func TestUseListenable_WithDerived(t *testing.T) {
	base := &StateBase{}
	src := NewSignal(0)
	d := NewDerived(func() int { return src.Value() * 2 }, src)

	UseListenable(base, d)

	if d.ListenerCount() != 1 {
		t.Errorf("expected 1 listener on derived, got %d", d.ListenerCount())
	}

	base.Dispose()

	if d.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners after dispose, got %d", d.ListenerCount())
	}

	d.Dispose()
}

func TestUseDerived(t *testing.T) {
	base := &StateBase{}
	a := NewSignal(3)
	b := NewSignal(4)

	d := UseDerived(base, func() int {
		return a.Value() + b.Value()
	}, a, b)

	if d.Value() != 7 {
		t.Errorf("expected 7, got %d", d.Value())
	}

	a.Set(10)
	if d.Value() != 14 {
		t.Errorf("expected 14, got %d", d.Value())
	}

	// Dispose should clean up both the derived subscription and its dep subscriptions
	base.Dispose()

	if a.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners on a after dispose, got %d", a.ListenerCount())
	}
	if b.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners on b after dispose, got %d", b.ListenerCount())
	}
}
