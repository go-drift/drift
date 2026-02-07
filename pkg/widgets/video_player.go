package widgets

import (
	"fmt"
	"sync"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// VideoPlayer embeds a native video player view with built-in platform controls.
//
// The native player provides standard controls (play/pause, seek bar, time display)
// on both platforms. No Drift overlay is needed.
//
// # Creation Pattern
//
//	controller := &widgets.VideoPlayerController{}
//	widgets.VideoPlayer{
//	    URL:        "https://example.com/video.mp4",
//	    Controller: controller,
//	    AutoPlay:   true,
//	    Width:      400,
//	    Height:     225,
//	}
type VideoPlayer struct {
	// URL is the media URL to play.
	URL string

	// Controller provides programmatic control over the video player.
	Controller *VideoPlayerController

	// AutoPlay starts playback automatically when the view is created.
	AutoPlay bool

	// Looping restarts playback when it reaches the end.
	Looping bool

	// Volume sets the playback volume (0.0 to 1.0). Nil uses the platform
	// default (1.0). Changes are propagated to the native player on rebuild.
	Volume *float64

	// Width of the video player (0 = expand to fill).
	Width float64

	// Height of the video player (0 = 200 default).
	Height float64

	// OnPlaybackStateChanged is called when the playback state changes.
	OnPlaybackStateChanged func(state platform.PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	OnPositionChanged func(positionMs, durationMs, bufferedMs int64)

	// OnError is called when a playback error occurs.
	OnError func(code string, message string)
}

// CreateElement creates the element for the stateful widget.
func (v VideoPlayer) CreateElement() core.Element {
	return core.NewStatefulElement(v, nil)
}

// Key returns the widget key.
func (v VideoPlayer) Key() any {
	return nil
}

// CreateState creates the state for this widget.
func (v VideoPlayer) CreateState() core.State {
	return &videoPlayerState{}
}

// VideoPlayerController provides programmatic control over a [VideoPlayer] widget.
// All methods are safe for concurrent use. Methods are no-ops when no native
// view is bound (before the widget is first painted or after it is disposed).
type VideoPlayerController struct {
	mu   sync.RWMutex
	view *platform.VideoPlayerView
}

// Play starts playback.
func (c *VideoPlayerController) Play() {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.Play()
	}
}

// Pause pauses playback.
func (c *VideoPlayerController) Pause() {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.Pause()
	}
}

// SeekTo seeks to a position in milliseconds.
func (c *VideoPlayerController) SeekTo(positionMs int64) {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.SeekTo(positionMs)
	}
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (c *VideoPlayerController) SetVolume(volume float64) {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.SetVolume(volume)
	}
}

// SetLooping sets whether playback should loop.
func (c *VideoPlayerController) SetLooping(looping bool) {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.SetLooping(looping)
	}
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (c *VideoPlayerController) SetPlaybackSpeed(rate float64) {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.SetPlaybackSpeed(rate)
	}
}

// State returns the current playback state, or [platform.PlaybackStateIdle]
// if no native view is bound.
func (c *VideoPlayerController) State() platform.PlaybackState {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.State()
	}
	return platform.PlaybackStateIdle
}

// PositionMs returns the current playback position in milliseconds,
// or 0 if no native view is bound.
func (c *VideoPlayerController) PositionMs() int64 {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.PositionMs()
	}
	return 0
}

// DurationMs returns the total media duration in milliseconds,
// or 0 if no native view is bound or no media is loaded.
func (c *VideoPlayerController) DurationMs() int64 {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.DurationMs()
	}
	return 0
}

// BufferedMs returns the buffered position in milliseconds,
// or 0 if no native view is bound.
func (c *VideoPlayerController) BufferedMs() int64 {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.BufferedMs()
	}
	return 0
}

func (c *VideoPlayerController) setView(v *platform.VideoPlayerView) {
	c.mu.Lock()
	c.view = v
	c.mu.Unlock()
}

func (c *VideoPlayerController) clearView() {
	c.mu.Lock()
	c.view = nil
	c.mu.Unlock()
}

type videoPlayerState struct {
	element      *core.StatefulElement
	platformView *platform.VideoPlayerView
}

func (s *videoPlayerState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *videoPlayerState) InitState() {}

func (s *videoPlayerState) Dispose() {
	if s.platformView != nil {
		w := s.element.Widget().(VideoPlayer)
		if w.Controller != nil {
			w.Controller.clearView()
		}
		platform.GetPlatformViewRegistry().Dispose(s.platformView.ViewID())
		s.platformView = nil
	}
}

func (s *videoPlayerState) DidChangeDependencies() {}

