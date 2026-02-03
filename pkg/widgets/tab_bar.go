package widgets

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// TabItem describes a single tab entry.
type TabItem struct {
	Label string
	Icon  core.Widget
}

// TabBar displays a row of tabs.
//
// # Styling Model
//
// TabBar is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - BackgroundColor: 0 means transparent background
//   - Height: 0 means zero height (not rendered)
//   - IndicatorHeight: 0 means no indicator
//
// For theme-styled tab bars, use [theme.TabBarOf] which pre-fills visual
// properties from the current theme's [theme.TabBarThemeData].
//
// # Creation Patterns
//
// Explicit with struct literal (full control):
//
//	widgets.TabBar{
//	    Items:           []widgets.TabItem{{Label: "Home"}, {Label: "Search"}},
//	    CurrentIndex:    0,
//	    OnTap:           func(i int) { s.SetState(func() { s.currentIndex = i }) },
//	    BackgroundColor: graphics.RGB(33, 33, 33),
//	    ActiveColor:     graphics.ColorWhite,
//	    InactiveColor:   graphics.RGBA(255, 255, 255, 0.6),
//	    IndicatorColor:  graphics.RGB(33, 150, 243),
//	    Height:          56,
//	}
//
// Themed (reads from current theme):
//
//	theme.TabBarOf(ctx, items, currentIndex, onTap)
//	// Pre-filled with theme colors, height, padding, indicator
type TabBar struct {
	// Items are the tab entries.
	Items []TabItem
	// CurrentIndex is the selected tab index.
	CurrentIndex int
	// OnTap is called when a tab is tapped.
	OnTap func(index int)
	// BackgroundColor is the bar background. Zero means transparent.
	BackgroundColor graphics.Color
	// ActiveColor is the selected tab text/icon color. Zero means transparent.
	ActiveColor graphics.Color
	// InactiveColor is the unselected tab text/icon color. Zero means transparent.
	InactiveColor graphics.Color
	// IndicatorColor is the selected tab indicator color. Zero means transparent.
	IndicatorColor graphics.Color
	// IndicatorHeight is the indicator bar height. Zero means no indicator.
	IndicatorHeight float64
	// Padding is the internal padding. Zero means no padding.
	Padding layout.EdgeInsets
	// Height is the bar height. Zero means zero height (not rendered).
	Height float64
	// LabelStyle is the text style for labels.
	LabelStyle graphics.TextStyle
}

func (t TabBar) CreateElement() core.Element {
	return core.NewStatelessElement(t, nil)
}

func (t TabBar) Key() any {
	return nil
}

func (t TabBar) Build(ctx core.BuildContext) core.Widget {
	// Use field values directly — zero means zero
	background := t.BackgroundColor
	active := t.ActiveColor
	inactive := t.InactiveColor
	indicatorColor := t.IndicatorColor
	indicatorHeight := t.IndicatorHeight
	padding := t.Padding
	height := t.Height
	labelStyle := t.LabelStyle

	children := make([]core.Widget, 0, len(t.Items))
	for i, item := range t.Items {
		children = append(children, t.buildTabItem(i, item, active, inactive, indicatorColor, indicatorHeight, padding, labelStyle))
	}

	return SizedBox{
		Height: height,
		Child: Container{
			Color: background,
			Child: Row{
				Children:           children,
				MainAxisAlignment:  MainAxisAlignmentStart,
				CrossAxisAlignment: CrossAxisAlignmentStretch,
				MainAxisSize:       MainAxisSizeMax,
			},
		},
	}
}

// buildTabItem creates a single tab item widget.
func (t TabBar) buildTabItem(index int, item TabItem, active, inactive, indicatorColor graphics.Color, indicatorHeight float64, padding layout.EdgeInsets, labelStyle graphics.TextStyle) core.Widget {
	isActive := index == t.CurrentIndex
	color := inactive
	if isActive {
		color = active
	}

	itemLabelStyle := labelStyle
	itemLabelStyle.Color = color

	iconWidget := item.Icon
	if icon, ok := iconWidget.(Icon); ok {
		icon.Color = color
		iconWidget = icon
	}

	content := []core.Widget{}
	if iconWidget != nil {
		content = append(content, iconWidget, VSpace(4))
	}
	content = append(content, Text{Content: item.Label, Style: itemLabelStyle, MaxLines: 1})

	// Build tab content column
	tabContent := Column{
		Children:           content,
		MainAxisAlignment:  MainAxisAlignmentCenter,
		CrossAxisAlignment: CrossAxisAlignmentCenter,
		MainAxisSize:       MainAxisSizeMin,
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
		Child: Semantics{
			// Note: Don't set Label here - it comes from merged descendant Text widgets
			Hint:             fmt.Sprintf("Tab %d of %d", index+1, len(t.Items)),
			Role:             semantics.SemanticsRoleTab,
			Flags:            flags,
			Container:        true,
			MergeDescendants: true, // Merge children so TalkBack highlights the tab, not individual text/icons
			OnTap:            onTap,
			Child: GestureDetector{
				OnTap: onTap,
				Child: Column{
					MainAxisAlignment:  MainAxisAlignmentEnd,
					CrossAxisAlignment: CrossAxisAlignmentStretch,
					MainAxisSize:       MainAxisSizeMax,
					Children: []core.Widget{
						Expanded{
							Flex: 1,
							Child: Container{
								Padding:   padding,
								Alignment: layout.AlignmentCenter,
								Child:     tabContent,
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
