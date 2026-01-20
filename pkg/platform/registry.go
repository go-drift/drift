package platform

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/errors"
)

// channelRegistry manages all registered platform channels.
type channelRegistry struct {
	methodChannels map[string]*MethodChannel
	eventChannels  map[string]*EventChannel
	mu             sync.RWMutex
}

var registry = &channelRegistry{
	methodChannels: make(map[string]*MethodChannel),
	eventChannels:  make(map[string]*EventChannel),
}

func (r *channelRegistry) registerMethod(name string, ch *MethodChannel) {
	r.mu.Lock()
	r.methodChannels[name] = ch
	r.mu.Unlock()
}

func (r *channelRegistry) registerEvent(name string, ch *EventChannel) {
	r.mu.Lock()
	r.eventChannels[name] = ch
	r.mu.Unlock()
}

func (r *channelRegistry) getMethodChannel(name string) *MethodChannel {
	r.mu.RLock()
	ch := r.methodChannels[name]
	r.mu.RUnlock()
	return ch
}

func (r *channelRegistry) getEventChannel(name string) *EventChannel {
	r.mu.RLock()
	ch := r.eventChannels[name]
	r.mu.RUnlock()
	return ch
}

// pendingCall represents a method call waiting for a response.
type pendingCall struct {
	done   chan struct{}
	result any
	err    error
}

var (
	pendingCalls   = make(map[int64]*pendingCall)
	pendingCallsMu sync.Mutex
	nextCallID     atomic.Int64
)

// nativeBridge is the interface to native platform code.
// This is set by the bridge package during initialization.
var nativeBridge NativeBridge

// NativeBridge defines the interface for calling native platform code.
type NativeBridge interface {
	// InvokeMethod calls a method on the native side.
	InvokeMethod(channel, method string, args []byte) ([]byte, error)

	// StartEventStream tells native to start sending events for a channel.
	StartEventStream(channel string) error

	// StopEventStream tells native to stop sending events for a channel.
	StopEventStream(channel string) error
}

// SetNativeBridge sets the native bridge implementation.
// Called by the bridge package during initialization.
func SetNativeBridge(bridge NativeBridge) {
	nativeBridge = bridge
}

// invokeNative calls a method on the native side.
func invokeNative(channel, method string, args any) (any, error) {
	if nativeBridge == nil {
		return nil, ErrPlatformUnavailable
	}

	// Encode arguments
	argsData, err := DefaultCodec.Encode(args)
	if err != nil {
		return nil, err
	}

	// Call native
	resultData, err := nativeBridge.InvokeMethod(channel, method, argsData)
	if err != nil {
		return nil, err
	}

	// Decode result
	return DefaultCodec.Decode(resultData)
}

// startEventStream notifies native to start sending events.
func startEventStream(channel string) error {
	if nativeBridge == nil {
		errors.Report(&errors.DriftError{
			Op:      "platform.startEventStream",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     ErrPlatformUnavailable,
		})
		return ErrPlatformUnavailable
	}
	if err := nativeBridge.StartEventStream(channel); err != nil {
		errors.Report(&errors.DriftError{
			Op:      "platform.startEventStream",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     err,
		})
		return err
	}
	return nil
}

// stopEventStream notifies native to stop sending events.
func stopEventStream(channel string) error {
	if nativeBridge == nil {
		errors.Report(&errors.DriftError{
			Op:      "platform.stopEventStream",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     ErrPlatformUnavailable,
		})
		return ErrPlatformUnavailable
	}
	if err := nativeBridge.StopEventStream(channel); err != nil {
		errors.Report(&errors.DriftError{
			Op:      "platform.stopEventStream",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     err,
		})
		return err
	}
	return nil
}

// HandleMethodCall is called from the bridge when native invokes a Go method.
func HandleMethodCall(channel, method string, argsData []byte) ([]byte, error) {
	ch := registry.getMethodChannel(channel)
	if ch == nil {
		return nil, ErrChannelNotFound
	}

	// Decode arguments
	args, err := DefaultCodec.Decode(argsData)
	if err != nil {
		return nil, err
	}

	// Handle the call
	result, err := ch.handleCall(method, args)
	if err != nil {
		return nil, err
	}

	// Encode result
	return DefaultCodec.Encode(result)
}

// ErrChannelNotRegistered is returned when an event is received for an unregistered channel.
var ErrChannelNotRegistered = fmt.Errorf("event channel not registered")

// HandleEvent is called from the bridge when native sends an event.
func HandleEvent(channel string, eventData []byte) error {
	ch := registry.getEventChannel(channel)
	if ch == nil {
		err := fmt.Errorf("%w: %s", ErrChannelNotRegistered, channel)
		errors.Report(&errors.DriftError{
			Op:      "platform.HandleEvent",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     err,
		})
		return err
	}

	data, err := DefaultCodec.Decode(eventData)
	if err != nil {
		ch.dispatchError(err)
		return err
	}

	ch.dispatchEvent(data)
	return nil
}

// HandleEventError is called from the bridge when an event stream errors.
func HandleEventError(channel string, code, message string) error {
	ch := registry.getEventChannel(channel)
	if ch == nil {
		err := fmt.Errorf("%w: %s", ErrChannelNotRegistered, channel)
		errors.Report(&errors.DriftError{
			Op:      "platform.HandleEventError",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     err,
		})
		return err
	}

	ch.dispatchError(NewChannelError(code, message))
	return nil
}

// HandleEventDone is called from the bridge when an event stream ends.
func HandleEventDone(channel string) error {
	ch := registry.getEventChannel(channel)
	if ch == nil {
		err := fmt.Errorf("%w: %s", ErrChannelNotRegistered, channel)
		errors.Report(&errors.DriftError{
			Op:      "platform.HandleEventDone",
			Kind:    errors.KindPlatform,
			Channel: channel,
			Err:     err,
		})
		return err
	}

	ch.dispatchDone()
	return nil
}
