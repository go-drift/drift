// Firebase demo: push notifications through the Firebase plugin.
//
// Shows this installation's FCM token (also logged, and copyable), asks for
// notification permission, and lists the messages the app receives and the
// notifications the user taps, including the one that launched it.
// README.md covers the Firebase setup and sending test messages.
package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"

	"github.com/go-drift/drift/plugins/firebase/runtime/messaging"
)

// maxEvents is how many messages and taps the screen lists.
const maxEvents = 8

func main() {
	drift.App{Root: demo{}}.Run()
}

type demo struct{ core.StatefulBase }

func (demo) CreateState() core.State { return &demoState{} }

type demoState struct {
	core.StateBase
	permission *core.Signal[string]
	events     *core.Signal[[]string]
}

func (s *demoState) InitState() {
	s.permission = core.NewSignal("unknown")
	s.events = core.NewSignalWithEquality[[]string](nil, slices.Equal)
	core.UseListenable(s, s.permission)
	core.UseListenable(s, s.events)
	core.UseListenable(s, messaging.Token())

	logToken := func() { log.Printf("FCM token: %s", messaging.Token().Value()) }
	if messaging.Token().Value() != "" {
		logToken()
	}
	unsubToken := messaging.Token().AddListener(logToken)
	unsubMessages := messaging.Messages().Listen(func(m messaging.Message) {
		s.record(fmt.Sprintf("received (foreground=%t)", m.Foreground), m)
	})
	unsubOpens := messaging.Opens().Listen(func(m messaging.Message) {
		s.record("tapped", m)
	})
	s.OnDispose(func() {
		unsubToken()
		unsubMessages()
		unsubOpens()
	})

	go func() {
		status, err := platform.Notifications.Permission.Status()
		drift.Dispatch(func() { s.setPermission(status, err) })
	}()
}

// record logs an event and lists it first. Called from the platform thread.
func (s *demoState) record(what string, m messaging.Message) {
	line := fmt.Sprintf("%s %s: %q %q %v", time.Now().Format("15:04:05"), what, m.Title, m.Body, m.Data)
	log.Printf("FCM %s id=%s title=%q body=%q data=%v", what, m.ID, m.Title, m.Body, m.Data)
	drift.Dispatch(func() {
		events := append([]string{line}, s.events.Value()...)
		s.events.Set(events[:min(len(events), maxEvents)])
	})
}

func (s *demoState) setPermission(status platform.PermissionResult, err error) {
	if err != nil {
		s.permission.Set("error: " + err.Error())
		return
	}
	s.permission.Set(string(status))
}

func (s *demoState) requestPermission() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		status, err := platform.Notifications.Permission.RequestWithOptions(ctx,
			platform.NotificationPermissionOptions{Alert: true, Sound: true, Badge: true})
		drift.Dispatch(func() { s.setPermission(status, err) })
	}()
}

func (s *demoState) Build(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)
	body := graphics.TextStyle{Color: colors.OnBackground, FontSize: 14}
	token := messaging.Token().Value()
	if token == "" {
		token = "(waiting for Firebase)"
	}

	children := []core.Widget{
		widgets.Text{Content: "drift firebase demo", Style: textTheme.HeadlineMedium},
		widgets.VSpace(16),
		widgets.Text{Content: "FCM token", Style: textTheme.TitleMedium},
		widgets.VSpace(4),
		widgets.Text{Content: token, Style: body},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Copy token", func() {
			if err := platform.Clipboard.SetText(messaging.Token().Value()); err != nil {
				log.Printf("copy token: %v", err)
			}
		}),
		widgets.VSpace(16),
		widgets.Text{Content: "Notification permission: " + s.permission.Value(), Style: body},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Request permission", s.requestPermission),
		widgets.VSpace(16),
		widgets.Text{Content: "Messages and taps", Style: textTheme.TitleMedium},
		widgets.VSpace(4),
	}
	if len(s.events.Value()) == 0 {
		children = append(children, widgets.Text{Content: "none yet", Style: body})
	}
	for _, e := range s.events.Value() {
		children = append(children, widgets.Text{Content: e, Style: body}, widgets.VSpace(4))
	}

	return widgets.Container{
		Color: colors.Background,
		Child: widgets.ScrollView{
			Padding: widgets.SafeAreaPadding(ctx).Add(20),
			Child: widgets.Column{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				Children:           children,
			},
		},
	}
}
