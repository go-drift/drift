package engine

import (
	"context"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

// initPhase tracks the OnInit callback state machine.
// Progresses linearly: Pending, Running, then Done or Failed.
// The zero value (initPhaseNone) means no OnInit was configured.
type initPhase int

const (
	initPhaseNone    initPhase = iota // no OnInit, mount immediately
	initPhasePending                  // OnInit registered, not yet started
	initPhaseRunning                  // goroutine executing
	initPhaseDone                     // OnInit succeeded
	initPhaseFailed                   // OnInit returned error
)

// appInit groups the state for App.OnInit / App.OnDispose lifecycle.
type appInit struct {
	onInit  func(ctx context.Context) error
	dispose func()
	phase   initPhase
	err     error
	ctx     context.Context
	cancel  context.CancelFunc
}

// start launches the OnInit goroutine. Transitions Pending to Running.
// The dispatch callback transitions Running to Done or Failed.
// Returns true if the goroutine was launched.
//
// dispatchFn must execute the callback under frameLock (e.g. engine.Dispatch),
// since the callback mutates ai.phase and ai.err without other synchronization.
func (ai *appInit) start(dispatchFn func(func())) bool {
	if ai.phase != initPhasePending {
		return false
	}
	ai.phase = initPhaseRunning
	ctx := ai.ctx
	onInit := ai.onInit
	go func() {
		err := onInit(ctx)
		dispatchFn(func() {
			if err != nil {
				ai.phase = initPhaseFailed
				ai.err = err
			} else {
				ai.phase = initPhaseDone
			}
		})
	}()
	return true
}

// initError returns a BoundaryError for display on the error screen,
// or nil if init did not fail.
func (ai *appInit) initError() *errors.BoundaryError {
	if ai.phase != initPhaseFailed {
		return nil
	}
	return &errors.BoundaryError{
		Phase:     "init",
		Err:       ai.err,
		Timestamp: time.Now(),
	}
}

// resetFailure transitions Failed to Done so that RestartApp
// mounts the original root widget without re-running OnInit.
func (ai *appInit) resetFailure() {
	if ai.phase == initPhaseFailed {
		ai.phase = initPhaseDone
		ai.err = nil
	}
}

// runDispose cancels the context and calls the dispose callback once.
// Subsequent calls are no-ops.
func (ai *appInit) runDispose() {
	if ai.cancel != nil {
		ai.cancel()
	}
	if ai.dispose != nil {
		fn := ai.dispose
		ai.dispose = nil
		fn()
	}
}
