package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// webViewPage is the webview demo widget.
type webViewPage struct{ core.StatefulBase }

func (webViewPage) CreateState() core.State { return &webViewState{} }

func buildWebViewPage(_ core.BuildContext) core.Widget { return webViewPage{} }

type webViewState struct {
	core.StateBase
	controller *platform.WebViewController
	createErr  error
	// failure is the last load failure, cleared when a page loads.
	failure *core.Signal[string]
}

func (s *webViewState) InitState() {
	s.controller, s.createErr = platform.NewWebViewController()
	if s.createErr != nil {
		return
	}
	core.UseDisposable(s, s.controller)
	core.UseListenable(s, s.controller.Page())
	s.failure = core.NewSignal("")
	core.UseListenable(s, s.failure)
	s.OnDispose(s.controller.Errors().Listen(platform.StreamHandler[error]{
		OnEvent: func(err error) {
			drift.Dispatch(func() { s.failure.Set(err.Error()) })
		},
	}))
	s.OnDispose(s.controller.Page().AddListener(func() {
		if s.controller.Page().Value().Loading {
			s.failure.Set("")
		}
	}))

	s.controller.Load("https://www.google.com")
}

// status describes the page for the status card.
func (s *webViewState) status() string {
	if failure := s.failure.Value(); failure != "" {
		return "Error: " + failure
	}
	page := s.controller.Page().Value()
	switch {
	case page.URL == "":
		return "Idle"
	case page.Loading:
		return "Loading: " + page.URL
	default:
		return "Loaded: " + page.URL
	}
}

func (s *webViewState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
	if s.createErr != nil {
		return demoPage(ctx, "WebView", statusCard("Error: "+s.createErr.Error(), colors))
	}

	return demoPage(ctx, "WebView",
		widgets.Text{
			Content: "Platform-native browser surface with navigation controls and page-loading callbacks.",
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
		statusCard(s.status(), colors),
		widgets.VSpace(40),
	)
}
