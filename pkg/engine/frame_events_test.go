package engine

import "testing"

// TestFrameEventsChannelNameLiteral pins the wire name as a literal string.
// The contract is that FrameEvents.Name() exactly equals
// "drift/rendering/frame_events"; native subscribers and Go-side subscribers
// must use the same key. A typo in either FrameEventsChannelName or this
// literal would otherwise pass silently — we guard against that here.
func TestFrameEventsChannelNameLiteral(t *testing.T) {
	const want = "drift/rendering/frame_events"
	if got := FrameEvents.Name(); got != want {
		t.Errorf("FrameEvents.Name() = %q, want %q", got, want)
	}
	if FrameEventsChannelName != want {
		t.Errorf("FrameEventsChannelName = %q, want %q", FrameEventsChannelName, want)
	}
}

// TestFrameEventsIsSticky guards against the FrameEvents declaration ever
// flipping back to NewEventChannel. Sticky semantics are the entire reason
// this channel exists — late-subscribing plugins (splash, perf, analytics)
// cannot afford to miss first_frame. The end-to-end sticky behaviour is
// exercised in pkg/platform/channel_sticky_test.go; here we just verify the
// constructor choice at the FrameEvents seam.
func TestFrameEventsIsSticky(t *testing.T) {
	if !FrameEvents.IsSticky() {
		t.Error("FrameEvents must be constructed via NewStickyEventChannel; " +
			"plugins like splash subscribe after first_frame may have already fired")
	}
}
