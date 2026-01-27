package animation

import "math"

// SpringDescription describes the physical properties of a spring for
// physics-based animations.
//
// Springs create natural-feeling motion that responds to velocity and can
// overshoot the target before settling. They're ideal for gesture-driven
// animations, scroll overscroll effects, and transitions that should feel
// physically plausible.
//
// The DampingRatio controls oscillation: <1.0 bounces, =1.0 is critically damped,
// >1.0 is overdamped. Use [IOSSpring] or [BouncySpring] for common configurations.
//
// See ExampleSpringSimulation for usage patterns.
type SpringDescription struct {
	// Stiffness controls how quickly the spring returns to rest (higher = faster).
	Stiffness float64
	// DampingRatio controls energy dissipation.
	// 1.0 = critically damped (no overshoot)
	// < 1.0 = underdamped (bouncy)
	// > 1.0 = overdamped (sluggish)
	DampingRatio float64
}

// IOSSpring returns spring parameters that approximate iOS scroll bounce.
func IOSSpring() SpringDescription {
	return SpringDescription{
		Stiffness:    500.0,
		DampingRatio: 1.0, // Critically damped for smooth return
	}
}

// BouncySpring returns slightly underdamped spring for a subtle bounce effect.
func BouncySpring() SpringDescription {
	return SpringDescription{
		Stiffness:    400.0,
		DampingRatio: 0.8,
	}
}

// SpringSimulation simulates a damped spring moving toward a target position.
//
// Unlike [AnimationController] which uses duration-based timing, SpringSimulation
// uses physics to determine motion. The simulation accounts for initial velocity,
// making it ideal for gesture-driven animations where the user "throws" an element.
//
// The simulation automatically handles numerical stability by sub-stepping
// large time deltas. See ExampleSpringSimulation for usage patterns.
type SpringSimulation struct {
	spring   SpringDescription
	target   float64
	position float64
	velocity float64
	omega    float64 // Natural frequency
	damping  float64 // Damping coefficient
}

// NewSpringSimulation creates a spring simulation from current position/velocity to target.
func NewSpringSimulation(spring SpringDescription, position, velocity, target float64) *SpringSimulation {
	omega := math.Sqrt(spring.Stiffness)
	damping := 2.0 * spring.DampingRatio * omega
	return &SpringSimulation{
		spring:   spring,
		target:   target,
		position: position,
		velocity: velocity,
		omega:    omega,
		damping:  damping,
	}
}

// Step advances the simulation by dt seconds.
// Returns true if the simulation has settled (is done).
func (s *SpringSimulation) Step(dt float64) bool {
	if dt <= 0 {
		return false
	}

	// Sub-step if dt is too large to maintain stability.
	// Stability requires dt * omega < 2, use 0.016 (60fps) as safe max step.
	const maxStep = 0.016
	for dt > 0 {
		step := dt
		if step > maxStep {
			step = maxStep
		}
		dt -= step

		if s.stepOnce(step) {
			return true
		}
	}
	return false
}

func (s *SpringSimulation) stepOnce(dt float64) bool {
	displacement := s.position - s.target

	// Spring force: F = -kx - cv (stiffness + damping)
	springForce := -s.spring.Stiffness * displacement
	dampingForce := -s.damping * s.velocity
	acceleration := springForce + dampingForce

	s.velocity += acceleration * dt
	s.position += s.velocity * dt

	// Check if settled (close to target with low velocity)
	// Use updated displacement to avoid oscillation when velocity is high near target
	newDisplacement := s.position - s.target
	if math.Abs(newDisplacement) < 0.5 && math.Abs(s.velocity) < 5 {
		s.position = s.target
		s.velocity = 0
		return true
	}

	return false
}

// Position returns the current position.
func (s *SpringSimulation) Position() float64 {
	return s.position
}

// Velocity returns the current velocity.
func (s *SpringSimulation) Velocity() float64 {
	return s.velocity
}

// Target returns the target position.
func (s *SpringSimulation) Target() float64 {
	return s.target
}

// IsDone returns true if the simulation has settled at the target.
func (s *SpringSimulation) IsDone() bool {
	displacement := s.position - s.target
	return math.Abs(displacement) < 0.5 && math.Abs(s.velocity) < 5
}
