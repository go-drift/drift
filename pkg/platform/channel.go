package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	drifterrors "github.com/go-drift/drift/pkg/errors"
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
// How an event dispatched with no subscriber, or before a subscriber joined,
// reaches later subscribers depends on the constructor:
//   - [NewEventChannel]: it does not; events go to current subscribers only.
//   - [NewStickyEventChannel]: the most recent event is replayed to every
//     new subscriber, for one-shot signals such as "first frame rendered".
//   - [NewQueuedEventChannel]: events dispatched while nobody listens are
//     queued and delivered, in order, when a subscriber joins, for events
//     that must each be handled once, such as a notification tap that
//     launched the app before the app subscribed.
type EventChannel struct {
	name          string
	codec         MessageCodec
	delivery      eventDelivery
	subscriptions []*Subscription
	started       bool // whether native event stream is active
	mu            sync.Mutex

	// Sticky channels: the single replay slot. Protected by mu.
	replayValid bool
	replayData  any

	// Queued channels: events awaiting a subscriber, oldest first, at most
	// queueCap of them. draining is set while Listen delivers the queue;
	// events dispatched meanwhile join the queue so order is kept.
	// Protected by mu.
	queue    []any
	queueCap int
	draining bool
}

// eventDelivery is how an [EventChannel] treats subscribers that join after
// an event was dispatched.
type eventDelivery int

const (
	deliverLive eventDelivery = iota
	deliverSticky
	deliverQueued
)

// NewEventChannel creates a new event channel with the given name.
func NewEventChannel(name string) *EventChannel {
	return newEventChannel(name, deliverLive, 0)
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
	return newEventChannel(name, deliverSticky, 0)
}

// NewQueuedEventChannel creates an event channel that keeps events
// dispatched while it has no subscribers, up to capacity (the oldest is
// dropped and reported beyond that), and delivers them in order to the next
// subscriber from [Listen], before any later event. Each queued event is
// delivered once: subscribers joining after the queue drained see only
// live events. Errors and Done are never queued.
//
// Panics if capacity is not positive.
func NewQueuedEventChannel(name string, capacity int) *EventChannel {
	if capacity <= 0 {
		panic(fmt.Sprintf("platform: queued event channel %q needs a positive capacity, got %d", name, capacity))
	}
	return newEventChannel(name, deliverQueued, capacity)
}

func newEventChannel(name string, delivery eventDelivery, queueCap int) *EventChannel {
	ch := &EventChannel{
		name:     name,
		codec:    DefaultCodec,
		delivery: delivery,
		queueCap: queueCap,
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
	return c.delivery == deliverSticky
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
	hasReplay := c.delivery == deliverSticky && c.replayValid
	if hasReplay {
		replay = c.replayData
	}
	drain := c.delivery == deliverQueued && len(c.queue) > 0 && !c.draining
	if drain {
		c.draining = true
	}
	c.mu.Unlock()

	// Replay the most recent sticky event to this fresh subscriber before
	// the live stream resumes. Delivered synchronously from Listen so the
	// caller can observe the replay before returning.
	if hasReplay && handler.OnEvent != nil && !sub.IsCanceled() {
		handler.OnEvent(replay)
	}

	// Deliver what was queued while nobody listened, synchronously like a
	// sticky replay, so the caller has seen it when Listen returns.
	if drain {
		c.drainQueue()
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
// will receive it via [Listen]. On queued channels, an event with no
// subscriber to take it, or arriving while [Listen] drains the queue, joins
// the queue. Errors and Done are neither stored nor queued.
func (c *EventChannel) dispatchEvent(data any) {
	c.mu.Lock()
	switch c.delivery {
	case deliverSticky:
		c.replayValid = true
		c.replayData = data
	case deliverQueued:
		if c.draining || len(c.subscriptions) == 0 {
			c.enqueueLocked(data)
			c.mu.Unlock()
			return
		}
	}
	subs := c.snapshotLocked()
	c.mu.Unlock()

	deliverEvent(subs, data)
}

// enqueueLocked appends data to the queue, dropping and reporting the
// oldest event when full. Caller holds mu.
func (c *EventChannel) enqueueLocked(data any) {
	if len(c.queue) == c.queueCap {
		c.queue = c.queue[1:]
		drifterrors.Report(&drifterrors.DriftError{
			Op:      "platform.queueEvent",
			Kind:    drifterrors.KindPlatform,
			Channel: c.name,
			Err:     fmt.Errorf("no subscriber and %d events queued; dropped the oldest", c.queueCap),
		})
	}
	c.queue = append(c.queue, data)
}

// drainQueue delivers queued events in order to the subscribers present at
// each delivery, without holding mu, so a handler may Listen or Cancel. It
// stops when the queue is empty, or keeps the rest queued if every
// subscriber left. Called by the Listen that set draining.
func (c *EventChannel) drainQueue() {
	for {
		c.mu.Lock()
		if len(c.queue) == 0 || len(c.subscriptions) == 0 {
			c.draining = false
			c.mu.Unlock()
			return
		}
		data := c.queue[0]
		c.queue[0] = nil
		c.queue = c.queue[1:]
		subs := c.snapshotLocked()
		c.mu.Unlock()

		deliverEvent(subs, data)
	}
}

// snapshotLocked copies the subscriber list. Caller holds mu.
func (c *EventChannel) snapshotLocked() []*Subscription {
	subs := make([]*Subscription, len(c.subscriptions))
	copy(subs, c.subscriptions)
	return subs
}

func deliverEvent(subs []*Subscription, data any) {
	for _, sub := range subs {
		if !sub.IsCanceled() && sub.handler.OnEvent != nil {
			sub.handler.OnEvent(data)
		}
	}
}

// dispatchError sends an error to all subscribers.
func (c *EventChannel) dispatchError(err error) {
	c.mu.Lock()
	subs := c.snapshotLocked()
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
	subs := c.snapshotLocked()
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
