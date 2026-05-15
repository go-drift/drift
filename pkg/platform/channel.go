package platform

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// MethodHandler handles incoming method calls on a channel.
type MethodHandler func(method string, args any) (any, error)

// MethodChannel provides bidirectional method-call communication with native code.
type MethodChannel struct {
	name    string
	codec   MessageCodec
	handler MethodHandler
}

// NewMethodChannel creates a new method channel with the given name.
func NewMethodChannel(name string) *MethodChannel {
	ch := &MethodChannel{
		name:  name,
		codec: DefaultCodec,
	}
	registry.registerMethod(name, ch)
	return ch
}

// Name returns the channel name.
func (c *MethodChannel) Name() string {
	return c.name
}

// SetHandler sets the handler for incoming method calls from native code.
func (c *MethodChannel) SetHandler(handler MethodHandler) {
	c.handler = handler
}

// Invoke calls a method on the native side and returns the result.
// Blocks until the native side responds, an error occurs, or ctx is canceled.
// See [invokeNative] for the ctx cancellation contract.
func (c *MethodChannel) Invoke(ctx context.Context, method string, args any) (any, error) {
	return invokeNative(ctx, c.name, method, args)
}

// handleCall processes an incoming method call from native code.
func (c *MethodChannel) handleCall(method string, args any) (any, error) {
	if c.handler == nil {
		return nil, ErrMethodNotFound
	}
	return c.handler(method, args)
}

// EventHandler receives events from an EventChannel.
type EventHandler struct {
	OnEvent func(data any)
	OnError func(err error)
	OnDone  func()
}

// Subscription represents an active event subscription.
type Subscription struct {
	channel  *EventChannel
	handler  *EventHandler
	canceled atomic.Bool
}

// Cancel stops receiving events on this subscription.
func (s *Subscription) Cancel() {
	if s.canceled.CompareAndSwap(false, true) {
		s.channel.removeSubscription(s)
	}
}

// IsCanceled returns true if this subscription has been canceled.
func (s *Subscription) IsCanceled() bool {
	return s.canceled.Load()
}

// EventChannel provides stream-based event communication from native to Go.
//
// When constructed via [NewStickyEventChannel], the channel remembers the most
// recently dispatched event payload (a single slot, overwritten on each
// dispatch). New subscribers receive that remembered payload exactly once on
// [Listen], before any subsequent live event. This solves the late-subscriber
// problem for one-shot events such as "first frame rendered."
type EventChannel struct {
	name          string
	codec         MessageCodec
	subscriptions []*Subscription
	started       bool // whether native event stream is active
	mu            sync.Mutex

	// sticky and replay form the per-channel single-slot buffer for
	// late-subscriber replay. Both are protected by mu.
	sticky      bool
	replayValid bool
	replayData  any
}

// NewEventChannel creates a new event channel with the given name.
func NewEventChannel(name string) *EventChannel {
	ch := &EventChannel{
		name:  name,
		codec: DefaultCodec,
	}
	registry.registerEvent(name, ch)
	return ch
}

// NewStickyEventChannel creates an event channel that remembers the most
// recently dispatched event payload and replays it to each new subscriber on
// [Listen]. Use this for one-shot lifecycle signals (e.g. first-frame
// rendered) where a subscriber registering after the event still needs to
// observe it.
//
// Sticky storage holds a single slot, overwritten on every subsequent
// dispatch. Errors and Done do not populate the slot.
func NewStickyEventChannel(name string) *EventChannel {
	ch := &EventChannel{
		name:   name,
		codec:  DefaultCodec,
		sticky: true,
	}
	registry.registerEvent(name, ch)
	return ch
}

// Name returns the channel name.
func (c *EventChannel) Name() string {
	return c.name
}

// IsSticky reports whether the channel was constructed via
// [NewStickyEventChannel]. Sticky channels remember the most recent
// emission and replay it to subscribers that register after the fact.
// Callers can use this to assert framework-level expectations (e.g. that
// a lifecycle channel is sticky in tests).
func (c *EventChannel) IsSticky() bool {
	return c.sticky
}

