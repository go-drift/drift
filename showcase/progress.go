package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildProgressPage creates a stateful widget for progress indicators demo.
func buildProgressPage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *progressState { return &progressState{} })
}

type progressState struct {
	core.StateBase
	progressValue *core.ManagedState[float64]
}

func (s *progressState) InitState() {
	s.progressValue = core.NewManagedState(&s.StateBase, 0.35)
}

func (s *progressState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Progress",
		// Native Activity Indicator
		sectionTitle("Activity Indicator", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Native platform spinner:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.ActivityIndicator{
					Animating: true,
					Size:      widgets.ActivityIndicatorSizeSmall,
				},
				widgets.HSpace(16),
				widgets.ActivityIndicator{
					Animating: true,
					Size:      widgets.ActivityIndicatorSizeMedium,
				},
				widgets.HSpace(16),
				widgets.ActivityIndicator{
					Animating: true,
					Size:      widgets.ActivityIndicatorSizeLarge,
					Color:     colors.Primary,
				},
			},
		},
		widgets.VSpace(24),

		// Circular Progress
		sectionTitle("Circular Progress", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Indeterminate:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.CircularProgressIndicator{
					Value: nil,
					Size:  24,
				},
				widgets.HSpace(16),
				widgets.CircularProgressIndicator{
					Value: nil,
					Size:  36,
					Color: colors.Secondary,
				},
				widgets.HSpace(16),
				widgets.CircularProgressIndicator{
					Value: nil,
					Size:  48,
					Color: colors.Tertiary,
				},
			},
		},
		widgets.VSpace(24),

		// Linear Progress
		sectionTitle("Linear Progress", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Indeterminate:", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.LinearProgressIndicatorOf(ctx, nil),
		widgets.VSpace(24),

		// Determinate
		sectionTitle("Determinate", colors),
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				s.buildDeterminateCircular(ctx),
				widgets.HSpace(16),
				theme.ButtonOf(ctx, "-10%", func() {
					v := s.progressValue.Get() - 0.1
					if v < 0 {
						v = 0
					}
					s.progressValue.Set(v)
				}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "+10%", func() {
					v := s.progressValue.Get() + 0.1
					if v > 1 {
						v = 1
					}
					s.progressValue.Set(v)
				}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			},
		},
		widgets.VSpace(12),
		s.buildDeterminateLinear(ctx),
		widgets.VSpace(40),
	)
}

// buildDeterminateCircular creates a determinate circular progress indicator.
func (s *progressState) buildDeterminateCircular(ctx core.BuildContext) core.Widget {
	progress := s.progressValue.Get()
	return theme.CircularProgressIndicatorOf(ctx, &progress)
}

// buildDeterminateLinear creates a determinate linear progress indicator.
func (s *progressState) buildDeterminateLinear(ctx core.BuildContext) core.Widget {
	progress := s.progressValue.Get()
	return theme.LinearProgressIndicatorOf(ctx, &progress)
}
