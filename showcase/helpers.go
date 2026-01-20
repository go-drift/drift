package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// sectionTitle creates a styled section header for demo pages.
func sectionTitle(text string, colors theme.ColorScheme) core.Widget {
	return widgets.TextOf(text, rendering.TextStyle{
		Color:      colors.OnSurface,
		FontSize:   20,
		FontWeight: rendering.FontWeightBold,
	})
}

// labelStyle returns a text style for descriptive labels.
func labelStyle(colors theme.ColorScheme) rendering.TextStyle {
	return rendering.TextStyle{
		Color:    colors.OnSurfaceVariant,
		FontSize: 14,
	}
}

// codeBlock displays code in a styled monospace container.
func codeBlock(code string, colors theme.ColorScheme) core.Widget {
	return widgets.NewContainer(
		widgets.PaddingAll(12,
			widgets.TextOf(code, rendering.TextStyle{
				Color:              colors.OnSurfaceVariant,
				FontFamily:         "monospace",
				FontSize:           12,
				PreserveWhitespace: true,
			}).WithWrap(true),
		),
	).WithColor(colors.SurfaceVariant).Build()
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
