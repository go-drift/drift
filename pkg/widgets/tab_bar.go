package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
)

// TabItem describes a single tab entry.
type TabItem struct {
	Label string
	Icon  core.Widget
}

// DefaultTabBarHeight is the default height for a tab bar.
const DefaultTabBarHeight = 56.0

// TabBar displays a row of tabs.
type TabBar struct {
	Items           []TabItem
	CurrentIndex    int
	OnTap           func(index int)
	BackgroundColor rendering.Color
	ActiveColor     rendering.Color
	InactiveColor   rendering.Color
	Padding         layout.EdgeInsets
	Height          float64
}

func (t TabBar) CreateElement() core.Element {
	return core.NewStatelessElement(t, nil)
}

func (t TabBar) Key() any {
	return nil
}

func (t TabBar) Build(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	background := colorOrDefault(t.BackgroundColor, colors.SurfaceVariant)
	active := colorOrDefault(t.ActiveColor, colors.Primary)
	inactive := colorOrDefault(t.InactiveColor, colors.OnSurfaceVariant)
	padding := t.effectivePadding()
	height := t.effectiveHeight()

	children := make([]core.Widget, 0, len(t.Items))
	for i, item := range t.Items {
		children = append(children, t.buildTabItem(i, item, active, inactive, padding, textTheme))
	}

	return SizedBox{
		Height: height,
		ChildWidget: Container{
			Color: background,
			ChildWidget: Row{
				ChildrenWidgets:   children,
				MainAxisAlignment: MainAxisAlignmentSpaceEvenly,
				MainAxisSize:      MainAxisSizeMax,
			},
		},
	}
}

// buildTabItem creates a single tab item widget.
func (t TabBar) buildTabItem(index int, item TabItem, active, inactive rendering.Color, padding layout.EdgeInsets, textTheme theme.TextTheme) core.Widget {
	color := inactive
	if index == t.CurrentIndex {
		color = active
	}

	labelStyle := textTheme.LabelSmall
	labelStyle.Color = color

	iconWidget := item.Icon
	if icon, ok := iconWidget.(Icon); ok {
		icon.Color = color
		iconWidget = icon
	}

	content := []core.Widget{}
	if iconWidget != nil {
		content = append(content, iconWidget, VSpace(4))
	}
	content = append(content, Text{Content: item.Label, Style: labelStyle, MaxLines: 1})

	return GestureDetector{
		OnTap: func() {
			if t.OnTap != nil {
				t.OnTap(index)
			}
		},
		ChildWidget: Container{
			Padding:   padding,
			Alignment: layout.AlignmentCenter,
			ChildWidget: Column{
				ChildrenWidgets:   content,
				MainAxisAlignment: MainAxisAlignmentCenter,
				MainAxisSize:      MainAxisSizeMin,
			},
		},
	}
}

// effectivePadding returns the padding, using defaults if not set.
func (t TabBar) effectivePadding() layout.EdgeInsets {
	if t.Padding == (layout.EdgeInsets{}) {
		return layout.EdgeInsetsSymmetric(12, 8)
	}
	return t.Padding
}

// effectiveHeight returns the height, using defaults if not set.
func (t TabBar) effectiveHeight() float64 {
	if t.Height <= 0 {
		return DefaultTabBarHeight
	}
	return t.Height
}

// colorOrDefault returns the color if set, otherwise returns the default.
func colorOrDefault(color, defaultColor rendering.Color) rendering.Color {
	if color == rendering.ColorTransparent {
		return defaultColor
	}
	return color
}
