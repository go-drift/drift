package theme_test

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// This example shows how to access theme colors in a widget's Build method.
// In a real widget, BuildContext is provided by the framework.
func ExampleColorsOf() {
	// Direct usage (outside of widget context) for demonstration:
	colors := theme.LightColorScheme()
	_ = widgets.Container{
		Color: colors.Primary,
		Child: widgets.Text{
			Content: "Themed text",
			Style:   graphics.TextStyle{Color: colors.OnPrimary},
		},
	}
}

// This example shows how to customize a theme using CopyWith.
func ExampleThemeData_CopyWith() {
	// Start with the default light theme
	baseTheme := theme.DefaultLightTheme()

	// Create a custom color scheme with a different primary color
	customColors := theme.LightColorScheme()
	customColors.Primary = graphics.RGB(0, 150, 136) // Teal

	// Create a new theme with the custom colors
	customTheme := baseTheme.CopyWith(&customColors, nil, nil)
	_ = customTheme
}

// This example shows how to wrap your app with a Theme provider.
func ExampleTheme() {
	root := widgets.Center{
		Child: widgets.Text{Content: "Themed App"},
	}

	// Wrap the root widget with a Theme
	themedApp := theme.Theme{
		Data:  theme.DefaultDarkTheme(),
		Child: root,
	}
	_ = themedApp
}

// This example shows how to derive a full color scheme from a single seed.
func ExampleColorSchemeFromSeed() {
	seed, err := graphics.ParseHexColor("#6750A4")
	if err != nil {
		panic(err)
	}

	colors, err := theme.ColorSchemeFromSeed(theme.ColorSchemeSeedOptions{
		Seed:       seed,
		Brightness: theme.BrightnessLight,
		Contrast:   0,
	})
	if err != nil {
		panic(err)
	}

	_ = theme.ThemeData{
		ColorScheme: colors,
		TextTheme:   theme.DefaultTextTheme(colors.OnBackground),
		Brightness:  theme.BrightnessLight,
	}
}
