package core

import (
	"sync"
	"testing"
)

func TestDerive_InitialValue(t *testing.T) {
	src := NewObservable(10)
	d := Derive(func() int { return src.Value() * 2 }, src)
	defer d.Dispose()

	if d.Value() != 20 {
		t.Errorf("expected 20, got %d", d.Value())
	}
}

func TestDerive_RecomputesOnChange(t *testing.T) {
	src := NewObservable(5)
	d := Derive(func() int { return src.Value() + 1 }, src)
	defer d.Dispose()

	src.Set(10)

	if d.Value() != 11 {
		t.Errorf("expected 11, got %d", d.Value())
	}
}

func TestDerive_MultipleDeps(t *testing.T) {
	a := NewObservable("Hello")
	b := NewObservable("World")
	d := Derive(func() string { return a.Value() + " " + b.Value() }, a, b)
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

func TestDerive_SkipsNotificationOnEqualValue(t *testing.T) {
	src := NewObservable(3)
	d := Derive(func() int { return src.Value() / 2 }, src) // integer division
	defer d.Dispose()

	notified := 0
	d.AddListener(func(int) { notified++ })

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

func TestDeriveWithEquality_CustomEqual(t *testing.T) {
	type pair struct{ x, y int }
	src := NewObservable(pair{1, 2})

	// Only care about x
	d := DeriveWithEquality(
		func() pair { return src.Value() },
		func(a, b pair) bool { return a.x == b.x },
		src,
	)
	defer d.Dispose()

	notified := 0
	d.AddListener(func(pair) { notified++ })

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

func TestDerive_AddListenerAndUnsubscribe(t *testing.T) {
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() }, src)
	defer d.Dispose()

	count := 0
	unsub := d.AddListener(func(int) { count++ })

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

func TestDerive_Dispose(t *testing.T) {
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() }, src)

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

func TestDerive_Chained(t *testing.T) {
	src := NewObservable(2)
	doubled := Derive(func() int { return src.Value() * 2 }, src)
	quadrupled := Derive(func() int { return doubled.Value() * 2 }, doubled)
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

func TestDerive_ConcurrentAccess(t *testing.T) {
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() }, src)
	defer d.Dispose()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			src.Set(val)
			_ = d.Value()
		}(i)
	}
	wg.Wait()
}

func TestDerive_ListenerCount(t *testing.T) {
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() }, src)
	defer d.Dispose()

	if d.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners initially, got %d", d.ListenerCount())
	}

	unsub1 := d.AddListener(func(int) {})
	unsub2 := d.AddListener(func(int) {})

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

func TestDerive_OnChange(t *testing.T) {
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() }, src)
	defer d.Dispose()

	called := 0
	unsub := d.OnChange(func() { called++ })
	defer unsub()

	src.Set(1)
	if called != 1 {
		t.Errorf("expected OnChange called 1 time, got %d", called)
	}
}

func TestObservable_OnChange(t *testing.T) {
	obs := NewObservable(0)
	called := 0
	unsub := obs.OnChange(func() { called++ })

	obs.Set(1)
	if called != 1 {
		t.Errorf("expected 1, got %d", called)
	}

	unsub()
	obs.Set(2)
	if called != 1 {
		t.Errorf("expected 1 after unsub, got %d", called)
	}
}

func TestObservable_ImplementsSubscribable(t *testing.T) {
	obs := NewObservable(0)
	var _ Subscribable = obs
}

func TestDerivedObservable_ImplementsSubscribable(t *testing.T) {
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() }, src)
	defer d.Dispose()
	var _ Subscribable = d
}

func TestUseObservable_WithDerived(t *testing.T) {
	base := &StateBase{}
	src := NewObservable(0)
	d := Derive(func() int { return src.Value() * 2 }, src)

	UseObservable(base, d)

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
	a := NewObservable(3)
	b := NewObservable(4)

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
