package navigation

import (
	"reflect"
	"strings"

	"github.com/go-drift/drift/pkg/core"
)

// RouteConfigurer is the interface implemented by route configuration types.
// The two implementations are [RouteConfig] for regular routes and [ShellRoute]
// for persistent layouts that wrap child routes.
type RouteConfigurer interface {
	routeConfig() // marker method
}

// RouteConfig defines a single route in the declarative [Router].
//
// Path patterns support:
//   - Static segments: "/products", "/users/list"
//   - Parameters: "/products/:id", "/users/:userId/posts/:postId"
//   - Wildcards: "/files/*path" (captures remaining path)
//
// Example with nested routes:
//
//	navigation.RouteConfig{
//	    Path:    "/products",
//	    Builder: buildProductList,
//	    Routes: []navigation.RouteConfigurer{
//	        navigation.RouteConfig{
//	            Path:    "/:id",           // Matches /products/:id
//	            Builder: buildProductDetail,
//	        },
//	        navigation.RouteConfig{
//	            Path:    "/:id/reviews",   // Matches /products/:id/reviews
//	            Builder: buildProductReviews,
//	        },
//	    },
//	}
type RouteConfig struct {
	// Path is the URL pattern for this route.
	// Use :param for path parameters and *param for wildcards.
	// Nested routes inherit the parent's path as a prefix.
	Path string

	// Builder creates the widget for this route.
	// RouteSettings includes extracted Params and Query from the URL.
	Builder func(ctx core.BuildContext, settings RouteSettings) core.Widget

	// Redirect defines route-specific redirect logic.
	// Checked after the Router's global Redirect callback.
	// Use for route-specific access control.
	Redirect func(ctx RedirectContext) RedirectResult

	// Routes defines nested child routes.
	// Child paths are concatenated with this route's path.
	Routes []RouteConfigurer
}

func (RouteConfig) routeConfig() {}

// Router provides declarative route configuration with automatic path matching
// and parameter extraction.
//
// Router is the recommended approach for apps with URL-based routing. It
// internally creates a [Navigator] with IsRoot=true, so you don't need to
// manage navigator registration manually.
//
// IMPORTANT: Router is designed to be used as a singleton at the root of your
// app. Do not nest Routers or use Router inside [TabScaffold] tabs. For tabs
// with their own navigation stacks, use [Navigator] with [Tab.OnGenerateRoute]
// instead.
//
// Basic usage:
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
// With authentication:
//
//	navigation.Router{
//	    InitialPath:       "/",
//	    RefreshListenable: authState,
//	    Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	        if !authState.IsLoggedIn() && isProtected(ctx.ToPath) {
//	            return navigation.RedirectTo("/login")
//	        }
//	        return navigation.NoRedirect()
//	    },
//	    Routes: routes,
//	}
//
// Access the router for navigation using [RouterOf]:
//
//	router := navigation.RouterOf(ctx)
//	router.Go("/products/123", nil)
type Router struct {
	// Routes defines the route tree.
	// Use [RouteConfig] for regular routes and [ShellRoute] for persistent layouts.
	Routes []RouteConfigurer

	// Redirect is the global redirect callback, checked before every navigation.
	// Return [NoRedirect] to allow, or [RedirectTo]/[RedirectWithArgs] to redirect.
	// Route-specific redirects in [RouteConfig.Redirect] are checked after this.
	Redirect func(ctx RedirectContext) RedirectResult

	// ErrorBuilder creates a widget for unmatched routes (404 pages).
	// If nil, navigation to unknown routes is silently ignored.
	ErrorBuilder func(ctx core.BuildContext, settings RouteSettings) core.Widget

	// InitialPath is the starting route path.
	// Defaults to "/" if not specified.
	InitialPath string

	// TrailingSlashBehavior controls how trailing slashes are handled in matching.
	// Default is [TrailingSlashStrip] which treats "/path/" same as "/path".
	TrailingSlashBehavior TrailingSlashBehavior

	// CaseSensitivity controls case handling in path matching.
	// Default is [CaseSensitive] which requires exact case match.
	CaseSensitivity CaseSensitivity

	// RefreshListenable triggers redirect re-evaluation when notified.
	// Connect this to auth state changes to automatically redirect users
	// when they log in or out.
	RefreshListenable core.Listenable
}

// CreateElement returns a StatefulElement for this Router.
func (r Router) CreateElement() core.Element {
	return core.NewStatefulElement(r, nil)
}

// Key returns nil (no key).
func (r Router) Key() any {
	return nil
}

