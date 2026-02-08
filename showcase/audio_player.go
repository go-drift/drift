package main

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

const audioURL = "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3"

func buildAudioPlayerPage(ctx core.BuildContext) core.Widget {
	return audioPlayerPage{}
}

type audioPlayerPage struct{}

func (a audioPlayerPage) CreateElement() core.Element {
	return core.NewStatefulElement(a, nil)
}

func (a audioPlayerPage) Key() any {
	return nil
}

func (a audioPlayerPage) CreateState() core.State {
	return &audioPlayerState{}
}

type audioPlayerState struct {
	core.StateBase
	audioStatus     *core.ManagedState[string]
	audioStateLabel string
	audioController *platform.AudioPlayerController
	audioLooping    bool
	audioMuted      bool
}

func (s *audioPlayerState) InitState() {
	s.audioStatus = core.NewManagedState(&s.StateBase, "Idle")
	s.audioStateLabel = "Idle"

	s.audioController = core.UseController(&s.StateBase, platform.NewAudioPlayerController)

	s.audioController.OnPlaybackStateChanged = func(state platform.PlaybackState) {
		s.audioStateLabel = state.String()
		s.audioStatus.Set(s.audioStateLabel)
	}
	s.audioController.OnPositionChanged = func(position, duration, buffered time.Duration) {
		pos := formatDuration(position)
		dur := formatDuration(duration)
		s.audioStatus.Set(s.audioStateLabel + " \u00b7 " + pos + " / " + dur)
	}
	s.audioController.OnError = func(code, message string) {
		s.audioStateLabel = "Error"
		s.audioStatus.Set("Error (" + code + "): " + message)
	}

	s.audioController.Load(audioURL)
}

func (s *audioPlayerState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Audio Player",
		widgets.Text{
			Content: "Standalone audio playback with no visual surface. Build your own UI with the controller.",
			Wrap:    true,
			Style:   labelStyle(colors),
		},
		widgets.VSpace(12),
		s.audioControls(ctx, colors),
		widgets.VSpace(12),
		statusCard(s.audioStatus.Get(), colors),
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
				Child: widgets.PaddingAll(10,
					widgets.Text{
						Content: "SoundHelix Sample Song",
						Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 13,
						},
					},
				),
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
						pos := s.audioController.Position()
						s.audioController.SeekTo(pos + 10*time.Second)
					}, colors),
					widgets.HSpace(6),
					smallButton(ctx, "Seek -10s", func() {
						pos := s.audioController.Position()
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
	secStr := itoa(seconds)
	if seconds < 10 {
		secStr = "0" + secStr
	}
	return itoa(minutes) + ":" + secStr
}
