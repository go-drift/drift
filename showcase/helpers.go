package main

import (
	"strconv"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// pageScaffold creates a consistent page layout with title and back button.
func pageScaffold(ctx core.BuildContext, title string, content core.Widget) core.Widget {
	colors, textTheme := theme.ColorsOf(ctx), theme.TextThemeOf(ctx)

	backButton := widgets.Button{
		Label: "Back",
		OnTap: func() {
			if nav := navigation.NavigatorOf(ctx); nav != nil {
				nav.Pop(nil)
			}
		},
		Color:        colors.SurfaceContainerHigh,
		TextColor:    colors.OnSurface,
		Padding:      layout.EdgeInsetsSymmetric(16, 10),
		BorderRadius: 8,
		FontSize:     14,
		Haptic:       true,
	}

	// Header needs top safe area padding so it sits below the status bar
	headerPadding := widgets.SafeAreaPadding(ctx).OnlyTop().Add(16)
	header := widgets.Container{
		Color: colors.Surface,
		Child: widgets.Padding{
			Padding: headerPadding,
			Child: widgets.Row{
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				Children: []core.Widget{
					backButton,
					widgets.HSpace(16),
					widgets.Text{Content: title, Style: textTheme.HeadlineMedium},
				},
			},
		},
	}

	return widgets.Expanded{
		Child: widgets.Container{
			Color: colors.Background,
			Child: widgets.Column{
				Children: []core.Widget{
					header,
					widgets.Expanded{Child: content},
				},
			},
		},
	}
}

// demoPage creates a standard demo page with scroll view and column layout.
// This is the common pattern used by most showcase pages.
func demoPage(ctx core.BuildContext, title string, items ...core.Widget) core.Widget {
	content := widgets.ScrollView{
		ScrollDirection: widgets.AxisVertical,
		Physics:         widgets.BouncingScrollPhysics{},
		Padding:         layout.EdgeInsetsAll(20),
		Child: widgets.Column{
			MainAxisSize: widgets.MainAxisSizeMin,
			Children:     items,
		},
	}
	return pageScaffold(ctx, title, content)
}

// categoryHubPage creates a standard hub page for a demo category.
func categoryHubPage(ctx core.BuildContext, category string, title, description string) core.Widget {
	colors := theme.ColorsOf(ctx)

	// Build page content: description followed by demo cards
	items := []core.Widget{
		widgets.Text{
			Content: description,
			Style: graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			},
		},
		widgets.VSpace(24),
	}
	for _, demo := range demosForCategory(category) {
		items = append(items, demoCard(ctx, demo, colors), widgets.VSpace(12))
	}

	return demoPage(ctx, title, items...)
}

// sectionTitle creates a styled section header for demo pages.
func sectionTitle(text string, colors theme.ColorScheme) core.Widget {
	return widgets.Text{
		Content: text,
		Style: graphics.TextStyle{
			Color:      colors.Primary,
			FontSize:   20,
			FontWeight: graphics.FontWeightBold,
		},
	}
}

// labelStyle returns a text style for descriptive labels.
func labelStyle(colors theme.ColorScheme) graphics.TextStyle {
	return graphics.TextStyle{
		Color:    colors.OnSurfaceVariant,
		FontSize: 14,
	}
}

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return strconv.Itoa(int(bytes/GB)) + " GB"
	case bytes >= MB:
		return strconv.Itoa(int(bytes/MB)) + " MB"
	case bytes >= KB:
		return strconv.Itoa(int(bytes/KB)) + " KB"
	default:
		return strconv.Itoa(int(bytes)) + " B"
	}
}

// smallButton creates a compact tappable button for secondary actions.
func smallButton(ctx core.BuildContext, label string, onTap func(), colors theme.ColorScheme) core.Widget {
	return widgets.GestureDetector{
		OnTap: onTap,
		Child: widgets.Container{
			Color:        colors.SurfaceContainerHigh,
			BorderRadius: 6,
			Padding:      layout.EdgeInsetsSymmetric(12, 6),
			Child: widgets.Text{
				Content: label,
				Style: graphics.TextStyle{
					Color:    colors.OnSurface,
					FontSize: 13,
				},
			},
		},
	}
}

