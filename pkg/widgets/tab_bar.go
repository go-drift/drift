package widgets

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// TabItem describes a single tab entry.
type TabItem struct {
	Label string
	Icon  core.Widget
}

// TabBar displays a row of tabs.
type TabBar struct {
	Items           []TabItem
	CurrentIndex    int
	OnTap           func(index int)
	BackgroundColor graphics.Color
	ActiveColor     graphics.Color
	InactiveColor   graphics.Color
	IndicatorColor  graphics.Color
	IndicatorHeight float64
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
	themeData, _, textTheme := theme.UseTheme(ctx)
	tabBarTheme := themeData.TabBarThemeOf()

	background := colorOrDefault(t.BackgroundColor, tabBarTheme.BackgroundColor)
	active := colorOrDefault(t.ActiveColor, tabBarTheme.ActiveColor)
	inactive := colorOrDefault(t.InactiveColor, tabBarTheme.InactiveColor)
	indicatorColor := colorOrDefault(t.IndicatorColor, tabBarTheme.IndicatorColor)
	indicatorHeight := t.IndicatorHeight
	if indicatorHeight <= 0 {
		indicatorHeight = tabBarTheme.IndicatorHeight
	}
	padding := t.Padding
	if padding == (layout.EdgeInsets{}) {
		padding = tabBarTheme.Padding
	}
	height := t.Height
	if height <= 0 {
		height = tabBarTheme.Height
	}

	children := make([]core.Widget, 0, len(t.Items))
	for i, item := range t.Items {
		children = append(children, t.buildTabItem(i, item, active, inactive, indicatorColor, indicatorHeight, padding, textTheme))
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
func (t TabBar) buildTabItem(index int, item TabItem, active, inactive, indicatorColor graphics.Color, indicatorHeight float64, padding layout.EdgeInsets, textTheme theme.TextTheme) core.Widget {
	isActive := index == t.CurrentIndex
	color := inactive
	if isActive {
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

	// Build tab content column
	tabContent := Column{
		ChildrenWidgets:   content,
		MainAxisAlignment: MainAxisAlignmentCenter,
		MainAxisSize:      MainAxisSizeMin,
	}

	// Build accessibility flags
	var flags semantics.SemanticsFlag = semantics.SemanticsHasSelectedState
	if isActive {
		flags = flags.Set(semantics.SemanticsIsSelected)
	}

	onTap := func() {
		if t.OnTap != nil {
			t.OnTap(index)
		}
	}

	// Wrap in Expanded to fill available space in the Row
	tabItem := Expanded{
		Flex: 1,
		ChildWidget: Semantics{
			// Note: Don't set Label here - it comes from merged descendant Text widgets
			Hint:             fmt.Sprintf("Tab %d of %d", index+1, len(t.Items)),
			Role:             semantics.SemanticsRoleTab,
			Flags:            flags,
			Container:        true,
			MergeDescendants: true, // Merge children so TalkBack highlights the tab, not individual text/icons
			OnTap:            onTap,
			ChildWidget: GestureDetector{
				OnTap: onTap,
				ChildWidget: Column{
					MainAxisAlignment:  MainAxisAlignmentEnd,
					CrossAxisAlignment: CrossAxisAlignmentStretch,
					MainAxisSize:       MainAxisSizeMax,
					ChildrenWidgets: []core.Widget{
						Expanded{
							Flex: 1,
							ChildWidget: Container{
								Padding:     padding,
								Alignment:   layout.AlignmentCenter,
								ChildWidget: tabContent,
							},
						},
						// Indicator at the bottom
						Container{
							Height: indicatorHeight,
							Color: func() graphics.Color {
								if isActive {
									return indicatorColor
								}
								return graphics.ColorTransparent
							}(),
						},
					},
				},
			},
		},
	}

	return tabItem
}

// colorOrDefault returns the color if set, otherwise returns the default.
func colorOrDefault(color, defaultColor graphics.Color) graphics.Color {
	if color == graphics.ColorTransparent {
		return defaultColor
	}
	return color
}
