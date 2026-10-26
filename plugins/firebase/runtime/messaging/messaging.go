// Package messaging is the Go API for Firebase Cloud Messaging, from the
// Drift Firebase plugin (github.com/go-drift/drift/plugins/firebase/plugin
// in drift.yaml).
//
// Reading never blocks: the token is state, and messages are streams.
// Topic subscriptions and DeleteToken wait for Firebase's servers, so call
// them from a goroutine: from the UI thread they fail with
// platform.ErrBlocksUIThread.
//
//	import "github.com/go-drift/drift/plugins/firebase/runtime/messaging"
//
//	// Send the token to your server whenever it changes.
//	messaging.Token().AddListener(func() {
//	    go register(messaging.Token().Value())
//	})
//
//	// A message received while the app runs.
//	unsub := messaging.Messages().Listen(platform.StreamHandler[messaging.Message]{OnEvent: func(m messaging.Message) {
//	    drift.Dispatch(func() { showBanner(m) })
//	}})
//
//	// A notification the user tapped, including the one that launched the
//	// app: taps are queued until the app listens.
//	messaging.Opens().Listen(platform.StreamHandler[messaging.Message]{OnEvent: func(m messaging.Message) {
//	    drift.Dispatch(func() { navigate(m.Data["route"]) })
//	}})
//
//	// Messages sent to a topic.
//	go func() {
//	    if err := messaging.SubscribeToTopic(ctx, "news"); err != nil { ... }
//	}()
//
//	// On logout: no more messages for this user.
//	go func() {
//	    if err := messaging.DeleteToken(ctx); err != nil { ... }
//	}()
//
// Showing notifications needs the user's permission:
// platform.Notifications.Permission.Request. On Android, notification
// messages that arrive while the app is in the background are shown by the
// system; in the foreground they reach Messages only, on both platforms.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/go-drift/drift/pkg/core"
	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// Wire channels. Must match the literals in the plugin's native sources
// (plugin/ios/DriftFirebasePlugin.swift, plugin/android/*.kt).
const (
	methodChannel  = "drift/firebase/messaging"
	tokenChannel   = "drift/firebase/messaging/token"
	messageChannel = "drift/firebase/messaging/message"
	openedChannel  = "drift/firebase/messaging/opened"
)

// queueCapacity bounds the messages and taps kept while the app has no
// listener.
const queueCapacity = 32

// Message is a Firebase Cloud Messaging message.
type Message struct {
	// ID is the FCM message ID.
	ID string
	// Title and Body are the notification's, empty for data-only messages
	// and for taps on Android, where the system showed them.
	Title string
	Body  string
	// Data is the message's custom key/value data.
	Data map[string]string
	// Foreground reports whether a received message arrived while the app
	// was in the foreground. Always false for taps.
	Foreground bool
}

// ErrInvalidTopic indicates a topic name FCM does not accept.
var ErrInvalidTopic = errors.New("invalid FCM topic name")

// topicName is FCM's topic name syntax, as its Android SDK checks it.
var topicName = regexp.MustCompile(`^[a-zA-Z0-9\-_.~%]{1,900}$`)

var (
	methods = platform.NewMethodChannel(methodChannel)
	token   = core.NewSignal("")
	// readToken is token, read-only for apps.
	readToken = core.NewDerived(token.Value, token)
	messages  = platform.NewStream(messageChannel,
		platform.NewQueuedEventChannel(messageChannel, queueCapacity), parseMessage(messageChannel))
	opens = platform.NewStream(openedChannel,
		platform.NewQueuedEventChannel(openedChannel, queueCapacity), parseMessage(openedChannel))
)

func init() {
	tokens := platform.NewStream(tokenChannel, platform.NewEventChannel(tokenChannel), parseToken)
	tokens.Listen(platform.StreamHandler[string]{OnEvent: setToken})
}

// setToken sets token on the UI thread, where listeners such as
// core.UseListenable may rebuild widgets. Before the app runs there is no
// UI thread to wait for, so it sets token at once.
func setToken(t string) {
	if !platform.Dispatch(func() { token.Set(t) }) {
		token.Set(t)
	}
}

