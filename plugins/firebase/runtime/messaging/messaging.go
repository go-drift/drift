// Package messaging is the Go API for Firebase Cloud Messaging, from the
// Drift Firebase plugin (github.com/go-drift/drift/plugins/firebase/plugin
// in drift.yaml).
//
// Nothing here blocks: the token is state, and messages are streams.
//
//	import "github.com/go-drift/drift/plugins/firebase/runtime/messaging"
//
//	// Send the token to your server whenever it changes.
//	messaging.Token().AddListener(func() {
//	    go register(messaging.Token().Value())
//	})
//
//	// A message received while the app runs.
//	unsub := messaging.Messages().Listen(func(m messaging.Message) {
//	    drift.Dispatch(func() { showBanner(m) })
//	})
//
//	// A notification the user tapped, including the one that launched the
//	// app: taps are queued until the app listens.
//	messaging.Opens().Listen(func(m messaging.Message) {
//	    drift.Dispatch(func() { navigate(m.Data["route"]) })
//	})
//
// Showing notifications needs the user's permission:
// platform.Notifications.Permission.Request. On Android, notification
// messages that arrive while the app is in the background are shown by the
// system; in the foreground they reach Messages only, on both platforms.
package messaging

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// Wire channels. Must match the literals in the plugin's native sources
// (plugin/ios/DriftFirebasePlugin.swift, plugin/android/*.kt).
const (
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

var (
	token = core.NewSignal("")
	// readToken is token, read-only for apps.
	readToken = core.NewDerived(token.Value, token)
	messages  = platform.NewStream(messageChannel,
		platform.NewQueuedEventChannel(messageChannel, queueCapacity), parseMessage(messageChannel))
	opens = platform.NewStream(openedChannel,
		platform.NewQueuedEventChannel(openedChannel, queueCapacity), parseMessage(openedChannel))
)

func init() {
	tokens := platform.NewStream(tokenChannel, platform.NewEventChannel(tokenChannel), parseToken)
	tokens.Listen(token.Set)
}

// Token is this installation's FCM registration token, "" until Firebase
// has one. It changes when Firebase rotates it; listen to send the new one
// to your server.
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

func parseToken(data any) (string, error) {
	m, ok := data.(map[string]any)
	token, isString := m["token"].(string)
	if !ok || !isString || token == "" {
		return "", &errors.ParseError{Channel: tokenChannel, DataType: "token", Got: data}
	}
	return token, nil
}

func parseMessage(channel string) func(data any) (Message, error) {
	return func(data any) (Message, error) {
		m, ok := data.(map[string]any)
		if !ok {
			return Message{}, &errors.ParseError{Channel: channel, DataType: "Message", Got: data}
		}
		id, _ := m["id"].(string)
		if id == "" {
			return Message{}, &errors.ParseError{Channel: channel, DataType: "Message", Got: data}
		}
		msg := Message{ID: id, Data: map[string]string{}}
		msg.Title, _ = m["title"].(string)
		msg.Body, _ = m["body"].(string)
		msg.Foreground, _ = m["foreground"].(bool)
		if raw, ok := m["data"].(map[string]any); ok {
			for k, v := range raw {
				s, ok := v.(string)
				if !ok {
					return Message{}, &errors.ParseError{
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
