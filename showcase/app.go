// Package main provides the Drift demo application.
// It demonstrates idiomatic patterns for building UIs with Drift.
package main

import (
	"log"
	"net/url"
	"strings"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/engine"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// App returns the root widget for the Drift showcase demo.
func App() core.Widget {
	return ShowcaseApp{}
}

// ShowcaseApp is the main demo application widget.
// It manages theme state and sets up navigation.
type ShowcaseApp struct{}

func (s ShowcaseApp) CreateElement() core.Element {
	return core.NewStatefulElement(s, nil)
}

func (s ShowcaseApp) Key() any {
	return nil
}

func (s ShowcaseApp) CreateState() core.State {
	return &showcaseState{}
}

type showcaseState struct {
	core.StateBase
	isDark              bool
	transparentSystemUI bool
	deepLinkController  *navigation.DeepLinkController
}

func (s *showcaseState) InitState() {
	s.isDark = true // Start with dark theme
	s.transparentSystemUI = false
	s.updateBackgroundColor()
	s.applySystemUI()
	s.deepLinkController = navigation.NewDeepLinkController(s.deepLinkRoute, func(err error) {
		log.Printf("deep link error: %v", err)
	})
}

func (s *showcaseState) Build(ctx core.BuildContext) core.Widget {
	themeData := s.currentThemeData()

	return theme.Theme{
		Data: themeData,
		ChildWidget: navigation.Navigator{
			InitialRoute: "/",
			OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
				// Home page (special case with state callbacks)
				if settings.Name == "/" {
					return navigation.NewMaterialPageRoute(
						func(ctx core.BuildContext) core.Widget {
							return buildHomePage(ctx, s.isDark, s.transparentSystemUI, s.toggleTheme, s.toggleSystemTransparency)
						},
						settings,
					)
				}

				// Theming page (special case needing isDark state)
				if settings.Name == "/theming" {
					return navigation.NewMaterialPageRoute(
						func(ctx core.BuildContext) core.Widget {
							return buildThemingPage(ctx, s.isDark)
						},
						settings,
					)
				}

				// All other demos from registry
				for _, demo := range demos {
					if settings.Name == demo.Route {
						builder := demo.Builder
						return navigation.NewMaterialPageRoute(
							func(ctx core.BuildContext) core.Widget {
								return builder(ctx)
							},
							settings,
						)
					}
				}
				return nil
			},
		},
	}
}

func (s *showcaseState) currentThemeData() *theme.ThemeData {
	if s.isDark {
		return theme.DefaultDarkTheme()
	}
	return theme.DefaultLightTheme()
}

func (s *showcaseState) updateBackgroundColor() {
	themeData := s.currentThemeData()
	engine.SetBackgroundColor(rendering.Color(themeData.ColorScheme.Background))
}

func (s *showcaseState) applySystemUI() {
	themeData := s.currentThemeData()
	statusStyle := platform.StatusBarStyleDark
	if themeData.Brightness == theme.BrightnessDark {
		statusStyle = platform.StatusBarStyleLight
	}
	backgroundColor := themeData.ColorScheme.Surface
	_ = platform.SetSystemUI(platform.SystemUIStyle{
		StatusBarHidden: false,
		StatusBarStyle:  statusStyle,
		TitleBarHidden:  false,
		BackgroundColor: &backgroundColor,
		Transparent:     s.transparentSystemUI,
	})
}

func (s *showcaseState) deepLinkRoute(link platform.DeepLink) (navigation.DeepLinkRoute, bool) {
	parsed, err := url.Parse(link.URL)
	if err != nil {
		return navigation.DeepLinkRoute{}, false
	}
	candidate := strings.Trim(parsed.Path, "/")
	if candidate == "" {
		candidate = parsed.Host
	}
	if candidate == "" {
		return navigation.DeepLinkRoute{}, false
	}

	// Home route
	if candidate == "home" {
		log.Printf("deep link received: %s (source=%s)", link.URL, link.Source)
		return navigation.DeepLinkRoute{Name: "/"}, true
	}

	// Theming route (special case)
	if candidate == "theming" {
		log.Printf("deep link received: %s (source=%s)", link.URL, link.Source)
		return navigation.DeepLinkRoute{Name: "/theming"}, true
	}

	// Check demos from registry
	for _, demo := range demos {
		routeName := strings.TrimPrefix(demo.Route, "/")
		if candidate == routeName {
			log.Printf("deep link received: %s (source=%s)", link.URL, link.Source)
			return navigation.DeepLinkRoute{Name: demo.Route}, true
		}
	}

	log.Printf("deep link ignored: %s (source=%s)", link.URL, link.Source)
	return navigation.DeepLinkRoute{}, false
}

func (s *showcaseState) toggleTheme() {
	s.SetState(func() {
		s.isDark = !s.isDark
	})
	s.updateBackgroundColor()
	s.applySystemUI()
}

func (s *showcaseState) toggleSystemTransparency() {
	s.SetState(func() {
		s.transparentSystemUI = !s.transparentSystemUI
	})
	s.applySystemUI()
}

// pageScaffold creates a consistent page layout with title and back button.
func pageScaffold(ctx core.BuildContext, title string, content core.Widget) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	// Header needs top safe area padding so it sits below the status bar
	headerPadding := widgets.SafeAreaPadding(ctx).OnlyTop().Add(16)

	return widgets.Expanded{
		ChildWidget: widgets.NewContainer(
			widgets.ColumnOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				// Header
				widgets.NewContainer(
					widgets.Padding{
						Padding: headerPadding,
						ChildWidget: widgets.RowOf(
							widgets.MainAxisAlignmentStart,
							widgets.CrossAxisAlignmentStart,
							widgets.MainAxisSizeMax,
							widgets.NewButton("Back", func() {
								nav := navigation.NavigatorOf(ctx)
								if nav != nil {
									nav.Pop(nil)
								}
							}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant).
								WithPadding(layout.EdgeInsetsSymmetric(16, 10)).
								WithFontSize(14),
							widgets.HSpace(16),
							widgets.TextOf(title, textTheme.HeadlineMedium),
						),
					},
				).WithColor(colors.Surface).Build(),
				// Content
				widgets.Expanded{ChildWidget: content},
			),
		).WithColor(colors.Background).Build(),
	}
}