// Listen subscribes to events on this channel.
// If the native bridge is not yet available (e.g., during init), the subscription
// is created but the event stream start is deferred until [SetNativeBridge] is called.
// Any error from starting the native event stream is reported via the error handler
// but does not prevent the subscription from being created.
func (c *EventChannel) Listen(handler EventHandler) *Subscription {
	sub := &Subscription{
		channel: c,
		handler: &handler,
	}

	c.mu.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	shouldStart := nativeBridge != nil && !c.started
	if shouldStart {
		c.started = true
	}
	var replay any
	hasReplay := c.sticky && c.replayValid
	if hasReplay {
		replay = c.replayData
	}
	c.mu.Unlock()

	// Replay the most recent sticky event to this fresh subscriber before
	// the live stream resumes. Delivered synchronously from Listen so the
	// caller can observe the replay before returning.
	if hasReplay && handler.OnEvent != nil && !sub.IsCanceled() {
		handler.OnEvent(replay)
	}

	// Notify native that we're listening. Skip if:
	// - bridge not yet set (SetNativeBridge will start pending streams), or
	// - stream already started by a prior subscriber or SetNativeBridge.
	if shouldStart {
		if err := startEventStream(c.name); err != nil {
			c.mu.Lock()
			c.started = false
			c.mu.Unlock()
			if handler.OnError != nil {
				handler.OnError(err)
			}
		}
	}

	return sub
}

// removeSubscription removes a subscription from the channel.
func (c *EventChannel) removeSubscription(sub *Subscription) {
	c.mu.Lock()
	for i, s := range c.subscriptions {
		if s == sub {
			c.subscriptions = append(c.subscriptions[:i], c.subscriptions[i+1:]...)
			break
		}
	}
	hasListeners := len(c.subscriptions) > 0
	if !hasListeners {
		c.started = false
	}
	c.mu.Unlock()

	// Notify native if no more listeners.
	// ErrClosed is expected during normal shutdown and not reported.
	if !hasListeners {
		if err := stopEventStream(c.name); err != nil && !errors.Is(err, ErrClosed) {
			// Unexpected teardown error - already reported by stopEventStream
		}
	}
}

// dispatchEvent sends an event to all subscribers.
//
// On sticky channels, the event payload is stored in the channel's single
// replay slot before being broadcast, so subscribers that join afterwards
// will receive it via [Listen]. Errors and Done do not populate the slot.
func (c *EventChannel) dispatchEvent(data any) {
	c.mu.Lock()
	if c.sticky {
		c.replayValid = true
		c.replayData = data
	}
	subs := make([]*Subscription, len(c.subscriptions))
	copy(subs, c.subscriptions)
	c.mu.Unlock()

	for _, sub := range subs {
		if !sub.IsCanceled() && sub.handler.OnEvent != nil {
			sub.handler.OnEvent(data)
		}
	}
}

// dispatchError sends an error to all subscribers.
func (c *EventChannel) dispatchError(err error) {
	c.mu.Lock()
	subs := make([]*Subscription, len(c.subscriptions))
	copy(subs, c.subscriptions)
	c.mu.Unlock()

	for _, sub := range subs {
		if !sub.IsCanceled() && sub.handler.OnError != nil {
			sub.handler.OnError(err)
		}
	}
}

// dispatchDone notifies all subscribers that the stream has ended.
func (c *EventChannel) dispatchDone() {
	c.mu.Lock()
	subs := make([]*Subscription, len(c.subscriptions))
	copy(subs, c.subscriptions)
	c.subscriptions = nil
	c.started = false
	c.mu.Unlock()

	for _, sub := range subs {
		sub.canceled.Store(true)
		if sub.handler.OnDone != nil {
			sub.handler.OnDone()
		}
	}
}
