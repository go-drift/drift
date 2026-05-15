package engine

import "github.com/go-drift/drift/pkg/platform"

// FrameEventsChannelName is the wire name of the rendering frame-events
// channel. Pinned as a constant so the test for FrameEvents can assert the
// literal string and catch typos that would otherwise be symmetric between
// the declaration and the subscriber.
const FrameEventsChannelName = "drift/rendering/frame_events"

// FrameEvents broadcasts rendering lifecycle events from the native side to
// Go-side subscribers. The channel is sticky (single-slot replay) so a
// subscriber that registers after the first event still observes it.
//
// Current events:
//   - {"type": "first_frame"} — fired once per process, after the first
//     non-empty layer tree is observably presented to the user (post-Metal
//     present on iOS via drawable.addPresentedHandler; post-GPU completion
//     on Android via HardwareRenderer.FrameCompleteCallback).
//
// Subscribers should treat the payload as map[string]any and dispatch on
// the "type" string. Sticky semantics ensure plugins (e.g. the splash
// plugin's overlay) can subscribe at any point during app startup without
// risking a missed first_frame.
var FrameEvents = platform.NewStickyEventChannel(FrameEventsChannelName)
