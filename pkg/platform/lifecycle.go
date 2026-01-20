package platform

import (
	"sync"

	"github.com/go-drift/drift/pkg/errors"
)

// LifecycleService provides app lifecycle state management.
var Lifecycle = &LifecycleService{
	channel:  NewMethodChannel("github.com/go-drift/drift/lifecycle"),
	events:   NewEventChannel("github.com/go-drift/drift/lifecycle/events"),
	state:    LifecycleStateResumed,
	handlers: make([]LifecycleHandler, 0),
}

// LifecycleService manages app lifecycle events.
type LifecycleService struct {
	channel  *MethodChannel
	events   *EventChannel
	state    LifecycleState
	handlers []LifecycleHandler
	mu       sync.RWMutex
}

// LifecycleState represents the current app lifecycle state.
type LifecycleState string

const (
	// LifecycleStateResumed indicates the app is visible and responding to user input.
	LifecycleStateResumed LifecycleState = "resumed"

	// LifecycleStateInactive indicates the app is transitioning (e.g., receiving a phone call).
	// On iOS, this occurs during app switcher or when a system dialog is shown.
	LifecycleStateInactive LifecycleState = "inactive"

	// LifecycleStatePaused indicates the app is not visible but still running.
	LifecycleStatePaused LifecycleState = "paused"

	// LifecycleStateDetached indicates the app is still hosted but detached from any view.
	LifecycleStateDetached LifecycleState = "detached"
)

// LifecycleHandler is called when lifecycle state changes.
type LifecycleHandler func(state LifecycleState)

func init() {
	// Set up event listener for lifecycle changes
	Lifecycle.events.Listen(EventHandler{
		OnEvent: func(data any) {
			m, ok := data.(map[string]any)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "lifecycle.parseEvent",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/lifecycle/events",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/lifecycle/events",
						DataType: "LifecycleState",
						Got:      data,
					},
				})
				return
			}
			state, ok := m["state"].(string)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "lifecycle.parseEvent",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/lifecycle/events",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/lifecycle/events",
						DataType: "LifecycleState",
						Got:      data,
					},
				})
				return
			}
			Lifecycle.updateState(LifecycleState(state))
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "lifecycle.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/lifecycle/events",
				Err:     err,
			})
		},
	})

	// Handle method calls from native (e.g., permission to close)
	Lifecycle.channel.SetHandler(func(method string, args any) (any, error) {
		switch method {
		case "didChangeState":
			if m, ok := args.(map[string]any); ok {
				if state, ok := m["state"].(string); ok {
					Lifecycle.updateState(LifecycleState(state))
				}
			}
			return nil, nil
		default:
			return nil, ErrMethodNotFound
		}
	})
}

// State returns the current lifecycle state.
func (l *LifecycleService) State() LifecycleState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

// AddHandler registers a handler to be called on lifecycle changes.
// Returns a function that can be called to remove the handler.
func (l *LifecycleService) AddHandler(handler LifecycleHandler) func() {
	l.mu.Lock()
	l.handlers = append(l.handlers, handler)
	index := len(l.handlers) - 1
	l.mu.Unlock()

	return func() {
		l.mu.Lock()
		if index < len(l.handlers) {
			l.handlers = append(l.handlers[:index], l.handlers[index+1:]...)
		}
		l.mu.Unlock()
	}
}

// IsResumed returns true if the app is in the resumed state.
func (l *LifecycleService) IsResumed() bool {
	return l.State() == LifecycleStateResumed
}

// IsPaused returns true if the app is paused.
func (l *LifecycleService) IsPaused() bool {
	return l.State() == LifecycleStatePaused
}

// updateState updates the lifecycle state and notifies handlers.
func (l *LifecycleService) updateState(newState LifecycleState) {
	l.mu.Lock()
	if l.state == newState {
		l.mu.Unlock()
		return
	}
	l.state = newState
	handlers := make([]LifecycleHandler, len(l.handlers))
	copy(handlers, l.handlers)
	l.mu.Unlock()

	for _, h := range handlers {
		h(newState)
	}
}
