package main

import (
	"embed"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/svg"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

//go:embed assets/*.svg assets/*.png
var assetFS embed.FS

func loadSVGAsset(name string) *svg.Icon {
	f, err := assetFS.Open("assets/" + name)
	if err != nil {
		return nil
	}
	defer f.Close()
	icon, _ := svg.Load(f)
	return icon
}

func buildTabsPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	iconColor := colors.Primary

	tabs := []navigation.Tab{
		buildTabSpec("Home", loadSVGAsset("home.svg"), iconColor),
		buildTabSpec("Search", loadSVGAsset("search.svg"), iconColor),
		buildTabSpec("Inbox", loadSVGAsset("inbox.svg"), iconColor),
		buildTabSpec("Explore", loadSVGAsset("explore.svg"), iconColor),
		buildTabSpec("Settings", loadSVGAsset("settings.svg"), iconColor),
	}

	return pageScaffold(ctx, "Tabs", navigation.TabScaffold{Tabs: tabs})
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
				return navigation.NewMaterialPageRoute(func(ctx core.BuildContext) core.Widget {
					return buildTabRootPage(ctx, label)
				}, settings)
			case "/detail":
				return navigation.NewMaterialPageRoute(func(ctx core.BuildContext) core.Widget {
					return buildTabDetailPage(ctx, label)
				}, settings)
			}
			return nil
		},
	}
}

func buildTabRootPage(ctx core.BuildContext, label string) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	return widgets.Container{
		Color: colors.Background,
		ChildWidget: widgets.Centered(
			widgets.ColumnOf(
				widgets.MainAxisAlignmentCenter,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMin,

				widgets.Text{Content: label + " Tab", Style: textTheme.HeadlineMedium},
				widgets.VSpace(16),
				theme.ButtonOf(ctx, "Open details", func() {
					nav := navigation.NavigatorOf(ctx)
					if nav != nil {
						nav.PushNamed("/detail", nil)
					}
				}),
			),
		),
	}
}

func buildTabDetailPage(ctx core.BuildContext, label string) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	content := widgets.Container{
		Color: colors.Background,
		ChildWidget: widgets.Centered(
			widgets.Text{Content: "Detail view for " + label, Style: textTheme.BodyLarge},
		),
	}

	return pageScaffold(ctx, label+" Details", content)
}
