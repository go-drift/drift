package messaging

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/platform"
)

// send delivers data on channel as the native plugin would.
func send(t *testing.T, channel string, data any) {
	t.Helper()
	encoded, err := platform.DefaultCodec.Encode(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.HandleEvent(channel, encoded); err != nil {
		t.Fatal(err)
	}
}

func TestTokenFollowsNative(t *testing.T) {
	if got := Token().Value(); got != "" {
		t.Fatalf("initial token = %q, want empty", got)
	}
	changes := 0
	defer Token().AddListener(func() { changes++ })()

	send(t, tokenChannel, map[string]any{"token": "abc"})
	send(t, tokenChannel, map[string]any{"token": "abc"})
	send(t, tokenChannel, map[string]any{"token": "def"})

	if got := Token().Value(); got != "def" {
		t.Errorf("token = %q, want def", got)
	}
	if changes != 2 {
		t.Errorf("listener ran %d times, want 2 (once per change)", changes)
	}
}

// A tap that launched the app arrives before the app listens; it is
// delivered to the first listener, once.
func TestOpensQueueUntilListened(t *testing.T) {
	send(t, openedChannel, map[string]any{
		"id": "m1", "title": "", "body": "", "foreground": false,
		"data": map[string]any{"route": "/inbox"},
	})

	var got []Message
	unsub := Opens().Listen(platform.StreamHandler[Message]{OnEvent: func(m Message) { got = append(got, m) }})
	unsub()
	var again []Message
	defer Opens().Listen(platform.StreamHandler[Message]{OnEvent: func(m Message) { again = append(again, m) }})()

	want := []Message{{ID: "m1", Data: map[string]string{"route": "/inbox"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("first listener got %+v, want %+v", got, want)
	}
	if len(again) != 0 {
		t.Errorf("tap delivered twice: %+v", again)
	}
}

func TestMessagesParse(t *testing.T) {
	var got []Message
	defer Messages().Listen(platform.StreamHandler[Message]{OnEvent: func(m Message) { got = append(got, m) }})()

	send(t, messageChannel, map[string]any{
		"id": "m2", "title": "Hi", "body": "There", "foreground": true,
		"data": map[string]any{"k": "v"},
	})
	// Malformed: reported, not delivered.
	send(t, messageChannel, map[string]any{"title": "no id"})
	send(t, messageChannel, map[string]any{"id": "m3", "data": map[string]any{"n": 1}})

	want := []Message{{ID: "m2", Title: "Hi", Body: "There", Data: map[string]string{"k": "v"}, Foreground: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// fakeNative is the plugin's native half: it records calls, fails them
// with err if set, and deletes the token as the plugin does.
type fakeNative struct {
	t     *testing.T
	err   error
	calls []call
}

type call struct {
	method string
	args   any
}

func (f *fakeNative) InvokeMethod(channel, method string, args []byte, reply func([]byte, error)) {
	if channel != methodChannel {
		f.t.Fatalf("call on %q, want %q", channel, methodChannel)
	}
	decoded, err := platform.DefaultCodec.Decode(args)
	if err != nil {
		f.t.Fatal(err)
	}
	f.calls = append(f.calls, call{method, decoded})
	if f.err != nil {
		reply(nil, f.err)
		return
	}
	if method == "deleteToken" {
		send(f.t, tokenChannel, map[string]any{"token": ""})
	}
	reply(platform.DefaultCodec.Encode(nil))
}

func (f *fakeNative) StartEventStream(string) error { return nil }
func (f *fakeNative) StopEventStream(string) error  { return nil }

// useNative installs a fresh fakeNative. Not undone with
// platform.ResetForTest: that would also drop the package's own token
// subscription, made once in init.
func useNative(t *testing.T) *fakeNative {
	t.Helper()
	f := &fakeNative{t: t}
	platform.SetNativeBridge(f)
	return f
}

func TestTopicCalls(t *testing.T) {
	native := useNative(t)
	ctx := context.Background()
	if err := SubscribeToTopic(ctx, "news-EU_1.~%"); err != nil {
		t.Fatal(err)
	}
	if err := UnsubscribeFromTopic(ctx, "news-EU_1.~%"); err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"subscribeToTopic", map[string]any{"topic": "news-EU_1.~%"}},
		{"unsubscribeFromTopic", map[string]any{"topic": "news-EU_1.~%"}},
	}
	if !reflect.DeepEqual(native.calls, want) {
		t.Errorf("calls = %+v, want %+v", native.calls, want)
	}
}

// Names FCM rejects fail in Go, before reaching native.
func TestInvalidTopicsFailBeforeNative(t *testing.T) {
	native := useNative(t)
	ctx := context.Background()
	for _, topic := range []string{"", "a b", "/topics/news", "café", "a,b", strings.Repeat("a", 901)} {
		if err := SubscribeToTopic(ctx, topic); !errors.Is(err, ErrInvalidTopic) {
			t.Errorf("SubscribeToTopic(%q) = %v, want ErrInvalidTopic", topic, err)
		}
		if err := UnsubscribeFromTopic(ctx, topic); !errors.Is(err, ErrInvalidTopic) {
			t.Errorf("UnsubscribeFromTopic(%q) = %v, want ErrInvalidTopic", topic, err)
		}
	}
	if len(native.calls) != 0 {
		t.Errorf("native got calls for invalid topics: %+v", native.calls)
	}
	if err := SubscribeToTopic(ctx, strings.Repeat("a", 900)); err != nil {
		t.Errorf("900-character topic: %v", err)
	}
}

func TestNativeErrorsPassThrough(t *testing.T) {
	native := useNative(t)
	native.err = platform.NewChannelError("blocks_ui_thread", "called from the UI thread")
	if err := SubscribeToTopic(context.Background(), "news"); !errors.Is(err, platform.ErrBlocksUIThread) {
		t.Errorf("SubscribeToTopic = %v, want ErrBlocksUIThread", err)
	}
	if err := DeleteToken(context.Background()); !errors.Is(err, platform.ErrBlocksUIThread) {
		t.Errorf("DeleteToken = %v, want ErrBlocksUIThread", err)
	}
}

// After DeleteToken, Token stops reporting the deleted token, and reports
// the next one Firebase issues.
func TestDeleteTokenClearsToken(t *testing.T) {
	native := useNative(t)
	send(t, tokenChannel, map[string]any{"token": "old"})

	if err := DeleteToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := Token().Value(); got != "" {
		t.Errorf("token after DeleteToken = %q, want empty", got)
	}
	if want := []call{{"deleteToken", nil}}; !reflect.DeepEqual(native.calls, want) {
		t.Errorf("calls = %+v, want %+v", native.calls, want)
	}

	send(t, tokenChannel, map[string]any{"token": "new"})
	if got := Token().Value(); got != "new" {
		t.Errorf("token after reissue = %q, want new", got)
	}
}

// A failed deletion leaves the token as it was.
func TestDeleteTokenFailureKeepsToken(t *testing.T) {
	native := useNative(t)
	send(t, tokenChannel, map[string]any{"token": "kept"})
	native.err = platform.NewChannelError("native_error", "offline")
	if err := DeleteToken(context.Background()); err == nil {
		t.Fatal("DeleteToken succeeded, want the native error")
	}
	if got := Token().Value(); got != "kept" {
		t.Errorf("token = %q, want kept", got)
	}
}
