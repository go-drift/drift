// Package animation provides animation primitives for building smooth,
// physics-based animations in Drift applications.
//
// # Core Components
//
// The animation system consists of several key components:
//
//   - [AnimationController]: Drives animations over time, managing value progression
//     from 0.0 to 1.0 with configurable duration and easing curves.
//
//   - [Tween]: Interpolates between begin and end values of any type using the
//     controller's current value. Generic tweens support float64, Color, Offset, etc.
//
//   - [Curves]: Easing functions that transform linear progress into natural-feeling
//     motion. Includes standard curves like [EaseIn], [EaseOut], [EaseInOut].
//
//   - [SpringSimulation]: Physics-based spring animation for natural bounce effects,
//     commonly used for scroll overscroll and gesture-driven animations.
//
// # Basic Usage
//
// Create a controller, configure a tween, and use AddListener to rebuild on changes:
//
//	// In InitState
//	s.controller = animation.NewAnimationController(300 * time.Millisecond)
//	s.controller.Curve = animation.EaseInOut
//	s.opacityTween = animation.TweenFloat64(0, 1)
//	s.controller.AddListener(func() {
//	    s.SetState(func() {})
//	})
//	s.controller.Forward()
//
//	// In Build
//	opacity := s.opacityTween.Transform(s.controller)
//	return widgets.Opacity{Opacity: opacity, ChildWidget: child}
//
//	// In Dispose
//	s.controller.Dispose()
//
// # Implicit Animations
//
// For simpler cases, use implicit animation widgets like [widgets.AnimatedContainer]
// or [widgets.AnimatedOpacity] which manage controllers internally.
package animation

import (
	"sync"
	"time"
)

var (
	tickerMu      sync.Mutex
	activeTickers = make(map[*Ticker]struct{})
	lastTickTime  time.Time
)

// Ticker calls a callback on each frame while active.
//
// Ticker is the low-level timing primitive used by [AnimationController].
// Most code should use AnimationController directly rather than Ticker.
//
// The callback receives the elapsed time since Start was called. Tickers are
// driven by the engine's frame loop via [StepTickers].
type Ticker struct {
	callback func(elapsed time.Duration)
	isActive bool
	start    time.Time
}

// NewTicker creates a new ticker with the given callback.
func NewTicker(callback func(elapsed time.Duration)) *Ticker {
	return &Ticker{
		callback: callback,
	}
}

// Start activates the ticker.
func (t *Ticker) Start() {
	if t.isActive {
		return
	}
	t.isActive = true
	t.start = time.Now()
	tickerMu.Lock()
	activeTickers[t] = struct{}{}
	tickerMu.Unlock()
}

// Stop deactivates the ticker.
func (t *Ticker) Stop() {
	if !t.isActive {
		return
	}
	t.isActive = false
	tickerMu.Lock()
	delete(activeTickers, t)
	tickerMu.Unlock()
}

// IsActive returns whether the ticker is currently running.
func (t *Ticker) IsActive() bool {
	return t.isActive
}

// Elapsed returns the time since the ticker started.
func (t *Ticker) Elapsed() time.Duration {
	if !t.isActive {
		return 0
	}
	return time.Since(t.start)
}

// TickerProvider creates tickers.
type TickerProvider interface {
	CreateTicker(callback func(time.Duration)) *Ticker
}

// StepTickers advances all active tickers.
// This should be called once per frame from the engine.
func StepTickers() {
	tickerMu.Lock()
	if len(activeTickers) == 0 {
		tickerMu.Unlock()
		return
	}
	// Make a copy to avoid holding lock during callbacks
	tickers := make([]*Ticker, 0, len(activeTickers))
	for ticker := range activeTickers {
		tickers = append(tickers, ticker)
	}
	tickerMu.Unlock()

	for _, ticker := range tickers {
		if ticker.isActive && ticker.callback != nil {
			elapsed := time.Since(ticker.start)
			ticker.callback(elapsed)
		}
	}
}

// HasActiveTickers returns true if any tickers are active.
func HasActiveTickers() bool {
	tickerMu.Lock()
	defer tickerMu.Unlock()
	return len(activeTickers) > 0
}
