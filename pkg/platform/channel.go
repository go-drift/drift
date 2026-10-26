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
	codes   ErrorCodes
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

// WithErrors declares the channel's own error codes (see [ErrorCodes]) and
// returns the channel. Call it once, where the channel is declared, before
// any call; a second call panics, as does a code repeating a transport
// code.
func (c *MethodChannel) WithErrors(codes ErrorCodes) *MethodChannel {
	if c.codes != nil {
		panic(fmt.Sprintf("platform: method channel %q declares its error codes twice", c.name))
	}
	codes.check(c.name)
	c.codes = codes
	return c
}

// SetHandler sets the handler for incoming method calls from native code.
func (c *MethodChannel) SetHandler(handler MethodHandler) {
	c.handler = handler
}

// Invoke calls a method on the native side and returns the result.
// Blocks until the native side responds, an error occurs, or ctx is canceled.
// See [invokeNative] for the ctx cancellation contract. A native error is a
// [*ChannelError] matching the channel's [ErrorCodes] under [errors.Is].
func (c *MethodChannel) Invoke(ctx context.Context, method string, args any) (any, error) {
	result, err := invokeNative(ctx, c.name, method, args)
	return result, c.codes.attach(err)
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
//
// Native is told when the channel gains its first subscriber and loses its
// last, so a stream it declares (DriftChannel.stream) starts and stops with
// Go's interest; see [streamsChannel].
type EventChannel struct {
	name          string
	codec         MessageCodec
	codes         ErrorCodes
	delivery      eventDelivery
	subscriptions []*Subscription
	mu            sync.Mutex

	// native is false for an Emitter's channel, which native never
	// addresses, so its subscriptions are not reported.
	native bool
	// generation numbers the listen and cancel requests sent to native,
	// which applies only the newest: requests from different goroutines
	// can reach native out of order. Protected by mu.
	generation int64

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
		native:   true,
		codec:    DefaultCodec,
		delivery: delivery,
		queueCap: queueCap,
	}
	registry.registerEvent(name, ch)
	return ch
}

// WithErrors declares the error codes native streams on the channel (see
// [ErrorCodes]) and returns the channel, so stream errors match their
// sentinels under [errors.Is]. Call it once, where the channel is declared,
// before any subscription; a second call panics, as does a code repeating
// a transport code.
func (c *EventChannel) WithErrors(codes ErrorCodes) *EventChannel {
	if c.codes != nil {
		panic(fmt.Sprintf("platform: event channel %q declares its error codes twice", c.name))
	}
	codes.check(c.name)
	c.codes = codes
	return c
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
func (c *EventChannel) Listen(handler EventHandler) *Subscription {
	sub := &Subscription{
		channel: c,
		handler: &handler,
	}

	c.mu.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	listen := c.native && len(c.subscriptions) == 1
	var generation int64
	if listen {
		generation = c.nextGenerationLocked()
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

	if listen {
		c.tellNative(streamListen, generation)
	}

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

	return sub
}

// removeSubscription removes a subscription from the channel, telling
// native when it was the last.
func (c *EventChannel) removeSubscription(sub *Subscription) {
	c.mu.Lock()
	cancel := false
	var generation int64
	for i, s := range c.subscriptions {
		if s == sub {
			c.subscriptions = append(c.subscriptions[:i], c.subscriptions[i+1:]...)
			if cancel = c.native && len(c.subscriptions) == 0; cancel {
				generation = c.nextGenerationLocked()
			}
			break
		}
	}
	c.mu.Unlock()
	if cancel {
		c.tellNative(streamCancel, generation)
	}
}

// streamsChannel is the native channel Go tells, with streamListen and
// streamCancel, when an event channel gains its first subscriber and loses
// its last; native runs the stream handler declared on the channel of the
// same name. Each request carries the channel and a generation, newest
// wins. Native answers on the main thread, and starts or stops the stream
// after replying.
const (
	streamsChannel = "drift/streams"
	streamListen   = "listen"
	streamCancel   = "cancel"
)

// nextGenerationLocked numbers a request to native. Caller holds mu.
func (c *EventChannel) nextGenerationLocked() int64 {
	c.generation++
	return c.generation
}

// tellNative sends a listen or cancel request for the channel, without
// holding mu: a call from a goroutine waits for the main thread, whose Go
// code may need mu, and a call from the UI thread is answered in place. Before native is connected the
// request fails with ErrPlatformUnavailable; [HandleNativeConnected]
// replays what is due then.
func (c *EventChannel) tellNative(method string, generation int64) {
	_, err := invokeNative(context.Background(), streamsChannel, method, map[string]any{
		"channel":    c.name,
		"generation": generation,
	})
	if err != nil && !errors.Is(err, ErrPlatformUnavailable) {
		drifterrors.Report(&drifterrors.DriftError{
			Op:      "platform.stream." + method,
			Kind:    drifterrors.KindPlatform,
			Channel: c.name,
			Err:     err,
		})
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

// dispatchError sends an error to all subscribers, a native one matching
// the channel's [ErrorCodes].
func (c *EventChannel) dispatchError(err error) {
	err = c.codes.attach(err)
	c.mu.Lock()
	subs := c.snapshotLocked()
	c.mu.Unlock()

	for _, sub := range subs {
		if !sub.IsCanceled() && sub.handler.OnError != nil {
			sub.handler.OnError(err)
		}
	}
}

// dispatchDone notifies all subscribers that the stream has ended. Native
// ended it, so no cancel is sent: native's stream bookkeeping resets on
// sendEventDone, and the next subscriber starts it again.
func (c *EventChannel) dispatchDone() {
	c.mu.Lock()
	subs := c.snapshotLocked()
	c.subscriptions = nil
	c.mu.Unlock()

	for _, sub := range subs {
		sub.canceled.Store(true)
		if sub.handler.OnDone != nil {
			sub.handler.OnDone()
		}
	}
}
