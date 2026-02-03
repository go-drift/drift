package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildHomePage creates the main landing page with navigation to demos.
func buildHomePage(ctx core.BuildContext, isDark bool, toggleTheme func()) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return widgets.Expanded{
		Child: widgets.Container{
			Color: colors.Background,
			Child: widgets.Stack{
				Children: []core.Widget{
					heroGradientBackground(isDark),
					homeScrollContent(ctx, colors, isDark, toggleTheme),
				},
			},
		},
	}
}

// heroGradientBackground creates the radial gradient overlay behind the home content.
func heroGradientBackground(isDark bool) core.Widget {
	cyanAlpha := 0.12
	pinkAlpha := 0.08
	if !isDark {
		cyanAlpha = 0.15
		pinkAlpha = 0.1
	}

	return widgets.Container{
		Gradient: graphics.NewRadialGradient(
			graphics.Alignment{X: 0, Y: -0.4}, // Center-top
			1.5,
			[]graphics.GradientStop{
				{Position: 0, Color: CyanSeed.WithAlpha(cyanAlpha)},
				{Position: 0.5, Color: PinkSeed.WithAlpha(pinkAlpha)},
				{Position: 1, Color: graphics.RGBA(0, 0, 0, 0)},
			},
		),
	}
}

// homeScrollContent builds the scrollable content area of the home page.
func homeScrollContent(ctx core.BuildContext, colors theme.ColorScheme, isDark bool, toggleTheme func()) core.Widget {
	return widgets.ScrollView{
		ScrollDirection: widgets.AxisVertical,
		Physics:         widgets.BouncingScrollPhysics{},
		Padding:         widgets.SafeAreaPadding(ctx).Add(20),
		Child: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				// Header
				headerRow(ctx, isDark, toggleTheme),

				// Hero section
				widgets.VSpace(50),
				logoWithGlow(),
				widgets.VSpace(28),
				taglineText(colors),
				widgets.VSpace(16),
				techPill(ctx, isDark),

				// Category grid
				widgets.VSpace(56),
				homeCategoryGrid(ctx, colors, isDark),

				// Footer
				widgets.VSpace(40),
				crashButton(ctx, colors),
				widgets.VSpace(40),
			},
		},
	}
}

// headerRow creates the top row with the theme toggle aligned right.
func headerRow(ctx core.BuildContext, isDark bool, toggleTheme func()) core.Widget {
	return widgets.Row{
		MainAxisAlignment:  widgets.MainAxisAlignmentEnd,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		MainAxisSize:       widgets.MainAxisSizeMax,
		Children: []core.Widget{
			themeToggleButton(ctx, isDark, toggleTheme),
		},
	}
}

// logoWithGlow creates the Drift logo with a decorative glow effect underneath.
func logoWithGlow() core.Widget {
	return widgets.DecoratedBox{
		Overflow: widgets.OverflowVisible,
		Gradient: graphics.NewRadialGradient(
			graphics.Alignment{X: 0, Y: -0.2},
			5,
			[]graphics.GradientStop{
				{Position: 0, Color: CyanSeed.WithAlpha(0.06)},
				{Position: 0.5, Color: PinkSeed.WithAlpha(0.1)},
				{Position: 1, Color: graphics.RGBA(0, 0, 0, 0)},
			},
		),
		Child: widgets.SvgImage{
			Source: loadSVGAsset("drift.svg"),
			Width:  200,
		},
	}
}

// taglineText creates the "Cross-platform mobile UI for Go" text.
func taglineText(colors theme.ColorScheme) core.Widget {
	return widgets.Text{
		Content: "Cross-platform mobile UI for Go",
		Style: graphics.TextStyle{
			Color:      colors.OnSurface,
			FontSize:   18,
			FontWeight: graphics.FontWeightMedium,
		},
	}
}

