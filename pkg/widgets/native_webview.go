package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// NativeWebView embeds a native web browser view.
//
// Create a [platform.WebViewController] with [core.UseController] and pass
// it to this widget:
//
//	s.web = core.UseController(&s.StateBase, platform.NewWebViewController)
//	s.web.OnPageFinished = func(url string) { ... }
//	s.web.Load("https://example.com")
//
//	// in Build:
//	widgets.NativeWebView{Controller: s.web, Height: 400}
//
// Width and Height set explicit dimensions. If Width is 0, the view expands
// to fill available width.
type NativeWebView struct {
	// Controller provides the native web view surface and navigation control.
	Controller *platform.WebViewController

	// Width of the web view in logical pixels (0 = expand to fill).
	Width float64

	// Height of the web view in logical pixels.
	Height float64
}

// CreateElement creates the element for this render object widget.
func (n NativeWebView) CreateElement() core.Element {
	return core.NewRenderObjectElement(n, nil)
}

// Key returns the widget key.
func (n NativeWebView) Key() any {
	return nil
}

// CreateRenderObject creates the render object for this widget.
func (n NativeWebView) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	height := n.Height
	if height == 0 {
		height = 300
	}
	r := &renderNativeWebView{
		controller: n.Controller,
		width:      n.Width,
		height:     height,
	}
	r.SetSelf(r)
	return r
}

// UpdateRenderObject updates the render object with new widget properties.
func (n NativeWebView) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderNativeWebView); ok {
		height := n.Height
		if height == 0 {
			height = 300
		}
		r.controller = n.Controller
		r.width = n.Width
		r.height = height
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderNativeWebView struct {
	layout.RenderBoxBase
	controller *platform.WebViewController
	width      float64
	height     float64
}

func (r *renderNativeWebView) PerformLayout() {
	constraints := r.Constraints()
	width := r.width
	if width == 0 {
		width = constraints.MaxWidth
	}
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)

	height := r.height
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)

	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderNativeWebView) Paint(ctx *layout.PaintContext) {
	size := r.Size()

	// Draw a placeholder background for the web view
	bgPaint := graphics.DefaultPaint()
	bgPaint.Color = graphics.Color(0xFFF0F0F0) // Light gray
	ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, size.Width, size.Height), bgPaint)

	if r.controller != nil && r.controller.ViewID() != 0 {
		ctx.EmbedPlatformView(r.controller.ViewID(), size)
	}
}

func (r *renderNativeWebView) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
