package main

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildLocationPage creates a stateful widget for location demos.
func buildLocationPage(ctx core.BuildContext) core.Widget {
	return locationPage{}
}

type locationPage struct{}

func (l locationPage) CreateElement() core.Element {
	return core.NewStatefulElement(l, nil)
}

func (l locationPage) Key() any {
	return nil
}

func (l locationPage) CreateState() core.State {
	return &locationState{}
}

type locationState struct {
	core.StateBase
	statusText  *core.ManagedState[string]
	location    *core.ManagedState[*platform.LocationUpdate]
	isStreaming *core.ManagedState[bool]
	isEnabled   *core.ManagedState[bool]
}

func (s *locationState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Tap a button to get location.")
	s.location = core.NewManagedState[*platform.LocationUpdate](&s.StateBase, nil)
	s.isStreaming = core.NewManagedState(&s.StateBase, false)
	s.isEnabled = core.NewManagedState(&s.StateBase, false)

	// Check if location services are enabled
	go func() {
		enabled, _ := platform.IsLocationEnabled()
		drift.Dispatch(func() {
			s.isEnabled.Set(enabled)
		})
	}()

	// Listen for location updates
	go func() {
		for update := range platform.LocationUpdates() {
			drift.Dispatch(func() {
				s.location.Set(&update)
				s.statusText.Set("Location updated")
			})
		}
	}()
}

func (s *locationState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	isStreaming := s.isStreaming.Get()
	isEnabled := s.isEnabled.Get()

	toggleLabel := "Start Updates"
	toggleColor := colors.Secondary
	if isStreaming {
		toggleLabel = "Stop Updates"
		toggleColor = colors.Error
	}

	enabledText := "Location services: disabled"
	if isEnabled {
		enabledText = "Location services: enabled"
	}

	return demoPage(ctx, "Location",
		sectionTitle("Location Services", colors),
		widgets.VSpace(12),
		widgets.Text{Content: enabledText, Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			ChildrenWidgets: []core.Widget{
				theme.ButtonOf(ctx, "Get Location", func() {
					s.getCurrentLocation()
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, toggleLabel, func() {
					s.toggleUpdates()
				}).WithColor(toggleColor, colors.OnSecondary),
			},
		},
		widgets.VSpace(24),

		sectionTitle("Location Data", colors),
		widgets.VSpace(12),
		s.locationCard(colors),
		widgets.VSpace(16),

		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *locationState) locationCard(colors theme.ColorScheme) core.Widget {
	loc := s.location.Get()

	if loc == nil {
		return widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			ChildWidget: widgets.PaddingAll(16,
				widgets.Text{Content: "No location data yet", Style: graphics.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 14,
				}},
			),
		}
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		ChildWidget: widgets.PaddingAll(16,
			widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				ChildrenWidgets: []core.Widget{
					s.locationRow("Latitude", fmt.Sprintf("%.6f", loc.Latitude), colors),
					widgets.VSpace(8),
					s.locationRow("Longitude", fmt.Sprintf("%.6f", loc.Longitude), colors),
					widgets.VSpace(8),
					s.locationRow("Accuracy", fmt.Sprintf("%.1f m", loc.Accuracy), colors),
					widgets.VSpace(8),
					s.locationRow("Altitude", fmt.Sprintf("%.1f m", loc.Altitude), colors),
					widgets.VSpace(8),
					s.locationRow("Timestamp", loc.Timestamp.Format("15:04:05"), colors),
				},
			},
		),
	}
}

func (s *locationState) locationRow(label, value string, colors theme.ColorScheme) core.Widget {
	return widgets.Row{
		MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: label, Style: graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}},
			widgets.Text{Content: value, Style: graphics.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   14,
				FontWeight: graphics.FontWeightSemibold,
			}},
		},
	}
}

func (s *locationState) getCurrentLocation() {
	s.statusText.Set("Getting location...")

	go func() {
		loc, err := platform.GetCurrentLocation(platform.LocationOptions{
			HighAccuracy: true,
		})
		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error: " + err.Error())
				return
			}
			if loc == nil {
				s.statusText.Set("No location available")
				return
			}
			s.location.Set(loc)
			s.statusText.Set("Location retrieved")
		})
	}()
}

func (s *locationState) toggleUpdates() {
	if s.isStreaming.Get() {
		err := platform.StopLocationUpdates()
		if err != nil {
			s.statusText.Set("Error stopping: " + err.Error())
			return
		}
		s.isStreaming.Set(false)
		s.statusText.Set("Location updates stopped")
	} else {
		err := platform.StartLocationUpdates(platform.LocationOptions{
			HighAccuracy:   true,
			DistanceFilter: 10, // 10 meters
		})
		if err != nil {
			s.statusText.Set("Error starting: " + err.Error())
			return
		}
		s.isStreaming.Set(true)
		s.statusText.Set("Location updates started")
	}
}
