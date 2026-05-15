package platform

import (
	"sync"
	"testing"
)

// TestStickyEventChannel_LateSubscriberReceivesReplay confirms a subscriber
// joining after the most recent dispatchEvent observes that event exactly
// once during Listen.
func TestStickyEventChannel_LateSubscriberReceivesReplay(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewStickyEventChannel("drift/test/sticky/late_subscriber")

	ch.dispatchEvent(map[string]any{"type": "first_frame"})

	var received []any
	var mu sync.Mutex
	sub := ch.Listen(EventHandler{OnEvent: func(data any) {
		mu.Lock()
		received = append(received, data)
		mu.Unlock()
	}})
	defer sub.Cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 replayed event, got %d", len(received))
	}
	m, ok := received[0].(map[string]any)
	if !ok || m["type"] != "first_frame" {
		t.Errorf("replay payload wrong: %#v", received[0])
	}
}

// TestStickyEventChannel_MultipleLateSubscribersEachReceiveReplay confirms
// the slot is not consumed by the first replay.
func TestStickyEventChannel_MultipleLateSubscribersEachReceiveReplay(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewStickyEventChannel("drift/test/sticky/multi_late")

	ch.dispatchEvent("payload")

	counts := make(map[int]int)
	var mu sync.Mutex
	for i := range 3 {
		id := i
		ch.Listen(EventHandler{OnEvent: func(data any) {
			if data != "payload" {
				t.Errorf("subscriber %d got wrong payload: %v", id, data)
			}
			mu.Lock()
			counts[id]++
			mu.Unlock()
		}})
	}

	mu.Lock()
	defer mu.Unlock()
	for i := range 3 {
		if counts[i] != 1 {
			t.Errorf("subscriber %d received %d replays, want 1", i, counts[i])
		}
	}
}

// TestStickyEventChannel_LiveEventOverwritesSlot confirms a subsequent
// dispatchEvent overwrites the remembered slot; later subscribers see the
// new payload, not the old one.
func TestStickyEventChannel_LiveEventOverwritesSlot(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewStickyEventChannel("drift/test/sticky/overwrite")

	ch.dispatchEvent("first")
	ch.dispatchEvent("second")

	var got any
	ch.Listen(EventHandler{OnEvent: func(data any) { got = data }})
	if got != "second" {
		t.Errorf("late subscriber got %v, want %q", got, "second")
	}
}

// TestStickyEventChannel_LiveSubscriberSeesReplayAndSubsequent confirms a
// subscriber that joined before any event still gets normal broadcast
// behaviour for subsequent events; sticky is only for late joiners.
func TestStickyEventChannel_LiveSubscriberSeesReplayAndSubsequent(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewStickyEventChannel("drift/test/sticky/live_then_late")

	var early []any
	var earlyMu sync.Mutex
	ch.Listen(EventHandler{OnEvent: func(data any) {
		earlyMu.Lock()
		early = append(early, data)
		earlyMu.Unlock()
	}})

	ch.dispatchEvent("one")
	ch.dispatchEvent("two")

	var late []any
	var lateMu sync.Mutex
	ch.Listen(EventHandler{OnEvent: func(data any) {
		lateMu.Lock()
		late = append(late, data)
		lateMu.Unlock()
	}})

	earlyMu.Lock()
	defer earlyMu.Unlock()
	lateMu.Lock()
	defer lateMu.Unlock()

	if got := early; len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("early subscriber events = %v, want [one two]", got)
	}
	if got := late; len(got) != 1 || got[0] != "two" {
		t.Errorf("late subscriber events = %v, want [two] (only the latest)", got)
	}
}

// TestNonStickyEventChannel_LateSubscriberMissesPriorEvent is the contrast
// test confirming the original non-sticky semantics are unchanged.
func TestNonStickyEventChannel_LateSubscriberMissesPriorEvent(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewEventChannel("drift/test/sticky/non_sticky_control")

	ch.dispatchEvent("missed")

	var received []any
	ch.Listen(EventHandler{OnEvent: func(data any) { received = append(received, data) }})
	if len(received) != 0 {
		t.Errorf("non-sticky channel must not replay; got %v", received)
	}
}
