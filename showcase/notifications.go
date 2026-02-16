package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildNotificationsPage creates a stateful widget for notification demos.
func buildNotificationsPage(ctx core.BuildContext) core.Widget {
	return notificationsPage{}
}

type notificationsPage struct{}

func (n notificationsPage) CreateElement() core.Element {
	return core.NewStatefulElement(n, nil)
}

func (n notificationsPage) Key() any {
	return nil
}

func (n notificationsPage) CreateState() core.State {
	return &notificationsState{}
}

type notificationsState struct {
	core.StateBase
	statusText       *core.ManagedState[string]
	receivedText     *core.ManagedState[string]
	openedText       *core.ManagedState[string]
	permissionStatus *core.ManagedState[platform.PermissionStatus]
	unsubFuncs       []func()
}

func (s *notificationsState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Request permission to enable notifications.")
	s.receivedText = core.NewManagedState(&s.StateBase, "No notifications received yet.")
	s.openedText = core.NewManagedState(&s.StateBase, "No notification opens yet.")
	s.permissionStatus = core.NewManagedState(&s.StateBase, platform.PermissionNotDetermined)

	ctx := context.Background()

	// Check initial permission status
	go func() {
		status, _ := platform.Notifications.Permission.Status(ctx)
		drift.Dispatch(func() {
			s.permissionStatus.Set(status)
		})
	}()

	// Listen for notification deliveries
	deliveriesUnsub := platform.Notifications.Deliveries().Listen(func(event platform.NotificationEvent) {
		message := fmt.Sprintf("Received (%s): %s", event.Source, event.Title)
		drift.Dispatch(func() {
			s.receivedText.Set(message)
		})
	})

	// Listen for notification opens
	opensUnsub := platform.Notifications.Opens().Listen(func(event platform.NotificationOpen) {
		message := fmt.Sprintf("Opened (%s): %s", event.Source, event.ID)
		drift.Dispatch(func() {
			s.openedText.Set(message)
		})
	})

	// Listen for permission changes
	permUnsub := platform.Notifications.Permission.Listen(func(status platform.PermissionStatus) {
		drift.Dispatch(func() {
			s.permissionStatus.Set(status)
			s.statusText.Set("Permission status: " + string(status))
		})
	})

	s.unsubFuncs = []func(){deliveriesUnsub, opensUnsub, permUnsub}

	s.OnDispose(func() {
		for _, unsub := range s.unsubFuncs {
			unsub()
		}
	})
}

func (s *notificationsState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Notifications",
		sectionTitle("Permission", colors),
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Text{Content: "Notification access:", Style: labelStyle(colors)},
				permissionBadge(s.permissionStatus.Get(), colors),
			},
		},
		widgets.VSpace(12),
		widgets.Text{Content: "Request notification permissions on iOS/Android:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.ButtonOf(ctx, "Request Permission", func() {
			s.requestPermissions()
		}),
		widgets.VSpace(12),
		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(24),

		sectionTitle("Local Notifications", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Schedule a notification 5 seconds from now:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.ButtonOf(ctx, "Schedule Local", func() {
			s.scheduleLocal()
		}).WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(12),
		statusCard(s.receivedText.Get(), colors),
		widgets.VSpace(8),
		statusCard(s.openedText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *notificationsState) requestPermissions() {
	go func() {
		// Use timeout to prevent blocking forever if OS never responds
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		status, err := platform.Notifications.Permission.RequestWithOptions(ctx,
			platform.NotificationPermissionOptions{
				Alert: true,
				Sound: true,
				Badge: true,
			},
		)

		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Permission error: " + err.Error())
				return
			}

			s.permissionStatus.Set(status)
			message := "Permission status: " + string(status)
			if status == platform.PermissionNotDetermined {
				message = "Waiting for permission…"
			}
			s.statusText.Set(message)
		})
	}()
}

func (s *notificationsState) scheduleLocal() {
	ctx := context.Background()

	request := platform.NotificationRequest{
		ID:    fmt.Sprintf("demo-%d", time.Now().Unix()),
		Title: "Drift Notification",
		Body:  "Hello from Drift!",
		At:    time.Now().Add(5 * time.Second),
		Data: map[string]any{
			"source": "showcase",
		},
	}

	if err := platform.Notifications.Schedule(ctx, request); err != nil {
		s.receivedText.Set("Schedule error: " + err.Error())
		return
	}

	s.receivedText.Set("Scheduled local notification.")
}
