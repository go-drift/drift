package platform

import (
	"context"
	"fmt"
	"sync"

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

// nativeBridge is the interface to native platform code.
// This is set by the bridge package during initialization.
var nativeBridge NativeBridge

// builtinInits holds functions that re-register the built-in event listeners
// set up during package init (lifecycle, safe area, accessibility, etc.).
// Each init() function appends its listener setup here so that ResetForTest
// can replay them after clearing subscriptions.
var builtinInits []func()

// registerBuiltinInit registers a function that sets up built-in event
// listeners. Called from init() functions in lifecycle.go, safe_area.go, etc.
// The registered function will be replayed by ResetForTest.
func registerBuiltinInit(fn func()) {
	builtinInits = append(builtinInits, fn)
}

// NativeBridge defines the interface for calling native platform code.
type NativeBridge interface {
	// InvokeMethod starts a native method call and delivers its outcome
	// through reply exactly once: before returning when native answers in
	// place, otherwise later, from any thread.
	//
	// It must start the call on the calling goroutine. Native decides where
	// a handler runs from the thread it is called on: a call from the UI
	// thread is answered in place (or refused, for a method that replies
	// asynchronously), since the UI thread cannot wait for a reply that may
	// need it. Moving the call to another goroutine hides that thread and
	// deadlocks the app.
	InvokeMethod(channel, method string, args []byte, reply func(result []byte, err error))

	// StartEventStream tells native to start sending events for a channel.
	StartEventStream(channel string) error

	// StopEventStream tells native to stop sending events for a channel.
	StopEventStream(channel string) error
}

// SetNativeBridge sets the native bridge implementation.
// Called by the bridge package during initialization.
//
// After setting the bridge, SetNativeBridge starts event streams for any
// event channels that acquired subscriptions before the bridge was available
// (e.g., during package init). This ensures that init-time Listen calls
// for Lifecycle, SafeArea, Accessibility, etc. are not silently lost.
// Startup errors are dispatched to subscribers' error handlers.
func SetNativeBridge(bridge NativeBridge) {
	nativeBridge = bridge

	// Start event streams for channels that subscribed before the bridge was set.
	registry.mu.RLock()
	channels := make([]*EventChannel, 0, len(registry.eventChannels))
	for _, ch := range registry.eventChannels {
		channels = append(channels, ch)
	}
	registry.mu.RUnlock()

	for _, ch := range channels {
		ch.mu.Lock()
		shouldStart := len(ch.subscriptions) > 0 && !ch.started
		if shouldStart {
			ch.started = true
		}
		ch.mu.Unlock()

		if shouldStart {
			if err := startEventStream(ch.name); err != nil {
				ch.mu.Lock()
				ch.started = false
				ch.mu.Unlock()
				ch.dispatchError(err)
			}
		}
	}
}

// invokeNative calls a native method and waits for its reply.
//
// The bridge starts the call on this goroutine (see
// [NativeBridge.InvokeMethod]), so a call from the UI thread never waits:
// native answers it in place. A call from any other goroutine waits for
// the reply or ctx, whichever comes first. Cancellation unblocks the caller
// with ctx.Err() but cannot abort native work: the call finishes in the
// background and its reply is discarded.
func invokeNative(ctx context.Context, channel, method string, args any) (any, error) {
	// Snapshot the bridge so a concurrent ResetForTest cannot swap it out
	// while a reply is outstanding.
	bridge := nativeBridge
	if bridge == nil {
		return nil, ErrPlatformUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	argsData, err := DefaultCodec.Encode(args)
	if err != nil {
		return nil, err
	}

	replies := make(chan nativeReply, 1)
	bridge.InvokeMethod(channel, method, argsData, func(data []byte, err error) {
		select {
		case replies <- nativeReply{data: data, err: err}:
		default:
			panic(fmt.Sprintf("platform: native replied twice to %s.%s", channel, method))
		}
	})

	select {
	case r := <-replies:
		return r.decode()
	case <-ctx.Done():
		// A reply that arrived together with cancellation still wins.
		select {
		case r := <-replies:
			return r.decode()
		default:
			return nil, ctx.Err()
		}
	}
}

// nativeReply is the outcome of one native method call.
type nativeReply struct {
	data []byte
	err  error
}

func (r nativeReply) decode() (any, error) {
	if r.err != nil {
		return nil, r.err
	}
	return DefaultCodec.Decode(r.data)
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

// ResetForTest resets all global platform state for test isolation.
// It clears the native bridge, resets cached state (lifecycle, safe area),
// removes all event subscriptions, and re-registers the built-in init-time
// listeners (lifecycle, safe area, accessibility) so that the package
// behaves as if freshly initialized. This should only be called from tests.
func ResetForTest() {
	nativeBridge = nil

	// Reset lifecycle
	Lifecycle.mu.Lock()
	Lifecycle.state = LifecycleStateResumed
	Lifecycle.handlers = Lifecycle.handlers[:0]
	Lifecycle.mu.Unlock()

	// Reset safe area
	SafeArea.mu.Lock()
	SafeArea.insets = EdgeInsets{}
	SafeArea.handlers = SafeArea.handlers[:0]
	SafeArea.mu.Unlock()

	// Clear all event channel subscriptions, started flags, sticky replay
	// slots and queues
	registry.mu.RLock()
	channels := make([]*EventChannel, 0, len(registry.eventChannels))
	for _, ch := range registry.eventChannels {
		channels = append(channels, ch)
	}
	registry.mu.RUnlock()

	for _, ch := range channels {
		ch.mu.Lock()
		ch.subscriptions = ch.subscriptions[:0]
		ch.started = false
		ch.replayValid = false
		ch.replayData = nil
		ch.queue = nil
		ch.draining = false
		ch.mu.Unlock()
	}

	// Reset dispatch function
	dispatchMu.Lock()
	dispatchFunc = nil
	dispatchMu.Unlock()

	// Reset audio player registry
	audioRegistryMu.Lock()
	audioRegistry = map[int64]*AudioPlayerController{}
	audioRegistryMu.Unlock()
	audioServiceOnce = sync.Once{}
	audioService = nil

	// Reset platform view registry (views, IDs, geometry cache)
	if platformViewRegistry != nil {
		platformViewRegistry.mu.Lock()
		platformViewRegistry.views = make(map[int64]PlatformView)
		platformViewRegistry.mu.Unlock()
		platformViewRegistry.nextID.Store(0)
		platformViewRegistry.batchMu.Lock()
		platformViewRegistry.geometryCache = make(map[int64]CapturedViewGeometry)
		platformViewRegistry.viewsSeenThisFrame = make(map[int64]struct{})
		platformViewRegistry.batchUpdates = nil
		platformViewRegistry.batchMu.Unlock()
	}

	// Re-register built-in listeners (lifecycle, safe area, accessibility)
	// so the package behaves as if freshly initialized.
	for _, fn := range builtinInits {
		fn()
	}
}