// waitForToken returns once every token change dispatched so far is set:
// the dispatch queue runs in order, so a later callback marks it. It stops
// waiting when ctx is done.
func waitForToken(ctx context.Context) error {
	done := make(chan struct{})
	if !platform.Dispatch(func() { close(done) }) {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Token is this installation's FCM registration token, "" while there is
// none: until Firebase issues one, and after DeleteToken. It changes when
// Firebase rotates it; listen to send the new one to your server. It
// changes on the UI thread, so a widget can rebuild on it with
// core.UseListenable.
func Token() *core.Derived[string] {
	return readToken
}

// Messages streams messages received while the app runs: data messages,
// and notification messages in the foreground. Messages received while no
// listener is subscribed are queued for the next one.
func Messages() *platform.Stream[Message] {
	return messages
}

// Opens streams notifications the user tapped, including the one that
// launched the app. Taps while no listener is subscribed are queued for the
// next one, so subscribe once the app can act on them.
func Opens() *platform.Stream[Message] {
	return opens
}

// SubscribeToTopic subscribes this installation to topic, so it receives
// the messages sent to it. A topic name is 1 to 900 characters from a-z,
// A-Z, 0-9 and "-_.~%"; any other fails with ErrInvalidTopic.
//
// It waits for Firebase's servers, so call it from a goroutine. On ctx
// cancellation it returns ctx's error; the subscription may still
// complete.
func SubscribeToTopic(ctx context.Context, topic string) error {
	return topicCall(ctx, "subscribeToTopic", topic)
}

// UnsubscribeFromTopic undoes SubscribeToTopic. Topic names, waiting and
// cancellation are as for SubscribeToTopic.
func UnsubscribeFromTopic(ctx context.Context, topic string) error {
	return topicCall(ctx, "unsubscribeFromTopic", topic)
}

// DeleteToken deletes this installation's FCM token, so neither messages
// sent to it nor topic messages arrive: call it on logout. Token is ""
// when it returns. Firebase issues a new token by the time the app next
// starts, which Token reports like any new token; send it to your server,
// and subscribe to the user's topics again, only once a user logs in.
//
// It waits for Firebase's servers, then for the UI thread to apply the
// change, so call it from a goroutine. On ctx cancellation it returns ctx's
// error; the deletion may still complete.
func DeleteToken(ctx context.Context) error {
	// The native half sends "" on tokenChannel before replying, so once the
	// change it dispatched is set, Token is "", ordered before any token
	// Firebase issues after.
	if err := invoke(ctx, "deleteToken", nil); err != nil {
		return err
	}
	if err := waitForToken(ctx); err != nil {
		return fmt.Errorf("messaging deleteToken: %w", err)
	}
	return nil
}

func topicCall(ctx context.Context, method, topic string) error {
	if !topicName.MatchString(topic) {
		return fmt.Errorf("messaging %s: %w %q: want 1 to 900 characters from a-z, A-Z, 0-9 and \"-_.~%%\"",
			method, ErrInvalidTopic, topic)
	}
	return invoke(ctx, method, map[string]any{"topic": topic})
}

func invoke(ctx context.Context, method string, args map[string]any) error {
	if _, err := methods.Invoke(ctx, method, args); err != nil {
		return fmt.Errorf("messaging %s: %w", method, err)
	}
	return nil
}

// parseToken parses a token event: the token, or "" once DeleteToken
// deleted it.
func parseToken(data any) (string, error) {
	m, ok := data.(map[string]any)
	token, isString := m["token"].(string)
	if !ok || !isString {
		return "", &drifterrors.ParseError{Channel: tokenChannel, DataType: "token", Got: data}
	}
	return token, nil
}

func parseMessage(channel string) func(data any) (Message, error) {
	return func(data any) (Message, error) {
		m, ok := data.(map[string]any)
		if !ok {
			return Message{}, &drifterrors.ParseError{Channel: channel, DataType: "Message", Got: data}
		}
		id, _ := m["id"].(string)
		if id == "" {
			return Message{}, &drifterrors.ParseError{Channel: channel, DataType: "Message", Got: data}
		}
		msg := Message{ID: id, Data: map[string]string{}}
		msg.Title, _ = m["title"].(string)
		msg.Body, _ = m["body"].(string)
		msg.Foreground, _ = m["foreground"].(bool)
		if raw, ok := m["data"].(map[string]any); ok {
			for k, v := range raw {
				s, ok := v.(string)
				if !ok {
					return Message{}, &drifterrors.ParseError{
						Channel: channel, DataType: "Message",
						Got: fmt.Sprintf("data[%q] = %#v, want a string", k, v),
					}
				}
				msg.Data[k] = s
			}
		}
		return msg, nil
	}
}
