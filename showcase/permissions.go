package main

import (
	"context"
	"maps"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildPermissionsPage creates a stateful widget for permissions demos.
func buildPermissionsPage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *permissionsState { return &permissionsState{} })
}

// permissionDemo represents a permission to display.
type permissionDemo struct {
	name       string
	permission platform.Permission
}

// Define which permissions to show (orphan permissions not covered by dedicated pages)
var otherPermissions = []permissionDemo{
	{"Contacts", platform.Contacts.Permission},
	{"Calendar", platform.Calendar.Permission},
	{"Storage", platform.StoragePermission.Permission},
	{"Microphone", platform.Microphone.Permission},
	{"Photos", platform.Photos.Permission},
}

type permissionsState struct {
	core.StateBase
	statuses   *core.ManagedState[map[string]platform.PermissionResult]
	statusText *core.ManagedState[string]
	unsubFuncs []func()
}

func (s *permissionsState) InitState() {
	s.statuses = core.NewManagedState(&s.StateBase, make(map[string]platform.PermissionResult))
	s.statusText = core.NewManagedState(&s.StateBase, "Tap 'Request' to request a permission.")

	ctx := context.Background()

	// Check initial status of all permissions
	go func() {
		statuses := make(map[string]platform.PermissionResult)
		for _, p := range otherPermissions {
			if result, err := p.permission.Status(ctx); err == nil {
				statuses[p.name] = result
			}
		}
		drift.Dispatch(func() {
			s.statuses.Set(statuses)
		})
	}()

	// Listen for permission changes
	for _, p := range otherPermissions {
		perm := p // capture for closure
		unsub := perm.permission.Listen(func(result platform.PermissionResult) {
			drift.Dispatch(func() {
				statuses := s.statuses.Get()
				newStatuses := make(map[string]platform.PermissionResult)
				maps.Copy(newStatuses, statuses)
				newStatuses[perm.name] = result
				s.statuses.Set(newStatuses)
				s.statusText.Set(perm.name + " permission: " + string(result))
			})
		})
		s.unsubFuncs = append(s.unsubFuncs, unsub)
	}

	s.OnDispose(func() {
		for _, unsub := range s.unsubFuncs {
			unsub()
		}
	})
}

func (s *permissionsState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
	statuses := s.statuses.Get()

	// Build permission rows
	var rows []core.Widget
	for _, p := range otherPermissions {
		rows = append(rows, s.permissionRow(p.name, statuses[p.name], colors), widgets.VSpace(12))
	}

	return demoPage(ctx, "Other Permissions",
		sectionTitle("Runtime Permissions", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Check and request app permissions:", Style: labelStyle(colors)},
		widgets.VSpace(16),

		widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children:           rows,
		},
		widgets.VSpace(12),

		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(24),

		sectionTitle("Settings", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Open app settings to manage permissions:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.ButtonOf(ctx, "Open Settings", func() {
			s.openSettings()
		}).WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(40),
	)
}

func (s *permissionsState) permissionRow(name string, status platform.PermissionResult, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding: layout.EdgeInsetsAll(12),
		Child: widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Column{
					MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
					CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
					MainAxisSize:       widgets.MainAxisSizeMin,
					Children: []core.Widget{
						widgets.Text{Content: name, Style: graphics.TextStyle{
							Color:      colors.OnSurface,
							FontSize:   16,
							FontWeight: graphics.FontWeightSemibold,
						}},
						widgets.VSpace(4),
						permissionBadge(status, colors),
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
	}
}

func (s *permissionsState) requestPermission(name string) {
	s.statusText.Set("Requesting " + name + " permission...")

	go func() {
		ctx := context.Background()
		var result platform.PermissionResult
		var err error

		// Find the permission by name
		for _, p := range otherPermissions {
			if p.name == name {
				result, err = p.permission.Request(ctx)
				break
			}
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
	ctx := context.Background()
	err := platform.OpenAppSettings(ctx)
	if err != nil {
		s.statusText.Set("Error opening settings: " + err.Error())
		return
	}
	s.statusText.Set("Opened app settings")
}