// CreateState creates the RouterState.
func (r Router) CreateState() core.State {
	return &routerState{}
}

// RouterState extends [NavigatorState] with path-based navigation methods.
//
// Obtain a RouterState using [RouterOf] from within the widget tree.
// RouterState embeds NavigatorState, so all standard navigation methods
// (Push, Pop, etc.) are available.
//
//	router := navigation.RouterOf(ctx)
//	router.Go("/products/123", nil)    // Navigate to path
//	router.Replace("/home", nil)       // Replace current route
//	router.Pop(nil)                    // Go back
type RouterState interface {
	NavigatorState // All NavigatorState methods are available

	// Go navigates to the given path, pushing a new route onto the stack.
	// Equivalent to PushNamed but with clearer intent for URL-based navigation.
	Go(path string, args any)

	// Replace replaces the current route with the given path.
	// The current route is removed and the new route takes its place.
	// Equivalent to PushReplacementNamed.
	Replace(path string, args any)
}

// routeIndex stores compiled route patterns for efficient lookup.
type routeIndex struct {
	patterns []*indexedRoute
}

type indexedRoute struct {
	pattern  *PathPattern
	config   RouteConfig
	fullPath string
	shells   []ShellRoute // Shell hierarchy from outermost to innermost
}

type routerState struct {
	element     *core.StatefulElement
	router      Router
	internalNav *navigatorState
	routeIndex  *routeIndex
}

func (s *routerState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *routerState) InitState() {
	s.router = s.element.Widget().(Router)
	s.routeIndex = s.buildRouteIndex()
}

func (s *routerState) buildRouteIndex() *routeIndex {
	index := &routeIndex{}
	s.indexRoutes("", nil, s.router.Routes, index)
	return index
}

func (s *routerState) indexRoutes(prefix string, shells []ShellRoute, routes []RouteConfigurer, index *routeIndex) {
	for _, rc := range routes {
		switch cfg := rc.(type) {
		case RouteConfig:
			fullPath := prefix + cfg.Path
			pattern := NewPathPattern(
				fullPath,
				WithTrailingSlash(s.router.TrailingSlashBehavior),
				WithCaseSensitivity(s.router.CaseSensitivity),
			)
			index.patterns = append(index.patterns, &indexedRoute{
				pattern:  pattern,
				config:   cfg,
				fullPath: fullPath,
				shells:   shells, // Capture current shell hierarchy
			})

			// Index nested routes (inherit same shells)
			if len(cfg.Routes) > 0 {
				s.indexRoutes(fullPath, shells, cfg.Routes, index)
			}

		case ShellRoute:
			// Shell routes add to the shell hierarchy for their children
			newShells := append([]ShellRoute{}, shells...)
			newShells = append(newShells, cfg)
			s.indexRoutes(prefix, newShells, cfg.Routes, index)
		}
	}
}

func (s *routerState) findRoute(path string) (*indexedRoute, RouteSettings) {
	// Extract query string but preserve path for pattern matching
	// (patterns handle trailing slash behavior themselves)
	_, query := ParsePath(path)

	// Get path portion without query/fragment for matching
	pathOnly := path
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		pathOnly = path[:idx]
	}

	for _, ir := range s.routeIndex.patterns {
		params, ok := ir.pattern.Match(pathOnly)
		if ok {
			return ir, RouteSettings{
				Name:   path,
				Params: params,
				Query:  query,
			}
		}
	}
	return nil, RouteSettings{}
}

func (s *routerState) generateRoute(settings RouteSettings) Route {
	ir, matchedSettings := s.findRoute(settings.Name)
	if ir == nil {
		return nil
	}

	// Merge arguments
	matchedSettings.Arguments = settings.Arguments

	// Capture for closure
	routeConfig := ir.config
	shells := ir.shells

	builder := func(ctx core.BuildContext) core.Widget {
		// Build the route's widget
		child := routeConfig.Builder(ctx, matchedSettings)

		// Wrap with shells from innermost to outermost
		// (shells slice is outermost-first, so iterate in reverse)
		for i := len(shells) - 1; i >= 0; i-- {
			shell := shells[i]
			child = shell.Builder(ctx, child)
		}

		return child
	}

	return NewMaterialPageRoute(builder, matchedSettings)
}

func (s *routerState) unknownRoute(settings RouteSettings) Route {
	if s.router.ErrorBuilder == nil {
		return nil
	}

	builder := func(ctx core.BuildContext) core.Widget {
		return s.router.ErrorBuilder(ctx, settings)
	}

	return NewMaterialPageRoute(builder, settings)
}

