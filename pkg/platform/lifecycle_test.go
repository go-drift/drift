package platform

import "testing"

func TestUseLifecycleObserver_HandlerCalled(t *testing.T) {
	SetupTestBridge(t.Cleanup)

	var received LifecycleState
	d := &testDisposable{}

	UseLifecycleObserver(d, func(state LifecycleState) {
		received = state
	})

	Lifecycle.updateState(LifecycleStatePaused)

	if received != LifecycleStatePaused {
		t.Errorf("expected LifecycleStatePaused, got %q", received)
	}
}

func TestUseLifecycleObserver_NotCalledAfterDispose(t *testing.T) {
	SetupTestBridge(t.Cleanup)

	callCount := 0
	d := &testDisposable{}

	UseLifecycleObserver(d, func(state LifecycleState) {
		callCount++
	})

	Lifecycle.updateState(LifecycleStatePaused)
	if callCount != 1 {
		t.Fatalf("expected 1 call before dispose, got %d", callCount)
	}

	// Dispose and verify handler is no longer called.
	d.dispose()

	Lifecycle.updateState(LifecycleStateResumed)
	if callCount != 1 {
		t.Errorf("expected no calls after dispose, got %d", callCount)
	}
}

func TestUseLifecycleObserver_DispatchUsed(t *testing.T) {
	SetupTestBridge(t.Cleanup)

	dispatched := false
	RegisterDispatch(func(cb func()) {
		dispatched = true
		cb()
	})

	d := &testDisposable{}
	UseLifecycleObserver(d, func(state LifecycleState) {})

	Lifecycle.updateState(LifecycleStatePaused)

	if !dispatched {
		t.Error("expected handler to be invoked via Dispatch")
	}
}

// testDisposable is a minimal implementation of the disposable interface for tests.
type testDisposable struct {
	cleanups []func()
}

func (d *testDisposable) OnDispose(cleanup func()) func() {
	d.cleanups = append(d.cleanups, cleanup)
	idx := len(d.cleanups) - 1
	return func() {
		d.cleanups[idx] = nil
	}
}

func (d *testDisposable) dispose() {
	for i := len(d.cleanups) - 1; i >= 0; i-- {
		if d.cleanups[i] != nil {
			d.cleanups[i]()
		}
	}
	d.cleanups = nil
}
