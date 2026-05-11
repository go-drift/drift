package platform

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingBridge gates each InvokeMethod call on a release channel so tests
// can simulate a slow native call. started is closed the first time the bridge
// is entered so tests can deterministically wait for the call to be in flight
// (no time.Sleep race). completedCalls counts every call that reached
// completion (i.e. native finished even if Go abandoned).
type blockingBridge struct {
	release        chan struct{}
	started        chan struct{}
	startedOnce    sync.Once
	completedCalls atomic.Int64
}

func newBlockingBridge() *blockingBridge {
	return &blockingBridge{
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
}

func (b *blockingBridge) InvokeMethod(_ context.Context, _, _ string, _ []byte) ([]byte, error) {
	b.startedOnce.Do(func() { close(b.started) })
	<-b.release
	b.completedCalls.Add(1)
	return DefaultCodec.Encode(nil)
}

func (b *blockingBridge) StartEventStream(string) error { return nil }
func (b *blockingBridge) StopEventStream(string) error  { return nil }

// countingBridge counts InvokeMethod calls and returns canned responses.
type countingBridge struct {
	mu       sync.Mutex
	calls    int
	response any
}

func (b *countingBridge) InvokeMethod(_ context.Context, _, _ string, _ []byte) ([]byte, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	return DefaultCodec.Encode(b.response)
}

func (b *countingBridge) StartEventStream(string) error { return nil }
func (b *countingBridge) StopEventStream(string) error  { return nil }

func (b *countingBridge) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func TestInvoke_CtxCanceledBeforeCall(t *testing.T) {
	bridge := &countingBridge{response: map[string]any{"ok": true}}
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/ctx_pre_cancel")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ch.Invoke(ctx, "noop", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := bridge.callCount(); got != 0 {
		t.Errorf("expected zero bridge calls when ctx pre-canceled; got %d", got)
	}
}

func TestInvoke_CtxCanceledDuringCall(t *testing.T) {
	bridge := newBlockingBridge()
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/ctx_during_cancel")
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := ch.Invoke(ctx, "noop", nil)
		errCh <- err
	}()

	// Wait deterministically for the goroutine to enter the bridge call.
	select {
	case <-bridge.started:
	case <-time.After(time.Second):
		t.Fatal("bridge.InvokeMethod was not entered within 1s")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Invoke did not unblock within 1s of ctx cancel")
	}

	if got := bridge.completedCalls.Load(); got != 0 {
		t.Errorf("expected bridge call still in flight; got completed=%d", got)
	}

	// Release the bridge and verify the abandoned call eventually completes
	// (proving the wrap unblocks Go without aborting native).
	close(bridge.release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if bridge.completedCalls.Load() == 1 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("bridge call never completed after release; completed=%d", bridge.completedCalls.Load())
}

func TestInvoke_CtxDeadlineExceeded(t *testing.T) {
	bridge := newBlockingBridge()
	defer close(bridge.release)
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/ctx_deadline")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := ch.Invoke(ctx, "noop", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestInvoke_NormalCallPropagatesResult(t *testing.T) {
	bridge := &countingBridge{response: map[string]any{"value": "hello"}}
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/ctx_normal")
	ctx := t.Context()

	got, err := ch.Invoke(ctx, "noop", nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", got)
	}
	if m["value"] != "hello" {
		t.Errorf("value = %v, want hello", m["value"])
	}
}

func TestInvoke_BackgroundCtxTakesFastPath(t *testing.T) {
	bridge := &countingBridge{response: map[string]any{"value": "fp"}}
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/ctx_fast_path")

	got, err := ch.Invoke(context.Background(), "noop", nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if m, ok := got.(map[string]any); !ok || m["value"] != "fp" {
		t.Errorf("result = %v, want {value: fp}", got)
	}
	if got := bridge.callCount(); got != 1 {
		t.Errorf("bridge call count = %d, want 1", got)
	}
}
