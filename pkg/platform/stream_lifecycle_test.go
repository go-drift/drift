package platform

import (
	"sync"
	"testing"
)

// streamRequest is one listen or cancel request Go sent to native.
type streamRequest struct {
	method     string
	channel    string
	generation int64
}

// streamsBridge records the requests on drift/streams and answers every
// call with nil.
type streamsBridge struct {
	mu       sync.Mutex
	requests []streamRequest
}

func (b *streamsBridge) InvokeMethod(channel, method string, args []byte, reply func([]byte, error)) {
	if channel == streamsChannel {
		decoded, _ := DefaultCodec.Decode(args)
		m := decoded.(map[string]any)
		b.mu.Lock()
		b.requests = append(b.requests, streamRequest{method, m["channel"].(string), int64(m["generation"].(float64))})
		b.mu.Unlock()
	}
	reply(DefaultCodec.Encode(nil))
}

// take returns the requests for channel since the last take.
func (b *streamsBridge) take(channel string) []streamRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	var mine, rest []streamRequest
	for _, r := range b.requests {
		if r.channel == channel {
			mine = append(mine, r)
		} else {
			rest = append(rest, r)
		}
	}
	b.requests = rest
	return mine
}

func expectRequests(t *testing.T, what string, got []streamRequest, methods ...string) {
	t.Helper()
	if len(got) != len(methods) {
		t.Fatalf("%s: requests = %+v, want %v", what, got, methods)
	}
	for i, m := range methods {
		if got[i].method != m {
			t.Errorf("%s: requests = %+v, want %v", what, got, methods)
		}
	}
}

func TestEventChannelTellsNativeAboutFirstAndLastSubscriber(t *testing.T) {
	t.Cleanup(ResetForTest)
	bridge := &streamsBridge{}
	SetNativeBridge(bridge)
	const name = "test/lifecycle"
	ch := NewEventChannel(name)

	a := ch.Listen(EventHandler{})
	b := ch.Listen(EventHandler{})
	expectRequests(t, "two subscribers", bridge.take(name), streamListen)
	a.Cancel()
	expectRequests(t, "one left", bridge.take(name))
	b.Cancel()
	b.Cancel()
	expectRequests(t, "none left", bridge.take(name), streamCancel)

	ch.Listen(EventHandler{})
	got := bridge.take(name)
	expectRequests(t, "listening again", got, streamListen)
	if got[0].generation != 3 {
		t.Errorf("generation = %d, want 3: every request numbered in order", got[0].generation)
	}
}

// Native ended the stream itself, so Go sends no cancel; the next
// subscriber starts it again.
func TestEventChannelDoneSendsNoCancel(t *testing.T) {
	t.Cleanup(ResetForTest)
	bridge := &streamsBridge{}
	SetNativeBridge(bridge)
	const name = "test/lifecycle_done"
	ch := NewEventChannel(name)
	ch.Listen(EventHandler{})
	if err := HandleEventDone(name); err != nil {
		t.Fatal(err)
	}
	expectRequests(t, "done", bridge.take(name), streamListen)
	ch.Listen(EventHandler{})
	expectRequests(t, "after done", bridge.take(name), streamListen)
}

func TestEmitterTellsNativeNothing(t *testing.T) {
	t.Cleanup(ResetForTest)
	bridge := &streamsBridge{}
	SetNativeBridge(bridge)
	NewEmitter[int]("test/lifecycle_emitter").Stream().Listen(StreamHandler[int]{})()
	if got := bridge.take("test/lifecycle_emitter"); len(got) != 0 {
		t.Errorf("requests = %+v, want none", got)
	}
}

// Subscriptions made before native connected (package init) reach native
// once it does.
func TestHandleNativeConnectedReplaysListens(t *testing.T) {
	t.Cleanup(ResetForTest)
	SetNativeBridge(nil)
	const name = "test/lifecycle_early"
	ch := NewEventChannel(name)
	ch.Listen(EventHandler{})
	idle := NewEventChannel("test/lifecycle_idle")
	idle.Listen(EventHandler{}).Cancel()

	bridge := &streamsBridge{}
	SetNativeBridge(bridge)
	HandleNativeConnected()
	expectRequests(t, "subscribed early", bridge.take(name), streamListen)
	expectRequests(t, "no longer subscribed", bridge.take("test/lifecycle_idle"))
}

// A request native refuses is reported; one sent before native is
// connected is not.
func TestStreamRequestFailuresAreReported(t *testing.T) {
	t.Cleanup(ResetForTest)
	reported := captureReports(t)
	SetNativeBridge(failingBridge{NewChannelError("native_error", "refused")})
	NewEventChannel("test/lifecycle_refused").Listen(EventHandler{})
	if len(*reported) != 1 || (*reported)[0].Channel != "test/lifecycle_refused" {
		t.Errorf("reported = %v, want the refused listen", *reported)
	}
	SetNativeBridge(nil)
	NewEventChannel("test/lifecycle_unconnected").Listen(EventHandler{})
	if len(*reported) != 1 {
		t.Errorf("reported = %v, want nothing more before native connects", *reported)
	}
}
