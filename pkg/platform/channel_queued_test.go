package platform

import (
	"slices"
	"sync"
	"testing"

	"github.com/go-drift/drift/pkg/errors"
)

// recorder collects the events one subscriber receives.
type recorder struct {
	mu   sync.Mutex
	got  []any
	hook func(data any)
}

func (r *recorder) handler() EventHandler {
	return EventHandler{OnEvent: func(data any) {
		r.mu.Lock()
		r.got = append(r.got, data)
		r.mu.Unlock()
		if r.hook != nil {
			r.hook(data)
		}
	}}
}

func (r *recorder) events() []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.got)
}

func assertEvents(t *testing.T, who string, got []any, want ...any) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s received %v, want %v", who, got, want)
	}
}

// TestQueuedEventChannel_DeliversBacklogInOrderOnce confirms events
// dispatched with no subscriber reach the first subscriber, in order, before
// Listen returns, and are not delivered again to a later subscriber.
func TestQueuedEventChannel_DeliversBacklogInOrderOnce(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewQueuedEventChannel("drift/test/queued/backlog", 8)

	ch.dispatchEvent(1)
	ch.dispatchEvent(2)

	var first recorder
	sub := ch.Listen(first.handler())
	defer sub.Cancel()
	assertEvents(t, "first subscriber", first.events(), 1, 2)

	ch.dispatchEvent(3)
	var second recorder
	sub2 := ch.Listen(second.handler())
	defer sub2.Cancel()
	ch.dispatchEvent(4)

	assertEvents(t, "first subscriber", first.events(), 1, 2, 3, 4)
	assertEvents(t, "second subscriber", second.events(), 4)
}

// TestQueuedEventChannel_QueuesAgainWhenSubscribersLeave confirms the queue
// applies whenever the channel has no subscriber, not only before the first.
func TestQueuedEventChannel_QueuesAgainWhenSubscribersLeave(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewQueuedEventChannel("drift/test/queued/requeue", 8)

	var first recorder
	ch.Listen(first.handler()).Cancel()
	ch.dispatchEvent("while away")

	var second recorder
	sub := ch.Listen(second.handler())
	defer sub.Cancel()

	assertEvents(t, "first subscriber", first.events())
	assertEvents(t, "second subscriber", second.events(), "while away")
}

// TestQueuedEventChannel_DropsOldestWhenFull confirms the queue is bounded
// and that a drop is reported.
func TestQueuedEventChannel_DropsOldestWhenFull(t *testing.T) {
	t.Cleanup(ResetForTest)
	var reported []error
	errors.SetHandler(errorHandlerFunc(func(err *errors.DriftError) {
		reported = append(reported, err)
	}))
	t.Cleanup(func() { errors.SetHandler(nil) })
	ch := NewQueuedEventChannel("drift/test/queued/full", 2)

	ch.dispatchEvent(1)
	ch.dispatchEvent(2)
	ch.dispatchEvent(3)

	var r recorder
	sub := ch.Listen(r.handler())
	defer sub.Cancel()

	assertEvents(t, "subscriber", r.events(), 2, 3)
	if len(reported) != 1 {
		t.Errorf("reported %d errors, want 1 for the dropped event: %v", len(reported), reported)
	}
}

// TestQueuedEventChannel_DispatchDuringDrainKeepsOrder confirms an event
// dispatched while Listen delivers the backlog lands after it, and that a
// handler may Listen on the same channel without deadlocking.
func TestQueuedEventChannel_DispatchDuringDrainKeepsOrder(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewQueuedEventChannel("drift/test/queued/drain_order", 8)
	ch.dispatchEvent(1)
	ch.dispatchEvent(2)

	var late recorder
	var first recorder
	first.hook = func(data any) {
		if data == 1 {
			ch.dispatchEvent(3)
			ch.Listen(late.handler())
		}
	}
	sub := ch.Listen(first.handler())
	defer sub.Cancel()

	assertEvents(t, "first subscriber", first.events(), 1, 2, 3)
	assertEvents(t, "subscriber joining mid-drain", late.events(), 2, 3)
}

// TestQueuedEventChannel_KeepsRestWhenSubscribersLeaveMidDrain confirms
// events not yet delivered stay queued if every subscriber leaves while the
// backlog is delivered.
func TestQueuedEventChannel_KeepsRestWhenSubscribersLeaveMidDrain(t *testing.T) {
	t.Cleanup(ResetForTest)
	ch := NewQueuedEventChannel("drift/test/queued/leave_mid_drain", 8)
	ch.dispatchEvent(1)
	ch.dispatchEvent(2)

	var first recorder
	first.hook = func(any) { ch.dispatchDone() }
	ch.Listen(first.handler())

	var second recorder
	sub := ch.Listen(second.handler())
	defer sub.Cancel()

	assertEvents(t, "first subscriber", first.events(), 1)
	assertEvents(t, "second subscriber", second.events(), 2)
}

// TestNewQueuedEventChannel_RejectsNonPositiveCapacity confirms a
// misconfigured channel fails at construction.
func TestNewQueuedEventChannel_RejectsNonPositiveCapacity(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for capacity 0")
		}
	}()
	NewQueuedEventChannel("drift/test/queued/zero", 0)
}

type errorHandlerFunc func(*errors.DriftError)

func (f errorHandlerFunc) HandleError(err *errors.DriftError)        { f(err) }
func (f errorHandlerFunc) HandlePanic(*errors.PanicError)            {}
func (f errorHandlerFunc) HandleBoundaryError(*errors.BoundaryError) {}

// TestResetForTest_ClearsBufferedEvents confirms neither a queued backlog
// nor a sticky replay slot leaks from one test into the next.
func TestResetForTest_ClearsBufferedEvents(t *testing.T) {
	t.Cleanup(ResetForTest)
	queued := NewQueuedEventChannel("drift/test/queued/reset", 8)
	sticky := NewStickyEventChannel("drift/test/sticky/reset")
	queued.dispatchEvent(1)
	sticky.dispatchEvent(1)

	ResetForTest()

	var q, s recorder
	defer queued.Listen(q.handler()).Cancel()
	defer sticky.Listen(s.handler()).Cancel()
	assertEvents(t, "queued subscriber", q.events())
	assertEvents(t, "sticky subscriber", s.events())
}
