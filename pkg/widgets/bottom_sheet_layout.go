package widgets

import (
	"math"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// sheetMetrics reports layout info for bottom sheet state.
type sheetMetrics struct {
	AvailableHeight float64
	ScreenHeight    float64
	TopInset        float64
	BottomInset     float64
	ContentHeight   float64
}

// bottomSheetPositioner positions the sheet at the bottom of the screen.
type bottomSheetPositioner struct {
	Extent       float64
	TopInset     float64
	BottomInset  float64
	ContentSized bool
	Child        core.Widget
	OnMetrics    func(sheetMetrics)
}

func (b bottomSheetPositioner) CreateElement() core.Element {
	return core.NewRenderObjectElement(b, nil)
}

func (b bottomSheetPositioner) Key() any {
	return nil
}

func (b bottomSheetPositioner) ChildWidget() core.Widget {
	return b.Child
}

func (b bottomSheetPositioner) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderBottomSheetPositioner{
		extent:       b.Extent,
		topInset:     b.TopInset,
		bottomInset:  b.BottomInset,
		contentSized: b.ContentSized,
		onMetrics:    b.OnMetrics,
	}
	r.SetSelf(r)
	return r
}

func (b bottomSheetPositioner) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderBottomSheetPositioner); ok {
		r.extent = b.Extent
		r.topInset = b.TopInset
		r.bottomInset = b.BottomInset
		r.contentSized = b.ContentSized
		r.onMetrics = b.OnMetrics
		r.MarkNeedsLayout()
	}
}

// renderBottomSheetPositioner is the render object for bottomSheetPositioner.
// It fills the available space (typically the full screen) and positions
// its child sheet at the bottom, with the visible portion determined by extent.
type renderBottomSheetPositioner struct {
	layout.RenderBoxBase
	child        layout.RenderBox
	extent       float64 // current visible extent in pixels (above safe area)
	topInset     float64 // top safe area inset
	bottomInset  float64 // bottom safe area inset
	contentSized bool    // if true, child determines its own height
	onMetrics    func(sheetMetrics)

	// Computed during layout for hit testing
	sheetTop    float64 // Y position of sheet's top edge
	sheetHeight float64 // visible height of sheet
}

func (r *renderBottomSheetPositioner) SetChild(child layout.RenderObject) {
	if r.child != nil {
		if s, ok := r.child.(interface{ SetParent(layout.RenderObject) }); ok {
			s.SetParent(nil)
		}
	}
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
		if s, ok := r.child.(interface{ SetParent(layout.RenderObject) }); ok {
			s.SetParent(r)
		}
	}
}

func (r *renderBottomSheetPositioner) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