// gradientBorderCard creates a card with pink-to-cyan gradient border.
// Used for the 6-card category grid on the home page.
func gradientBorderCard(ctx core.BuildContext, title, description, route string, colors theme.ColorScheme, isDark bool) core.Widget {
	// Gradient border from pink to cyan at 135 degrees
	borderGradient := graphics.NewLinearGradient(
		graphics.AlignTopLeft,
		graphics.AlignBottomRight,
		[]graphics.GradientStop{
			{Position: 0, Color: PinkSeed},
			{Position: 1, Color: CyanSeed},
		},
	)

	// Shadow glow: pink glow + cyan glow
	// Adjust opacity based on theme
	pinkAlpha := float64(0.3)
	cyanAlpha := float64(0.2)
	if !isDark {
		pinkAlpha = 0.2
		cyanAlpha = 0.1
	}

	content := widgets.Column{
		Children: []core.Widget{
			widgets.Text{
				Content: title,
				Style: graphics.TextStyle{
					Color:      colors.OnSurface,
					FontSize:   15,
					FontWeight: graphics.FontWeightSemibold,
				},
			},
			widgets.VSpace(6),
			widgets.Text{
				Content:  description,
				MaxLines: 2,
				Style: graphics.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 11,
				},
			},
		},
	}

	return widgets.Tappable(
		"",
		func() {
			if nav := navigation.NavigatorOf(ctx); nav != nil {
				nav.PushNamed(route, nil)
			}
		},
		widgets.Container{
			BorderGradient: borderGradient,
			BorderWidth:    1,
			BorderRadius:   12,
			Color:          colors.Background, // Inner fill matches page background
			Height:         84,
			Alignment:      layout.AlignmentTopLeft,
			Shadow: &graphics.BoxShadow{
				Color:      PinkSeed.WithAlpha(pinkAlpha),
				BlurStyle:  graphics.BlurStyleOuter,
				BlurRadius: 18,
			},
			// Second shadow for cyan glow (using overlay effect)
			Child: widgets.Container{
				Overflow: widgets.OverflowVisible,
				Shadow: &graphics.BoxShadow{
					Color:      CyanSeed.WithAlpha(cyanAlpha),
					BlurStyle:  graphics.BlurStyleOuter,
					BlurRadius: 14,
				},
				Padding: layout.EdgeInsetsAll(18),
				Child:   content,
			},
		},
	)
}

// themeToggleButton creates the theme toggle pill button.
func themeToggleButton(ctx core.BuildContext, isDark bool, onToggle func()) core.Widget {
	colors := theme.ColorsOf(ctx)

	label := "Light"
	icon := "\u2600" // Sun
	if isDark {
		label = "Dark"
		icon = "\u263E" // Moon
	}

	textStyle := graphics.TextStyle{
		Color:    colors.OnSurfaceVariant,
		FontSize: 12,
	}

	return widgets.GestureDetector{
		OnTap: onToggle,
		Child: widgets.Container{
			Color:        colors.SurfaceContainer,
			BorderRadius: 20,
			BorderWidth:  1,
			BorderColor:  colors.OutlineVariant,
			Padding:      layout.EdgeInsetsSymmetric(14, 8),
			Child: widgets.Row{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				Children: []core.Widget{
					widgets.Text{Content: icon, Style: textStyle},
					widgets.HSpace(6),
					widgets.Text{Content: label, Style: textStyle},
				},
			},
		},
	}
}

