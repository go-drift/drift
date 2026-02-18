package main

import (
	"sync"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/lottie"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

var (
	lottieAssetOnce sync.Once
	lottieAssetAnim *lottie.Animation
	lottieAssetErr  error
)

func loadLottieAsset() (*lottie.Animation, error) {
	lottieAssetOnce.Do(func() {
		f, err := assetFS.Open("assets/bouncing-ball.json")
		if err != nil {
			lottieAssetErr = err
			return
		}
		defer f.Close()
		lottieAssetAnim, lottieAssetErr = lottie.Load(f)
	})
	return lottieAssetAnim, lottieAssetErr
}

func buildLottiePage(_ core.BuildContext) core.Widget {
	return lottieDemo{}
}

type lottieDemo struct{}

func (l lottieDemo) CreateElement() core.Element {
	return core.NewStatefulElement(l, nil)
}

func (l lottieDemo) Key() any { return nil }

func (l lottieDemo) CreateState() core.State {
	return &lottieDemoState{}
}

type lottieDemoState struct {
	core.StateBase
	anim    *lottie.Animation
	loadErr error

	// Play-once section
	playOnceCtrl *animation.AnimationController
	playOnceDone bool

	// Controlled playback section
	controlCtrl    *animation.AnimationController
	controlPlaying bool
}

func (s *lottieDemoState) InitState() {
	s.anim, s.loadErr = loadLottieAsset()
	if s.loadErr != nil || s.anim == nil {
		return
	}

	dur := s.anim.Duration()
	if dur <= 0 {
		return
	}

	// Play-once controller: starts playing immediately
	s.playOnceCtrl = core.UseController(&s.StateBase, func() *animation.AnimationController {
		c := animation.NewAnimationController(dur)
		c.Curve = animation.LinearCurve
		return c
	})
	core.UseListenable(&s.StateBase, s.playOnceCtrl)
	s.playOnceCtrl.AddStatusListener(func(status animation.AnimationStatus) {
		if status == animation.AnimationCompleted {
			s.SetState(func() {
				s.playOnceDone = true
			})
		}
	})
	s.playOnceCtrl.Forward()

	// Controlled controller: starts paused at 0
	s.controlCtrl = core.UseController(&s.StateBase, func() *animation.AnimationController {
		c := animation.NewAnimationController(dur)
		c.Curve = animation.LinearCurve
		return c
	})
	core.UseListenable(&s.StateBase, s.controlCtrl)
	s.controlCtrl.AddStatusListener(func(status animation.AnimationStatus) {
		if status == animation.AnimationCompleted || status == animation.AnimationDismissed {
			s.SetState(func() {
				s.controlPlaying = false
			})
		}
	})
}

func (s *lottieDemoState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	if s.loadErr != nil {
		return demoPage(ctx, "Lottie",
			statusCard("Failed to load animation: "+s.loadErr.Error(), colors),
			widgets.VSpace(40),
		)
	}

	return demoPage(ctx, "Lottie",
		// Section 1: Looping
		sectionTitle("Looping", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Continuous loop with LottieLoop:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Lottie{
			Source: s.anim,
			Width:  200,
			Height: 200,
			Repeat: widgets.LottieLoop,
		},
		widgets.VSpace(24),

		// Section 2: Play Once
		sectionTitle("Play Once", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Plays once, tap Replay to restart:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		s.buildPlayOnce(ctx, colors),
		widgets.VSpace(24),

		// Section 3: Controlled Playback
		sectionTitle("Controlled Playback", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "External controller with play, pause, and restart:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		s.buildControlled(ctx, colors),
		widgets.VSpace(40),
	)
}

func (s *lottieDemoState) buildPlayOnce(ctx core.BuildContext, colors theme.ColorScheme) core.Widget {
	if s.playOnceCtrl == nil {
		return widgets.Lottie{Source: s.anim, Width: 200, Height: 200}
	}

	statusLabel := "Playing..."
	if s.playOnceDone {
		statusLabel = "Completed"
	}

	children := []core.Widget{
		widgets.Lottie{
			Source:     s.anim,
			Controller: s.playOnceCtrl,
			Width:      200,
			Height:     200,
		},
		widgets.VSpace(8),
		widgets.Text{Content: statusLabel, Style: labelStyle(colors)},
		widgets.VSpace(8),
	}

	if s.playOnceDone {
		children = append(children, theme.ButtonOf(ctx, "Replay", func() {
			s.SetState(func() {
				s.playOnceDone = false
			})
			s.playOnceCtrl.Reset()
			s.playOnceCtrl.Forward()
		}))
	}

	return widgets.Column{
		MainAxisSize:       widgets.MainAxisSizeMin,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		Children:           children,
	}
}

func (s *lottieDemoState) buildControlled(ctx core.BuildContext, colors theme.ColorScheme) core.Widget {
	if s.controlCtrl == nil {
		return widgets.Lottie{Source: s.anim, Width: 200, Height: 200}
	}

	progressStr := "Progress: " + itoa(int(s.controlCtrl.Value*100)) + "%"

	playLabel := "Play"
	if s.controlPlaying {
		playLabel = "Pause"
	}

	return widgets.Column{
		MainAxisSize:       widgets.MainAxisSizeMin,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		Children: []core.Widget{
			widgets.Lottie{
				Source:     s.anim,
				Controller: s.controlCtrl,
				Width:      200,
				Height:     200,
			},
			widgets.VSpace(8),
			widgets.Text{Content: progressStr, Style: labelStyle(colors)},
			widgets.VSpace(8),
			widgets.Row{
				MainAxisAlignment: widgets.MainAxisAlignmentStart,
				Children: []core.Widget{
					theme.ButtonOf(ctx, playLabel, func() {
						s.SetState(func() {
							if s.controlPlaying {
								s.controlCtrl.Stop()
								s.controlPlaying = false
							} else {
								if s.controlCtrl.IsCompleted() {
									s.controlCtrl.Reset()
								}
								s.controlCtrl.Forward()
								s.controlPlaying = true
							}
						})
					}),
					widgets.HSpace(8),
					theme.ButtonOf(ctx, "Restart", func() {
						s.SetState(func() {
							s.controlCtrl.Reset()
							s.controlCtrl.Forward()
							s.controlPlaying = true
						})
					}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
				},
			},
		},
	}
}
