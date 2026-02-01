package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/navigation"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

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

// itoa converts an integer to a string without importing strconv.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	neg := false
	if value < 0 {
		neg = true
		value = -value
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// demoPage creates a standard demo page with scroll view and column layout.
// This is the common pattern used by most showcase pages.
func demoPage(ctx core.BuildContext, title string, items ...core.Widget) core.Widget {
	content := widgets.ScrollView{
		ScrollDirection: widgets.AxisVertical,
		Physics:         widgets.BouncingScrollPhysics{},
		Padding:         layout.EdgeInsetsAll(20),
		ChildWidget: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			MainAxisSize:       widgets.MainAxisSizeMin,
			ChildrenWidgets:    items,
		},
	}
	return pageScaffold(ctx, title, content)
}

// gradientBorderCard creates a card with pink-to-cyan gradient border (Style A).
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

	return widgets.GestureDetector{
		OnTap: func() {
			nav := navigation.NavigatorOf(ctx)
			if nav != nil {
				nav.PushNamed(route, nil)
			}
		},
		ChildWidget: widgets.Container{
			BorderGradient: borderGradient,
			BorderWidth:    1,
			BorderRadius:   12,
			Color:          colors.Background, // Inner fill matches page background
			Height:         96,
			Alignment:      layout.AlignmentTopLeft,
			Shadow: &graphics.BoxShadow{
				Color:      PinkSeed.WithAlpha(pinkAlpha),
				Offset:     graphics.Offset{X: 0, Y: 0},
				BlurStyle:  graphics.BlurStyleOuter,
				BlurRadius: 18,
			},
			// Second shadow for cyan glow (using overlay effect)
			ChildWidget: widgets.Container{
				Overflow: widgets.OverflowVisible,
				Shadow: &graphics.BoxShadow{
					Color:      CyanSeed.WithAlpha(cyanAlpha),
					Offset:     graphics.Offset{X: 0, Y: 0},
					BlurStyle:  graphics.BlurStyleOuter,
					BlurRadius: 14,
				},
				Padding: layout.EdgeInsetsAll(18),
				ChildWidget: widgets.Column{
					MainAxisAlignment:  widgets.MainAxisAlignmentStart,
					CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
					MainAxisSize:       widgets.MainAxisSizeMax,
					ChildrenWidgets: []core.Widget{
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
							Wrap:     true,
							MaxLines: 2,
							Style: graphics.TextStyle{
								Color:    colors.OnSurfaceVariant,
								FontSize: 11,
							},
						},
					},
				},
			},
		},
	}
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

	return widgets.GestureDetector{
		OnTap: onToggle,
		ChildWidget: widgets.Container{
			Color:        colors.SurfaceContainer,
			BorderRadius: 20,
			BorderWidth:  1,
			BorderColor:  colors.OutlineVariant,
			Padding:      layout.EdgeInsetsSymmetric(14, 8),
			ChildWidget: widgets.Row{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
				ChildrenWidgets: []core.Widget{
					widgets.Text{
						Content: icon,
						Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 12,
						},
					},
					widgets.HSpace(6),
					widgets.Text{
						Content: label,
						Style: graphics.TextStyle{
							Color:    colors.OnSurfaceVariant,
							FontSize: 12,
						},
					},
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
		ChildWidget: widgets.SvgImage{
			Source:    loadSVGAsset(demo.Icon),
			Width:     20,
			Height:    20,
			TintColor: colors.OnSurface,
		},
	}

	return widgets.GestureDetector{
		OnTap: func() {
			nav := navigation.NavigatorOf(ctx)
			if nav != nil {
				nav.PushNamed(demo.Route, nil)
			}
		},
		ChildWidget: widgets.DecoratedBox{
			BorderColor:  colors.OutlineVariant, // Border stroke color; transparent = no border
			BorderWidth:  1,                     // Border stroke width in pixels; 0 = no border
			BorderRadius: 12,
			Overflow:     widgets.OverflowClip,
			ChildWidget: widgets.Column{
				MainAxisSize:       widgets.MainAxisSizeMin,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
				ChildrenWidgets: []core.Widget{
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
						ChildWidget: widgets.Row{
							MainAxisAlignment:  widgets.MainAxisAlignmentStart,
							CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
							MainAxisSize:       widgets.MainAxisSizeMax,
							ChildrenWidgets: []core.Widget{
								iconWidget,
								widgets.HSpace(14),
								widgets.Expanded{
									ChildWidget: widgets.Column{
										MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
										CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
										MainAxisSize:       widgets.MainAxisSizeMin,
										ChildrenWidgets: []core.Widget{
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
	}
}

// categoryHubPage creates a standard hub page for a demo category.
func categoryHubPage(ctx core.BuildContext, category string, title, description string) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	// Build page content: description followed by demo cards
	items := []core.Widget{
		widgets.Text{
			Content: description,
			Wrap:    true,
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
