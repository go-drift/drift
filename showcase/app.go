// Package main provides the Drift demo application.
// It demonstrates idiomatic patterns for building UIs with Drift.
package main

import (
	"log"
	"net/url"
	"strings"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/engine"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/platform"
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
	isDark             bool
	isCupertino        bool
	deepLinkController *navigation.DeepLinkController
	// Memoized theme data to avoid churn in UpdateShouldNotify
	cachedThemeData *theme.AppThemeData
}

func (s *showcaseState) InitState() {
	s.isDark = true // Start with dark theme
	s.updateBackgroundColor()
	s.applySystemUI()
	s.deepLinkController = navigation.NewDeepLinkController(s.deepLinkRoute, func(err error) {
		log.Printf("deep link error: %v", err)
	})
}

func (s *showcaseState) Build(ctx core.BuildContext) core.Widget {
	// Get memoized theme data (only recreated when values change)
	appThemeData := s.getAppThemeData()

	// Build routes for the declarative Router
	routes := s.buildRoutes()

	router := navigation.Router{
		InitialPath:  "/",
		Routes:       routes,
		ErrorBuilder: buildNotFoundPage,
	}

	// Single AppTheme - no tree structure change when platform toggles
	return theme.AppTheme{
		Data:  appThemeData,
		Child: router,
	}
}

// buildRoutes constructs the declarative route configuration.
func (s *showcaseState) buildRoutes() []navigation.RouteConfigurer {
	routes := []navigation.RouteConfigurer{
		// Home page
		navigation.RouteConfig{
			Path: "/",
			Builder: func(ctx core.BuildContext, settings navigation.RouteSettings) core.Widget {
				return buildHomePage(ctx, s.isDark, s.toggleTheme)
			},
		},

		// Theming page (special case needing theme state)
		navigation.RouteConfig{
			Path: "/theming",
			Builder: func(ctx core.BuildContext, settings navigation.RouteSettings) core.Widget {
				return buildThemingPage(ctx, s.isDark, s.isCupertino)
			},
		},

		// Category hub pages
		navigation.RouteConfig{
			Path:    "/theming-hub",
			Builder: navigation.SimpleBuilder(buildThemingHubPage),
		},
		navigation.RouteConfig{
			Path:    "/layout-hub",
			Builder: navigation.SimpleBuilder(buildLayoutHubPage),
		},
		navigation.RouteConfig{
			Path:    "/widgets-hub",
			Builder: navigation.SimpleBuilder(buildWidgetsHubPage),
		},
		navigation.RouteConfig{
			Path:    "/motion-hub",
			Builder: navigation.SimpleBuilder(buildMotionHubPage),
		},
		navigation.RouteConfig{
			Path:    "/media-hub",
			Builder: navigation.SimpleBuilder(buildMediaHubPage),
		},
		navigation.RouteConfig{
			Path:    "/system-hub",
			Builder: navigation.SimpleBuilder(buildSystemHubPage),
		},

		// Legacy routes (redirect to new hubs)
		navigation.RouteConfig{
			Path:    "/foundations",
			Builder: navigation.SimpleBuilder(buildLayoutHubPage),
		},
		navigation.RouteConfig{
			Path:    "/components",
			Builder: navigation.SimpleBuilder(buildWidgetsHubPage),
		},
		navigation.RouteConfig{
			Path:    "/interactions",
			Builder: navigation.SimpleBuilder(buildMotionHubPage),
		},
		navigation.RouteConfig{
			Path:    "/platform",
			Builder: navigation.SimpleBuilder(buildSystemHubPage),
		},
	}

	// Add all demos from registry
	for _, demo := range demos {
		if demo.Builder != nil {
			builder := demo.Builder
			routes = append(routes, navigation.RouteConfig{
				Path:    demo.Route,
				Builder: navigation.SimpleBuilder(builder),
			})
		}
	}

	return routes
}

// buildNotFoundPage shows a 404 error page.
func buildNotFoundPage(ctx core.BuildContext, settings navigation.RouteSettings) core.Widget {
	colors, textTheme := theme.ColorsOf(ctx), theme.TextThemeOf(ctx)
	return pageScaffold(ctx, "Not Found", widgets.Container{
		Color: colors.Background,
		Child: widgets.Center{
			Child: widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				MainAxisSize:       widgets.MainAxisSizeMin,
				Children: []core.Widget{
					widgets.Text{Content: "404", Style: textTheme.DisplayLarge},
					widgets.VSpace(16),
					widgets.Text{Content: "Page not found: " + settings.Name, Style: textTheme.BodyLarge},
				},
			},
		},
	})
}

