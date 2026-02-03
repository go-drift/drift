package navigation

import (
	"testing"

	"github.com/go-drift/drift/pkg/core"
)

func TestRouter_RouteIndex_Basic(t *testing.T) {
	// Create a router state and manually build the index
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/"},
			RouteConfig{Path: "/products"},
			RouteConfig{Path: "/settings"},
		},
	}

	state := &routerState{router: router}
	index := state.buildRouteIndex()

	if len(index.patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(index.patterns))
	}

	// Verify paths are indexed correctly
	paths := []string{"/", "/products", "/settings"}
	for i, ir := range index.patterns {
		if ir.fullPath != paths[i] {
			t.Errorf("Pattern %d: expected path %q, got %q", i, paths[i], ir.fullPath)
		}
	}
}

func TestRouter_RouteIndex_NestedRoutes(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/"},
			RouteConfig{
				Path: "/products",
				Routes: []RouteConfigurer{
					RouteConfig{Path: "/:id"},
					RouteConfig{Path: "/:id/reviews"},
				},
			},
		},
	}

	state := &routerState{router: router}
	index := state.buildRouteIndex()

	if len(index.patterns) != 4 {
		t.Errorf("Expected 4 patterns (root + products + 2 nested), got %d", len(index.patterns))
	}

	// Check nested paths are correctly concatenated
	expectedPaths := map[string]bool{
		"/":                     true,
		"/products":             true,
		"/products/:id":         true,
		"/products/:id/reviews": true,
	}

	for _, ir := range index.patterns {
		if !expectedPaths[ir.fullPath] {
			t.Errorf("Unexpected path in index: %q", ir.fullPath)
		}
		delete(expectedPaths, ir.fullPath)
	}

	if len(expectedPaths) > 0 {
		t.Errorf("Missing paths: %v", expectedPaths)
	}
}

func TestRouter_RouteIndex_ShellRoute(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/login"},
			ShellRoute{
				Routes: []RouteConfigurer{
					RouteConfig{Path: "/home"},
					RouteConfig{Path: "/profile"},
				},
			},
		},
	}

	state := &routerState{router: router}
	index := state.buildRouteIndex()

	// ShellRoute doesn't add patterns, just its children
	if len(index.patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(index.patterns))
	}

	expectedPaths := map[string]bool{
		"/login":   true,
		"/home":    true,
		"/profile": true,
	}

	for _, ir := range index.patterns {
		if !expectedPaths[ir.fullPath] {
			t.Errorf("Unexpected path: %q", ir.fullPath)
		}
	}
}

func TestRouter_FindRoute_Basic(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/"},
			RouteConfig{Path: "/products"},
			RouteConfig{Path: "/products/:id"},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	tests := []struct {
		path       string
		wantMatch  bool
		wantParams map[string]string
	}{
		{"/", true, nil},
		{"/products", true, nil},
		{"/products/123", true, map[string]string{"id": "123"}},
		{"/unknown", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ir, settings := state.findRoute(tt.path)
			gotMatch := ir != nil

			if gotMatch != tt.wantMatch {
				t.Errorf("findRoute(%q) match = %v, want %v", tt.path, gotMatch, tt.wantMatch)
			}

			if tt.wantMatch && tt.wantParams != nil {
				for k, v := range tt.wantParams {
					if settings.Params[k] != v {
						t.Errorf("findRoute(%q) param[%s] = %q, want %q", tt.path, k, settings.Params[k], v)
					}
				}
			}
		})
	}
}

func TestRouter_FindRoute_WithQuery(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/search"},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	ir, settings := state.findRoute("/search?q=hello&page=2")
	if ir == nil {
		t.Fatal("Expected to find /search route")
	}

	if settings.QueryValue("q") != "hello" {
		t.Errorf("QueryValue(q) = %q, want hello", settings.QueryValue("q"))
	}
	if settings.QueryValue("page") != "2" {
		t.Errorf("QueryValue(page) = %q, want 2", settings.QueryValue("page"))
	}
}

