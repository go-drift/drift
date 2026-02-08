package main

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

func buildVideoPlayerPage(ctx core.BuildContext) core.Widget {
	return videoPlayerPage{}
}

type videoPlayerPage struct{}

func (v videoPlayerPage) CreateElement() core.Element {
	return core.NewStatefulElement(v, nil)
}

func (v videoPlayerPage) Key() any {
	return nil
}

func (v videoPlayerPage) CreateState() core.State {
	return &videoPlayerState{}
}

type videoPlayerState struct {
	core.StateBase
	videoStatus     *core.ManagedState[string]
	videoStateLabel string
	videoController *platform.VideoPlayerController
	videoLooping    bool
	videoMuted      bool
}

func (s *videoPlayerState) InitState() {
	s.videoStatus = core.NewManagedState(&s.StateBase, "Idle")
	s.videoStateLabel = "Idle"

	s.videoController = core.UseController(&s.StateBase, platform.NewVideoPlayerController)

	s.videoController.OnPlaybackStateChanged = func(state platform.PlaybackState) {
		s.videoStateLabel = state.String()
		s.videoStatus.Set(s.videoStateLabel)
	}
	s.videoController.OnPositionChanged = func(position, duration, buffered time.Duration) {
		pos := formatDuration(position)
		dur := formatDuration(duration)
		s.videoStatus.Set(s.videoStateLabel + " \u00b7 " + pos + " / " + dur)
	}
	s.videoController.OnError = func(code, message string) {
		s.videoStateLabel = "Error"
		s.videoStatus.Set("Error (" + code + "): " + message)
	}

	s.videoController.Load("https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4")
}

func (s *videoPlayerState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Video Player",
		widgets.Text{
			Content: "Native platform video player with built-in controls. Use a VideoPlayerController for programmatic control.",
			Wrap:    true,
			Style:   labelStyle(colors),
		},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisSize: widgets.MainAxisSizeMax,
			Children: []core.Widget{
				widgets.Expanded{
					Child: widgets.VideoPlayer{
						Controller: s.videoController,
						Height:     220,
					},
				},
			},
		},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				smallButton(ctx, "Seek +10s", func() {
					pos := s.videoController.Position()
					s.videoController.SeekTo(pos + 10*time.Second)
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "Seek -10s", func() {
					pos := s.videoController.Position()
					if pos > 10*time.Second {
						s.videoController.SeekTo(pos - 10*time.Second)
					} else {
						s.videoController.SeekTo(0)
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
					s.videoController.SetPlaybackSpeed(0.5)
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "1x", func() {
					s.videoController.SetPlaybackSpeed(1.0)
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "1.5x", func() {
					s.videoController.SetPlaybackSpeed(1.5)
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "2x", func() {
					s.videoController.SetPlaybackSpeed(2.0)
				}, colors),
			},
		},
		widgets.VSpace(8),
		// Volume and looping
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				smallButton(ctx, toggleLabel("Mute", "Unmute", s.videoMuted), func() {
					s.videoMuted = !s.videoMuted
					if s.videoMuted {
						s.videoController.SetVolume(0)
					} else {
						s.videoController.SetVolume(1.0)
					}
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, toggleLabel("Loop", "Unloop", s.videoLooping), func() {
					s.videoLooping = !s.videoLooping
					s.videoController.SetLooping(s.videoLooping)
				}, colors),
			},
		},
		widgets.VSpace(8),
		statusCard(s.videoStatus.Get(), colors),
		widgets.VSpace(40),
	)
}
