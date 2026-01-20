package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
)

// NativeWebView embeds a native web browser view.
type NativeWebView struct {
	// InitialURL is the URL to load when the view is created.
	InitialURL string

	// Controller provides programmatic control over the web view.
	Controller *WebViewController

	// OnPageStarted is called when a page starts loading.
	OnPageStarted func(url string)

	// OnPageFinished is called when a page finishes loading.
	OnPageFinished func(url string)

	// OnError is called when a loading error occurs.
	OnError func(err error)

	// Width of the web view (0 = expand to fill).
	Width float64

	// Height of the web view.
	Height float64
}

// CreateElement creates the element for the stateful widget.
func (n NativeWebView) CreateElement() core.Element {
	return core.NewStatefulElement(n, nil)
}

// Key returns the widget key.
func (n NativeWebView) Key() any {
	return nil
}

// CreateState creates the state for this widget.
func (n NativeWebView) CreateState() core.State {
	return &nativeWebViewState{}
}

// WebViewController provides control over a NativeWebView.
type WebViewController struct {
	viewID int64
}

// LoadURL loads the specified URL.
func (c *WebViewController) LoadURL(url string) error {
	if c.viewID == 0 {
		return nil
	}
	_, err := platform.GetPlatformViewRegistry().InvokeViewMethod(c.viewID, "loadUrl", map[string]any{
		"url": url,
	})
	return err
}

// GoBack navigates back in history.
func (c *WebViewController) GoBack() error {
	if c.viewID == 0 {
		return nil
	}
	_, err := platform.GetPlatformViewRegistry().InvokeViewMethod(c.viewID, "goBack", nil)
	return err
}

// GoForward navigates forward in history.
func (c *WebViewController) GoForward() error {
	if c.viewID == 0 {
		return nil
	}
	_, err := platform.GetPlatformViewRegistry().InvokeViewMethod(c.viewID, "goForward", nil)
	return err
}

// Reload reloads the current page.
func (c *WebViewController) Reload() error {
	if c.viewID == 0 {
		return nil
	}
	_, err := platform.GetPlatformViewRegistry().InvokeViewMethod(c.viewID, "reload", nil)
	return err
}

type nativeWebViewState struct {
	element *core.StatefulElement
	viewID  int64
}

func (s *nativeWebViewState) ensurePlatformView(initialURL string, controller *WebViewController) {
	if s.viewID != 0 {
		if controller != nil {
			controller.viewID = s.viewID
		}
		return
	}

	params := map[string]any{}
	if initialURL != "" {
		params["initialUrl"] = initialURL
	}

	view, err := platform.GetPlatformViewRegistry().Create("native_webview", params)
	if err != nil {
		return
	}

	s.viewID = view.ViewID()
	if controller != nil {
		controller.viewID = s.viewID
	}
}

func (s *nativeWebViewState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *nativeWebViewState) InitState() {}

func (s *nativeWebViewState) Dispose() {
	if s.viewID != 0 {
		platform.GetPlatformViewRegistry().Dispose(s.viewID)
		s.viewID = 0
	}
}

func (s *nativeWebViewState) DidChangeDependencies() {}

func (s *nativeWebViewState) DidUpdateWidget(oldWidget core.StatefulWidget) {}

func (s *nativeWebViewState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *nativeWebViewState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(NativeWebView)

	// Default height
	height := w.Height
	if height == 0 {
		height = 300
	}

	return nativeWebViewRender{
		initialURL: w.InitialURL,
		width:      w.Width,
		height:     height,
		controller: w.Controller,
		state:      s,
		config:     w,
	}
}

// nativeWebViewRender is a render widget for the web view.
type nativeWebViewRender struct {
	initialURL string
	width      float64
	height     float64
	controller *WebViewController
	state      *nativeWebViewState
	config     NativeWebView
}

func (n nativeWebViewRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(n, nil)
}

func (n nativeWebViewRender) Key() any {
	return nil
}

func (n nativeWebViewRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderNativeWebView{
		initialURL: n.initialURL,
		width:      n.width,
		height:     n.height,
		controller: n.controller,
		state:      n.state,
	}

	r.SetSelf(r)
	return r
}

func (n nativeWebViewRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderNativeWebView); ok {
		r.initialURL = n.initialURL
		r.width = n.width
		r.height = n.height
		r.controller = n.controller
		r.state = n.state
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderNativeWebView struct {
	layout.RenderBoxBase
	initialURL string
	width      float64
	height     float64
	controller *WebViewController
	state      *nativeWebViewState
}

func (r *renderNativeWebView) Layout(constraints layout.Constraints) {
	width := r.width
	if width == 0 {
		width = constraints.MaxWidth
	}
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)

	height := r.height
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)

	r.SetSize(rendering.Size{Width: width, Height: height})
}

func (r *renderNativeWebView) ensurePlatformView() {
	if r.state == nil {
		return
	}
	r.state.ensurePlatformView(r.initialURL, r.controller)
}

func (r *renderNativeWebView) updatePlatformView() {
	if r.state == nil || r.state.viewID == 0 {
		return
	}
	view := platform.GetPlatformViewRegistry().GetView(r.state.viewID)
	if view == nil {
		return
	}

	offset := rendering.Offset{}
	if r.state.element != nil {
		offset = core.GlobalOffsetOf(r.state.element)
	} else if parentData, ok := r.ParentData().(*layout.BoxParentData); ok && parentData != nil {
		offset = parentData.Offset
	}

	view.SetOffset(offset)
	view.SetSize(r.Size())
	view.SetVisible(true)
}

func (r *renderNativeWebView) Paint(ctx *layout.PaintContext) {
	r.ensurePlatformView()
	r.updatePlatformView()

	size := r.Size()

	// Draw a placeholder background for the web view
	bgPaint := rendering.DefaultPaint()
	bgPaint.Color = rendering.Color(0xFFF0F0F0) // Light gray

	ctx.Canvas.DrawRect(rendering.RectFromLTWH(0, 0, size.Width, size.Height), bgPaint)

	// Draw border
	borderPaint := rendering.DefaultPaint()
	borderPaint.Color = rendering.Color(0xFFCCCCCC)
	borderPaint.Style = rendering.PaintStyleStroke
	borderPaint.StrokeWidth = 1

	ctx.Canvas.DrawRect(rendering.RectFromLTWH(0.5, 0.5, size.Width-1, size.Height-1), borderPaint)

	// Draw a "web view" label in the center (placeholder until native view is positioned)
	textStyle := rendering.TextStyle{
		FontSize: 14,
		Color:    rendering.Color(0xFF999999),
	}
	manager, _ := rendering.DefaultFontManagerErr()
	if manager == nil {
		// Error already reported by DefaultFontManagerErr
		return
	}
	layout, err := rendering.LayoutText("WebView", textStyle, manager)
	if err == nil {
		textX := (size.Width - layout.Size.Width) / 2
		textY := (size.Height - layout.Size.Height) / 2
		ctx.Canvas.DrawText(layout, rendering.Offset{X: textX, Y: textY})
	}
}

func (r *renderNativeWebView) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