// PerformLayout positions the sheet child at the bottom of the screen.
// The sheet's visible height is determined by extent (for snap-point sheets)
// or by the child's intrinsic size (for content-sized sheets).
// The sheet background extends into the bottom safe area, but content does not.
func (r *renderBottomSheetPositioner) PerformLayout() {
	constraints := r.Constraints()
	screenWidth := constraints.MaxWidth
	screenHeight := constraints.MaxHeight
	if screenWidth <= 0 {
		screenWidth = constraints.MinWidth
	}
	if screenHeight <= 0 {
		screenHeight = constraints.MinHeight
	}

	// Available height for sheet content (excluding safe areas)
	availableHeight := math.Max(0, screenHeight-r.topInset-r.bottomInset)

	// Layout the child to determine its size
	childHeight := 0.0
	if r.child != nil {
		var childConstraints layout.Constraints
		if r.contentSized {
			// Content-sized: let child determine height up to available space
			childConstraints = layout.Constraints{
				MinWidth:  screenWidth,
				MaxWidth:  screenWidth,
				MinHeight: 0,
				MaxHeight: availableHeight + r.bottomInset,
			}
		} else {
			// Snap-point: force child to exact height (extent + bottom inset for background)
			extent := clampFloat(r.extent, 0, availableHeight)
			sheetHeight := extent + r.bottomInset
			childConstraints = layout.Constraints{
				MinWidth:  screenWidth,
				MaxWidth:  screenWidth,
				MinHeight: sheetHeight,
				MaxHeight: sheetHeight,
			}
		}
		r.child.Layout(childConstraints, true)
		childHeight = r.child.Size().Height
	}

	// Report metrics back to sheet state (used for snap height calculations)
	contentHeight := 0.0
	if childHeight > 0 {
		contentHeight = math.Max(0, childHeight-r.bottomInset)
	}
	if r.onMetrics != nil {
		r.onMetrics(sheetMetrics{
			AvailableHeight: availableHeight,
			ScreenHeight:    screenHeight,
			TopInset:        r.topInset,
			BottomInset:     r.bottomInset,
			ContentHeight:   contentHeight,
		})
	}

	// Calculate visible portion of the sheet
	visibleExtent := clampFloat(r.extent, 0, availableHeight)
	visibleHeight := visibleExtent + r.bottomInset
	if visibleExtent <= 0 {
		visibleHeight = 0
	}
	if r.contentSized && childHeight > 0 {
		// For content-sized sheets, clamp to actual content height
		maxExtent := math.Max(0, childHeight-r.bottomInset)
		visibleExtent = clampFloat(r.extent, 0, maxExtent)
		visibleHeight = visibleExtent
		if visibleExtent > 0 {
			visibleHeight = visibleExtent + r.bottomInset
		}
		if visibleHeight > childHeight {
			visibleHeight = childHeight
		}
	}

	// Position the child at the bottom of the screen
	if r.child != nil {
		yOffset := screenHeight - visibleHeight
		if yOffset < r.topInset {
			yOffset = r.topInset
		}
		if r.contentSized && childHeight > 0 && childHeight < visibleHeight {
			yOffset = screenHeight - childHeight
		}
		r.sheetTop = yOffset
		r.sheetHeight = visibleHeight
		r.child.SetParentData(&layout.BoxParentData{
			Offset: graphics.Offset{X: 0, Y: yOffset},
		})
	}

	r.SetSize(graphics.Size{Width: screenWidth, Height: screenHeight})
}

func (r *renderBottomSheetPositioner) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	parentData := r.child.ParentData().(*layout.BoxParentData)
	ctx.PaintChildWithLayer(r.child, parentData.Offset)
}

func (r *renderBottomSheetPositioner) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	if position.Y < r.sheetTop || position.Y > r.sheetTop+r.sheetHeight {
		return false
	}
	local := graphics.Offset{X: position.X, Y: position.Y - r.sheetTop}
	r.child.HitTest(local, result)
	result.Add(r)
	return true
}

// bottomSheetBody lays out the handle and content inside the sheet.
type bottomSheetBody struct {
	Handle       core.Widget
	Content      core.Widget
	BottomInset  float64
	Background   graphics.Color
	BorderRadius float64
	ContentSized bool
}

func (b bottomSheetBody) CreateElement() core.Element {
	return core.NewRenderObjectElement(b, nil)
}

func (b bottomSheetBody) Key() any {
	return nil
}

func (b bottomSheetBody) ChildrenWidgets() []core.Widget {
	handle := b.Handle
	if handle == nil {
		handle = SizedBox{}
	}
	content := b.Content
	if content == nil {
		content = SizedBox{}
	}
	return []core.Widget{handle, content}
}

func (b bottomSheetBody) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderBottomSheetBody{
		bottomInset:  b.BottomInset,
		background:   b.Background,
		borderRadius: b.BorderRadius,
		contentSized: b.ContentSized,
	}
	r.SetSelf(r)
	return r
}

