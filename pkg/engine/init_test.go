package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
)

// testSize is a small logical size used for runPipeline calls in tests.
var testSize = graphics.Size{Width: 100, Height: 100}

func newTestRunner() *appRunner {
	r := newAppRunner()
	r.buildOwner.OnNeedsFrame = func() {}
	return r
}

// swapApp replaces the global app with a fresh test runner and returns
// it. Registers cleanup via t.Cleanup.
func swapApp(t *testing.T) *appRunner {
	t.Helper()
	saved := app
	t.Cleanup(func() { app = saved })
	app = newTestRunner()
	app.userApp = defaultPlaceholder{}
	return app
}

// waitForDispatch waits until the dispatch queue has at least one entry,
// or the timeout expires.
func waitForDispatch(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		app.dispatchMu.Lock()
		n := len(app.dispatchQueue)
		app.dispatchMu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatal("timed out waiting for dispatch callback to be enqueued")
}

// runPipelineLocked acquires frameLock and runs a single pipeline frame.
func runPipelineLocked() bool {
	frameLock.Lock()
	defer frameLock.Unlock()
	return app.runPipeline(testSize, nil)
}

func TestInitPhaseNone_MountsImmediately(t *testing.T) {
	swapApp(t)

	if !runPipelineLocked() {
		t.Fatal("expected runPipeline to return true when no OnInit is set")
	}
	if app.root == nil {
		t.Fatal("expected root to be mounted")
	}
}

func TestInitPhasePending_ReturnsFalse(t *testing.T) {
	swapApp(t)
	app.lifecycle.phase = initPhasePending
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())
	app.lifecycle.onInit = func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	if runPipelineLocked() {
		t.Fatal("expected runPipeline to return false while OnInit is pending")
	}
	if app.lifecycle.phase != initPhaseRunning {
		t.Fatalf("expected initPhaseRunning, got %d", app.lifecycle.phase)
	}

	app.lifecycle.cancel() // clean up goroutine
}

func TestInitPhaseRunning_ReturnsFalse(t *testing.T) {
	swapApp(t)
	app.lifecycle.phase = initPhaseRunning

	if runPipelineLocked() {
		t.Fatal("expected runPipeline to return false while OnInit is running")
	}
}

func TestInitSuccess_MountsRoot(t *testing.T) {
	swapApp(t)
	app.lifecycle.phase = initPhasePending
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())

	done := make(chan struct{})
	app.lifecycle.onInit = func(ctx context.Context) error {
		close(done)
		return nil
	}

	// First frame starts the goroutine
	if runPipelineLocked() {
		t.Fatal("expected first frame to return false")
	}

	<-done
	waitForDispatch(t, 2*time.Second)

	if !runPipelineLocked() {
		t.Fatal("expected second frame to return true after successful OnInit")
	}
	if app.root == nil {
		t.Fatal("expected root to be mounted")
	}
	if app.lifecycle.phase != initPhaseDone {
		t.Fatalf("expected initPhaseDone, got %d", app.lifecycle.phase)
	}
}

func TestInitFailure_MountsErrorScreen(t *testing.T) {
	swapApp(t)
	app.lifecycle.phase = initPhasePending
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())

	done := make(chan struct{})
	app.lifecycle.onInit = func(ctx context.Context) error {
		close(done)
		return fmt.Errorf("database connection failed")
	}

	runPipelineLocked()
	<-done
	waitForDispatch(t, 2*time.Second)

	if !runPipelineLocked() {
		t.Fatal("expected runPipeline to return true with error screen mounted")
	}
	if app.lifecycle.phase != initPhaseFailed {
		t.Fatalf("expected initPhaseFailed, got %d", app.lifecycle.phase)
	}
	if app.capturedError.Load() == nil {
		t.Fatal("expected capturedError to be set")
	}
}

func TestInitFailure_RestartMountsOriginalApp(t *testing.T) {
	swapApp(t)
	app.lifecycle.phase = initPhasePending
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())
	originalApp := app.userApp

	done := make(chan struct{})
	app.lifecycle.onInit = func(ctx context.Context) error {
		close(done)
		return fmt.Errorf("init failed")
	}

	// Run through init failure
	runPipelineLocked()
	<-done
	waitForDispatch(t, 2*time.Second)
	runPipelineLocked()

	if app.userApp != originalApp {
		t.Fatal("expected userApp to be preserved after init failure")
	}

	RestartApp()
	waitForDispatch(t, 2*time.Second)
	runPipelineLocked()

	if app.lifecycle.phase != initPhaseDone {
		t.Fatalf("expected initPhaseDone after restart, got %d", app.lifecycle.phase)
	}
	if app.capturedError.Load() != nil {
		t.Fatal("expected capturedError to be nil after restart")
	}
	if app.root == nil {
		t.Fatal("expected root to be mounted after restart")
	}
}

func TestRunDispose_CallsCallback(t *testing.T) {
	swapApp(t)
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())

	disposed := false
	app.lifecycle.dispose = func() { disposed = true }

	app.lifecycle.runDispose()

	if !disposed {
		t.Fatal("expected dispose to be called")
	}
}

func TestRunDispose_ContextCancelledFirst(t *testing.T) {
	swapApp(t)
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())

	var ctxErr error
	app.lifecycle.dispose = func() { ctxErr = app.lifecycle.ctx.Err() }

	app.lifecycle.runDispose()

	if ctxErr != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", ctxErr)
	}
}

func TestRunDispose_CalledOnlyOnce(t *testing.T) {
	swapApp(t)
	app.lifecycle.ctx, app.lifecycle.cancel = context.WithCancel(context.Background())

	callCount := 0
	app.lifecycle.dispose = func() { callCount++ }

	app.lifecycle.runDispose()
	app.lifecycle.runDispose()

	if callCount != 1 {
		t.Fatalf("expected dispose called once, got %d", callCount)
	}
}

func TestNeedsFrame_FalseWhileInitRunning(t *testing.T) {
	swapApp(t)
	app.lifecycle.phase = initPhaseRunning

	frameLock.Lock()
	needs := app.needsFrameLocked()
	frameLock.Unlock()

	if needs {
		t.Fatal("expected needsFrame to return false while init is running")
	}
}

func TestLifecycleDetachedTriggersDispose(t *testing.T) {
	swapApp(t)

	disposed := false
	SetOnDispose(func() { disposed = true })

	platform.Lifecycle.SetStateForTest(platform.LifecycleStateDetached)
	t.Cleanup(func() {
		platform.Lifecycle.SetStateForTest(platform.LifecycleStateResumed)
	})

	if !disposed {
		t.Fatal("expected dispose to be called on LifecycleStateDetached")
	}
}
