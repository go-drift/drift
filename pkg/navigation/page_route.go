package navigation

import (
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
)

// TransitionDuration is the default duration for page transitions.
const TransitionDuration = 350 * time.Millisecond

// MaterialPageRoute provides a route with Material Design page transitions.
type MaterialPageRoute struct {
	BaseRoute

	// Builder creates the page content.
	Builder func(ctx core.BuildContext) core.Widget

	// animation controller for this route's transition
	controller *animation.AnimationController

	// isInitialRoute tracks if this is the first route (no animation needed)
	isInitialRoute bool
}

// NewMaterialPageRoute creates a MaterialPageRoute with the given builder and settings.
func NewMaterialPageRoute(builder func(core.BuildContext) core.Widget, settings RouteSettings) *MaterialPageRoute {
	return &MaterialPageRoute{
		BaseRoute: NewBaseRoute(settings),
		Builder:   builder,
	}
}

// Build returns the page content wrapped in a transition.
func (m *MaterialPageRoute) Build(ctx core.BuildContext) core.Widget {
	if m.Builder == nil {
		return nil
	}

	content := m.Builder(ctx)

	// Wrap in slide transition if we have an animation
	if m.controller != nil {
		return SlideTransition{
			Animation: m.controller,
			Direction: SlideFromRight,
			Child:     content,
		}
	}

	return content
}

// DidPush is called when the route is pushed.
func (m *MaterialPageRoute) DidPush() {
	// Only animate if not the initial route
	if !m.isInitialRoute {
		m.controller = animation.NewAnimationController(TransitionDuration)
		m.controller.Curve = animation.IOSNavigationCurve
		m.controller.Forward()
	}
}

// SetInitialRoute marks this as the initial route (no animation).
func (m *MaterialPageRoute) SetInitialRoute() {
	m.isInitialRoute = true
}

// DidPop is called when the route is popped.
func (m *MaterialPageRoute) DidPop(result any) {
	// Reverse the animation (in a full implementation, we'd wait for it to complete)
	if m.controller != nil {
		m.controller.Reverse()
	}
}

// PageRoute is a simpler route without transitions.
type PageRoute struct {
	BaseRoute

	// Builder creates the page content.
	Builder func(ctx core.BuildContext) core.Widget
}

// NewPageRoute creates a PageRoute with the given builder and settings.
func NewPageRoute(builder func(core.BuildContext) core.Widget, settings RouteSettings) *PageRoute {
	return &PageRoute{
		BaseRoute: NewBaseRoute(settings),
		Builder:   builder,
	}
}

// Build returns the page content.
func (p *PageRoute) Build(ctx core.BuildContext) core.Widget {
	if p.Builder == nil {
		return nil
	}
	return p.Builder(ctx)
}