func TestRouter_FindRoute_TrailingSlash(t *testing.T) {
	// Default behavior: strip trailing slash
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/products/:id"},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	ir, _ := state.findRoute("/products/123/")
	if ir == nil {
		t.Error("Should match /products/123/ with trailing slash strip")
	}

	// Strict mode
	routerStrict := Router{
		TrailingSlashBehavior: TrailingSlashStrict,
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/products/:id"},
		},
	}

	stateStrict := &routerState{router: routerStrict}
	stateStrict.routeIndex = stateStrict.buildRouteIndex()

	ir, _ = stateStrict.findRoute("/products/123/")
	if ir != nil {
		t.Error("Should not match /products/123/ with trailing slash strict")
	}

	ir, _ = stateStrict.findRoute("/products/123")
	if ir == nil {
		t.Error("Should match /products/123 without trailing slash")
	}
}

func TestRouter_FindRoute_CaseSensitivity(t *testing.T) {
	// Default: case sensitive
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/Products"},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	ir, _ := state.findRoute("/products")
	if ir != nil {
		t.Error("Case sensitive: should not match /products for /Products")
	}

	ir, _ = state.findRoute("/Products")
	if ir == nil {
		t.Error("Case sensitive: should match /Products")
	}

	// Case insensitive
	routerInsensitive := Router{
		CaseSensitivity: CaseInsensitive,
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/Products"},
		},
	}

	stateInsensitive := &routerState{router: routerInsensitive}
	stateInsensitive.routeIndex = stateInsensitive.buildRouteIndex()

	ir, _ = stateInsensitive.findRoute("/products")
	if ir == nil {
		t.Error("Case insensitive: should match /products")
	}
}