// demoCard creates a navigation card for a demo within a category hub.
// Uses a dark card with thin cyan-to-pink gradient bar at the top.
func demoCard(ctx core.BuildContext, demo Demo, colors theme.ColorScheme) core.Widget {
	iconWidget := widgets.Container{
		Width:        40,
		Height:       40,
		BorderRadius: 20, // Circle
		Color:        colors.Surface,
		BorderColor:  CyanSeed.WithAlpha(0.3),
		BorderWidth:  1,
		Overflow:     widgets.OverflowVisible,
		Shadow: &graphics.BoxShadow{
			Color:      PinkSeed.WithAlpha(0.4),
			BlurRadius: 10,
			BlurStyle:  graphics.BlurStyleOuter,
		},
		Alignment: layout.AlignmentCenter,
		Child: widgets.SvgImage{
			Source:    loadSVGAsset(demo.Icon),
			Width:     20,
			Height:    20,
			TintColor: colors.OnSurface,
		},
	}

	return widgets.Tappable(
		"",
		func() {
			if nav := navigation.NavigatorOf(ctx); nav != nil {
				nav.PushNamed(demo.Route, nil)
			}
		},
		widgets.DecoratedBox{
			BorderColor:  colors.OutlineVariant, // Border stroke color; transparent = no border
			BorderWidth:  1,                     // Border stroke width in pixels; 0 = no border
			BorderRadius: 12,
			Overflow:     widgets.OverflowClip,
			Child: widgets.Column{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
				Children: []core.Widget{
					// Thin gradient bar at top
					widgets.Container{
						Height: 4,
						Gradient: graphics.NewLinearGradient(
							graphics.AlignCenterLeft,
							graphics.AlignCenterRight,
							[]graphics.GradientStop{
								{Position: 0, Color: PinkSeed},
								{Position: 1, Color: CyanSeed},
							},
						),
					},
					// Content with padding
					widgets.Padding{
						Padding: layout.EdgeInsetsOnly(16, 14, 16, 16),
						Child: widgets.Row{
							CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
							Children: []core.Widget{
								iconWidget,
								widgets.HSpace(14),
								widgets.Expanded{
									Child: widgets.Column{
										MainAxisAlignment: widgets.MainAxisAlignmentCenter,
										MainAxisSize:      widgets.MainAxisSizeMin,
										Children: []core.Widget{
											widgets.Text{
												Content: demo.Title,
												Style: graphics.TextStyle{
													Color:      colors.OnSurface,
													FontSize:   15,
													FontWeight: graphics.FontWeightSemibold,
												},
											},
											widgets.VSpace(2),
											widgets.Text{
												Content: demo.Subtitle,
												Style: graphics.TextStyle{
													Color:    colors.OnSurfaceVariant,
													FontSize: 12,
												},
											},
										},
									},
								},
								widgets.Text{
									Content: "\u203A", // Chevron
									Style: graphics.TextStyle{
										Color:    colors.OnSurfaceVariant,
										FontSize: 18,
									},
								},
							},
						},
					},
				},
			},
		},
	)
}

// permissionBadge renders a colored badge showing permission status.
func permissionBadge(status platform.PermissionStatus, colors theme.ColorScheme) core.Widget {
	label := string(status)
	if label == "" {
		label = "unknown"
	}

	bgColor := graphics.Color(0xFFFFFFFF)
	textColor := graphics.Color(0xFFFFFFFF)

	switch status {
	case platform.PermissionGranted:
		bgColor = 0xFF4CAF50 // green
	case platform.PermissionDenied, platform.PermissionPermanentlyDenied:
		bgColor = 0xFFF44336 // red
	case platform.PermissionLimited, platform.PermissionProvisional:
		bgColor = 0xFFFF9800 // orange
	default:
		bgColor = colors.SurfaceContainerHigh
		textColor = colors.OnSurfaceVariant
	}

	return widgets.DecoratedBox{
		Color:        bgColor,
		BorderRadius: 4,
		Child: widgets.Padding{
			Padding: layout.EdgeInsetsSymmetric(8, 4),
			Child: widgets.Text{Content: label, Style: graphics.TextStyle{
				Color:    textColor,
				FontSize: 12,
			}},
		},
	}
}

// statusCard creates a styled status message card used across demo pages.
func statusCard(text string, colors theme.ColorScheme) core.Widget {
	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding:      layout.EdgeInsetsAll(12),
		Child: widgets.Text{Content: text, Style: graphics.TextStyle{
			Color:    colors.OnSurfaceVariant,
			FontSize: 14,
		}},
	}
}
