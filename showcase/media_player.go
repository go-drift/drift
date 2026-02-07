package main

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

func buildMediaPlayerPage(ctx core.BuildContext) core.Widget {
	return mediaPlayerPage{}
}

type mediaPlayerPage struct{}

func (m mediaPlayerPage) CreateElement() core.Element {
	return core.NewStatefulElement(m, nil)
}

func (m mediaPlayerPage) Key() any {
	return nil
}

func (m mediaPlayerPage) CreateState() core.State {
	return &mediaPlayerState{}
}

const audioURL = "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3"

type mediaPlayerState struct {
	core.StateBase
	videoStatus     *core.ManagedState[string]
	audioStatus     *core.ManagedState[string]
	audioStateLabel string
	audioController *platform.AudioPlayerController
}

func (s *mediaPlayerState) InitState() {
	s.videoStatus = core.NewManagedState(&s.StateBase, "Idle")
	s.audioStatus = core.NewManagedState(&s.StateBase, "Idle")
	s.audioStateLabel = "Idle"

	s.audioController = platform.NewAudioPlayerController()

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
		s.audioStatus.Set("Error: " + message)
	}

	s.OnDispose(func() {
		s.audioController.Dispose()
	})
}

func (s *mediaPlayerState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Media Player",
		// Video section
		sectionTitle("Video Player", colors),
		widgets.VSpace(8),
		widgets.Text{
			Content: "Native platform video player with built-in controls.",
			Wrap:    true,
			Style:   labelStyle(colors),
		},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisSize: widgets.MainAxisSizeMax,
			Children: []core.Widget{
				widgets.Expanded{
					Child: widgets.VideoPlayer{
						URL:      "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
						AutoPlay: false,
						Volume:   1.0,
						Height:   220,
						OnPlaybackStateChanged: func(state platform.PlaybackState) {
							s.videoStatus.Set(state.String())
						},
						OnError: func(code string, message string) {
							s.videoStatus.Set("Error: " + message)
						},
					},
				},
			},
		},
		widgets.VSpace(8),
		statusCard(s.videoStatus.Get(), colors),
		widgets.VSpace(32),

		// Audio section
		sectionTitle("Audio Player", colors),
		widgets.VSpace(8),
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

func (s *mediaPlayerState) audioControls(ctx core.BuildContext, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		MainAxisSize:       widgets.MainAxisSizeMin,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		Children: []core.Widget{
			// URL display
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
						s.audioController.Play(audioURL)
					}),
					widgets.HSpace(8),
					theme.ButtonOf(ctx, "Pause", func() {
						s.audioController.Pause()
					}),
					widgets.HSpace(8),
					theme.ButtonOf(ctx, "Stop", func() {
						s.audioController.Stop()
						s.audioStatus.Set("Stopped")
					}),
				},
			},
			widgets.VSpace(8),
			// Playback options
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
		},
	}
}

func smallButton(ctx core.BuildContext, label string, onTap func(), colors theme.ColorScheme) core.Widget {
	return widgets.GestureDetector{
		OnTap: onTap,
		Child: widgets.Container{
			Color:        colors.SurfaceContainerHigh,
			BorderRadius: 6,
			Padding:      layout.EdgeInsetsSymmetric(12, 6),
			Child: widgets.Text{
				Content: label,
				Style: graphics.TextStyle{
					Color:    colors.OnSurface,
					FontSize: 13,
				},
			},
		},
	}
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
