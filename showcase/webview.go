package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

func buildWebViewPage(ctx core.BuildContext) core.Widget {
	return webViewPage{}
}

type webViewPage struct{}

func (w webViewPage) CreateElement() core.Element {
	return core.NewStatefulElement(w, nil)
}

func (w webViewPage) Key() any {
	return nil
}

func (w webViewPage) CreateState() core.State {
	return &webViewState{}
}

type webViewState struct {
	core.StateBase
	controller *platform.WebViewController
	status     *core.ManagedState[string]
}

func (s *webViewState) InitState() {
	s.status = core.NewManagedState(&s.StateBase, "Idle")
	s.controller = core.UseController(&s.StateBase, platform.NewWebViewController)

	s.controller.OnPageStarted = func(url string) {
		s.status.Set("Loading: " + url)
	}
	s.controller.OnPageFinished = func(url string) {
		s.status.Set("Loaded: " + url)
	}
	s.controller.OnError = func(code, message string) {
		s.status.Set("Error (" + code + "): " + message)
	}

	s.controller.Load("https://www.google.com")
}

func (s *webViewState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "WebView",
		widgets.Text{
			Content: "Platform-native browser surface with navigation controls and page-loading callbacks.",
			Wrap:    true,
			Style:   labelStyle(colors),
		},
		widgets.VSpace(12),
		// URL buttons
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				smallButton(ctx, "Load google.com", func() {
					s.controller.Load("https://www.google.com")
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "Load go.dev", func() {
					s.controller.Load("https://go.dev")
				}, colors),
			},
		},
		widgets.VSpace(8),
		// Navigation controls
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				smallButton(ctx, "Back", func() {
					s.controller.GoBack()
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "Forward", func() {
					s.controller.GoForward()
				}, colors),
				widgets.HSpace(6),
				smallButton(ctx, "Reload", func() {
					s.controller.Reload()
				}, colors),
			},
		},
		widgets.VSpace(12),
		widgets.NativeWebView{
			Controller: s.controller,
			Height:     420,
		},
		widgets.VSpace(8),
		statusCard(s.status.Get(), colors),
		widgets.VSpace(40),
	)
}
