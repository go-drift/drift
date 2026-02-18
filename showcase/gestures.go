package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildGesturesPage demonstrates drag gestures with axis locking.
func buildGesturesPage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *gesturesDemoState { return &gesturesDemoState{} })
}

type gesturesDemoState struct {
	core.StateBase
	// Pan gesture state (omnidirectional)
	panX, panY float64
	// Horizontal drag state
	sliderX float64
	// Vertical drag state
	verticalY float64
	// Swipe card state
	cardX float64
}

func (s *gesturesDemoState) InitState() {
	s.panX = 0
	s.panY = 0
	s.sliderX = 100  // Start in the middle of 200px track
	s.verticalY = 40 // Start in middle of vertical area
	s.cardX = 0
}

func (s *gesturesDemoState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	items := []core.Widget{
		// Section 1: Pan Gesture (omnidirectional)
		sectionTitle("Pan Gesture", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Drag in any direction:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		s.buildPanDemo(colors),
		widgets.VSpace(24),

		// Section 2: Horizontal Drag
		sectionTitle("Horizontal Drag", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Only responds to horizontal drags:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		s.buildHorizontalSlider(colors),
		widgets.VSpace(24),

		// Section 3: Vertical Drag
		sectionTitle("Vertical Drag", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Only responds to vertical drags:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		s.buildVerticalDemo(colors),
		widgets.VSpace(24),

		// Section 4: Axis Competition
		sectionTitle("Axis Competition", colors),
		widgets.VSpace(8),
		widgets.Text{Content: "Horizontal swipe on card inside vertical scroll - card moves, scroll doesn't:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		s.buildSwipeCard(colors),
		widgets.VSpace(40),
	}

	content := widgets.ScrollView{
		ScrollDirection: widgets.AxisVertical,
		Physics:         widgets.BouncingScrollPhysics{},
		Padding:         layout.EdgeInsetsAll(20),
		Child: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children:           items,
		},
	}

	return pageScaffold(ctx, "Gestures", content)
}

// buildPanDemo creates a draggable box using pan gesture (any direction).
func (s *gesturesDemoState) buildPanDemo(colors theme.ColorScheme) core.Widget {
	boxSize := 80.0
	areaWidth := 280.0
	areaHeight := 160.0

	return widgets.DecoratedBox{
		Color:        colors.SurfaceVariant,
		BorderRadius: 12,
		Child: widgets.SizedBox{
			Width:  areaWidth,
			Height: areaHeight,
			Child: widgets.Stack{
				Children: []core.Widget{
					widgets.Positioned{
						Left: widgets.Ptr(s.panX),
						Top:  widgets.Ptr(s.panY),
						Child: widgets.Drag(func(d widgets.DragUpdateDetails) {
							s.SetState(func() {
								s.panX = widgets.Clamp(s.panX+d.Delta.X, 0, areaWidth-boxSize)
								s.panY = widgets.Clamp(s.panY+d.Delta.Y, 0, areaHeight-boxSize)
							})
						}, widgets.DecoratedBox{
							Color:        colors.Primary,
							BorderRadius: 8,
							Child: widgets.SizedBox{
								Width:  boxSize,
								Height: boxSize,
								Child: widgets.Center{
									Child: widgets.Text{Content: "Drag me", Style: graphics.TextStyle{
										Color:    colors.OnPrimary,
										FontSize: 12,
									}},
								},
							},
						}),
					},
				},
			},
		},
	}
}

// buildHorizontalSlider creates a slider that only responds to horizontal drags.
func (s *gesturesDemoState) buildHorizontalSlider(colors theme.ColorScheme) core.Widget {
	trackWidth := 240.0
	thumbSize := 32.0

	return widgets.Column{
		MainAxisAlignment:  widgets.MainAxisAlignmentStart,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		MainAxisSize:       widgets.MainAxisSizeMin,
		Children: []core.Widget{
			widgets.SizedBox{
				Width:  trackWidth,
				Height: thumbSize,
				Child: widgets.Stack{
					Alignment: layout.AlignmentCenterLeft,
					Children: []core.Widget{
						// Track
						widgets.DecoratedBox{
							Color:        colors.SurfaceVariant,
							BorderRadius: 4,
							Child: widgets.SizedBox{
								Width:  trackWidth,
								Height: 8,
							},
						},
						// Thumb
						widgets.Positioned{
							Left: widgets.Ptr(s.sliderX - thumbSize/2),
							Child: widgets.HorizontalDrag(func(d widgets.DragUpdateDetails) {
								s.SetState(func() {
									s.sliderX = widgets.Clamp(s.sliderX+d.PrimaryDelta, thumbSize/2, trackWidth-thumbSize/2)
								})
							}, widgets.DecoratedBox{
								Color:        colors.Primary,
								BorderRadius: thumbSize / 2,
								Child: widgets.SizedBox{
									Width:  thumbSize,
									Height: thumbSize,
								},
							}),
						},
					},
				},
			},
			widgets.VSpace(8),
			widgets.Text{Content: "Value: " + itoa(int(((s.sliderX-thumbSize/2)/(trackWidth-thumbSize))*100)) + "%", Style: labelStyle(colors)},
		},
	}
}

// buildVerticalDemo creates a box that only responds to vertical drags.
func (s *gesturesDemoState) buildVerticalDemo(colors theme.ColorScheme) core.Widget {
	boxWidth := 120.0
	boxHeight := 48.0
	areaWidth := 200.0
	areaHeight := 120.0

	return widgets.DecoratedBox{
		Color:        colors.SurfaceVariant,
		BorderRadius: 12,
		Child: widgets.SizedBox{
			Width:  areaWidth,
			Height: areaHeight,
			Child: widgets.Stack{
				Children: []core.Widget{
					widgets.Positioned{
						Left: widgets.Ptr((areaWidth - boxWidth) / 2), // Center horizontally
						Top:  widgets.Ptr(s.verticalY),
						Child: widgets.VerticalDrag(func(d widgets.DragUpdateDetails) {
							s.SetState(func() {
								s.verticalY = widgets.Clamp(s.verticalY+d.PrimaryDelta, 0, areaHeight-boxHeight)
							})
						}, widgets.DecoratedBox{
							Color:        colors.Secondary,
							BorderRadius: 8,
							Child: widgets.SizedBox{
								Width:  boxWidth,
								Height: boxHeight,
								Child: widgets.Center{
									Child: widgets.Text{Content: "Drag up/down", Style: graphics.TextStyle{
										Color:    colors.OnSecondary,
										FontSize: 12,
									}},
								},
							},
						}),
					},
				},
			},
		},
	}
}

// buildSwipeCard creates a horizontally-swipeable card demonstrating axis competition.
func (s *gesturesDemoState) buildSwipeCard(colors theme.ColorScheme) core.Widget {
	cardWidth := 280.0
	cardHeight := 64.0

	return widgets.SizedBox{
		Width:  cardWidth,
		Height: cardHeight,
		Child: widgets.Stack{
			Children: []core.Widget{
				// Background (delete indicator)
				widgets.DecoratedBox{
					Color:        colors.Error,
					BorderRadius: 8,
					Child: widgets.SizedBox{
						Width:  cardWidth,
						Height: cardHeight,
						Child: widgets.Padding{
							Padding: layout.EdgeInsetsOnly(0, 0, 16, 0),
							Child: widgets.Row{
								MainAxisAlignment:  widgets.MainAxisAlignmentEnd,
								CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
								MainAxisSize:       widgets.MainAxisSizeMax,
								Children: []core.Widget{
									widgets.Text{Content: "Swipe", Style: graphics.TextStyle{
										Color:    colors.OnError,
										FontSize: 14,
									}},
								},
							},
						},
					},
				},
				// Foreground card
				widgets.Positioned{
					Left: widgets.Ptr(s.cardX),
					Child: widgets.GestureDetector{
						OnHorizontalDragUpdate: func(d widgets.DragUpdateDetails) {
							s.SetState(func() {
								s.cardX = widgets.Clamp(s.cardX+d.PrimaryDelta, -100, 0)
							})
						},
						OnHorizontalDragEnd: func(d widgets.DragEndDetails) {
							// Snap back or dismiss based on position
							s.SetState(func() {
								if s.cardX < -50 {
									s.cardX = -100
								} else {
									s.cardX = 0
								}
							})
						},
						Child: widgets.DecoratedBox{
							Color:        colors.SurfaceVariant,
							BorderRadius: 8,
							Child: widgets.SizedBox{
								Width:  cardWidth,
								Height: cardHeight,
								Child: widgets.Padding{
									Padding: layout.EdgeInsetsAll(16),
									Child: widgets.Text{Content: "Swipe me left", Style: graphics.TextStyle{
										Color:    colors.OnSurface,
										FontSize: 14,
									}},
								},
							},
						},
					},
				},
			},
		},
	}
}
