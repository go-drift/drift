package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildHomePage creates the main landing page with navigation to demos.
func buildHomePage(ctx core.BuildContext, isDark bool, isCupertino bool, systemTransparent bool, toggleTheme func(), togglePlatform func(), toggleTransparency func()) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	themeLabel := "Switch to Dark"
	if isDark {
		themeLabel = "Switch to Light"
	}

	platformLabel := "Switch to Cupertino"
	if isCupertino {
		platformLabel = "Switch to Material"
	}

	transparencyLabel := "Enable Transparent System UI"
	if systemTransparent {
		transparencyLabel = "Disable Transparent System UI"
	}

	// Build navigation items from registry
	navItems := make([]core.Widget, 0, len(demos)*2+10)
	for i, demo := range demos {
		navItems = append(navItems, navButton(ctx, demo.Title, demo.Subtitle, demo.Route, colors))
		// Insert theming after scroll (index 6)
		if i == 6 {
			navItems = append(navItems, widgets.VSpace(12))
			td := themingDemo()
			navItems = append(navItems, navButton(ctx, td.Title, td.Subtitle, td.Route, colors))
		}
		if i < len(demos)-1 {
			navItems = append(navItems, widgets.VSpace(12))
		}
	}

	// ScrollView with SafeAreaPadding: content scrolls behind the status bar
	// but starts with safe area padding plus 24px on all sides.
	return widgets.Expanded{
		ChildWidget: widgets.NewContainer(
			widgets.ScrollView{
				ScrollDirection: widgets.AxisVertical,
				Physics:         widgets.BouncingScrollPhysics{},
				Padding:         widgets.SafeAreaPadding(ctx).Add(20),
				ChildWidget: widgets.Column{
					MainAxisAlignment:  widgets.MainAxisAlignmentStart,
					CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
					MainAxisSize:       widgets.MainAxisSizeMin,
					ChildrenWidgets: append([]core.Widget{
						// Logo/Title section
						widgets.TextOf("Drift", rendering.TextStyle{
							Color:      colors.Primary,
							FontSize:   48,
							FontWeight: rendering.FontWeightBold,
						}),
						widgets.VSpace(8),
						widgets.TextOf("Cross-platform UI for Go", textTheme.HeadlineSmall),
						widgets.VSpace(4),
						widgets.TextOf("Build native iOS & Android apps with idiomatic Go", rendering.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 14,
						}),
						widgets.VSpace(40),

						// Demo sections
						widgets.TextOf("Explore Features", textTheme.TitleLarge),
						widgets.VSpace(16),
					}, append(navItems,
						widgets.VSpace(32),

						// Theme toggle
						widgets.NewButton(themeLabel, toggleTheme).
							WithColor(colors.Secondary, colors.OnSecondary),
						widgets.VSpace(12),
						// Platform toggle
						widgets.NewButton(platformLabel, togglePlatform).
							WithColor(colors.Tertiary, colors.OnTertiary),
						widgets.VSpace(12),
						widgets.NewButton(transparencyLabel, toggleTransparency).
							WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
						widgets.VSpace(40),
					)...),
				},
			},
		).WithColor(colors.Background).Build(),
	}
}

// navButton creates a navigation button for the home page.
func navButton(ctx core.BuildContext, title, subtitle, route string, colors theme.ColorScheme) core.Widget {
	return widgets.GestureDetector{
		OnTap: func() {
			nav := navigation.NavigatorOf(ctx)
			if nav != nil {
				nav.PushNamed(route, nil)
			}
		},
		ChildWidget: widgets.NewContainer(
			widgets.PaddingAll(16,
				widgets.ColumnOf(
					widgets.MainAxisAlignmentStart,
					widgets.CrossAxisAlignmentStart,
					widgets.MainAxisSizeMin,
					widgets.TextOf(title, rendering.TextStyle{
						Color:      colors.OnSurface,
						FontSize:   18,
						FontWeight: rendering.FontWeightBold,
					}),
					widgets.VSpace(4),
					widgets.TextOf(subtitle, rendering.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 14,
					}),
				),
			),
		).WithColor(colors.SurfaceContainerHigh).Build(),
	}
}
