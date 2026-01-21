package widgets_test

import (
	"fmt"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/widgets"
)

// This example shows how to create a basic button with a tap handler.
func ExampleButton() {
	button := widgets.NewButton("Click Me", func() {
		fmt.Println("Button tapped!")
	})
	_ = button
}

// This example shows how to customize a button's appearance.
func ExampleButton_withStyles() {
	button := widgets.NewButton("Submit", func() {
		fmt.Println("Submitted!")
	}).
		WithColor(rendering.RGB(33, 150, 243), rendering.ColorWhite).
		WithFontSize(18).
		WithPadding(layout.EdgeInsetsSymmetric(32, 16)).
		WithHaptic(true)
	_ = button
}

// This example shows how to create a horizontal layout with Row.
func ExampleRow() {
	row := widgets.Row{
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: "Left"},
			widgets.Text{Content: "Center"},
			widgets.Text{Content: "Right"},
		},
		MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
	}
	_ = row
}

// This example shows how to create a vertical layout with Column.
func ExampleColumn() {
	column := widgets.Column{
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: "First"},
			widgets.Text{Content: "Second"},
			widgets.Text{Content: "Third"},
		},
		MainAxisAlignment:  widgets.MainAxisAlignmentStart,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
	}
	_ = column
}

// This example shows the helper function for creating columns.
func ExampleColumnOf() {
	column := widgets.ColumnOf(
		widgets.MainAxisAlignmentCenter,
		widgets.CrossAxisAlignmentCenter,
		widgets.MainAxisSizeMin,
		widgets.Text{Content: "Hello"},
		widgets.VSpace(16),
		widgets.Text{Content: "World"},
	)
	_ = column
}

// This example shows how to create a styled container.
func ExampleContainer() {
	container := widgets.Container{
		Padding: layout.EdgeInsetsAll(16),
		Color:   rendering.RGB(245, 245, 245),
		Width:   200,
		Height:  100,
		ChildWidget: widgets.Text{
			Content: "Centered content",
		},
		Alignment: layout.AlignmentCenter,
	}
	_ = container
}

// This example shows how to display styled text.
func ExampleText() {
	text := widgets.Text{
		Content: "Hello, Drift!",
		Style: rendering.TextStyle{
			FontSize: 24,
			Color:    rendering.RGB(33, 33, 33),
		},
		Wrap:     true,
		MaxLines: 2,
	}
	_ = text
}

// This example shows how to create a dynamic list with ListViewBuilder.
func ExampleListViewBuilder() {
	items := []string{"Apple", "Banana", "Cherry", "Date", "Elderberry"}

	listView := widgets.ListViewBuilder{
		ItemCount:  len(items),
		ItemExtent: 48,
		ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
			return widgets.Container{
				Padding:     layout.EdgeInsetsSymmetric(16, 12),
				ChildWidget: widgets.Text{Content: items[index]},
			}
		},
		Padding: layout.EdgeInsetsAll(8),
	}
	_ = listView
}

// This example shows how to create scrollable content.
func ExampleScrollView() {
	scrollView := widgets.ScrollView{
		ChildWidget: widgets.Column{
			ChildrenWidgets: []core.Widget{
				widgets.SizedBox{Height: 1000, ChildWidget: widgets.Text{Content: "Tall content"}},
			},
		},
		ScrollDirection: widgets.AxisVertical,
		Physics:         widgets.BouncingScrollPhysics{},
		Padding:         layout.EdgeInsetsAll(16),
	}
	_ = scrollView
}

// This example shows how to handle tap gestures.
func ExampleGestureDetector() {
	detector := widgets.GestureDetector{
		OnTap: func() {
			fmt.Println("Tapped!")
		},
		ChildWidget: widgets.Container{
			Color:   rendering.RGB(200, 200, 200),
			Padding: layout.EdgeInsetsAll(20),
			ChildWidget: widgets.Text{
				Content: "Tap me",
			},
		},
	}
	_ = detector
}

// This example shows how to create a checkbox form control.
func ExampleCheckbox() {
	var isChecked bool

	checkbox := widgets.Checkbox{
		Value: isChecked,
		OnChanged: func(value bool) {
			isChecked = value
			fmt.Printf("Checkbox is now: %v\n", isChecked)
		},
		Size:         24,
		BorderRadius: 4,
	}
	_ = checkbox
}

// This example shows how to create a stack with overlapping children.
func ExampleStack() {
	stack := widgets.Stack{
		ChildrenWidgets: []core.Widget{
			// Background
			widgets.Container{
				Color:  rendering.RGB(200, 200, 200),
				Width:  200,
				Height: 200,
			},
			// Foreground centered via Alignment
			widgets.Container{
				Color:  rendering.RGB(100, 149, 237),
				Width:  100,
				Height: 100,
			},
		},
		Alignment: layout.AlignmentCenter,
	}
	_ = stack
}

