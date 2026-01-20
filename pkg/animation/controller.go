package animation

import (
	"time"
)

// AnimationStatus represents the current state of an animation.
type AnimationStatus int

const (
	// AnimationDismissed means the animation is at 0.0.
	AnimationDismissed AnimationStatus = iota
	// AnimationForward means the animation is playing toward 1.0.
	AnimationForward
	// AnimationReverse means the animation is playing toward 0.0.
	AnimationReverse
	// AnimationCompleted means the animation is at 1.0.
	AnimationCompleted
)

// AnimationController drives an animation over time.
type AnimationController struct {
	// Value is the current animation value, ranging from 0.0 to 1.0.
	Value float64

	// Duration is the length of the animation.
	Duration time.Duration

	// Curve transforms linear progress (optional).
	Curve func(float64) float64

	// LowerBound is the minimum value (default 0.0).
	LowerBound float64

	// UpperBound is the maximum value (default 1.0).
	UpperBound float64

	status           AnimationStatus
	ticker           *Ticker
	target           float64
	startValue       float64
	listeners        map[int]func()
	statusListeners  map[int]func(AnimationStatus)
	nextListenerID   int
}

// NewAnimationController creates an animation controller with the given duration.
func NewAnimationController(duration time.Duration) *AnimationController {
	return &AnimationController{
		Value:           0,
		Duration:        duration,
		LowerBound:      0,
		UpperBound:      1,
		Curve:           LinearCurve,
		status:          AnimationDismissed,
		listeners:       make(map[int]func()),
		statusListeners: make(map[int]func(AnimationStatus)),
	}
}

// Forward animates from the current value to the upper bound (1.0).
func (c *AnimationController) Forward() {
	c.animateTo(c.UpperBound, AnimationForward)
}

// Reverse animates from the current value to the lower bound (0.0).
func (c *AnimationController) Reverse() {
	c.animateTo(c.LowerBound, AnimationReverse)
}

// AnimateTo animates to a specific target value.
func (c *AnimationController) AnimateTo(target float64) {
	if target > c.Value {
		c.animateTo(target, AnimationForward)
	} else {
		c.animateTo(target, AnimationReverse)
	}
}

func (c *AnimationController) animateTo(target float64, direction AnimationStatus) {
	if c.ticker != nil {
		c.ticker.Stop()
	}

	c.target = target
	c.startValue = c.Value
	c.setStatus(direction)

	c.ticker = NewTicker(func(elapsed time.Duration) {
		c.tick(elapsed)
	})
	c.ticker.Start()
}

func (c *AnimationController) tick(elapsed time.Duration) {
	if c.Duration <= 0 {
		c.Value = c.target
		c.stop()
		return
	}

	// Calculate progress as fraction of duration
	progress := float64(elapsed) / float64(c.Duration)
	if progress >= 1.0 {
		progress = 1.0
	}

	// Interpolate from start to target
	eased := progress
	if c.Curve != nil {
		eased = c.Curve(progress)
	}
	c.Value = c.startValue + (c.target-c.startValue)*eased
	c.notifyListeners()

	if progress >= 1.0 {
		c.stop()
	}
}

func (c *AnimationController) stop() {
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = nil
	}

	// Update status based on final value
	if c.Value <= c.LowerBound {
		c.setStatus(AnimationDismissed)
	} else if c.Value >= c.UpperBound {
		c.setStatus(AnimationCompleted)
	}
}

// Reset immediately sets the value to the lower bound.
func (c *AnimationController) Reset() {
	c.Stop()
	c.Value = c.LowerBound
	c.setStatus(AnimationDismissed)
	c.notifyListeners()
}

// Stop stops the animation at the current value.
func (c *AnimationController) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = nil
	}
}

// Status returns the current animation status.
func (c *AnimationController) Status() AnimationStatus {
	return c.status
}

// IsAnimating returns true if the animation is currently running.
func (c *AnimationController) IsAnimating() bool {
	return c.status == AnimationForward || c.status == AnimationReverse
}

// IsCompleted returns true if the animation finished at the upper bound.
func (c *AnimationController) IsCompleted() bool {
	return c.status == AnimationCompleted
}

// IsDismissed returns true if the animation is at the lower bound.
func (c *AnimationController) IsDismissed() bool {
	return c.status == AnimationDismissed
}

// AddListener adds a callback that fires whenever the value changes.
// Returns an unsubscribe function.
func (c *AnimationController) AddListener(fn func()) func() {
	id := c.nextListenerID
	c.nextListenerID++
	c.listeners[id] = fn
	return func() {
		delete(c.listeners, id)
	}
}

// AddStatusListener adds a callback that fires whenever the status changes.
// Returns an unsubscribe function.
func (c *AnimationController) AddStatusListener(fn func(AnimationStatus)) func() {
	id := c.nextListenerID
	c.nextListenerID++
	c.statusListeners[id] = fn
	return func() {
		delete(c.statusListeners, id)
	}
}

func (c *AnimationController) setStatus(status AnimationStatus) {
	if c.status == status {
		return
	}
	c.status = status
	for _, listener := range c.statusListeners {
		listener(status)
	}
}

func (c *AnimationController) notifyListeners() {
	for _, listener := range c.listeners {
		listener()
	}
}

// Dispose cleans up resources used by the controller.
func (c *AnimationController) Dispose() {
	c.Stop()
	c.listeners = nil
	c.statusListeners = nil
}
