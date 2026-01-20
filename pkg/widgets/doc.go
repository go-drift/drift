// Package widgets provides UI components for building widget trees.
//
// This package contains the concrete widget implementations that developers
// use to build user interfaces, including layout widgets (Row, Column, Stack),
// display widgets (Text, Icon, Image), input widgets (Button, TextField, Checkbox),
// and container widgets (Container, Padding, SizedBox).
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
// Use the builder pattern for customization:
//
//	widgets.NewButton("Submit", onTap).WithPadding(padding).WithColor(bg, text)
//
// # Scrolling
//
// ScrollView provides scrollable content with customizable physics:
//
//	widgets.ScrollView{Child: content, Physics: widgets.BouncingScrollPhysics{}}
package widgets
