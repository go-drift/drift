package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// getChildOffset extracts the offset from a child's parent data.
func getChildOffset(child layout.RenderBox) rendering.Offset {
	if child == nil {
		return rendering.Offset{}
	}
	if data, ok := child.ParentData().(*layout.BoxParentData); ok {
		return data.Offset
	}
	return rendering.Offset{}
}

// withinBounds checks if a position is within the given size.
func withinBounds(position rendering.Offset, size rendering.Size) bool {
	return position.X >= 0 && position.Y >= 0 && position.X <= size.Width && position.Y <= size.Height
}

// setChildFromRenderObject converts a RenderObject to a RenderBox.
// Returns nil if the child is nil or not a RenderBox.
func setChildFromRenderObject(child layout.RenderObject) layout.RenderBox {
	box, _ := child.(layout.RenderBox)
	return box
}

// Root creates a top-level view widget with the given child.
func Root(child core.Widget) View {
	return View{ChildWidget: child}
}

// Centered wraps a child in a Center widget.
func Centered(child core.Widget) Center {
	return Center{ChildWidget: child}
}

// Padded wraps a child with the specified padding.
func Padded(padding layout.EdgeInsets, child core.Widget) Padding {
	return Padding{Padding: padding, ChildWidget: child}
}

// ColumnOf creates a vertical layout with the specified alignments and sizing behavior.
func ColumnOf(alignment MainAxisAlignment, crossAlignment CrossAxisAlignment, size MainAxisSize, children ...core.Widget) Column {
	return Column{
		ChildrenWidgets:    children,
		MainAxisAlignment:  alignment,
		CrossAxisAlignment: crossAlignment,
		MainAxisSize:       size,
	}
}

// TextOf creates a styled text widget.
func TextOf(content string, style rendering.TextStyle) Text {
	return Text{Content: content, Style: style, Wrap: true}
}

// VSpace creates a fixed-height vertical spacer.
func VSpace(height float64) SizedBox {
	return SizedBox{Height: height}
}

// HSpace creates a fixed-width horizontal spacer.
func HSpace(width float64) SizedBox {
	return SizedBox{Width: width}
}

// Gesture wraps a child with a tap handler.
func Gesture(onTap func(), child core.Widget) GestureDetector {
	return GestureDetector{OnTap: onTap, ChildWidget: child}
}

// Box creates a container with padding, background color, and alignment.
func Box(child core.Widget, padding layout.EdgeInsets, color rendering.Color, alignment layout.Alignment) Container {
	return Container{
		ChildWidget: child,
		Padding:     padding,
		Color:       color,
		Alignment:   alignment,
	}
}

// RowOf creates a horizontal layout with the specified alignments and sizing behavior.
func RowOf(alignment MainAxisAlignment, crossAlignment CrossAxisAlignment, size MainAxisSize, children ...core.Widget) Row {
	return Row{
		ChildrenWidgets:    children,
		MainAxisAlignment:  alignment,
		CrossAxisAlignment: crossAlignment,
		MainAxisSize:       size,
	}
}

// PaddingAll wraps a child with uniform padding on all sides.
func PaddingAll(value float64, child core.Widget) Padding {
	return Padding{Padding: layout.EdgeInsetsAll(value), ChildWidget: child}
}

// PaddingSym wraps a child with symmetric horizontal and vertical padding.
func PaddingSym(horizontal, vertical float64, child core.Widget) Padding {
	return Padding{Padding: layout.EdgeInsetsSymmetric(horizontal, vertical), ChildWidget: child}
}

// PaddingOnly wraps a child with specific padding on each side.
func PaddingOnly(left, top, right, bottom float64, child core.Widget) Padding {
	return Padding{Padding: layout.EdgeInsetsOnly(left, top, right, bottom), ChildWidget: child}
}

// Spacer creates a fixed-size spacer (alias for VSpace).
func Spacer(size float64) SizedBox {
	return SizedBox{Height: size}
}

// Ptr returns a pointer to the given float64 value.
// This is a convenience helper for Positioned widget fields:
//
//	Positioned{Left: widgets.Ptr(8), Top: widgets.Ptr(16), ChildWidget: child}
func Ptr(v float64) *float64 {
	return &v
}

// ContainerBuilder provides a fluent API for building Container widgets.
type ContainerBuilder struct {
	c Container
}

// NewContainer creates a new ContainerBuilder with the given child.
func NewContainer(child core.Widget) *ContainerBuilder {
	return &ContainerBuilder{c: Container{ChildWidget: child}}
}

// WithColor sets the background color.
func (b *ContainerBuilder) WithColor(color rendering.Color) *ContainerBuilder {
	b.c.Color = color
	return b
}

// WithGradient sets the background gradient.
func (b *ContainerBuilder) WithGradient(gradient *rendering.Gradient) *ContainerBuilder {
	b.c.Gradient = gradient
	return b
}

// WithPadding sets the padding.
func (b *ContainerBuilder) WithPadding(padding layout.EdgeInsets) *ContainerBuilder {
	b.c.Padding = padding
	return b
}

// WithPaddingAll sets uniform padding on all sides.
func (b *ContainerBuilder) WithPaddingAll(value float64) *ContainerBuilder {
	b.c.Padding = layout.EdgeInsetsAll(value)
	return b
}

// WithSize sets the width and height.
func (b *ContainerBuilder) WithSize(width, height float64) *ContainerBuilder {
	b.c.Width = width
	b.c.Height = height
	return b
}

// WithAlignment sets the child alignment.
func (b *ContainerBuilder) WithAlignment(alignment layout.Alignment) *ContainerBuilder {
	b.c.Alignment = alignment
	return b
}

// Build returns the configured Container.
func (b *ContainerBuilder) Build() Container {
	return b.c
}