func (b bottomSheetBody) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderBottomSheetBody); ok {
		r.bottomInset = b.BottomInset
		r.background = b.Background
		r.borderRadius = b.BorderRadius
		r.contentSized = b.ContentSized
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

// renderBottomSheetBody is the render object for bottomSheetBody.
// It arranges the drag handle above the content and draws the sheet background.
// The content viewport height adjusts based on the current sheet extent,
// which allows scroll views inside to compute their scroll range correctly.
type renderBottomSheetBody struct {
	layout.RenderBoxBase
	handle       layout.RenderBox // optional drag handle at top
	content      layout.RenderBox // main sheet content
	bottomInset  float64          // space reserved for bottom safe area
	background   graphics.Color   // sheet background color
	borderRadius float64          // top corner radius
	contentSized bool             // if true, content determines sheet height
}

func (r *renderBottomSheetBody) SetChildren(children []layout.RenderObject) {
	setParentOnChild(r.handle, nil)
	setParentOnChild(r.content, nil)
	r.handle = nil
	r.content = nil
	if len(children) > 0 {
		if box, ok := children[0].(layout.RenderBox); ok {
			r.handle = box
			setParentOnChild(r.handle, r)
		}
	}
	if len(children) > 1 {
		if box, ok := children[1].(layout.RenderBox); ok {
			r.content = box
			setParentOnChild(r.content, r)
		}
	}
}

func (r *renderBottomSheetBody) VisitChildren(visitor func(layout.RenderObject)) {
	if r.handle != nil {
		visitor(r.handle)
	}
	if r.content != nil {
		visitor(r.content)
	}
}

// PerformLayout arranges the handle and content vertically within the sheet.
// The layout is: [handle] [content] [bottom inset padding].
// For content-sized sheets, the sheet height is derived from content.
// For snap-point sheets, the content is forced to fill the available space,
// ensuring scroll views get the correct viewport height for their scroll extent.
func (r *renderBottomSheetBody) PerformLayout() {
	constraints := r.Constraints()
	sheetWidth := constraints.MaxWidth
	sheetHeight := constraints.MaxHeight
	if sheetWidth <= 0 {
		sheetWidth = constraints.MinWidth
	}
	if sheetHeight <= 0 {
		sheetHeight = constraints.MinHeight
	}

	// Layout handle first (intrinsic height)
	handleHeight := 0.0
	if r.handle != nil {
		r.handle.Layout(layout.Constraints{
			MinWidth:  sheetWidth,
			MaxWidth:  sheetWidth,
			MinHeight: 0,
			MaxHeight: sheetHeight,
		}, true)
		handleHeight = r.handle.Size().Height
		r.handle.SetParentData(&layout.BoxParentData{Offset: graphics.Offset{X: 0, Y: 0}})
	}

	// Content-sized: let content determine its height, then set sheet size
	if r.contentSized {
		maxContentHeight := math.Max(0, sheetHeight-handleHeight-r.bottomInset)
		contentHeight := 0.0
		if r.content != nil {
			r.content.Layout(layout.Constraints{
				MinWidth:  sheetWidth,
				MaxWidth:  sheetWidth,
				MinHeight: 0,
				MaxHeight: maxContentHeight,
			}, true)
			contentHeight = r.content.Size().Height
			r.content.SetParentData(&layout.BoxParentData{
				Offset: graphics.Offset{X: 0, Y: handleHeight},
			})
		}
		sheetHeight = clampFloat(handleHeight+contentHeight+r.bottomInset, 0, constraints.MaxHeight)
		r.SetSize(graphics.Size{Width: sheetWidth, Height: sheetHeight})
		return
	}

	// Snap-point mode: force content to fill remaining space.
	// This ensures scroll views get viewport height = currentExtent - handleHeight,
	// allowing their scroll extent to grow as the sheet expands.
	contentHeight := math.Max(0, sheetHeight-handleHeight-r.bottomInset)
	if r.content != nil {
		r.content.Layout(layout.Constraints{
			MinWidth:  sheetWidth,
			MaxWidth:  sheetWidth,
			MinHeight: contentHeight,
			MaxHeight: contentHeight,
		}, true)
		r.content.SetParentData(&layout.BoxParentData{
			Offset: graphics.Offset{X: 0, Y: handleHeight},
		})
	}

	r.SetSize(graphics.Size{Width: sheetWidth, Height: sheetHeight})
}

func (r *renderBottomSheetBody) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}

	radius := r.borderRadius
	if radius < 0 {
		radius = 0
	}
	rect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)
	rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(radius))

	ctx.Canvas.Save()
	ctx.Canvas.ClipRRect(rrect)
	ctx.PushClipRect(rect)

	if r.background != 0 {
		paint := graphics.DefaultPaint()
		paint.Color = r.background
		ctx.Canvas.DrawRRect(rrect, paint)
	}

	if r.handle != nil {
		ctx.PaintChildWithLayer(r.handle, getChildOffset(r.handle))
	}
	if r.content != nil {
		ctx.PaintChildWithLayer(r.content, getChildOffset(r.content))
	}

	ctx.PopClipRect()
	ctx.Canvas.Restore()
}

func (r *renderBottomSheetBody) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.handle != nil {
		offset := getChildOffset(r.handle)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if r.handle.HitTest(local, result) {
			return true
		}
	}
	if r.content != nil {
		offset := getChildOffset(r.content)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if r.content.HitTest(local, result) {
			return true
		}
	}
	result.Add(r)
	return true
}
