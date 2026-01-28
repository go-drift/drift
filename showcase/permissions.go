package main

import (
	"log"
	"maps"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildPermissionsPage creates a stateful widget for permissions demos.
func buildPermissionsPage(ctx core.BuildContext) core.Widget {
	return permissionsPage{}
}

type permissionsPage struct{}

func (p permissionsPage) CreateElement() core.Element {
	return core.NewStatefulElement(p, nil)
}

func (p permissionsPage) Key() any {
	return nil
}

func (p permissionsPage) CreateState() core.State {
	return &permissionsState{}
}

type permissionsState struct {
	core.StateBase
	statuses   *core.ManagedState[map[string]platform.PermissionResult]
	statusText *core.ManagedState[string]
}

func (s *permissionsState) InitState() {
	s.statuses = core.NewManagedState(&s.StateBase, make(map[string]platform.PermissionResult))
	s.statusText = core.NewManagedState(&s.StateBase, "Tap 'Request' to request a permission.")

	// Check initial status of all permissions
	go func() {
		statuses := make(map[string]platform.PermissionResult)
		if result, err := platform.Permissions.Camera.Status(); err == nil {
			statuses["Camera"] = result
		}
		if result, err := platform.Permissions.Location.Status(); err == nil {
			statuses["Location"] = result
		}
		if result, err := platform.Permissions.Photos.Status(); err == nil {
			statuses["Photos"] = result
		}
		if result, err := platform.Permissions.Microphone.Status(); err == nil {
			statuses["Microphone"] = result
		}
		if result, err := platform.Permissions.Notification.Status(); err == nil {
			statuses["Notifications"] = result
		}
		drift.Dispatch(func() {
			s.statuses.Set(statuses)
		})
	}()

	// Listen for permission changes
	s.listenForChanges("Camera", platform.Permissions.Camera.Changes())
	s.listenForChanges("Location", platform.Permissions.Location.Changes())
	s.listenForChanges("Photos", platform.Permissions.Photos.Changes())
	s.listenForChanges("Microphone", platform.Permissions.Microphone.Changes())
	s.listenForChanges("Notifications", platform.Permissions.Notification.Changes())
}

func (s *permissionsState) listenForChanges(name string, ch <-chan platform.PermissionResult) {
	go func() {
		for result := range ch {
			r := result
			log.Printf("permission change %s %s", name, string(r))
			drift.Dispatch(func() {
				statuses := s.statuses.Get()
				newStatuses := make(map[string]platform.PermissionResult)
				maps.Copy(newStatuses, statuses)
				newStatuses[name] = r
				s.statuses.Set(newStatuses)
				s.statusText.Set(name + " permission: " + string(r))
			})
		}
	}()
}

func (s *permissionsState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	statuses := s.statuses.Get()

	// Build permission rows
	permissionNames := []string{"Camera", "Location", "Photos", "Microphone", "Notifications"}
	rows := make([]core.Widget, 0, len(permissionNames)*2)
	for _, name := range permissionNames {
		status := statuses[name]
		rows = append(rows, s.permissionRow(name, status, colors))
		rows = append(rows, widgets.VSpace(12))
	}

	return demoPage(ctx, "Permissions",
		sectionTitle("Runtime Permissions", colors),
		widgets.VSpace(12),
		widgets.TextOf("Check and request app permissions:", labelStyle(colors)),
		widgets.VSpace(16),

		widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
			MainAxisSize:       widgets.MainAxisSizeMin,
			ChildrenWidgets:    rows,
		},
		widgets.VSpace(12),

		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(24),

		sectionTitle("Settings", colors),
		widgets.VSpace(12),
		widgets.TextOf("Open app settings to manage permissions:", labelStyle(colors)),
		widgets.VSpace(8),

		widgets.ButtonOf("Open Settings", func() {
			s.openSettings()
		}).WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(40),
	)
}

func (s *permissionsState) permissionRow(name string, status platform.PermissionResult, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color: colors.SurfaceVariant,
		ChildWidget: widgets.PaddingAll(12,
			widgets.Row{
				MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				ChildrenWidgets: []core.Widget{
					widgets.Column{
						MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
						CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
						MainAxisSize:       widgets.MainAxisSizeMin,
						ChildrenWidgets: []core.Widget{
							widgets.TextOf(name, graphics.TextStyle{
								Color:      colors.OnSurface,
								FontSize:   16,
								FontWeight: graphics.FontWeightSemibold,
							}),
							widgets.VSpace(4),
							s.statusBadge(status, colors),
						},
					},
					widgets.Button{
						Label: "Request",
						OnTap: func() {
							s.requestPermission(name)
						},
						Color:        colors.Primary,
						TextColor:    colors.OnPrimary,
						Padding:      layout.EdgeInsetsSymmetric(12, 8),
						BorderRadius: 6,
					},
				},
			},
		),
	}
}

func (s *permissionsState) statusBadge(status platform.PermissionResult, colors theme.ColorScheme) core.Widget {
	var bgColor, textColor graphics.Color
	label := string(status)
	if label == "" {
		label = "unknown"
	}

	switch status {
	case platform.PermissionGranted:
		bgColor = 0xFF4CAF50 // green
		textColor = 0xFFFFFFFF
	case platform.PermissionDenied, platform.PermissionPermanentlyDenied:
		bgColor = 0xFFF44336 // red
		textColor = 0xFFFFFFFF
	case platform.PermissionLimited, platform.PermissionProvisional:
		bgColor = 0xFFFF9800 // orange
		textColor = 0xFFFFFFFF
	default:
		bgColor = colors.SurfaceContainerHigh
		textColor = colors.OnSurfaceVariant
	}

	return widgets.DecoratedBox{
		Color:        bgColor,
		BorderRadius: 4,
		ChildWidget: widgets.Padding{
			Padding: layout.EdgeInsetsSymmetric(8, 4),
			ChildWidget: widgets.TextOf(label, graphics.TextStyle{
				Color:    textColor,
				FontSize: 12,
			}),
		},
	}
}

func (s *permissionsState) requestPermission(name string) {
	s.statusText.Set("Requesting " + name + " permission...")

	go func() {
		var result platform.PermissionResult
		var err error

		switch name {
		case "Camera":
			result, err = platform.Permissions.Camera.Request()
		case "Location":
			result, err = platform.Permissions.Location.RequestWhenInUse()
		case "Photos":
			result, err = platform.Permissions.Photos.Request()
		case "Microphone":
			result, err = platform.Permissions.Microphone.Request()
		case "Notifications":
			result, err = platform.Permissions.Notification.Request(platform.NotificationOptions{
				Alert: true,
				Sound: true,
				Badge: true,
			})
		default:
			drift.Dispatch(func() {
				s.statusText.Set("Unknown permission: " + name)
			})
			return
		}

		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error requesting " + name + ": " + err.Error())
				return
			}
			// Update UI with the blocking result
			statuses := s.statuses.Get()
			newStatuses := make(map[string]platform.PermissionResult)
			maps.Copy(newStatuses, statuses)
			newStatuses[name] = result
			s.statuses.Set(newStatuses)
			s.statusText.Set(name + " permission: " + string(result))
		})
	}()
}

func (s *permissionsState) openSettings() {
	err := platform.OpenAppSettings()
	if err != nil {
		s.statusText.Set("Error opening settings: " + err.Error())
		return
	}
	s.statusText.Set("Opened app settings")
}