func TestRouter_ApplyRedirect_GlobalRedirect(t *testing.T) {
	router := Router{
		Redirect: func(ctx RedirectContext) RedirectResult {
			if ctx.ToPath == "/protected" {
				return RedirectTo("/login")
			}
			return NoRedirect()
		},
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/"},
			RouteConfig{Path: "/login"},
			RouteConfig{Path: "/protected"},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Protected route should redirect
	result := state.applyRedirect(RedirectContext{ToPath: "/protected"})
	if result.Path != "/login" {
		t.Errorf("Expected redirect to /login, got %q", result.Path)
	}

	// Non-protected route should not redirect
	result = state.applyRedirect(RedirectContext{ToPath: "/"})
	if result.Path != "" {
		t.Errorf("Expected no redirect, got %q", result.Path)
	}
}

func TestRouter_ApplyRedirect_RouteLevel(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/"},
			RouteConfig{
				Path: "/admin",
				Redirect: func(ctx RedirectContext) RedirectResult {
					return RedirectTo("/login")
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// /admin has route-level redirect
	result := state.applyRedirect(RedirectContext{ToPath: "/admin"})
	if result.Path != "/login" {
		t.Errorf("Expected redirect to /login, got %q", result.Path)
	}

	// / has no redirect
	result = state.applyRedirect(RedirectContext{ToPath: "/"})
	if result.Path != "" {
		t.Errorf("Expected no redirect, got %q", result.Path)
	}
}

func TestRouter_ApplyRedirect_GlobalOverridesRoute(t *testing.T) {
	router := Router{
		Redirect: func(ctx RedirectContext) RedirectResult {
			// Global redirect always sends to /maintenance
			return RedirectTo("/maintenance")
		},
		Routes: []RouteConfigurer{
			RouteConfig{
				Path: "/admin",
				Redirect: func(ctx RedirectContext) RedirectResult {
					return RedirectTo("/login")
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Global redirect takes precedence
	result := state.applyRedirect(RedirectContext{ToPath: "/admin"})
	if result.Path != "/maintenance" {
		t.Errorf("Expected global redirect to /maintenance, got %q", result.Path)
	}
}

func TestRouter_DeeplyNestedRoutes(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			RouteConfig{
				Path: "/api",
				Routes: []RouteConfigurer{
					RouteConfig{
						Path: "/v1",
						Routes: []RouteConfigurer{
							RouteConfig{Path: "/users"},
							RouteConfig{Path: "/users/:id"},
						},
					},
					RouteConfig{Path: "/v2"},
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	tests := []struct {
		path      string
		wantMatch bool
	}{
		{"/api", true},
		{"/api/v1", true},
		{"/api/v1/users", true},
		{"/api/v1/users/42", true},
		{"/api/v2", true},
		{"/api/v3", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ir, _ := state.findRoute(tt.path)
			gotMatch := ir != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("findRoute(%q) = %v, want %v", tt.path, gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestRouter_ShellRoute_ShellsTracked(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			ShellRoute{
				Builder: func(ctx core.BuildContext, child core.Widget) core.Widget {
					return child // Simplified for test
				},
				Routes: []RouteConfigurer{
					RouteConfig{Path: "/home"},
					RouteConfig{Path: "/profile"},
				},
			},
			RouteConfig{Path: "/login"}, // Outside shell
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Routes inside shell should have shell tracked
	ir, _ := state.findRoute("/home")
	if ir == nil {
		t.Fatal("Should find /home")
	}
	if len(ir.shells) != 1 {
		t.Errorf("/home should have 1 shell, got %d", len(ir.shells))
	}

	ir, _ = state.findRoute("/profile")
	if ir == nil {
		t.Fatal("Should find /profile")
	}
	if len(ir.shells) != 1 {
		t.Errorf("/profile should have 1 shell, got %d", len(ir.shells))
	}

	// Route outside shell should have no shells
	ir, _ = state.findRoute("/login")
	if ir == nil {
		t.Fatal("Should find /login")
	}
	if len(ir.shells) != 0 {
		t.Errorf("/login should have 0 shells, got %d", len(ir.shells))
	}
}

func TestRouter_ShellRoute_NestedShells(t *testing.T) {
	router := Router{
		Routes: []RouteConfigurer{
			ShellRoute{
				Builder: func(ctx core.BuildContext, child core.Widget) core.Widget {
					return child // Outer shell
				},
				Routes: []RouteConfigurer{
					ShellRoute{
						Builder: func(ctx core.BuildContext, child core.Widget) core.Widget {
							return child // Inner shell
						},
						Routes: []RouteConfigurer{
							RouteConfig{Path: "/nested"},
						},
					},
					RouteConfig{Path: "/single-shell"},
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Route in nested shells should have both shells
	ir, _ := state.findRoute("/nested")
	if ir == nil {
		t.Fatal("Should find /nested")
	}
	if len(ir.shells) != 2 {
		t.Errorf("/nested should have 2 shells, got %d", len(ir.shells))
	}

	// Route in single shell should have 1 shell
	ir, _ = state.findRoute("/single-shell")
	if ir == nil {
		t.Fatal("Should find /single-shell")
	}
	if len(ir.shells) != 1 {
		t.Errorf("/single-shell should have 1 shell, got %d", len(ir.shells))
	}
}

func TestRouter_TrailingSlash_RequiresTrailingSlash(t *testing.T) {
	// In strict mode, pattern ending with / should require trailing slash
	router := Router{
		TrailingSlashBehavior: TrailingSlashStrict,
		Routes: []RouteConfigurer{
			RouteConfig{Path: "/products/"}, // Requires trailing slash
			RouteConfig{Path: "/users"},     // No trailing slash
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// /products/ pattern should match /products/ but not /products
	ir, _ := state.findRoute("/products/")
	if ir == nil {
		t.Error("Should match /products/ with trailing slash")
	}

	ir, _ = state.findRoute("/products")
	if ir != nil {
		t.Error("Should NOT match /products without trailing slash when pattern has it")
	}

	// /users pattern should match /users but not /users/
	ir, _ = state.findRoute("/users")
	if ir == nil {
		t.Error("Should match /users without trailing slash")
	}

	ir, _ = state.findRoute("/users/")
	if ir != nil {
		t.Error("Should NOT match /users/ with trailing slash when pattern doesn't have it")
	}
}
