package widgets

import (
	"fmt"
	"sync"
	"time"

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
// Width and Height set explicit dimensions. Use layout widgets such as [Expanded]
// to fill available space.
//
// Set callback fields (OnPlaybackStateChanged, OnPositionChanged, OnError) in
// the struct literal that first supplies a URL. Because the native player begins
// loading as soon as the widget is painted, callbacks added in a later rebuild
// may miss early events such as the initial state transition.
//
// # Creation Pattern
//
//	controller := &widgets.VideoPlayerController{}
//	widgets.VideoPlayer{
//	    URL:        "https://example.com/video.mp4",
//	    Controller: controller,
//	    AutoPlay:   true,
//	    Volume:     1.0,
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

	// Volume sets the playback volume (0.0 to 1.0). Zero means muted.
	Volume float64

	// Width of the video player in logical pixels.
	Width float64

	// Height of the video player in logical pixels.
	Height float64

	// OnPlaybackStateChanged is called when the playback state changes.
	// Called on the UI thread.
	// Set this in the struct literal that first supplies a URL to avoid
	// missing events.
	OnPlaybackStateChanged func(state platform.PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	// Called on the UI thread.
	// Set this in the struct literal that first supplies a URL to avoid
	// missing events.
	OnPositionChanged func(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
	// Called on the UI thread.
	// Set this in the struct literal that first supplies a URL to avoid
	// missing events.
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

// Load loads a new media URL, replacing the current media item.
// Call [VideoPlayerController.Play] to start playback.
func (c *VideoPlayerController) Load(url string) {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.Load(url)
	}
}

// Play starts or resumes playback. Call [VideoPlayerController.Load] first
// to set the media URL.
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

// Stop stops playback and resets the player to the idle state.
func (c *VideoPlayerController) Stop() {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.Stop()
	}
}

// SeekTo seeks to the given position.
func (c *VideoPlayerController) SeekTo(position time.Duration) {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		v.SeekTo(position)
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

// Position returns the current playback position,
// or 0 if no native view is bound.
func (c *VideoPlayerController) Position() time.Duration {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.Position()
	}
	return 0
}

// Duration returns the total media duration,
// or 0 if no native view is bound or no media is loaded.
func (c *VideoPlayerController) Duration() time.Duration {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.Duration()
	}
	return 0
}

// Buffered returns the buffered position,
// or 0 if no native view is bound.
func (c *VideoPlayerController) Buffered() time.Duration {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.Buffered()
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
		s.platformView.Load(w.URL)
	}
	if old.Looping != w.Looping {
		s.platformView.SetLooping(w.Looping)
	}
	if old.Volume != w.Volume {
		s.platformView.SetVolume(w.Volume)
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

	return videoPlayerRender{
		width:  w.Width,
		height: w.Height,
		state:  s,
		config: w,
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
		"volume":   config.Volume,
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

	// Set callbacks that forward to the current widget's callbacks.
	s.platformView.OnPlaybackStateChanged = func(state platform.PlaybackState) {
		w := s.element.Widget().(VideoPlayer)
		if w.OnPlaybackStateChanged != nil {
			w.OnPlaybackStateChanged(state)
		}
	}
	s.platformView.OnPositionChanged = func(position, duration, buffered time.Duration) {
		w := s.element.Widget().(VideoPlayer)
		if w.OnPositionChanged != nil {
			w.OnPositionChanged(position, duration, buffered)
		}
	}
	s.platformView.OnError = func(code, message string) {
		w := s.element.Widget().(VideoPlayer)
		if w.OnError != nil {
			w.OnError(code, message)
		}
	}

	if config.Controller != nil {
		config.Controller.setView(s.platformView)
	}
}

type videoPlayerRender struct {
	width  float64
	height float64
	state  *videoPlayerState
	config VideoPlayer
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
