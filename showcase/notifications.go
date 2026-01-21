package main

import (
	"fmt"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
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
	statusText   *core.ManagedState[string]
	receivedText *core.ManagedState[string]
	openedText   *core.ManagedState[string]
}

func (s *notificationsState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Request permission to enable notifications.")
	s.receivedText = core.NewManagedState(&s.StateBase, "No notifications received yet.")
	s.openedText = core.NewManagedState(&s.StateBase, "No notification opens yet.")

	go func() {
		for event := range platform.Notifications() {
			message := fmt.Sprintf("Received (%s): %s", event.Source, event.Title)
			drift.Dispatch(func() {
				s.receivedText.Set(message)
			})
		}
	}()

	go func() {
		for event := range platform.NotificationOpens() {
			message := fmt.Sprintf("Opened (%s): %s", event.Source, event.ID)
			drift.Dispatch(func() {
				s.openedText.Set(message)
			})
		}
	}()

	go func() {
		for status := range platform.NotificationPermissionUpdates() {
			message := "Permission status: " + string(status)
			drift.Dispatch(func() {
				s.statusText.Set(message)
			})
		}
	}()
}

func (s *notificationsState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Notifications",
		sectionTitle("Permissions", colors),
		widgets.VSpace(12),
		widgets.TextOf("Request notification permissions on iOS/Android:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("Request Permission", func() {
			s.requestPermissions()
		}).WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(12),
		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(24),
		sectionTitle("Local Notifications", colors),
		widgets.VSpace(12),
		widgets.TextOf("Schedule a notification 5 seconds from now:", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NewButton("Schedule Local", func() {
			s.scheduleLocal()
		}).WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(12),
		statusCard(s.receivedText.Get(), colors),
		widgets.VSpace(12),
		statusCard(s.openedText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *notificationsState) requestPermissions() {
	status, err := platform.RequestNotificationPermissions(platform.PermissionOptions{
		Alert: true,
		Sound: true,
		Badge: true,
	})

	if err != nil {
		s.statusText.Set("Permission error: " + err.Error())
		return
	}

	message := "Permission status: " + string(status)
	if status == platform.PermissionStatusNotDetermined {
		message = "Waiting for permission…"
	}
	s.statusText.Set(message)
}

func (s *notificationsState) scheduleLocal() {
	request := platform.NotificationRequest{
		ID:    fmt.Sprintf("demo-%d", time.Now().Unix()),
		Title: "Drift Notification",
		Body:  "Hello from Drift!",
		At:    time.Now().Add(5 * time.Second),
		Data: map[string]any{
			"source": "showcase",
		},
	}

	if err := platform.ScheduleLocalNotification(request); err != nil {
		s.receivedText.Set("Schedule error: " + err.Error())
		return
	}

	s.receivedText.Set("Scheduled local notification.")
}

func statusCard(text string, colors theme.ColorScheme) core.Widget {
	return widgets.NewContainer(
		widgets.PaddingAll(12,
			widgets.TextOf(text, rendering.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}),
		),
	).WithColor(colors.SurfaceVariant).Build()
}
