package widgets_test

import (
	"fmt"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/widgets"
)

// This example shows how to create a basic button with a tap handler.
func ExampleButton() {
	button := widgets.Button{
		Label: "Click Me",
		OnTap: func() {
			fmt.Println("Button tapped!")
		},
		Haptic: true,
	}
	_ = button
}

// This example shows how to customize a button's appearance.
func ExampleButton_withStyles() {
	button := widgets.Button{
		Label:     "Submit",
		OnTap:     func() { fmt.Println("Submitted!") },
		Color:     graphics.RGB(33, 150, 243),
		TextColor: graphics.ColorWhite,
		FontSize:  18,
		Padding:   layout.EdgeInsetsSymmetric(32, 16),
		Haptic:    true,
	}
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
		Color:   graphics.RGB(245, 245, 245),
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
		Style: graphics.TextStyle{
			FontSize: 24,
			Color:    graphics.RGB(33, 33, 33),
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
			Color:   graphics.RGB(200, 200, 200),
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
				Color:  graphics.RGB(200, 200, 200),
				Width:  200,
				Height: 200,
			},
			// Foreground centered via Alignment
			widgets.Container{
				Color:  graphics.RGB(100, 149, 237),
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
				Color:  graphics.RGB(240, 240, 240),
				Width:  300,
				Height: 200,
			},
			// Badge in top-right corner
			widgets.Positioned{
				Top:   widgets.Ptr(8),
				Right: widgets.Ptr(8),
				ChildWidget: widgets.Container{
					Color:   graphics.RGB(255, 0, 0),
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
					Color:  graphics.RGB(33, 33, 33),
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
			Color: graphics.RGB(100, 149, 237),
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
			Color:  graphics.RGB(200, 200, 200),
			Height: 2, // Divider line
		},
	}

	// Stretch vertically (top + bottom set, no height)
	verticalStretch := widgets.Positioned{
		Top:    widgets.Ptr(50),
		Bottom: widgets.Ptr(50),
		Left:   widgets.Ptr(0),
		ChildWidget: widgets.Container{
			Color: graphics.RGB(100, 100, 100),
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
			Color: graphics.RGBA(0, 0, 0, 0.5), // Semi-transparent overlay
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

// This example shows how to use Expanded for flexible sizing in Row/Column.
func ExampleRow_withExpanded() {
	row := widgets.Row{
		ChildrenWidgets: []core.Widget{
			// Fixed width
			widgets.Container{Width: 80, Color: graphics.RGB(200, 200, 200)},
			// Flexible - takes remaining space
			widgets.Expanded{
				ChildWidget: widgets.Container{Color: graphics.RGB(100, 149, 237)},
			},
			// Fixed width
			widgets.Container{Width: 80, Color: graphics.RGB(200, 200, 200)},
		},
	}
	_ = row
}

// This example shows flexible children in a vertical layout.
func ExampleColumn_withExpanded() {
	column := widgets.Column{
		ChildrenWidgets: []core.Widget{
			// Fixed header
			widgets.Container{Height: 60, Color: graphics.RGB(33, 33, 33)},
			// Content takes remaining space
			widgets.Expanded{
				ChildWidget: widgets.ScrollView{
					ChildWidget: widgets.Text{Content: "Scrollable content..."},
				},
			},
			// Fixed footer
			widgets.Container{Height: 48, Color: graphics.RGB(66, 66, 66)},
		},
	}
	_ = column
}

// This example shows a simple static list.
func ExampleListView() {
	items := []string{"Apple", "Banana", "Cherry"}
	children := make([]core.Widget, len(items))
	for i, item := range items {
		children[i] = widgets.Container{
			Padding:     layout.EdgeInsetsSymmetric(16, 12),
			ChildWidget: widgets.Text{Content: item},
		}
	}

	listView := widgets.ListView{
		ChildrenWidgets: children,
		Padding:         layout.EdgeInsetsAll(8),
	}
	_ = listView
}

// This example shows how to use a scroll controller for programmatic scrolling.
func ExampleScrollView_withController() {
	// Create a scroll controller to programmatically control scroll position
	controller := &widgets.ScrollController{}

	scrollView := widgets.ScrollView{
		Controller: controller,
		ChildWidget: widgets.Column{
			ChildrenWidgets: []core.Widget{
				widgets.SizedBox{Height: 2000, ChildWidget: widgets.Text{Content: "Long content"}},
			},
		},
	}

	// Jump to a specific position
	controller.JumpTo(500)

	// Or animate the scroll
	controller.AnimateTo(1000, 300*time.Millisecond)

	_ = scrollView
}

// This example shows a container with gradient and shadow.
func ExampleContainer_withGradient() {
	container := widgets.Container{
		Width:  200,
		Height: 100,
		Gradient: graphics.NewLinearGradient(
			graphics.AlignTopLeft,
			graphics.AlignBottomRight,
			[]graphics.GradientStop{
				{Position: 0.0, Color: graphics.RGB(66, 133, 244)},
				{Position: 1.0, Color: graphics.RGB(15, 157, 88)},
			},
		),
		Shadow: &graphics.BoxShadow{
			Color:      graphics.RGBA(0, 0, 0, 0.25),
			BlurRadius: 8,
			Offset:     graphics.Offset{X: 0, Y: 4},
		},
		ChildWidget: widgets.Center{ChildWidget: widgets.Text{Content: "Gradient Card"}},
	}
	_ = container
}

// This example shows how to use Expanded for flexible sizing.
func ExampleExpanded() {
	row := widgets.Row{
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: "Label:"},
			widgets.HSpace(8),
			// Expanded takes all remaining horizontal space
			widgets.Expanded{
				ChildWidget: widgets.Container{
					Color:       graphics.RGB(240, 240, 240),
					ChildWidget: widgets.Text{Content: "Flexible content"},
				},
			},
		},
	}
	_ = row
}

// This example shows proportional sizing with flex factors.
func ExampleExpanded_flexFactors() {
	row := widgets.Row{
		ChildrenWidgets: []core.Widget{
			// Takes 1/3 of available space
			widgets.Expanded{
				Flex:        1,
				ChildWidget: widgets.Container{Color: graphics.RGB(255, 0, 0)},
			},
			// Takes 2/3 of available space
			widgets.Expanded{
				Flex:        2,
				ChildWidget: widgets.Container{Color: graphics.RGB(0, 0, 255)},
			},
		},
	}
	_ = row
}

// This example shows various padding helpers.
func ExamplePadding() {
	// Uniform padding on all sides
	all := widgets.Padding{
		Padding:     layout.EdgeInsetsAll(16),
		ChildWidget: widgets.Text{Content: "All sides"},
	}

	// Symmetric horizontal/vertical padding
	symmetric := widgets.Padding{
		Padding:     layout.EdgeInsetsSymmetric(24, 12), // horizontal, vertical
		ChildWidget: widgets.Text{Content: "Symmetric"},
	}

	// Different padding per side
	custom := widgets.Padding{
		Padding:     layout.EdgeInsetsOnly(8, 16, 8, 0), // left, top, right, bottom
		ChildWidget: widgets.Text{Content: "Custom"},
	}

	_ = all
	_ = symmetric
	_ = custom
}

// This example shows how to center a widget.
func ExampleCenter() {
	center := widgets.Center{
		ChildWidget: widgets.Text{Content: "Centered!"},
	}
	_ = center
}

// This example shows fixed-size boxes.
func ExampleSizedBox() {
	// Fixed dimensions
	box := widgets.SizedBox{
		Width:       100,
		Height:      50,
		ChildWidget: widgets.Container{Color: graphics.RGB(200, 200, 200)},
	}
	_ = box
}

// This example shows SizedBox as a spacer.
func ExampleSizedBox_spacer() {
	column := widgets.Column{
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: "Top"},
			widgets.SizedBox{Height: 16}, // Vertical spacer
			widgets.Text{Content: "Bottom"},
		},
	}

	row := widgets.Row{
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: "Left"},
			widgets.SizedBox{Width: 24}, // Horizontal spacer
			widgets.Text{Content: "Right"},
		},
	}

	_ = column
	_ = row
}

