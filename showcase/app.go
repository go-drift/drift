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

	navigator := navigation.Navigator{
		InitialRoute: "/",
		OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
			// Home page
			if settings.Name == "/" {
				return navigation.NewMaterialPageRoute(
					func(ctx core.BuildContext) core.Widget {
						return buildHomePage(ctx, s.isDark, s.toggleTheme)
					},
					settings,
				)
			}

			// Theming page (special case needing theme state)
			if settings.Name == "/theming" {
				return navigation.NewMaterialPageRoute(
					func(ctx core.BuildContext) core.Widget {
						return buildThemingPage(ctx, s.isDark, s.isCupertino)
					},
					settings,
				)
			}

			// Category hub pages (new 6-category layout)
			switch settings.Name {
			case "/theming-hub":
				return navigation.NewMaterialPageRoute(buildThemingHubPage, settings)
			case "/layout-hub":
				return navigation.NewMaterialPageRoute(buildLayoutHubPage, settings)
			case "/widgets-hub":
				return navigation.NewMaterialPageRoute(buildWidgetsHubPage, settings)
			case "/motion-hub":
				return navigation.NewMaterialPageRoute(buildMotionHubPage, settings)
			case "/media-hub":
				return navigation.NewMaterialPageRoute(buildMediaHubPage, settings)
			case "/system-hub":
				return navigation.NewMaterialPageRoute(buildSystemHubPage, settings)
			// Legacy routes (redirect to new)
			case "/foundations":
				return navigation.NewMaterialPageRoute(buildLayoutHubPage, settings)
			case "/components":
				return navigation.NewMaterialPageRoute(buildWidgetsHubPage, settings)
			case "/interactions":
				return navigation.NewMaterialPageRoute(buildMotionHubPage, settings)
			case "/platform":
				return navigation.NewMaterialPageRoute(buildSystemHubPage, settings)
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
	}

	// Single AppTheme - no tree structure change when platform toggles
	return theme.AppTheme{
		Data:  appThemeData,
		Child: navigator,
	}
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
	_, colors, textTheme := theme.UseTheme(ctx)

	// Header needs top safe area padding so it sits below the status bar
	headerPadding := widgets.SafeAreaPadding(ctx).OnlyTop().Add(16)

	return widgets.Expanded{
		Child: widgets.Container{
			Color: colors.Background,
			Child: widgets.ColumnOf(
				widgets.MainAxisAlignmentStart,
				widgets.CrossAxisAlignmentStart,
				widgets.MainAxisSizeMax,
				// Header
				widgets.Container{
					Color: colors.Surface,
					Child: widgets.Padding{
						Padding: headerPadding,
						Child: widgets.RowOf(
							widgets.MainAxisAlignmentStart,
							widgets.CrossAxisAlignmentCenter,
							widgets.MainAxisSizeMax,
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
						),
					},
				},
				// Content
				widgets.Expanded{Child: content},
			),
		},
	}
}