func (s *routerState) applyRedirect(ctx RedirectContext) RedirectResult {
	// First check router-level redirect
	if s.router.Redirect != nil {
		result := s.router.Redirect(ctx)
		if result.Path != "" {
			return result
		}
	}

	// Then check route-level redirect
	ir, _ := s.findRoute(ctx.ToPath)
	if ir != nil && ir.config.Redirect != nil {
		return ir.config.Redirect(ctx)
	}

	return NoRedirect()
}

func (s *routerState) Build(ctx core.BuildContext) core.Widget {
	initialPath := s.router.InitialPath
	if initialPath == "" {
		initialPath = "/"
	}

	// Build internal Navigator
	nav := Navigator{
		IsRoot:            true,
		InitialRoute:      initialPath,
		OnGenerateRoute:   s.generateRoute,
		OnUnknownRoute:    s.unknownRoute,
		Redirect:          s.applyRedirect,
		RefreshListenable: s.router.RefreshListenable,
	}

	// Wrap in inherited widget for RouterOf access
	return routerInherited{
		state: s,
		child: nav,
	}
}

func (s *routerState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *routerState) Dispose() {}

func (s *routerState) DidChangeDependencies() {}

func (s *routerState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	s.router = s.element.Widget().(Router)
	s.routeIndex = s.buildRouteIndex()
}

// NavigatorState interface implementation - delegate to RootNavigator

func (s *routerState) Push(route Route) {
	if nav := RootNavigator(); nav != nil {
		nav.Push(route)
	}
}

func (s *routerState) PushNamed(name string, args any) {
	if nav := RootNavigator(); nav != nil {
		nav.PushNamed(name, args)
	}
}

func (s *routerState) PushReplacementNamed(name string, args any) {
	if nav := RootNavigator(); nav != nil {
		nav.PushReplacementNamed(name, args)
	}
}

func (s *routerState) Pop(result any) {
	if nav := RootNavigator(); nav != nil {
		nav.Pop(result)
	}
}

func (s *routerState) PopUntil(predicate func(Route) bool) {
	if nav := RootNavigator(); nav != nil {
		nav.PopUntil(predicate)
	}
}

func (s *routerState) PushReplacement(route Route) {
	if nav := RootNavigator(); nav != nil {
		nav.PushReplacement(route)
	}
}

func (s *routerState) CanPop() bool {
	if nav := RootNavigator(); nav != nil {
		return nav.CanPop()
	}
	return false
}

func (s *routerState) MaybePop(result any) bool {
	if nav := RootNavigator(); nav != nil {
		return nav.MaybePop(result)
	}
	return false
}

// RouterState-specific methods

// Go navigates to the given path.
func (s *routerState) Go(path string, args any) {
	s.PushNamed(path, args)
}

// Replace replaces the current route with the given path.
func (s *routerState) Replace(path string, args any) {
	s.PushReplacementNamed(path, args)
}

// routerInherited provides RouterState to descendants.
type routerInherited struct {
	state *routerState
	child core.Widget
}

func (r routerInherited) CreateElement() core.Element {
	return core.NewInheritedElement(r, nil)
}

func (r routerInherited) Key() any {
	return nil
}

func (r routerInherited) ChildWidget() core.Widget {
	return r.child
}

func (r routerInherited) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(routerInherited); ok {
		return r.state != old.state
	}
	return true
}

func (r routerInherited) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return r.UpdateShouldNotify(oldWidget)
}

var routerInheritedType = reflect.TypeOf(routerInherited{})

// RouterOf returns the [RouterState] from the nearest [Router] ancestor.
//
// Use this for navigation from within the widget tree when you need the
// Router's path-based methods (Go, Replace). Returns nil if no Router
// ancestor exists.
//
//	func handleProductTap(ctx core.BuildContext, productID string) {
//	    if router := navigation.RouterOf(ctx); router != nil {
//	        router.Go("/products/"+productID, nil)
//	    }
//	}
//
// RouterState embeds [NavigatorState], so you can also use standard methods:
//
//	router := navigation.RouterOf(ctx)
//	router.Pop(nil)  // Go back
func RouterOf(ctx core.BuildContext) RouterState {
	inherited := ctx.DependOnInherited(routerInheritedType, nil)
	if inherited == nil {
		return nil
	}
	if router, ok := inherited.(routerInherited); ok {
		return router.state
	}
	return nil
}
