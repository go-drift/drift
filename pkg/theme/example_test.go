package theme_test

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// mockBuildContext is a minimal implementation for examples.
// In real code, BuildContext is provided by the framework.
type mockBuildContext struct{}

func (m mockBuildContext) Widget() core.Widget { return nil }
func (m mockBuildContext) FindAncestor(func(core.Element) bool) core.Element {
	return nil
}
func (m mockBuildContext) DependOnInherited(inheritedType reflect.Type, aspect any) any { return nil }
func (m mockBuildContext) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return nil
}

// This example shows how to access theme colors in a widget's Build method.
// In a real widget, BuildContext is provided by the framework.
func ExampleColorsOf() {
	// In a widget's Build method:
	// func (w MyWidget) Build(ctx core.BuildContext) core.Widget {
	//     colors := theme.ColorsOf(ctx)
	//     return widgets.Container{
	//         Color: colors.Primary,
	//         ChildWidget: widgets.Text{
	//             Content: "Themed text",
	//             Style: rendering.TextStyle{Color: colors.OnPrimary},
	//         },
	//     }
	// }

	// Direct usage (outside of widget context) for demonstration:
	colors := theme.LightColorScheme()
	_ = widgets.Container{
		Color: colors.Primary,
		ChildWidget: widgets.Text{
			Content: "Themed text",
			Style:   rendering.TextStyle{Color: colors.OnPrimary},
		},
	}
}

// This example shows how to customize a theme using CopyWith.
func ExampleThemeData_CopyWith() {
	// Start with the default light theme
	baseTheme := theme.DefaultLightTheme()

	// Create a custom color scheme with a different primary color
	customColors := theme.LightColorScheme()
	customColors.Primary = rendering.RGB(0, 150, 136) // Teal

	// Create a new theme with the custom colors
	customTheme := baseTheme.CopyWith(&customColors, nil, nil)
	_ = customTheme
}

// This example shows how to wrap your app with a Theme provider.
func ExampleTheme() {
	root := widgets.Center{
		ChildWidget: widgets.Text{Content: "Themed App"},
	}

	// Wrap the root widget with a Theme
	themedApp := theme.Theme{
		Data:        theme.DefaultDarkTheme(),
		ChildWidget: root,
	}
	_ = themedApp
}

// This example shows how to get all theme components at once using UseTheme.
// This is the recommended approach when you need multiple theme values.
func ExampleUseTheme() {
	// In a widget's Build method:
	// func (w MyWidget) Build(ctx core.BuildContext) core.Widget {
	//     themeData, colors, textTheme := theme.UseTheme(ctx)
	//
	//     return widgets.Column{
	//         ChildrenWidgets: []core.Widget{
	//             widgets.Text{
	//                 Content: "Headline",
	//                 Style:   textTheme.HeadlineMedium,
	//             },
	//             widgets.Container{
	//                 Color: colors.Surface,
	//                 ChildWidget: widgets.Text{
	//                     Content: "Body text",
	//                     Style:   textTheme.BodyLarge,
	//                 },
	//             },
	//         },
	//     }
	// }
	_ = theme.DefaultLightTheme()
}