// This example shows drag gesture handling.
func ExampleGestureDetector_drag() {
	var offsetX, offsetY float64

	detector := widgets.GestureDetector{
		OnPanStart: func(details widgets.DragStartDetails) {
			fmt.Println("Drag started")
		},
		OnPanUpdate: func(details widgets.DragUpdateDetails) {
			offsetX += details.Delta.X
			offsetY += details.Delta.Y
			fmt.Printf("Position: (%.0f, %.0f)\n", offsetX, offsetY)
		},
		OnPanEnd: func(details widgets.DragEndDetails) {
			fmt.Printf("Drag ended with velocity: (%.0f, %.0f)\n",
				details.Velocity.X, details.Velocity.Y)
		},
		ChildWidget: widgets.Container{
			Width:  100,
			Height: 100,
			Color:  graphics.RGB(100, 149, 237),
		},
	}
	_ = detector
}

// This example shows image display with fit modes.
func ExampleImage() {
	// Image accepts a Go image.Image as its source.
	// You would typically load images from files, assets, or network.
	//
	// var loadedImage image.Image // from image.Decode, etc.
	//
	// coverImage := widgets.Image{
	//     Source: loadedImage,
	//     Width:  200,
	//     Height: 150,
	//     Fit:    widgets.ImageFitCover,
	// }
	//
	// containImage := widgets.Image{
	//     Source: loadedImage,
	//     Width:  100,
	//     Height: 100,
	//     Fit:    widgets.ImageFitContain,
	// }

	// Example with nil source (placeholder)
	placeholder := widgets.Image{
		Width:  200,
		Height: 150,
		Fit:    widgets.ImageFitCover,
	}
	_ = placeholder
}

