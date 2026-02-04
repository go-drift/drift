// Package navigation provides routing and navigation for the Drift framework.
//
// The package offers two approaches to navigation:
//
// # Imperative Navigation with Navigator
//
// Use [Navigator] for imperative, stack-based navigation where you manually
// define route generation:
//
//	navigation.Navigator{
//	    InitialRoute: "/",
//	    IsRoot:       true,
//	    OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
//	        switch settings.Name {
//	        case "/":
//	            return navigation.NewMaterialPageRoute(buildHome, settings)
//	        case "/details":
//	            return navigation.NewMaterialPageRoute(buildDetails, settings)
//	        }
//	        return nil
//	    },
//	}
//
// # Declarative Navigation with Router
//
// Use [Router] for declarative route configuration with automatic path parameter
// extraction:
//
//	navigation.Router{
//	    InitialPath: "/",
//	    Routes: []navigation.RouteConfigurer{
//	        navigation.RouteConfig{Path: "/", Builder: buildHome},
//	        navigation.RouteConfig{Path: "/products/:id", Builder: buildProduct},
//	    },
//	    ErrorBuilder: build404Page,
//	}
//
// # Accessing Navigation
//
// From within the widget tree, use [NavigatorOf] or [RouterOf]:
//
//	nav := navigation.NavigatorOf(ctx)
//	nav.PushNamed("/details", args)
//
// From outside the widget tree (deep links, platform callbacks), use [RootNavigator]:
//
//	nav := navigation.RootNavigator()
//	nav.PushNamed("/details", args)
//
// # Route Guards
//
// Both Navigator and Router support redirect callbacks for authentication
// and authorization:
//
//	Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	    if !isLoggedIn && strings.HasPrefix(ctx.ToPath, "/protected") {
//	        return navigation.RedirectTo("/login")
//	    }
//	    return navigation.NoRedirect()
//	},
//
// # Tab Navigation
//
// Use [TabScaffold] for bottom tab navigation with separate navigation stacks
// per tab. TabScaffold automatically manages which tab's navigator is active
// for back button handling.
package navigation

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/overlay"
)

// OverlayState is an alias for overlay.OverlayState for convenience.
type OverlayState = overlay.OverlayState

// RouteSettings contains configuration and parameters for a route.
//
// When using the declarative [Router], Params and Query are automatically
// populated from the URL. When using [Navigator] directly, you can populate
// these fields manually in OnGenerateRoute.
type RouteSettings struct {
	// Name is the route path (e.g., "/home", "/products/123").
	Name string

	// Arguments contains arbitrary data passed during navigation.
	// Use this for complex objects that don't fit in the URL.
	Arguments any

	// Params contains path parameters extracted from the URL.
	// For example, "/products/:id" matching "/products/123" yields {"id": "123"}.
	// Values are automatically percent-decoded.
	Params map[string]string

	// Query contains query string parameters from the URL.
	// Supports multiple values per key (e.g., "?tag=a&tag=b").
	// Values are automatically percent-decoded.
	Query map[string][]string
}

// Param returns a path parameter value or empty string if not found.
func (s RouteSettings) Param(key string) string {
	if s.Params == nil {
		return ""
	}
	return s.Params[key]
}

// QueryValue returns the first query parameter value or empty string if not found.
func (s RouteSettings) QueryValue(key string) string {
	if s.Query == nil {
		return ""
	}
	if vals, ok := s.Query[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// QueryValues returns all query parameter values for a key.
func (s RouteSettings) QueryValues(key string) []string {
	if s.Query == nil {
		return nil
	}
	return s.Query[key]
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

	// SetOverlay is called by Navigator when OverlayState becomes available.
	// Routes that use overlay entries store this reference.
	SetOverlay(overlay OverlayState)
}

// TransparentRoute is implemented by routes that should keep previous routes visible.
// Routes like bottom sheets and dialogs that have semi-transparent barriers
// should implement this and return true from IsTransparent.
type TransparentRoute interface {
	Route
	// IsTransparent returns true if previous routes should remain visible.
	IsTransparent() bool
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

// SetOverlay is a no-op for non-modal routes.
func (r *BaseRoute) SetOverlay(overlay OverlayState) {}
