package main

import (
	"embed"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/svg"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

//go:embed assets/*.svg assets/*.png assets/*.json
var assetFS embed.FS

var svgAssetCache = svg.NewIconCache()

func loadSVGAsset(name string) *svg.Icon {
	icon, err := svgAssetCache.Get(name, func() (*svg.Icon, error) {
		f, err := assetFS.Open("assets/" + name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return svg.Load(f)
	})
	if err != nil {
		return nil
	}
	return icon
}

func buildTabsPage(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
	iconColor := colors.Primary

	tabs := []navigation.Tab{
		buildTabSpec("Home", loadSVGAsset("home.svg"), iconColor),
		buildTabSpec("Search", loadSVGAsset("search.svg"), iconColor),
		buildTabSpec("Inbox", loadSVGAsset("inbox.svg"), iconColor),
		buildTabSpec("Explore", loadSVGAsset("explore.svg"), iconColor),
		buildTabSpec("Settings", loadSVGAsset("settings.svg"), iconColor),
	}

	return pageScaffold(ctx, "Tabs", navigation.TabNavigator{Tabs: tabs})
}

func buildTabSpec(label string, icon *svg.Icon, color graphics.Color) navigation.Tab {
	item := widgets.TabItem{
		Label: label,
		Icon:  widgets.SvgIcon{Source: icon, Size: 24, TintColor: color},
	}

	return navigation.Tab{
		Item:         item,
		InitialRoute: "/",
		OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
			switch settings.Name {
			case "/":
				return navigation.NewAnimatedPageRoute(func(ctx core.BuildContext) core.Widget {
					return buildTabRootPage(ctx, label)
				}, settings)
			case "/detail":
				return navigation.NewAnimatedPageRoute(func(ctx core.BuildContext) core.Widget {
					return buildTabDetailPage(ctx, label)
				}, settings)
			}
			return nil
		},
	}
}

func buildTabRootPage(ctx core.BuildContext, label string) core.Widget {
	colors, textTheme := theme.ColorsOf(ctx), theme.TextThemeOf(ctx)

	return widgets.Container{
		Color: colors.Background,
		Child: widgets.Centered(
			widgets.Column{
				MainAxisAlignment: widgets.MainAxisAlignmentCenter,
				MainAxisSize:      widgets.MainAxisSizeMin,
				Children: []core.Widget{
					widgets.Text{Content: label + " Tab", Style: textTheme.HeadlineMedium},
					widgets.VSpace(16),
					theme.ButtonOf(ctx, "Open details", func() {
						nav := navigation.NavigatorOf(ctx)
						if nav != nil {
							nav.PushNamed("/detail", nil)
						}
					}),
				},
			},
		),
	}
}

func buildTabDetailPage(ctx core.BuildContext, label string) core.Widget {
	colors, textTheme := theme.ColorsOf(ctx), theme.TextThemeOf(ctx)

	content := widgets.Container{
		Color: colors.Background,
		Child: widgets.Padding{
			Padding: layout.EdgeInsetsAll(24),
			Child: widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				Children: []core.Widget{
					widgets.Text{Content: label + " Details", Style: textTheme.HeadlineSmall},
					widgets.VSpace(12),
					theme.ButtonOf(ctx, "Back", func() {
						nav := navigation.NavigatorOf(ctx)
						if nav != nil {
							nav.Pop(nil)
						}
					}),
					widgets.VSpace(24),
					widgets.Text{Content: "Detail view for " + label, Style: textTheme.BodyLarge},
				},
			},
		},
	}

	return content
}
