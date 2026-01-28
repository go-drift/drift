package animation_test

import (
	"fmt"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/graphics"
)

// This example shows how to create and control an animation.
func ExampleAnimationController() {
	controller := animation.NewAnimationController(300 * time.Millisecond)
	controller.Curve = animation.EaseOut

	// Listen for value changes
	controller.AddListener(func() {
		fmt.Printf("Value: %.2f\n", controller.Value)
	})

	// Animate forward (0 -> 1)
	controller.Forward()

	// Later, animate in reverse (1 -> 0)
	controller.Reverse()

	// Clean up when done
	controller.Dispose()
}

// This example shows how to use tweens with an animation controller.
func ExampleAnimationController_withTween() {
	controller := animation.NewAnimationController(500 * time.Millisecond)

	// Create tweens to map 0-1 range to other values
	sizeTween := animation.TweenFloat64(100, 200)
	colorTween := animation.TweenColor(
		graphics.RGB(255, 0, 0),   // red
		graphics.RGB(0, 0, 255),   // blue
	)

	controller.AddListener(func() {
		size := sizeTween.Transform(controller)
		color := colorTween.Transform(controller)
		_ = size
		_ = color
	})

	controller.Forward()
	controller.Dispose()
}

// This example shows how to listen for animation status changes.
func ExampleAnimationController_statusListener() {
	controller := animation.NewAnimationController(300 * time.Millisecond)

	controller.AddStatusListener(func(status animation.AnimationStatus) {
		switch status {
		case animation.AnimationDismissed:
			fmt.Println("Animation at start (0)")
		case animation.AnimationForward:
			fmt.Println("Animating forward")
		case animation.AnimationReverse:
			fmt.Println("Animating in reverse")
		case animation.AnimationCompleted:
			fmt.Println("Animation completed (1)")
		}
	})

	controller.Forward()
	controller.Dispose()
}

// This example shows how to create a tween for basic interpolation.
func ExampleTween() {
	// Create tweens for different value types
	opacity := animation.TweenFloat64(0.0, 1.0)
	position := animation.TweenOffset(
		graphics.Offset{X: 0, Y: 0},
		graphics.Offset{X: 100, Y: 50},
	)

	// Evaluate at different progress values
	fmt.Printf("Opacity at 0.5: %.1f\n", opacity.Evaluate(0.5))
	fmt.Printf("Position at 1.0: (%.0f, %.0f)\n", position.Evaluate(1.0).X, position.Evaluate(1.0).Y)

	// Output:
	// Opacity at 0.5: 0.5
	// Position at 1.0: (100, 50)
}

// This example shows how to create a custom tween with a Lerp function.
func ExampleTween_customType() {
	type Point struct {
		X, Y float64
	}

	pointTween := &animation.Tween[Point]{
		Begin: Point{0, 0},
		End:   Point{100, 200},
		Lerp: func(a, b Point, t float64) Point {
			return Point{
				X: a.X + (b.X-a.X)*t,
				Y: a.Y + (b.Y-a.Y)*t,
			}
		},
	}

	midpoint := pointTween.Evaluate(0.5)
	fmt.Printf("Midpoint: (%.0f, %.0f)\n", midpoint.X, midpoint.Y)

	// Output:
	// Midpoint: (50, 100)
}

// This example shows how to use spring physics for natural motion.
func ExampleSpringSimulation() {
	// Create a bouncy spring simulation
	spring := animation.BouncySpring()
	sim := animation.NewSpringSimulation(
		spring,
		0,     // current position
		500,   // initial velocity (e.g., from a fling gesture)
		300,   // target position
	)

	// Step the simulation (typically done each frame)
	dt := 0.016 // ~60fps
	for !sim.IsDone() {
		done := sim.Step(dt)
		_ = sim.Position()
		_ = sim.Velocity()
		if done {
			break
		}
	}

	fmt.Printf("Final position: %.0f\n", sim.Position())

	// Output:
	// Final position: 300
}

// This example shows how to create a custom easing curve.
func ExampleCubicBezier() {
	// Create a custom curve matching CSS cubic-bezier(0.4, 0.0, 0.2, 1.0)
	customEase := animation.CubicBezier(0.4, 0.0, 0.2, 1.0)

	// The curve transforms linear progress to eased progress
	fmt.Printf("Progress 0.0 -> %.2f\n", customEase(0.0))
	fmt.Printf("Progress 0.5 -> %.2f\n", customEase(0.5))
	fmt.Printf("Progress 1.0 -> %.2f\n", customEase(1.0))

	// Output:
	// Progress 0.0 -> 0.00
	// Progress 0.5 -> 0.78
	// Progress 1.0 -> 1.00
}
