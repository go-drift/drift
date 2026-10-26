package platform

import "github.com/go-drift/drift/pkg/errors"

// Stream is a typed broadcast of platform events: every subscriber receives
// every event and error independently. Native events arrive through an
// [EventChannel] ([NewStream]); Go code can feed one too ([NewEmitter]).
//
// Handlers run on the goroutine that delivers the event, which need not be
// the UI thread: update widgets inside drift.Dispatch.
type Stream[T any] struct {
	eventChannel *EventChannel
	channelName  string
	parser       func(data any) (T, error)
}

// StreamHandler receives a [Stream]'s events and errors.
type StreamHandler[T any] struct {
	// OnEvent receives each event.
	OnEvent func(T)
	// OnError receives each error the stream reports, such as a native
	// failure mid-stream (a revoked permission); for a native stream it is
	// a [*ChannelError] matching the channel's [ErrorCodes]. When nil, the
	// error is reported through Drift's error handling instead.
	OnError func(error)
}

// Listen subscribes handler to the stream and returns the function that
// unsubscribes it.
//
// An event the parser rejects is malformed native data, a bug on the
// native side: it is reported through Drift's error handling (KindParsing),
// never delivered.
func (s *Stream[T]) Listen(handler StreamHandler[T]) (unsubscribe func()) {
	onError := handler.OnError
	if onError == nil {
		onError = func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "stream.error",
				Kind:    errors.KindPlatform,
				Channel: s.channelName,
				Err:     err,
			})
		}
	}
	sub := s.eventChannel.Listen(EventHandler{
		OnEvent: func(data any) {
			val, err := s.parser(data)
			if err != nil {
				errors.Report(&errors.DriftError{
					Op:      "stream.parse",
					Kind:    errors.KindParsing,
					Channel: s.channelName,
					Err:     err,
				})
				return
			}
			if handler.OnEvent != nil {
				handler.OnEvent(val)
			}
		},
		OnError: onError,
	})
	return sub.Cancel
}

// NewStream creates a Stream wrapping an EventChannel.
// The parser converts raw event data to the typed value, returning error on parse failure.
func NewStream[T any](name string, channel *EventChannel, parser func(data any) (T, error)) *Stream[T] {
	return &Stream[T]{
		eventChannel: channel,
		channelName:  name,
		parser:       parser,
	}
}

// Emitter feeds a [Stream] from Go, for events that do not arrive on an
// event channel of their own: a plugin's runtime half that receives one
// view's or one player's events and republishes them, say. Delivery is
// live: subscribers receive what is emitted after they listen.
type Emitter[T any] struct {
	channel *EventChannel
	stream  *Stream[T]
}

// NewEmitter returns an emitter and its stream. name identifies the stream
// in error reports.
func NewEmitter[T any](name string) *Emitter[T] {
	// Not registered: native never addresses it.
	channel := &EventChannel{name: name, codec: DefaultCodec, delivery: deliverLive}
	return &Emitter[T]{
		channel: channel,
		stream: NewStream(name, channel, func(data any) (T, error) {
			return data.(T), nil
		}),
	}
}

// Stream returns the stream the emitter feeds.
func (e *Emitter[T]) Stream() *Stream[T] {
	return e.stream
}

// Emit delivers value to the stream's subscribers, on the calling
// goroutine.
func (e *Emitter[T]) Emit(value T) {
	e.channel.dispatchEvent(value)
}

// Error delivers err to the stream's subscribers (their OnError), on the
// calling goroutine.
func (e *Emitter[T]) Error(err error) {
	e.channel.dispatchError(err)
}