func (s *videoPlayerState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(VideoPlayer)
	w := s.element.Widget().(VideoPlayer)

	// Rebind controller if it changed
	if old.Controller != w.Controller {
		if old.Controller != nil {
			old.Controller.clearView()
		}
		if w.Controller != nil && s.platformView != nil {
			w.Controller.setView(s.platformView)
		}
	}

	if s.platformView == nil {
		return
	}

	// Propagate property changes to the native view
	if old.URL != w.URL {
		s.platformView.LoadURL(w.URL)
	}
	if old.Looping != w.Looping {
		s.platformView.SetLooping(w.Looping)
	}
	if volumeChanged(old.Volume, w.Volume) {
		v := 1.0
		if w.Volume != nil {
			v = *w.Volume
		}
		s.platformView.SetVolume(v)
	}
}

func (s *videoPlayerState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *videoPlayerState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(VideoPlayer)

	height := w.Height
	if height == 0 {
		height = 200
	}

	return videoPlayerRender{
		url:      w.URL,
		autoPlay: w.AutoPlay,
		looping:  w.Looping,
		volume:   w.Volume,
		width:    w.Width,
		height:   height,
		state:    s,
		config:   w,
	}
}

// OnPlaybackStateChanged implements platform.VideoPlayerClient.
func (s *videoPlayerState) OnPlaybackStateChanged(state platform.PlaybackState) {
	w := s.element.Widget().(VideoPlayer)
	if w.OnPlaybackStateChanged != nil {
		w.OnPlaybackStateChanged(state)
	}
}

// OnPositionChanged implements platform.VideoPlayerClient.
func (s *videoPlayerState) OnPositionChanged(positionMs, durationMs, bufferedMs int64) {
	w := s.element.Widget().(VideoPlayer)
	if w.OnPositionChanged != nil {
		w.OnPositionChanged(positionMs, durationMs, bufferedMs)
	}
}

// OnError implements platform.VideoPlayerClient.
func (s *videoPlayerState) OnError(code string, message string) {
	w := s.element.Widget().(VideoPlayer)
	if w.OnError != nil {
		w.OnError(code, message)
	}
}

func (s *videoPlayerState) ensurePlatformView(config VideoPlayer) {
	if s.platformView != nil {
		if config.Controller != nil {
			config.Controller.setView(s.platformView)
		}
		return
	}

	params := map[string]any{
		"url":      config.URL,
		"autoPlay": config.AutoPlay,
		"looping":  config.Looping,
	}
	if config.Volume != nil {
		params["volume"] = *config.Volume
	}

	view, err := platform.GetPlatformViewRegistry().Create("video_player", params)
	if err != nil {
		errors.Report(&errors.DriftError{
			Op:  "VideoPlayer.ensurePlatformView",
			Err: fmt.Errorf("failed to create video player view: %w", err),
		})
		return
	}

	videoView, ok := view.(*platform.VideoPlayerView)
	if !ok {
		errors.Report(&errors.DriftError{
			Op:  "VideoPlayer.ensurePlatformView",
			Err: fmt.Errorf("unexpected view type: %T", view),
		})
		return
	}

	s.platformView = videoView
	s.platformView.SetClient(s)

	if config.Controller != nil {
		config.Controller.setView(s.platformView)
	}
}

type videoPlayerRender struct {
	url      string
	autoPlay bool
	looping  bool
	volume   *float64
	width    float64
	height   float64
	state    *videoPlayerState
	config   VideoPlayer
}

func (v videoPlayerRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(v, nil)
}

func (v videoPlayerRender) Key() any {
	return nil
}

func (v videoPlayerRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderVideoPlayer{
		width:  v.width,
		height: v.height,
		state:  v.state,
		config: v.config,
	}
	r.SetSelf(r)
	return r
}

func (v videoPlayerRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderVideoPlayer); ok {
		r.width = v.width
		r.height = v.height
		r.state = v.state
		r.config = v.config
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderVideoPlayer struct {
	layout.RenderBoxBase
	width  float64
	height float64
	state  *videoPlayerState
	config VideoPlayer
}

func (r *renderVideoPlayer) PerformLayout() {
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

func (r *renderVideoPlayer) Paint(ctx *layout.PaintContext) {
	size := r.Size()

	// Draw a dark placeholder background behind the platform view
	bgPaint := graphics.DefaultPaint()
	bgPaint.Color = graphics.Color(0xFF1A1A1A)
	ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, size.Width, size.Height), bgPaint)

	if r.state != nil {
		r.state.ensurePlatformView(r.config)
		if r.state.platformView != nil {
			ctx.EmbedPlatformView(r.state.platformView.ViewID(), size)
		}
	}
}

func (r *renderVideoPlayer) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func volumeChanged(a, b *float64) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return *a != *b
}