// This example shows a Stack with Positioned children for absolute positioning.
func ExampleStack_withPositioned() {
	stack := widgets.Stack{
		ChildrenWidgets: []core.Widget{
			// Background fills the stack
			widgets.Container{
				Color:  rendering.RGB(240, 240, 240),
				Width:  300,
				Height: 200,
			},
			// Badge in top-right corner
			widgets.Positioned{
				Top:   widgets.Ptr(8),
				Right: widgets.Ptr(8),
				ChildWidget: widgets.Container{
					Color:   rendering.RGB(255, 0, 0),
					Width:   20,
					Height:  20,
					Padding: layout.EdgeInsetsAll(4),
				},
			},
			// Bottom toolbar stretching horizontally
			widgets.Positioned{
				Left:   widgets.Ptr(0),
				Right:  widgets.Ptr(0),
				Bottom: widgets.Ptr(0),
				ChildWidget: widgets.Container{
					Color:  rendering.RGB(33, 33, 33),
					Height: 48,
				},
			},
		},
	}
	_ = stack
}

// This example shows how to use Positioned for absolute positioning within a Stack.
func ExamplePositioned() {
	// Pin to top-left corner with margins
	topLeft := widgets.Positioned{
		Left: widgets.Ptr(8),
		Top:  widgets.Ptr(8),
		ChildWidget: widgets.Text{
			Content: "Top Left",
		},
	}

	// Pin to bottom-right corner
	bottomRight := widgets.Positioned{
		Right:  widgets.Ptr(16),
		Bottom: widgets.Ptr(16),
		ChildWidget: widgets.Text{
			Content: "Bottom Right",
		},
	}

	// Fixed size at specific position
	fixedBox := widgets.Positioned{
		Left:   widgets.Ptr(50),
		Top:    widgets.Ptr(50),
		Width:  widgets.Ptr(100),
		Height: widgets.Ptr(60),
		ChildWidget: widgets.Container{
			Color: rendering.RGB(100, 149, 237),
		},
	}

	_ = topLeft
	_ = bottomRight
	_ = fixedBox
}

// This example shows how Positioned can stretch children by setting opposite edges.
func ExamplePositioned_stretch() {
	// Stretch horizontally (left + right set, no width)
	horizontalStretch := widgets.Positioned{
		Left:  widgets.Ptr(16),
		Right: widgets.Ptr(16),
		Top:   widgets.Ptr(100),
		ChildWidget: widgets.Container{
			Color:  rendering.RGB(200, 200, 200),
			Height: 2, // Divider line
		},
	}

	// Stretch vertically (top + bottom set, no height)
	verticalStretch := widgets.Positioned{
		Top:    widgets.Ptr(50),
		Bottom: widgets.Ptr(50),
		Left:   widgets.Ptr(0),
		ChildWidget: widgets.Container{
			Color: rendering.RGB(100, 100, 100),
			Width: 4, // Vertical bar
		},
	}

	// Stretch both ways (all four edges set)
	fillWithMargins := widgets.Positioned{
		Left:   widgets.Ptr(20),
		Top:    widgets.Ptr(20),
		Right:  widgets.Ptr(20),
		Bottom: widgets.Ptr(20),
		ChildWidget: widgets.Container{
			Color: rendering.RGBA(0, 0, 0, 128), // Semi-transparent overlay
		},
	}

	_ = horizontalStretch
	_ = verticalStretch
	_ = fillWithMargins
}

// This example shows partial positioning where unset axes use Stack.Alignment.
func ExamplePositioned_partialAlignment() {
	// Position only vertically at top - horizontal position uses Stack.Alignment.
	// With AlignmentCenter, this centers the header horizontally.
	stack := widgets.Stack{
		Alignment: layout.AlignmentCenter,
		ChildrenWidgets: []core.Widget{
			widgets.Container{Width: 300, Height: 200},
			// Only Top is set, so X position comes from Stack.Alignment (centered)
			widgets.Positioned{
				Top: widgets.Ptr(16),
				ChildWidget: widgets.Text{
					Content: "Centered Header",
				},
			},
			// Only Left is set, so Y position comes from Stack.Alignment (centered)
			widgets.Positioned{
				Left: widgets.Ptr(8),
				ChildWidget: widgets.Text{
					Content: "Left Sidebar",
				},
			},
		},
	}
	_ = stack
}