// This example shows a toggle switch.
func ExampleSwitch() {
	var isEnabled bool

	toggle := widgets.Switch{
		Value: isEnabled,
		OnChanged: func(value bool) {
			isEnabled = value
			fmt.Printf("Switch is now: %v\n", isEnabled)
		},
	}
	_ = toggle
}

// This example shows a radio button group.
func ExampleRadio() {
	var selectedOption string

	options := []string{"Small", "Medium", "Large"}
	radios := make([]core.Widget, len(options))

	for i, option := range options {
		opt := option // capture for closure
		radios[i] = widgets.Row{
			ChildrenWidgets: []core.Widget{
				widgets.Radio[string]{
					Value:      opt,
					GroupValue: selectedOption,
					OnChanged: func(value string) {
						selectedOption = value
						fmt.Printf("Selected: %s\n", selectedOption)
					},
				},
				widgets.HSpace(8),
				widgets.Text{Content: opt},
			},
		}
	}
	_ = radios
}

// This example shows a dropdown selection menu.
func ExampleDropdown() {
	var selectedCountry string

	dropdown := widgets.Dropdown[string]{
		Value: selectedCountry,
		Hint:  "Select a country",
		Items: []widgets.DropdownItem[string]{
			{Value: "us", Label: "United States"},
			{Value: "ca", Label: "Canada"},
			{Value: "mx", Label: "Mexico"},
		},
		OnChanged: func(value string) {
			selectedCountry = value
			fmt.Printf("Selected: %s\n", selectedCountry)
		},
	}
	_ = dropdown
}

// This example shows a styled text input.
func ExampleTextField() {
	controller := platform.NewTextEditingController("")

	textField := widgets.TextField{
		Controller:   controller,
		Label:        "Email",
		Placeholder:  "you@example.com",
		HelperText:   "We'll never share your email",
		KeyboardType: platform.KeyboardTypeEmail,
	}
	_ = textField
}

