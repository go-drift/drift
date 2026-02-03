package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// getChildOffset extracts the offset from a child's parent data.
func getChildOffset(child layout.RenderBox) graphics.Offset {
	if child == nil {
		return graphics.Offset{}
	}
	if data, ok := child.ParentData().(*layout.BoxParentData); ok {
		return data.Offset
	}
	return graphics.Offset{}
}

// withinBounds checks if a position is within the given size.
func withinBounds(position graphics.Offset, size graphics.Size) bool {
	return position.X >= 0 && position.Y >= 0 && position.X <= size.Width && position.Y <= size.Height
}

// setChildFromRenderObject converts a RenderObject to a RenderBox.
// Returns nil if the child is nil or not a RenderBox.
func setChildFromRenderObject(child layout.RenderObject) layout.RenderBox {
	box, _ := child.(layout.RenderBox)
	return box
}

// setParentOnChild sets the parent reference on a child render object.
func setParentOnChild(child, parent layout.RenderObject) {
	if child == nil {
		return
	}
	getter, _ := child.(interface{ Parent() layout.RenderObject })
	setter, ok := child.(interface{ SetParent(layout.RenderObject) })
	if !ok {
		return
	}
	currentParent := layout.RenderObject(nil)
	if getter != nil {
		currentParent = getter.Parent()
	}
	if currentParent == parent {
		return
	}
	setter.SetParent(parent)
	if currentParent != nil {
		if marker, ok := currentParent.(interface{ MarkNeedsLayout() }); ok {
			marker.MarkNeedsLayout()
		}
	}
	if parent != nil {
		if marker, ok := parent.(interface{ MarkNeedsLayout() }); ok {
			marker.MarkNeedsLayout()
		}
	}
}

// Root creates a top-level view widget with the given child.
func Root(child core.Widget) View {
	return View{Child: child}
}

// Centered wraps a child in a Center widget.
func Centered(child core.Widget) Center {
	return Center{Child: child}
}

// Padded wraps a child with the specified padding.
func Padded(padding layout.EdgeInsets, child core.Widget) Padding {
	return Padding{Padding: padding, Child: child}
}

// VSpace creates a fixed-height vertical spacer.
func VSpace(height float64) SizedBox {
	return SizedBox{Height: height}
}

// HSpace creates a fixed-width horizontal spacer.
func HSpace(width float64) SizedBox {
	return SizedBox{Width: width}
}

// Tap wraps a child with a tap handler.
func Tap(onTap func(), child core.Widget) GestureDetector {
	return GestureDetector{OnTap: onTap, Child: child}
}

// Drag wraps a child with pan (omnidirectional) drag handlers.
func Drag(onUpdate func(DragUpdateDetails), child core.Widget) GestureDetector {
	return GestureDetector{OnPanUpdate: onUpdate, Child: child}
}

// HorizontalDrag wraps a child with horizontal-only drag handlers.
func HorizontalDrag(onUpdate func(DragUpdateDetails), child core.Widget) GestureDetector {
	return GestureDetector{OnHorizontalDragUpdate: onUpdate, Child: child}
}

// VerticalDrag wraps a child with vertical-only drag handlers.
func VerticalDrag(onUpdate func(DragUpdateDetails), child core.Widget) GestureDetector {
	return GestureDetector{OnVerticalDragUpdate: onUpdate, Child: child}
}

// Clamp constrains a value between min and max bounds.
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// PaddingAll wraps a child with uniform padding on all sides.
func PaddingAll(value float64, child core.Widget) Padding {
	return Padding{Padding: layout.EdgeInsetsAll(value), Child: child}
}

// PaddingSym wraps a child with symmetric horizontal and vertical padding.
func PaddingSym(horizontal, vertical float64, child core.Widget) Padding {
	return Padding{Padding: layout.EdgeInsetsSymmetric(horizontal, vertical), Child: child}
}

// PaddingOnly wraps a child with specific padding on each side.
func PaddingOnly(left, top, right, bottom float64, child core.Widget) Padding {
	return Padding{Padding: layout.EdgeInsetsOnly(left, top, right, bottom), Child: child}
}

// Spacer creates a fixed-size spacer (alias for VSpace).
func Spacer(size float64) SizedBox {
	return SizedBox{Height: size}
}

// Ptr returns a pointer to the given float64 value.
// This is a convenience helper for Positioned widget fields:
//
//	Positioned{Left: widgets.Ptr(8), Top: widgets.Ptr(16), Child: child}
func Ptr(v float64) *float64 {
	return &v
}