// getAppThemeData returns memoized theme data, recreating only when state changes.
func (s *showcaseState) getAppThemeData() *theme.AppThemeData {
	targetPlatform := theme.TargetPlatformMaterial
	if s.isCupertino {
		targetPlatform = theme.TargetPlatformCupertino
	}

	brightness := theme.BrightnessLight
	if s.isDark {
		brightness = theme.BrightnessDark
	}

	// Only recreate if values changed
	if s.cachedThemeData == nil ||
		s.cachedThemeData.Platform != targetPlatform ||
		s.cachedThemeData.Brightness() != brightness {

		// Create new theme data
		var material *theme.ThemeData
		var cupertino *theme.CupertinoThemeData

		if s.isDark {
			// Use showcase dark theme
			material = ShowcaseDarkTheme()
			cupertino = theme.DefaultCupertinoDarkTheme()
		} else {
			// Use showcase light theme for light mode
			material = ShowcaseLightTheme()
			cupertino = theme.DefaultCupertinoLightTheme()
		}

		s.cachedThemeData = &theme.AppThemeData{
			Platform:  targetPlatform,
			Material:  material,
			Cupertino: cupertino,
		}
	}
	return s.cachedThemeData
}

func (s *showcaseState) updateBackgroundColor() {
	appThemeData := s.getAppThemeData()
	engine.SetBackgroundColor(graphics.Color(appThemeData.Material.ColorScheme.Background))
}

func (s *showcaseState) applySystemUI() {
	appThemeData := s.getAppThemeData()
	statusStyle := platform.StatusBarStyleDark
	if appThemeData.Brightness() == theme.BrightnessDark {
		statusStyle = platform.StatusBarStyleLight
	}
	backgroundColor := appThemeData.Material.ColorScheme.Surface
	_ = platform.SetSystemUI(platform.SystemUIStyle{
		StatusBarHidden: false,
		StatusBarStyle:  statusStyle,
		TitleBarHidden:  false,
		BackgroundColor: &backgroundColor,
		Transparent:     true,
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

	// Category hub routes (new 6-category layout)
	categoryRoutes := map[string]string{
		"theming-hub": "/theming-hub",
		"layout-hub":  "/layout-hub",
		"widgets-hub": "/widgets-hub",
		"motion-hub":  "/motion-hub",
		"media-hub":   "/media-hub",
		"system-hub":  "/system-hub",
		// Legacy routes (redirect to new)
		"foundations":  "/layout-hub",
		"components":   "/widgets-hub",
		"interactions": "/motion-hub",
		"platform":     "/system-hub",
	}
	if route, ok := categoryRoutes[candidate]; ok {
		log.Printf("deep link received: %s (source=%s)", link.URL, link.Source)
		return navigation.DeepLinkRoute{Name: route}, true
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

// pageScaffold creates a consistent page layout with title and back button.
func pageScaffold(ctx core.BuildContext, title string, content core.Widget) core.Widget {
	colors, textTheme := theme.ColorsOf(ctx), theme.TextThemeOf(ctx)

	// Header needs top safe area padding so it sits below the status bar
	headerPadding := widgets.SafeAreaPadding(ctx).OnlyTop().Add(16)

	return widgets.Expanded{
		Child: widgets.Container{
			Color: colors.Background,
			Child: widgets.Column{
				Children: []core.Widget{
					// Header
					widgets.Container{
						Color: colors.Surface,
						Child: widgets.Padding{
							Padding: headerPadding,
							Child: widgets.Row{
								CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
								Children: []core.Widget{
									widgets.Button{
										Label: "Back",
										OnTap: func() {
											nav := navigation.NavigatorOf(ctx)
											if nav != nil {
												nav.Pop(nil)
											}
										},
										Color:        colors.SurfaceContainerHigh,
										TextColor:    colors.OnSurface,
										Padding:      layout.EdgeInsetsSymmetric(16, 10),
										BorderRadius: 8,
										FontSize:     14,
										Haptic:       true,
									},
									widgets.HSpace(16),
									widgets.Text{Content: title, Style: textTheme.HeadlineMedium},
								},
							},
						},
					},
					// Content
					widgets.Expanded{Child: content},
				},
			},
		},
	}
}
