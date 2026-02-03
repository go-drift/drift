// Package widgets provides UI components for building widget trees.
//
// This package contains the concrete widget implementations that developers
// use to build user interfaces, including layout widgets (Row, Column, Stack),
// display widgets (Text, Icon, Image), input widgets (Button, TextField, Checkbox),
// and container widgets (Container, Padding, SizedBox).
//
// # Widget Construction
//
// Drift widgets use a two-tier construction pattern:
//
// ## Tier 1: Struct Literal (canonical, full control)
//
//	btn := Button{
//	    Label:     "Submit",
//	    OnTap:     handleSubmit,
//	    Color:     colors.Primary,
//	    Disabled:  !isValid,
//	    Haptic:    true,
//	}
//
// This is the PRIMARY way to create widgets. All fields are accessible.
// For themed styling, use theme.XxxOf constructors from pkg/theme instead.
//
// ## Tier 2: Layout Helpers (ergonomics for layout widgets)
//
// Layout helpers remain for ergonomic creation of Row, Column, and Stack:
//
//	col := ColumnOf(
//	    MainAxisAlignmentCenter,
//	    CrossAxisAlignmentCenter,
//	    MainAxisSizeMin,
//	    child1, child2,
//	)
//
// Also: RowOf, StackOf, VSpace, HSpace, Centered.
//
// ## WithX Chaining (for themed widgets)
//
// WithX methods on widgets allow overriding themed defaults:
//
//	btn := theme.ButtonOf(ctx, "Submit", handleSubmit).
//	    WithBorderRadius(0).  // Sharp corners
//	    WithPadding(layout.EdgeInsetsAll(20))
//
// WithX methods return COPIES; they never mutate the receiver.
//
// # API Rules
//
//   - Canonical = struct literal. Always works, always documented.
//   - Layout helpers (ColumnOf, RowOf, StackOf) exist for ergonomics.
//   - For themed widgets, use theme.XxxOf constructors from pkg/theme.
//   - WithX returns copies. Doc comment must state "returns a copy".
//
// # Layout Widgets
//
// Use Row and Column for horizontal and vertical layouts:
//
//	widgets.Row{Children: []core.Widget{...}}
//	widgets.Column{Children: []core.Widget{...}}
//
// Helper functions provide a more concise syntax:
//
//	widgets.RowOf(alignment, crossAlignment, size, child1, child2, child3)
//	widgets.ColumnOf(alignment, crossAlignment, size, child1, child2, child3)
//
// # Input Widgets
//
// Button, TextField, Checkbox, Radio, and Switch handle user input.
// Use struct literals for explicit control or theme.XxxOf for themed widgets:
//
//	// Struct literal (explicit)
//	widgets.Button{Label: "Submit", OnTap: onTap, Color: colors.Primary, Haptic: true}
//
//	// Themed constructor (from pkg/theme)
//	theme.ButtonOf(ctx, "Submit", onTap)
//
// # Scrolling
//
// ScrollView provides scrollable content with customizable physics:
//
//	widgets.ScrollView{Child: content, Physics: widgets.BouncingScrollPhysics{}}
//
// # Style Guide for Widget Authors
//
// When adding WithX methods:
//   - Use value receiver, not pointer
//   - Return the modified copy, never mutate
//   - Doc comment: "returns a copy of X with..."
package widgets
