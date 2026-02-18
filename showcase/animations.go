package main

import (
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildAnimationsPage creates the animations demo page.
func buildAnimationsPage(_ core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *animationsDemoState { return &animationsDemoState{} })
}

type animationsDemoState struct {
	core.StateBase
	curveIdx       int
	curveExpanded  bool
	colorIdx       int
	opacityVisible bool
}

func (s *animationsDemoState) InitState() {
	s.curveIdx = 0
	s.curveExpanded = false
	s.colorIdx = 0
	s.opacityVisible = true
}

func (s *animationsDemoState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	// Available curves
	curveNames := []string{"Linear", "EaseIn", "EaseOut", "EaseInOut"}
	curves := []func(float64) float64{animation.LinearCurve, animation.EaseIn, animation.EaseOut, animation.EaseInOut}
	currentCurveName := curveNames[s.curveIdx%len(curveNames)]
	currentCurve := curves[s.curveIdx%len(curves)]

	// Position for curve demo (280 width - 32 box - 4 padding = 244 max)
	curvePos := 4.0
	if s.curveExpanded {
		curvePos = 244.0
	}

	// Colors and sizes for animation
	demoColors := []graphics.Color{CyanSeed, PinkSeed, colors.Tertiary, colors.Primary}
	currentColor := demoColors[s.colorIdx%len(demoColors)]
	sizes := []float64{80, 100, 60, 90}
	currentSize := sizes[s.colorIdx%len(sizes)]

	return demoPage(ctx, "Animations",
		// Easing Curves section
		sectionTitle("Easing Curves", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Tap 'Animate' to see curve: " + currentCurveName, Style: labelStyle(colors)},
		widgets.VSpace(12),

		// Single curve demo - AnimatedContainer with padding positions the child
		widgets.Container{
			Color:  colors.SurfaceVariant,
			Width:  280,
			Height: 40,
			Child: widgets.AnimatedContainer{
				Duration:  500 * time.Millisecond,
				Curve:     currentCurve,
				Width:     280,
				Height:    40,
				Padding:   layout.EdgeInsets{Left: curvePos, Top: 4, Bottom: 4},
				Alignment: layout.AlignmentCenterLeft,
				Child: widgets.Container{
					Width:  32,
					Height: 32,
					Color:  PinkSeed,
				},
			},
		},
		widgets.VSpace(12),

		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Animate", func() {
					s.SetState(func() {
						s.curveExpanded = !s.curveExpanded
					})
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Next Curve", func() {
					s.SetState(func() {
						s.curveIdx++
						s.curveExpanded = false // Reset position
					})
				}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			},
		},
		widgets.VSpace(24),

		// AnimatedContainer section
		sectionTitle("AnimatedContainer", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Animates size, color, and padding:", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.AnimatedContainer{
					Duration:  400 * time.Millisecond,
					Curve:     animation.EaseInOut,
					Width:     currentSize,
					Height:    currentSize,
					Color:     currentColor,
					Alignment: layout.AlignmentCenter,
					Child: widgets.Text{Content: "Tap", Style: graphics.TextStyle{
						Color:    textColorFor(currentColor),
						FontSize: 14,
					}},
				},
				widgets.HSpace(16),
				theme.ButtonOf(ctx, "Change", func() {
					s.SetState(func() {
						s.colorIdx++
					})
				}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			},
		},
		widgets.VSpace(24),

		// AnimatedOpacity section
		sectionTitle("AnimatedOpacity", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Smooth fade transitions:", Style: labelStyle(colors)},
		widgets.VSpace(12),

		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.SizedBox{
					Width:  100,
					Height: 60,
					Child: widgets.AnimatedOpacity{
						Duration: 300 * time.Millisecond,
						Curve:    animation.EaseOut,
						Opacity:  boolToOpacity(s.opacityVisible),
						Child: widgets.Container{
							Width:     100,
							Height:    60,
							Color:     colors.Secondary,
							Alignment: layout.AlignmentCenter,
							Child: widgets.Text{Content: "Hello!", Style: graphics.TextStyle{
								Color:    colors.OnSecondary,
								FontSize: 14,
							}},
						},
					},
				},
				widgets.HSpace(16),
				theme.ButtonOf(ctx, "Toggle", func() {
					s.SetState(func() {
						s.opacityVisible = !s.opacityVisible
					})
				}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			},
		},
		widgets.VSpace(40),
	)
}

func boolToOpacity(visible bool) float64 {
	if visible {
		return 1.0
	}
	return 0.0
}
