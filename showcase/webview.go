package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildWebViewPage showcases embedding a native web view.
func buildWebViewPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "WebView",
		sectionTitle("Native WebView", colors),
		widgets.VSpace(12),
		widgets.TextOf("This view renders a platform-native browser surface.", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.TextOf("Load any HTTPS URL in the native layer.", rendering.TextStyle{
			Color:    colors.OnSurfaceVariant,
			FontSize: 13,
		}),
		widgets.VSpace(16),
		widgets.NativeWebView{
			InitialURL: "https://www.google.com",
			Height:     420,
		},
		widgets.VSpace(24),
		sectionTitle("Usage", colors),
		widgets.VSpace(12),
		codeBlock(`widgets.NativeWebView{
    InitialURL: "https://www.google.com",
    Height:     420,
}`, colors),
		widgets.VSpace(40),
	)
}
