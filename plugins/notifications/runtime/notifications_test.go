package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/platform"
)

// fakeNative records each call's method and arguments and answers with
// reply.
type fakeNative struct {
	reply any
	calls []call
}

type call struct {
	method string
	args   map[string]any
}

func (f *fakeNative) InvokeMethod(_, method string, args []byte, reply func([]byte, error)) {
	decoded, _ := platform.DefaultCodec.Decode(args)
	m, _ := decoded.(map[string]any)
	f.calls = append(f.calls, call{method, m})
	reply(platform.DefaultCodec.Encode(f.reply))
}

func (f *fakeNative) StartEventStream(string) error { return nil }
func (f *fakeNative) StopEventStream(string) error  { return nil }

func useNative(t *testing.T, f *fakeNative) {
	t.Helper()
	platform.SetNativeBridge(f)
	platform.RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(platform.ResetForTest)
}

func TestScheduleSendsRequest(t *testing.T) {
	native := &fakeNative{}
	useNative(t, native)
	at := time.UnixMilli(1_900_000_000_000)
	badge := 3
	err := Schedule(Request{ID: "r1", Title: "T", Body: "B", Data: map[string]any{"route": "/x"}, At: at, Badge: &badge})
	if err != nil {
		t.Fatal(err)
	}
	if len(native.calls) != 1 || native.calls[0].method != "schedule" {
		t.Fatalf("calls = %+v", native.calls)
	}
	args := native.calls[0].args
	if args["id"] != "r1" || args["at"] != float64(at.UnixMilli()) || args["badge"] != float64(3) ||
		args["data"].(map[string]any)["route"] != "/x" {
		t.Errorf("args = %v", args)
	}
}

// Combinations the platforms cannot both honour fail before reaching native.
func TestScheduleRejects(t *testing.T) {
	native := &fakeNative{}
	useNative(t, native)
	for _, tc := range []struct {
		req  Request
		want string
	}{
		{Request{}, "ID"},
		{Request{ID: "a", Interval: -time.Second}, "negative"},
		{Request{ID: "a", Interval: 1500 * time.Millisecond}, "whole seconds"},
		{Request{ID: "a", Repeats: true, Interval: 30 * time.Second}, "at least"},
		{Request{ID: "a", Repeats: true, Interval: time.Minute, At: time.Now()}, "At"},
	} {
		if err := Schedule(tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Schedule(%+v) = %v, want an error mentioning %q", tc.req, err, tc.want)
		}
	}
	if len(native.calls) != 0 {
		t.Errorf("native called for invalid requests: %+v", native.calls)
	}
}

func TestCurrentSettings(t *testing.T) {
	useNative(t, &fakeNative{reply: map[string]any{"alertsEnabled": true, "soundsEnabled": false, "badgesEnabled": true}})
	got, err := CurrentSettings(context.Background())
	if err != nil || got != (Settings{AlertsEnabled: true, BadgesEnabled: true}) {
		t.Errorf("CurrentSettings = %+v, %v", got, err)
	}
	useNative(t, &fakeNative{reply: map[string]any{"alertsEnabled": true}})
	if _, err := CurrentSettings(context.Background()); err == nil {
		t.Error("partial settings accepted")
	}
}

// A tap that launched the app arrives before the app listens; it is
// delivered to the first Opens listener, once.
func TestOpensQueueUntilListened(t *testing.T) {
	useNative(t, &fakeNative{})
	encoded, err := platform.DefaultCodec.Encode(map[string]any{"id": "n1", "data": map[string]any{"route": "/inbox"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.HandleEvent(openedChannel, encoded); err != nil {
		t.Fatal(err)
	}

	var got []Open
	unsub := Opens().Listen(func(o Open) { got = append(got, o) })
	defer unsub()
	if len(got) != 1 || got[0].ID != "n1" || got[0].Data["route"] != "/inbox" {
		t.Fatalf("first listener got %+v, want the queued tap n1", got)
	}

	var later []Open
	unsubLater := Opens().Listen(func(o Open) { later = append(later, o) })
	defer unsubLater()
	if len(later) != 0 {
		t.Errorf("later listener got %+v, want nothing (queued taps are delivered once)", later)
	}
}

func TestParseNotification(t *testing.T) {
	n, err := parseNotification(map[string]any{"id": "n", "title": "T", "foreground": true})
	if err != nil || n.ID != "n" || n.Title != "T" || !n.Foreground {
		t.Errorf("parse = %+v, %v", n, err)
	}
	if _, err := parseNotification(map[string]any{"title": "no id"}); err == nil {
		t.Error("notification without id accepted")
	}
}
