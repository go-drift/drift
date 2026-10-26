package main

import (
	"strconv"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
	media "github.com/go-drift/drift/plugins/media/runtime"
)

const audioURL = "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3"

// audioPlayerPage is the audio player demo widget.
type audioPlayerPage struct{ core.StatefulBase }

func (audioPlayerPage) CreateState() core.State { return &audioPlayerState{} }

func buildAudioPlayerPage(_ core.BuildContext) core.Widget { return audioPlayerPage{} }

type audioPlayerState struct {
	core.StateBase
	audioController *media.AudioPlayerController
	audioFailure    *core.Signal[string]
	audioLooping    bool
	audioMuted      bool
}

func (s *audioPlayerState) InitState() {
	s.audioController = media.NewAudioPlayerController()
	s.audioFailure = watchPlayback(&s.StateBase, s.audioController.State(), s.audioController.Progress(), s.audioController.Errors())
	core.UseDisposable(s, s.audioController)
	s.audioController.Load(audioURL)
}

func (s *audioPlayerState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Audio Player",
		widgets.Text{
			Content: "Standalone audio playback with no visual surface. Build your own UI with the controller.",
			Style:   labelStyle(colors),
		},
		widgets.VSpace(12),
		s.audioControls(ctx, colors),
		widgets.VSpace(12),
		statusCard(playbackStatus(s.audioController.State().Value(), s.audioController.Progress().Value(), s.audioFailure.Value()), colors),
		widgets.VSpace(40),
	)
}

func (s *audioPlayerState) audioControls(ctx core.BuildContext, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		MainAxisSize:       widgets.MainAxisSizeMin,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		Children: []core.Widget{
			// Song label
			widgets.Container{
				Color:        colors.SurfaceVariant,
				BorderRadius: 6,
				Padding:      layout.EdgeInsetsAll(10),
				Child: widgets.Text{
					Content: "SoundHelix Sample Song",
					Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 13,
					},
				},
			},
			widgets.VSpace(12),
			// Transport controls
			widgets.Row{
				MainAxisAlignment: widgets.MainAxisAlignmentStart,
				Children: []core.Widget{
					theme.ButtonOf(ctx, "Play", func() {
						s.audioController.Play()
					}),
					widgets.HSpace(8),
					theme.ButtonOf(ctx, "Pause", func() {
						s.audioController.Pause()
					}),
					widgets.HSpace(8),
					theme.ButtonOf(ctx, "Stop", func() {
						s.audioController.Stop()
					}),
				},
			},
			widgets.VSpace(8),
			// Seek controls
			widgets.Row{
				MainAxisAlignment: widgets.MainAxisAlignmentStart,
				Children: []core.Widget{
					smallButton(ctx, "Seek +10s", func() {
						pos := s.audioController.Progress().Value().Position
						s.audioController.SeekTo(pos + 10*time.Second)
					}, colors),
					widgets.HSpace(6),
					smallButton(ctx, "Seek -10s", func() {
						pos := s.audioController.Progress().Value().Position
						if pos > 10*time.Second {
							s.audioController.SeekTo(pos - 10*time.Second)
						} else {
							s.audioController.SeekTo(0)
						}
					}, colors),
				},
			},
			widgets.VSpace(8),
			// Playback speed
			widgets.Row{
				MainAxisAlignment: widgets.MainAxisAlignmentStart,
				Children: []core.Widget{
					smallButton(ctx, "0.5x", func() {
						s.audioController.SetPlaybackSpeed(0.5)
					}, colors),
					widgets.HSpace(6),
					smallButton(ctx, "1x", func() {
						s.audioController.SetPlaybackSpeed(1.0)
					}, colors),
					widgets.HSpace(6),
					smallButton(ctx, "1.5x", func() {
						s.audioController.SetPlaybackSpeed(1.5)
					}, colors),
					widgets.HSpace(6),
					smallButton(ctx, "2x", func() {
						s.audioController.SetPlaybackSpeed(2.0)
					}, colors),
				},
			},
			widgets.VSpace(8),
			// Volume and looping
			widgets.Row{
				MainAxisAlignment: widgets.MainAxisAlignmentStart,
				Children: []core.Widget{
					smallButton(ctx, toggleLabel("Mute", "Unmute", s.audioMuted), func() {
						s.audioMuted = !s.audioMuted
						if s.audioMuted {
							s.audioController.SetVolume(0)
						} else {
							s.audioController.SetVolume(1.0)
						}
					}, colors),
					widgets.HSpace(6),
					smallButton(ctx, toggleLabel("Loop", "Unloop", s.audioLooping), func() {
						s.audioLooping = !s.audioLooping
						s.audioController.SetLooping(s.audioLooping)
					}, colors),
				},
			},
		},
	}
}

func toggleLabel(off, on string, active bool) string {
	if active {
		return on
	}
	return off
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0:00"
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	secStr := strconv.Itoa(seconds)
	if seconds < 10 {
		secStr = "0" + secStr
	}
	return strconv.Itoa(minutes) + ":" + secStr
}

// watchPlayback rebuilds s on a player's state and progress, and returns
// the signal holding its last failure ("" while none), kept on screen
// since the player goes idle after one.
func watchPlayback(s *core.StateBase, state *core.Derived[media.PlaybackState], progress *core.Derived[media.Progress], errors *platform.Stream[error]) *core.Signal[string] {
	failure := core.NewSignal("")
	core.UseListenable(s, state)
	core.UseListenable(s, progress)
	core.UseListenable(s, failure)
	s.OnDispose(errors.Listen(platform.StreamHandler[error]{OnEvent: func(err error) {
		drift.Dispatch(func() { failure.Set(err.Error()) })
	}}))
	return failure
}

// playbackStatus describes a player's state for the status card.
func playbackStatus(state media.PlaybackState, progress media.Progress, failure string) string {
	if failure != "" {
		return "Error: " + failure
	}
	if progress.Duration <= 0 {
		return state.String()
	}
	return state.String() + " \u00b7 " + formatDuration(progress.Position) + " / " + formatDuration(progress.Duration)
}
