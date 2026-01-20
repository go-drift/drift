// Package navigation provides routing and navigation for the Drift framework.
package navigation

import "github.com/go-drift/drift/pkg/core"

// RouteSettings contains route configuration.
type RouteSettings struct {
	// Name is the route name (e.g., "/home", "/details").
	Name string
	// Arguments contains any arguments passed to the route.
	Arguments any
}

// Route represents a screen in the navigation stack.
type Route interface {
	// Build creates the widget for this route.
	Build(ctx core.BuildContext) core.Widget

	// Settings returns the route configuration.
	Settings() RouteSettings

	// DidPush is called when the route is pushed onto the navigator.
	DidPush()

	// DidPop is called when the route is popped from the navigator.
	DidPop(result any)

	// DidChangeNext is called when the next route in the stack changes.
	DidChangeNext(nextRoute Route)

	// DidChangePrevious is called when the previous route in the stack changes.
	DidChangePrevious(previousRoute Route)

	// WillPop is called before the route is popped.
	// Return false to prevent the pop.
	WillPop() bool
}

// BaseRoute provides a default implementation of Route lifecycle methods.
type BaseRoute struct {
	settings RouteSettings
}

// NewBaseRoute creates a BaseRoute with the given settings.
func NewBaseRoute(settings RouteSettings) BaseRoute {
	return BaseRoute{settings: settings}
}

// Settings returns the route settings.
func (r *BaseRoute) Settings() RouteSettings {
	return r.settings
}

// DidPush is a no-op by default.
func (r *BaseRoute) DidPush() {}

// DidPop is a no-op by default.
func (r *BaseRoute) DidPop(result any) {}

// DidChangeNext is a no-op by default.
func (r *BaseRoute) DidChangeNext(nextRoute Route) {}

// DidChangePrevious is a no-op by default.
func (r *BaseRoute) DidChangePrevious(previousRoute Route) {}

// WillPop returns true by default, allowing the pop.
func (r *BaseRoute) WillPop() bool {
	return true
}
