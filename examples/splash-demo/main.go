// Splash demo: a minimal Drift app that demonstrates the splash plugin's
// runtime API.
//
// `App.OnInit` blocks root mounting (and frame composition) while it runs,
// so by default the native splash holds for as long as OnInit takes to
// return. To extend the hold beyond OnInit — for example, to keep the
// splash overlay visible while async background work continues after the
// first frame paints — call `splash.Preserve()` from inside OnInit and
// match it with a later `splash.Remove()`. This demo does exactly that:
// OnInit returns immediately after spawning a goroutine that sleeps 2
// seconds before releasing the splash.
//
// `splash.Preserve()` is the runtime-side counterpart to Drift's lifecycle
// — call it from `App.OnInit` or a `StateBase.InitState`. Go's package
// `func init()` runs before Drift's bridge is up and cannot reach native;
// the splash plugin's lifecycle hooks are the supported call sites.
package main

import (
	"context"
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
			// Preserve from OnInit: bumps the native preserve count
			// before the engine starts composing real frames, so the
			// splash overlay continues past OnInit's return.
			splash.Preserve()
			go func() {
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
				splash.Remove()
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
