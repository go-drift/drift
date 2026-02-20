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
type ShowcaseApp struct{ core.StatefulBase }

func (ShowcaseApp) CreateState() core.State {
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
func (s *showcaseState) buildRoutes() []navigation.ScreenRoute {
	routes := []navigation.ScreenRoute{
		// Home page
		{
			Path: "/",
			Screen: func(ctx core.BuildContext, settings navigation.RouteSettings) core.Widget {
				return buildHomePage(ctx, s.isDark, s.toggleTheme)
			},
		},

		// Theming page (special case needing theme state)
		{
			Path: "/theming",
			Screen: func(ctx core.BuildContext, settings navigation.RouteSettings) core.Widget {
				return buildThemingPage(ctx, s.isDark, s.isCupertino)
			},
		},

		// Category hub pages
		{Path: "/theming-hub", Screen: navigation.ScreenOnly(buildThemingHubPage)},
		{Path: "/layout-hub", Screen: navigation.ScreenOnly(buildLayoutHubPage)},
		{Path: "/widgets-hub", Screen: navigation.ScreenOnly(buildWidgetsHubPage)},
		{Path: "/motion-hub", Screen: navigation.ScreenOnly(buildMotionHubPage)},
		{Path: "/media-hub", Screen: navigation.ScreenOnly(buildMediaHubPage)},
		{Path: "/system-hub", Screen: navigation.ScreenOnly(buildSystemHubPage)},
	}

	// Add all demos from registry.
	// Capture demo.Builder in a local variable so each closure gets its own copy.
	for _, demo := range demos {
		if demo.Builder != nil {
			builder := demo.Builder
			routes = append(routes, navigation.ScreenRoute{
				Path:   demo.Route,
				Screen: navigation.ScreenOnly(builder),
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
		Child: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Text{Content: "404", Style: textTheme.DisplayLarge},
				widgets.VSpace(16),
				widgets.Text{Content: "Page not found: " + settings.Name, Style: textTheme.BodyLarge},
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
		StatusBarStyle:  statusStyle,
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

	// Static routes
	routes := map[string]string{
		"home":        "/",
		"theming":     "/theming",
		"theming-hub": "/theming-hub",
		"layout-hub":  "/layout-hub",
		"widgets-hub": "/widgets-hub",
		"motion-hub":  "/motion-hub",
		"media-hub":   "/media-hub",
		"system-hub":  "/system-hub",
	}
	if name, ok := routes[candidate]; ok {
		log.Printf("deep link received: %s (source=%s)", link.URL, link.Source)
		return navigation.DeepLinkRoute{Name: name}, true
	}

	// Demo routes from registry
	for _, demo := range demos {
		if candidate == strings.TrimPrefix(demo.Route, "/") {
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
