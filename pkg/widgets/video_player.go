package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// VideoPlayer embeds a native video player view with built-in platform controls.
//
// The native player provides standard controls (play/pause, seek bar, time display)
// on both platforms. No Drift overlay is needed.
//
// Create a [platform.VideoPlayerController] with [core.UseController] and pass
// it to this widget:
//
//	s.video = core.UseController(&s.StateBase, platform.NewVideoPlayerController)
//	s.video.OnPlaybackStateChanged = func(state platform.PlaybackState) { ... }
//	s.video.Load(url)
//
//	// in Build:
//	widgets.VideoPlayer{Controller: s.video, Height: 225}
//
// Width and Height set explicit dimensions. Use layout widgets such as [Expanded]
// to fill available space.
type VideoPlayer struct {
	// Controller provides the native video player surface and playback control.
	Controller *platform.VideoPlayerController

	// Width of the video player in logical pixels.
	Width float64

	// Height of the video player in logical pixels.
	Height float64
}

// CreateElement creates the element for this render object widget.
func (v VideoPlayer) CreateElement() core.Element {
	return core.NewRenderObjectElement(v, nil)
}

// Key returns the widget key.
func (v VideoPlayer) Key() any {
	return nil
}

// CreateRenderObject creates the render object for this widget.
func (v VideoPlayer) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderVideoPlayer{
		controller: v.Controller,
		width:      v.Width,
		height:     v.Height,
	}
	r.SetSelf(r)
	return r
}

// UpdateRenderObject updates the render object with new widget properties.
func (v VideoPlayer) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderVideoPlayer); ok {
		r.controller = v.Controller
		r.width = v.Width
		r.height = v.Height
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderVideoPlayer struct {
	layout.RenderBoxBase
	controller *platform.VideoPlayerController
	width      float64
	height     float64
}

func (r *renderVideoPlayer) PerformLayout() {
	constraints := r.Constraints()
	width := min(max(r.width, constraints.MinWidth), constraints.MaxWidth)
	height := min(max(r.height, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderVideoPlayer) Paint(ctx *layout.PaintContext) {
	size := r.Size()

	// Draw a dark placeholder background behind the platform view
	bgPaint := graphics.DefaultPaint()
	bgPaint.Color = graphics.Color(0xFF1A1A1A)
	ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, size.Width, size.Height), bgPaint)

	if r.controller != nil && r.controller.ViewID() != 0 {
		ctx.EmbedPlatformView(r.controller.ViewID(), size)
	}
}

func (r *renderVideoPlayer) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