// techPill creates the "GPU Accelerated • Powered by Skia" pill badge.
func techPill(ctx core.BuildContext, isDark bool) core.Widget {
	colors := theme.ColorsOf(ctx)

	// Theme-dependent opacity values
	pillAlpha, borderAlpha, shadowAlpha := 0.15, 0.2, 0.15
	if !isDark {
		pillAlpha, borderAlpha, shadowAlpha = 0.1, 0.2, 0.1
	}

	return widgets.Container{
		Gradient: graphics.NewLinearGradient(
			graphics.AlignTopLeft,
			graphics.AlignBottomRight,
			[]graphics.GradientStop{
				{Position: 0, Color: PinkSeed.WithAlpha(pillAlpha)},
				{Position: 1, Color: CyanSeed.WithAlpha(pillAlpha)},
			},
		),
		BorderRadius: 100,
		BorderWidth:  1,
		BorderColor:  colors.Primary.WithAlpha(borderAlpha),
		Padding:      layout.EdgeInsetsSymmetric(16, 8),
		Shadow: &graphics.BoxShadow{
			Color:      colors.Primary.WithAlpha(shadowAlpha),
			Offset:     graphics.Offset{X: 0, Y: 0},
			BlurStyle:  graphics.BlurStyleNormal,
			BlurRadius: 20,
		},
		Child: widgets.Row{
			MainAxisSize:       widgets.MainAxisSizeMin,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Text{
					Content: "GPU Accelerated",
					Style: graphics.TextStyle{
						Color:    colors.OnSurface,
						FontSize: 14,
					},
				},
				widgets.HSpace(8),
				widgets.Text{
					Content: "\u2022", // Bullet
					Style: graphics.TextStyle{
						Color:    colors.OnSurface.WithAlpha(0.5),
						FontSize: 14,
					},
				},
				widgets.HSpace(8),
				widgets.Text{
					Content: "Powered by Skia",
					Style: graphics.TextStyle{
						Color:    colors.OnSurface,
						FontSize: 14,
					},
				},
			},
		},
	}
}

// crashButton creates a playful skull button that triggers an error to demo error boundaries.
func crashButton(_ core.BuildContext, colors theme.ColorScheme) core.Widget {
	icon := loadSVGAsset("icon-skull.svg")

	return widgets.GestureDetector{
		OnTap: func() {
			// Trigger a panic to demonstrate error boundaries
			panic("You found the skull! This crash demonstrates Drift's error boundary recovery.")
		},
		Child: widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentCenter,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMax,
			Children: []core.Widget{
				widgets.Container{
					Width:        32,
					Height:       32,
					BorderRadius: 8,
					Color:        colors.SurfaceContainer,
					Alignment:    layout.AlignmentCenter,
					Child: widgets.SvgImage{
						Source:    icon,
						Width:     18,
						Height:    18,
						TintColor: colors.OnSurfaceVariant.WithAlpha(0.5),
					},
				},
			},
		},
	}
}

// homeCategoryGrid creates the 3x2 grid of category cards.
func homeCategoryGrid(ctx core.BuildContext, colors theme.ColorScheme, isDark bool) core.Widget {
	// Build all card widgets
	cards := make([]core.Widget, len(categories))
	for i, cat := range categories {
		cards[i] = widgets.Expanded{
			Child: gradientBorderCard(ctx, cat.Title, cat.Description, cat.Route, colors, isDark),
		}
	}

	// Arrange cards into 2-column rows
	var rows []core.Widget
	for i := 0; i < len(cards); i += 2 {
		left := cards[i]
		right := core.Widget(widgets.Expanded{Child: widgets.SizedBox{}}) // Empty spacer
		if i+1 < len(cards) {
			right = cards[i+1]
		}

		rows = append(rows,
			widgets.Row{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMax,
				Children:           []core.Widget{left, widgets.HSpace(20), right},
			},
			widgets.VSpace(20),
		)
	}

	return widgets.Column{
		MainAxisAlignment:  widgets.MainAxisAlignmentStart,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		MainAxisSize:       widgets.MainAxisSizeMin,
		Children:           rows,
	}
}
