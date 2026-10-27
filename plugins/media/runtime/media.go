// Package runtime is the Go API of the Drift media plugin
// (github.com/go-drift/drift/plugins/media/plugin in drift.yaml): video in
// the widget tree and audio players without a view.
//
//	import media "github.com/go-drift/drift/plugins/media/runtime"
//
//	// In a State's InitState:
//	s.video, err = media.NewVideoPlayerController()
//	if err != nil { ... }
//	core.UseDisposable(s, s.video)
//	core.UseListenable(s, s.video.State())
//	s.video.Load(url)
//
//	// In Build:
//	media.VideoPlayer{Controller: s.video, Height: 225}
//
// Controller methods are safe from any goroutine and do not wait for
// playback. A controller publishes playback as state, State and Progress,
// which change on the UI thread (so core.UseListenable rebuilds on them),
// and failures as the Errors stream.
package runtime

import (
	"errors"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
)

// PlaybackState is the playback state of a video or audio player.
type PlaybackState int

const (
	// PlaybackStateIdle: no media loaded, or stopped.
	PlaybackStateIdle PlaybackState = iota
	// PlaybackStateBuffering: waiting for media data before playing.
	PlaybackStateBuffering
	// PlaybackStatePlaying: playing.
	PlaybackStatePlaying
	// PlaybackStateCompleted: reached the end of the media.
	PlaybackStateCompleted
	// PlaybackStatePaused: paused; Play resumes.
	PlaybackStatePaused
)

func (s PlaybackState) String() string {
	switch s {
	case PlaybackStateIdle:
		return "Idle"
	case PlaybackStateBuffering:
		return "Buffering"
	case PlaybackStatePlaying:
		return "Playing"
	case PlaybackStateCompleted:
		return "Completed"
	case PlaybackStatePaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// Playback failures, the same on both platforms. Errors streams them as
// errors matching these under errors.Is.
var (
	// ErrSource indicates the media could not be loaded (network, URL,
	// format, container parsing).
	ErrSource = errors.New("media could not be loaded")
	// ErrDecoder indicates the media could not be decoded or rendered
	// (codec, audio track, DRM).
	ErrDecoder = errors.New("media could not be decoded")
	// ErrPlaybackFailed indicates any other playback failure.
	ErrPlaybackFailed = errors.New("media playback failed")
)

// errorCodes are the native halves' codes for the playback failures
// (plugin/ios/MediaErrors.swift, plugin/android/MediaErrors.kt).
var errorCodes = platform.ErrorCodes{
	"source_error":    ErrSource,
	"decoder_error":   ErrDecoder,
	"playback_failed": ErrPlaybackFailed,
}

// Progress is how far playback has got.
type Progress struct {
	// Position is the playback position.
	Position time.Duration
	// Duration is the media's duration.
	Duration time.Duration
	// Buffered is how far the media is buffered.
	Buffered time.Duration
}

// playback is what a controller publishes: its state, progress and
// failures. Both controllers embed it.
type playback struct {
	state        *core.Signal[PlaybackState]
	progress     *core.Signal[Progress]
	readState    *core.Derived[PlaybackState]
	readProgress *core.Derived[Progress]
	errors       *platform.Emitter[error]
}

func newPlayback(name string) playback {
	state := core.NewSignal(PlaybackStateIdle)
	progress := core.NewSignal(Progress{})
	return playback{
		state:        state,
		progress:     progress,
		readState:    core.NewDerived(state.Value, state),
		readProgress: core.NewDerived(progress.Value, progress),
		errors:       platform.NewEmitter[error](name),
	}
}

// State is the playback state. It changes on the UI thread.
func (p *playback) State() *core.Derived[PlaybackState] {
	return p.readState
}

// Progress is the playback position, duration and buffered length, updated
// about every 250ms while media is loaded, playing or paused, and at once
// when the state changes or a seek moves the position (Stop's return to
// the start included). It changes on the UI thread.
func (p *playback) Progress() *core.Derived[Progress] {
	return p.readProgress
}

// Errors streams playback failures, each matching ErrSource, ErrDecoder or
// ErrPlaybackFailed under errors.Is, with the platform's message. Like any
// stream, its handlers run off the UI thread: update widgets inside
// drift.Dispatch.
func (p *playback) Errors() *platform.Stream[error] {
	return p.errors.Stream()
}

// publish sets the state and progress on the UI thread, where their
// listeners may rebuild widgets. Without a UI thread yet (tests, package
// init), it sets them at once.
func (p *playback) publish(set func()) {
	if !platform.Dispatch(set) {
		set()
	}
}

func (p *playback) setState(state PlaybackState) {
	p.publish(func() { p.state.Set(state) })
}

func (p *playback) setProgress(progress Progress) {
	p.publish(func() { p.progress.Set(progress) })
}

// fail streams a native playback failure.
func (p *playback) fail(code, message string) {
	p.errors.Emit(errorCodes.NewError(code, message))
}

// The native halves send numbers as JSON numbers (float64 in Go).

func intArg(args map[string]any, key string) (int64, bool) {
	f, ok := args[key].(float64)
	return int64(f), ok && f == float64(int64(f))
}

func stringArg(args map[string]any, key string) (string, bool) {
	s, ok := args[key].(string)
	return s, ok
}
