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
	videoController *platform.VideoPlayerController
}

func (s *videoPlayerState) InitState() {
	s.videoStatus = core.NewManagedState(&s.StateBase, "Idle")

	s.videoController = core.UseController(&s.StateBase, platform.NewVideoPlayerController)

	s.videoController.OnPlaybackStateChanged = func(state platform.PlaybackState) {
		s.videoStatus.Set(state.String())
	}
	s.videoController.OnError = func(code string, message string) {
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
		statusCard(s.videoStatus.Get(), colors),
		widgets.VSpace(40),
	)
}
