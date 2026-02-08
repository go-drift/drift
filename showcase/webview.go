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
}

func (s *webViewState) InitState() {
	s.controller = core.UseController(&s.StateBase, platform.NewWebViewController)

	s.controller.Load("https://www.google.com")
}

func (s *webViewState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "WebView",
		sectionTitle("Native WebView", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "This view renders a platform-native browser surface.", Style: labelStyle(colors)},
		widgets.VSpace(16),
		widgets.NativeWebView{
			Controller: s.controller,
			Height:     420,
		},
		widgets.VSpace(40),
	)
}
