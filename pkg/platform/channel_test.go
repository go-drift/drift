package platform

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// deferredBridge holds each call's reply until the test releases it, as
// native does for a call it answers later. started is closed when the first
// call arrives, so tests can wait for it deterministically.
type deferredBridge struct {
	started     chan struct{}
	startedOnce sync.Once
	mu          sync.Mutex
	pending     []func([]byte, error)
}

func newDeferredBridge() *deferredBridge {
	return &deferredBridge{started: make(chan struct{})}
}

func (b *deferredBridge) InvokeMethod(_, _ string, _ []byte, reply func([]byte, error)) {
	b.mu.Lock()
	b.pending = append(b.pending, reply)
	b.mu.Unlock()
	b.startedOnce.Do(func() { close(b.started) })
}

// release answers every held call with value.
func (b *deferredBridge) release(value any) {
	b.mu.Lock()
	pending := b.pending
	b.pending = nil
	b.mu.Unlock()
	for _, reply := range pending {
		reply(DefaultCodec.Encode(value))
	}
}

func (b *deferredBridge) StartEventStream(string) error { return nil }
func (b *deferredBridge) StopEventStream(string) error  { return nil }

// countingBridge counts InvokeMethod calls and answers them in place with a
// canned response.
type countingBridge struct {
	mu       sync.Mutex
	calls    int
	response any
}

func (b *countingBridge) InvokeMethod(_, _ string, _ []byte, reply func([]byte, error)) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	reply(DefaultCodec.Encode(b.response))
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
	bridge := newDeferredBridge()
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

	// Native finishes the abandoned call later; its reply is discarded.
	bridge.release("late")
}

func TestInvoke_CtxDeadlineExceeded(t *testing.T) {
	bridge := newDeferredBridge()
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

func TestInvoke_LaterReplyArrives(t *testing.T) {
	bridge := newDeferredBridge()
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/later_reply")
	go func() {
		<-bridge.started
		bridge.release("token")
	}()
	got, err := ch.Invoke(context.Background(), "token", nil)
	if err != nil || got != "token" {
		t.Fatalf("Invoke = %v, %v; want token", got, err)
	}
}

// goroutineID returns the current goroutine's ID, parsed from its stack
// header ("goroutine 7 [running]:").
func goroutineID() string {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	return strings.Fields(string(buf))[1]
}

// threadBridge records which goroutine each call starts on.
type threadBridge struct{ callers []string }

func (b *threadBridge) InvokeMethod(_, _ string, _ []byte, reply func([]byte, error)) {
	b.callers = append(b.callers, goroutineID())
	reply(DefaultCodec.Encode(nil))
}
func (b *threadBridge) StartEventStream(string) error { return nil }
func (b *threadBridge) StopEventStream(string) error  { return nil }

// Native decides where a handler runs from the thread it is called on, and
// a UI-thread call must be answered there. A cgo callback's goroutine is
// locked to its thread, so the bridge must be entered on the caller's own
// goroutine, cancelable context or not; a hop to another goroutine is the
// deadlock where native queues the handler for the UI thread while the UI
// thread waits on Go.
func TestInvoke_StartsCallOnCallerGoroutine(t *testing.T) {
	bridge := &threadBridge{}
	SetNativeBridge(bridge)
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/caller_thread")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, c := range []context.Context{context.Background(), ctx} {
		if _, err := ch.Invoke(c, "noop", nil); err != nil {
			t.Fatal(err)
		}
	}
	me := goroutineID()
	for i, g := range bridge.callers {
		if g != me {
			t.Errorf("call %d started on goroutine %s, want the caller's (%s)", i, g, me)
		}
	}
}

func TestInvoke_DoubleReplyPanics(t *testing.T) {
	SetNativeBridge(doubleReplyBridge{})
	t.Cleanup(ResetForTest)
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "replied twice") {
			t.Errorf("recover() = %v, want a replied-twice panic", r)
		}
	}()
	_, _ = NewMethodChannel("drift/test/double").Invoke(context.Background(), "noop", nil)
}

type doubleReplyBridge struct{}

func (doubleReplyBridge) InvokeMethod(_, _ string, _ []byte, reply func([]byte, error)) {
	reply(nil, nil)
	reply(nil, nil)
}
func (doubleReplyBridge) StartEventStream(string) error { return nil }
func (doubleReplyBridge) StopEventStream(string) error  { return nil }

// Native refuses an async method called from the UI thread with a wire
// code; Go callers match it by sentinel.
func TestChannelErrorMatchesStandardSentinels(t *testing.T) {
	for _, sentinel := range []error{ErrBlocksUIThread, ErrReplyDropped, ErrMethodNotFound} {
		err := fmt.Errorf("invoke: %w", AsChannelError(sentinel))
		if !errors.Is(err, sentinel) {
			t.Errorf("native error %v does not match %v", err, sentinel)
		}
	}
	if errors.Is(NewChannelError("native_error", "x"), ErrBlocksUIThread) {
		t.Error("an unrelated code matched ErrBlocksUIThread")
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

func TestInvoke_BackgroundCtx(t *testing.T) {
	bridge := &countingBridge{response: map[string]any{"value": "fp"}}
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	ch := NewMethodChannel("drift/test/ctx_background")

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