// This example shows a low-level text input.
func ExampleTextInput() {
	controller := platform.NewTextEditingController("")

	textInput := widgets.TextInput{
		Controller:   controller,
		Placeholder:  "Enter text",
		KeyboardType: platform.KeyboardTypeText,
		OnChanged: func(text string) {
			fmt.Printf("Text changed: %s\n", text)
		},
		OnSubmitted: func(text string) {
			fmt.Printf("Submitted: %s\n", text)
		},
	}
	_ = textInput
}

// This example shows a form field with validation.
func ExampleTextFormField() {
	textFormField := widgets.TextFormField{
		Label:       "Username",
		Placeholder: "Enter username",
		Validator: func(value string) string {
			if len(value) < 3 {
				return "Username must be at least 3 characters"
			}
			return ""
		},
		OnSaved: func(value string) {
			fmt.Printf("Saved username: %s\n", value)
		},
	}
	_ = textFormField
}

// This example shows a form with validation.
func ExampleForm() {
	form := widgets.Form{
		Autovalidate: true,
		OnChanged: func() {
			fmt.Println("Form changed")
		},
		ChildWidget: widgets.Column{
			ChildrenWidgets: []core.Widget{
				widgets.TextFormField{
					Label: "Email",
					Validator: func(value string) string {
						if value == "" {
							return "Email is required"
						}
						return ""
					},
				},
				widgets.TextFormField{
					Label:   "Password",
					Obscure: true,
					Validator: func(value string) string {
						if len(value) < 8 {
							return "Password must be at least 8 characters"
						}
						return ""
					},
				},
			},
		},
	}
	_ = form
}

// This example shows a custom form field with FormField.
func ExampleFormField() {
	formField := widgets.FormField[bool]{
		InitialValue: false,
		Validator: func(checked bool) string {
			if !checked {
				return "You must accept the terms"
			}
			return ""
		},
		Builder: func(state *widgets.FormFieldState[bool]) core.Widget {
			return widgets.Row{
				ChildrenWidgets: []core.Widget{
					widgets.Checkbox{
						Value:     state.Value(),
						OnChanged: func(v bool) { state.DidChange(v) },
					},
					widgets.HSpace(8),
					widgets.Text{Content: "I accept the terms"},
				},
			}
		},
		OnSaved: func(checked bool) {
			fmt.Printf("Terms accepted: %v\n", checked)
		},
	}
	_ = formField
}

// This example shows accessing FormState from a build context.
func ExampleFormOf() {
	// In a widget's Build method:
	//
	// func (w MyWidget) Build(ctx core.BuildContext) core.Widget {
	//     formState := widgets.FormOf(ctx)
	//     return widgets.Button{
	//         Label: "Submit",
	//         OnTap: func() {
	//             if formState != nil && formState.Validate() {
	//                 formState.Save()
	//             }
	//         },
	//     }
	// }
}

// This example shows tab-like switching with IndexedStack.
func ExampleIndexedStack() {
	var currentTab int

	indexedStack := widgets.IndexedStack{
		Index: currentTab,
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: "Home Tab Content"},
			widgets.Text{Content: "Search Tab Content"},
			widgets.Text{Content: "Profile Tab Content"},
		},
	}
	_ = indexedStack
}

// This example shows implicit animation with AnimatedContainer.
func ExampleAnimatedContainer() {
	var isExpanded bool

	// Properties animate automatically when they change
	container := widgets.AnimatedContainer{
		Duration: 300 * time.Millisecond,
		Curve:    animation.EaseInOut,
		Width:    func() float64 { if isExpanded { return 200 } else { return 100 } }(),
		Height:   func() float64 { if isExpanded { return 200 } else { return 100 } }(),
		Color:    func() graphics.Color { if isExpanded { return graphics.RGB(100, 149, 237) } else { return graphics.RGB(200, 200, 200) } }(),
		ChildWidget: widgets.Center{ChildWidget: widgets.Text{Content: "Tap to toggle"}},
	}
	_ = container
}

