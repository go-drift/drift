package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildHomePage creates the main landing page with navigation to demos.
func buildHomePage(ctx core.BuildContext, isDark bool, isCupertino bool, toggleTheme func(), togglePlatform func()) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	themeLabel := "Switch to Dark"
	if isDark {
		themeLabel = "Switch to Light"
	}

	platformLabel := "Switch to Cupertino"
	if isCupertino {
		platformLabel = "Switch to Material"
	}

	// Separate demos by category
	var widgetDemos, platformDemos []Demo
	for _, demo := range demos {
		switch demo.Category {
		case CategoryWidgets:
			widgetDemos = append(widgetDemos, demo)
		case CategoryPlatform:
			platformDemos = append(platformDemos, demo)
		}
	}

	// Build navigation items grouped by category
	navItems := make([]core.Widget, 0, len(demos)*2+20)

	// Widgets & UI section
	navItems = append(navItems, sectionHeader("Widgets & UI", colors))
	navItems = append(navItems, widgets.VSpace(12))
	for i, demo := range widgetDemos {
		navItems = append(navItems, navButton(ctx, demo.Title, demo.Subtitle, demo.Route, colors))
		// Insert theming after gestures
		if demo.Route == "/gestures" {
			navItems = append(navItems, widgets.VSpace(12))
			td := themingDemo()
			navItems = append(navItems, navButton(ctx, td.Title, td.Subtitle, td.Route, colors))
		}
		if i < len(widgetDemos)-1 {
			navItems = append(navItems, widgets.VSpace(12))
		}
	}

	// Platform Services section
	navItems = append(navItems, widgets.VSpace(24))
	navItems = append(navItems, sectionHeader("Platform Services", colors))
	navItems = append(navItems, widgets.VSpace(12))
	for i, demo := range platformDemos {
		navItems = append(navItems, navButton(ctx, demo.Title, demo.Subtitle, demo.Route, colors))
		if i < len(platformDemos)-1 {
			navItems = append(navItems, widgets.VSpace(12))
		}
	}

	// ScrollView with SafeAreaPadding: content scrolls behind the status bar
	// but starts with safe area padding plus 20px on all sides.
	return widgets.Expanded{
		ChildWidget: widgets.Container{
			Color: colors.Background,
			ChildWidget: widgets.ScrollView{
				ScrollDirection: widgets.AxisVertical,
				Physics:         widgets.BouncingScrollPhysics{},
				Padding:         widgets.SafeAreaPadding(ctx).Add(20),
				ChildWidget: widgets.Column{
					MainAxisAlignment:  widgets.MainAxisAlignmentStart,
					CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
					MainAxisSize:       widgets.MainAxisSizeMin,
					ChildrenWidgets: append([]core.Widget{
						// Logo/Title section
						widgets.Container{
							Width:  200,
							Height: 100,
							Color:  graphics.ColorWhite,
							Gradient: graphics.NewRadialGradient(
								graphics.Offset{X: 100, Y: 50}, // Center
								100,                             // Radius
								[]graphics.GradientStop{
									{Position: 0, Color: graphics.RGBA(47, 249, 238, 60)},   // cyan center
									{Position: 0.5, Color: graphics.RGBA(238, 23, 130, 20)}, // magenta mid
									{Position: 1, Color: graphics.RGBA(238, 23, 130, 0)},    // fade out
								},
							),
							Alignment: layout.AlignmentCenter,
							ChildWidget: widgets.SvgImage{
								Source: loadSVGAsset("drift.svg"),
								Width:  200,
							},
						}, widgets.VSpace(8),
						widgets.TextOf("Cross-platform UI for Go", textTheme.HeadlineSmall),
						widgets.VSpace(4),
						widgets.TextOf("Build native iOS & Android apps with idiomatic Go", graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						}),
						widgets.VSpace(40),
					}, append(navItems,
						widgets.VSpace(32),

						// Theme toggle
						widgets.ButtonOf(themeLabel, toggleTheme).
							WithColor(colors.Secondary, colors.OnSecondary),
						widgets.VSpace(12),
						// Platform toggle
						widgets.ButtonOf(platformLabel, togglePlatform).
							WithColor(colors.Tertiary, colors.OnTertiary),
						widgets.VSpace(40),
					)...),
				},
			},
		},
	}
}

// sectionHeader creates a styled section header for the home page.
func sectionHeader(text string, colors theme.ColorScheme) core.Widget {
	return widgets.TextOf(text, graphics.TextStyle{
		Color:      colors.OnSurface,
		FontSize:   20,
		FontWeight: graphics.FontWeightBold,
	})
}

// navButton creates a navigation button for the home page.
func navButton(ctx core.BuildContext, title, subtitle, route string, colors theme.ColorScheme) core.Widget {
	return widgets.Button{
		Label: title,
		OnTap: func() {
			nav := navigation.NavigatorOf(ctx)
			if nav != nil {
				nav.PushNamed(route, nil)
			}
		},
		Color:        colors.SurfaceContainerHigh,
		TextColor:    colors.OnSurface,
		Padding:      layout.EdgeInsetsAll(16),
		BorderRadius: 8,
	}
}
