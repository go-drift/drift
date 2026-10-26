// Hello app: the smallest app using a plugin. It shows the greeting the
// hello plugin's native code reads from drift.yaml.
package main

import (
	"context"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"

	hello "github.com/go-drift/drift/examples/plugins/hello/runtime"
)

func main() {
	drift.App{Root: app{}}.Run()
}

type app struct {
	core.StatefulBase
}

func (app) CreateState() core.State {
	return &appState{}
}

type appState struct {
	core.StateBase
	greeting string
}

func (s *appState) InitState() {
	// Greeting's native method replies in place, so the UI thread may
	// call it.
	greeting, err := hello.Greeting(context.Background())
	if err != nil {
		greeting = err.Error()
	}
	s.greeting = greeting
}

func (s *appState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	return widgets.Container{
		Color: colors.Background,
		Child: widgets.Centered(widgets.Text{
			Content: s.greeting,
			Style:   graphics.TextStyle{Color: colors.OnBackground, FontSize: 24},
		}),
	}
}