// This example shows fade animation with AnimatedOpacity.
func ExampleAnimatedOpacity() {
	var isVisible bool

	opacity := widgets.AnimatedOpacity{
		Duration:    200 * time.Millisecond,
		Curve:       animation.EaseOut,
		Opacity:     func() float64 { if isVisible { return 1.0 } else { return 0.0 } }(),
		ChildWidget: widgets.Text{Content: "Fading content"},
	}
	_ = opacity
}

// This example shows error boundary for catching widget errors.
func ExampleErrorBoundary() {
	boundary := widgets.ErrorBoundary{
		OnError: func(err *drifterrors.BoundaryError) {
			fmt.Printf("Widget error: %v\n", err)
		},
		FallbackBuilder: func(err *drifterrors.BoundaryError) core.Widget {
			return widgets.Container{
				Padding:     layout.EdgeInsetsAll(16),
				Color:       graphics.RGBA(255, 0, 0, 0.13),
				ChildWidget: widgets.Text{Content: "Something went wrong"},
			}
		},
		ChildWidget: widgets.Text{Content: "Protected content"},
	}
	_ = boundary
}

// This example shows an accessible tappable widget.
func ExampleTappable() {
	// Tappable wraps a child with tap handling AND accessibility semantics
	tappable := widgets.Tappable(
		"Submit form", // accessibility label
		func() { fmt.Println("Submitted!") },
		widgets.Container{
			Padding:     layout.EdgeInsetsAll(16),
			Color:       graphics.RGB(33, 150, 243),
			ChildWidget: widgets.Text{Content: "Submit"},
		},
	)
	_ = tappable
}

// This example shows how to add an accessibility label.
func ExampleSemanticLabel() {
	// Provides a description for widgets without built-in semantics
	labeled := widgets.SemanticLabel(
		"Company logo",
		widgets.Image{Width: 100, Height: 100},
	)
	_ = labeled
}

// This example shows how to mark an image for accessibility.
func ExampleSemanticImage() {
	// Marks a widget as an image with a description
	img := widgets.SemanticImage(
		"Chart showing sales growth over Q4",
		widgets.Image{Width: 300, Height: 200},
	)
	_ = img
}

// This example shows how to create an accessible heading.
func ExampleSemanticHeading() {
	// Screen readers use headings for navigation
	heading := widgets.SemanticHeading(
		1, // heading level 1-6
		widgets.Text{
			Content: "Welcome",
			Style:   graphics.TextStyle{FontSize: 32, FontWeight: 700},
		},
	)
	_ = heading
}

// This example shows how to create an accessible link.
func ExampleSemanticLink() {
	link := widgets.SemanticLink(
		"Visit our website",
		func() { fmt.Println("Opening website...") },
		widgets.Text{
			Content: "www.example.com",
			Style:   graphics.TextStyle{Color: graphics.RGB(33, 150, 243)},
		},
	)
	_ = link
}

// This example shows how to group widgets for accessibility.
func ExampleSemanticGroup() {
	// Groups related widgets so screen reader announces them together
	group := widgets.SemanticGroup(
		widgets.Row{
			ChildrenWidgets: []core.Widget{
				widgets.Text{Content: "$"},
				widgets.Text{Content: "99"},
				widgets.Text{Content: ".99"},
			},
		},
	)
	_ = group
}

// This example shows a live region for dynamic announcements.
func ExampleSemanticLiveRegion() {
	// Content changes are announced to screen readers
	liveRegion := widgets.SemanticLiveRegion(
		widgets.Text{Content: "3 items in cart"},
	)
	_ = liveRegion
}

// This example shows how to hide decorative elements from accessibility.
func ExampleDecorative() {
	// Hides purely visual elements from screen readers
	divider := widgets.Decorative(
		widgets.Container{
			Height: 1,
			Color:  graphics.RGB(200, 200, 200),
		},
	)
	_ = divider
}
