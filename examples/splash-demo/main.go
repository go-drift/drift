// Splash demo: a minimal Drift app that demonstrates the splash plugin's
// runtime API.
//
// The splash stays up until the app draws its first frame with content,
// which is after App.OnInit returns. To hold it longer, for work the first
// screen should not show without, call splash.Preserve() before the first
// frame (App.OnInit or the root's InitState) and splash.Remove() when done.
// This demo holds it for 2 seconds after OnInit returns.
package main

import (
	"context"
	"log"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"

	splash "github.com/go-drift/drift/plugins/splash/runtime"
)

func main() {
	drift.App{
		Root: App(),
		OnInit: func(ctx context.Context) error {
			if err := splash.Preserve(); err != nil {
				return err
			}
			go func() {
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
				if err := splash.Remove(); err != nil {
					log.Printf("splash: %v", err)
				}
			}()
			return nil
		},
	}.Run()
}

func App() core.Widget {
	return app{}
}

type app struct {
	core.StatefulBase
}

func (app) CreateState() core.State {
	return &appState{}
}

type appState struct {
	core.StateBase
}

func (s *appState) Build(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)
	return widgets.Container{
		Color: colors.Background,
		Child: widgets.Centered(
			widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				MainAxisSize:       widgets.MainAxisSizeMin,
				Children: []core.Widget{
					widgets.Text{Content: "drift splash demo", Style: textTheme.HeadlineMedium},
					widgets.VSpace(16),
					widgets.Text{
						Content: "splash held via OnInit + Preserve/Remove",
						Style:   graphics.TextStyle{Color: colors.OnBackground, FontSize: 14},
					},
				},
			},
		),
	}
}
