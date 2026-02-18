package main

import (
	"context"
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildLocationPage creates a stateful widget for location demos.
func buildLocationPage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *locationState { return &locationState{} })
}

type locationState struct {
	core.StateBase
	statusText      *core.ManagedState[string]
	location        *core.ManagedState[*platform.LocationUpdate]
	isStreaming     *core.ManagedState[bool]
	isEnabled       *core.ManagedState[bool]
	whenInUseStatus *core.ManagedState[platform.PermissionStatus]
	alwaysStatus    *core.ManagedState[platform.PermissionStatus]
	unsubFuncs      []func()
}

func (s *locationState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Tap a button to get location.")
	s.location = core.NewManagedState[*platform.LocationUpdate](&s.StateBase, nil)
	s.isStreaming = core.NewManagedState(&s.StateBase, false)
	s.isEnabled = core.NewManagedState(&s.StateBase, false)
	s.whenInUseStatus = core.NewManagedState(&s.StateBase, platform.PermissionNotDetermined)
	s.alwaysStatus = core.NewManagedState(&s.StateBase, platform.PermissionNotDetermined)

	ctx := context.Background()

	// Check if location services are enabled
	go func() {
		enabled, _ := platform.Location.IsEnabled(ctx)
		drift.Dispatch(func() {
			s.isEnabled.Set(enabled)
		})
	}()

	// Check initial permission statuses
	go func() {
		whenInUse, _ := platform.Location.Permission.WhenInUse.Status(ctx)
		always, _ := platform.Location.Permission.Always.Status(ctx)
		drift.Dispatch(func() {
			s.whenInUseStatus.Set(whenInUse)
			s.alwaysStatus.Set(always)
		})
	}()

	// Listen for permission changes
	whenInUseUnsub := platform.Location.Permission.WhenInUse.Listen(func(status platform.PermissionStatus) {
		drift.Dispatch(func() { s.whenInUseStatus.Set(status) })
	})
	alwaysUnsub := platform.Location.Permission.Always.Listen(func(status platform.PermissionStatus) {
		drift.Dispatch(func() { s.alwaysStatus.Set(status) })
	})

	// Listen for location updates using Stream
	updatesUnsub := platform.Location.Updates().Listen(func(update platform.LocationUpdate) {
		drift.Dispatch(func() {
			s.location.Set(&update)
			s.statusText.Set("Location updated")
		})
	})

	s.unsubFuncs = []func(){whenInUseUnsub, alwaysUnsub, updatesUnsub}

	s.OnDispose(func() {
		for _, unsub := range s.unsubFuncs {
			unsub()
		}
	})
}

func (s *locationState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
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
		sectionTitle("Permission", colors),
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Text{Content: "When In Use:", Style: labelStyle(colors)},
				permissionBadge(s.whenInUseStatus.Get(), colors),
			},
		},
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Text{Content: "Always:", Style: labelStyle(colors)},
				permissionBadge(s.alwaysStatus.Get(), colors),
			},
		},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Request When In Use", func() {
					go func() {
						ctx := context.Background()
						status, _ := platform.Location.Permission.WhenInUse.Request(ctx)
						drift.Dispatch(func() {
							s.whenInUseStatus.Set(status)
						})
					}()
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Request Always", func() {
					go func() {
						ctx := context.Background()
						status, _ := platform.Location.Permission.Always.Request(ctx)
						drift.Dispatch(func() {
							s.alwaysStatus.Set(status)
						})
					}()
				}),
			},
		},
		widgets.VSpace(24),

		sectionTitle("Location Services", colors),
		widgets.VSpace(12),
		widgets.Text{Content: enabledText, Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
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
			Padding:      layout.EdgeInsetsAll(16),
			Child: widgets.Text{Content: "No location data yet", Style: graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}},
		}
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding:      layout.EdgeInsetsAll(16),
		Child: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
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
	}
}

func (s *locationState) locationRow(label, value string, colors theme.ColorScheme) core.Widget {
	return widgets.Row{
		MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		Children: []core.Widget{
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
		ctx := context.Background()
		loc, err := platform.Location.GetCurrent(ctx, platform.LocationOptions{
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
	ctx := context.Background()

	if s.isStreaming.Get() {
		err := platform.Location.StopUpdates(ctx)
		if err != nil {
			s.statusText.Set("Error stopping: " + err.Error())
			return
		}
		s.isStreaming.Set(false)
		s.statusText.Set("Location updates stopped")
	} else {
		err := platform.Location.StartUpdates(ctx, platform.LocationOptions{
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
