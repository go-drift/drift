package platform

import (
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
// This blocks until the native side responds or an error occurs.
func (c *MethodChannel) Invoke(method string, args any) (any, error) {
	return invokeNative(c.name, method, args)
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
type EventChannel struct {
	name          string
	codec         MessageCodec
	subscriptions []*Subscription
	mu            sync.Mutex
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

// Name returns the channel name.
func (c *EventChannel) Name() string {
	return c.name
}

// Listen subscribes to events on this channel.
// Any error from starting the native event stream is reported via the error handler
// but does not prevent the subscription from being created.
func (c *EventChannel) Listen(handler EventHandler) *Subscription {
	sub := &Subscription{
		channel: c,
		handler: &handler,
	}
	c.mu.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	c.mu.Unlock()

	// Notify native that we're listening (errors are reported internally)
	_ = startEventStream(c.name)

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
	c.mu.Unlock()

	// Notify native if no more listeners (errors are reported internally)
	if !hasListeners {
		_ = stopEventStream(c.name)
	}
}

// dispatchEvent sends an event to all subscribers.
func (c *EventChannel) dispatchEvent(data any) {
	c.mu.Lock()
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
	c.mu.Unlock()

	for _, sub := range subs {
		sub.canceled.Store(true)
		if sub.handler.OnDone != nil {
			sub.handler.OnDone()
		}
	}
}
