package messaging

import (
	"reflect"
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
	unsub := Opens().Listen(func(m Message) { got = append(got, m) })
	unsub()
	var again []Message
	defer Opens().Listen(func(m Message) { again = append(again, m) })()

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
	defer Messages().Listen(func(m Message) { got = append(got, m) })()

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
