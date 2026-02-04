---
id: navigation
title: Navigation
sidebar_position: 9
---

# Navigation

Drift provides stack-based navigation with support for named routes, route guards, path parameters, deep linking, and tab navigation.

## Setting Up Routes

Use a Navigator with route generation:

```go
func App() core.Widget {
    return navigation.Navigator{
        InitialRoute: "/",
        IsRoot:       true, // Mark as root navigator for back button handling
        OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
            switch settings.Name {
            case "/":
                return navigation.NewMaterialPageRoute(buildHome, settings)
            case "/details":
                return navigation.NewMaterialPageRoute(buildDetails, settings)
            case "/settings":
                return navigation.NewMaterialPageRoute(buildSettings, settings)
            default:
                return nil
            }
        },
    }
}
```

### Navigator Ownership

When using multiple navigators (e.g., with TabScaffold), set `IsRoot: true` on your main navigator. This registers it as the root navigator for back button handling and deep links. Tab navigators automatically register with TabScaffold and become active when their tab is selected.

## Route Builders

Route builders receive `BuildContext` and return a widget:

```go
func buildHome(ctx core.BuildContext) core.Widget {
    return HomePage{}
}

func buildDetails(ctx core.BuildContext) core.Widget {
    return DetailsPage{}
}
```

## Navigating

Get the navigator from context and call navigation methods:

```go
func handleTap(ctx core.BuildContext) {
    nav := navigation.NavigatorOf(ctx)
    if nav == nil {
        return
    }

    // Push a named route
    nav.PushNamed("/details", nil)
}
```

### Navigation Methods

| Method | Description |
|--------|-------------|
| `Push(route)` | Push a route onto the stack |
| `PushNamed(name, args)` | Push a named route with arguments |
| `Pop(result)` | Pop the current route, optionally with a result |
| `CanPop()` | Check if there's a route to pop |
| `MaybePop(result)` | Pop if possible, otherwise do nothing |
| `PopUntil(predicate)` | Pop routes until predicate returns true |

### Example Navigation Flow

```go
// Push to details
nav.PushNamed("/details", nil)

// Go back
nav.Pop(nil)

// Only go back if possible
if nav.CanPop() {
    nav.Pop(nil)
}

// Pop to root
nav.PopUntil(func(route navigation.Route) bool {
    return route.Settings().Name == "/"
})
```

## Passing Data

Pass arguments when navigating:

```go
// Navigate with arguments
nav.PushNamed("/details", map[string]any{
    "id":    123,
    "title": "Item Title",
})
```

Access arguments in the route builder via route settings:

```go
func App() core.Widget {
    return navigation.Navigator{
        InitialRoute: "/",
        OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
            switch settings.Name {
            case "/":
                return navigation.NewMaterialPageRoute(buildHome, settings)
            case "/details":
                // Access arguments from settings
                return navigation.NewMaterialPageRoute(func(ctx core.BuildContext) core.Widget {
                    args := settings.Arguments.(map[string]any)
                    id := args["id"].(int)
                    title := args["title"].(string)
                    return DetailsPage{ID: id, Title: title}
                }, settings)
            default:
                return nil
            }
        },
    }
}
```

## Returning Results

Return data when popping a route:

```go
// In the destination screen - pop with result
nav.Pop("selected_item_id")
```

## Modal Bottom Sheets

Use `ShowModalBottomSheet` to present a bottom sheet and await a result.

```go
result := <-navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
    return widgets.Padding{
        Padding: layout.EdgeInsetsAll(24),
        Child: widgets.Column{
            MainAxisSize: widgets.MainAxisSizeMin,
            Children: []core.Widget{
                widgets.Text{Content: "Select an option"},
                theme.ButtonOf(ctx, "Option A", func() {
                    widgets.BottomSheetScope{}.Of(ctx).Close("A")
                }),
            },
        },
    }
})
```

### Snap Points and Drag Modes

```go
navigation.ShowModalBottomSheet(
    ctx,
    func(ctx core.BuildContext) core.Widget { return sheetContent() },
    navigation.WithSnapPoints(widgets.SnapHalf, widgets.SnapFull),
    navigation.WithInitialSnapPoint(0),
    navigation.WithDragMode(widgets.DragModeContentAware),
)
```

### Scrollable Content

For scrollable content, wrap the list in `BottomSheetScrollable` so dragging and
scrolling are coordinated:

```go
navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
    return widgets.BottomSheetScrollable{
        Builder: func(controller *widgets.ScrollController) core.Widget {
            return widgets.ListView{
                Controller: controller,
                Children:   items,
            }
        },
    }
})
```

Handle results by using a callback pattern:

```go
// Selection screen that returns a result
type SelectionScreen struct {
    OnSelect func(item string)
}

func (s SelectionScreen) Build(ctx core.BuildContext) core.Widget {
    return widgets.ListView{
        Children: []core.Widget{
            widgets.Tap(func() {
                s.OnSelect("item_1")
                navigation.NavigatorOf(ctx).Pop(nil)
            }, widgets.Text{Content: "Item 1"}),
            // ...
        },
    }
}
```

## Unknown Routes

Handle navigation to undefined routes:

```go
navigation.Navigator{
    InitialRoute: "/",
    OnGenerateRoute: generateRoute,
    OnUnknownRoute: func(settings navigation.RouteSettings) navigation.Route {
        return navigation.NewMaterialPageRoute(
            func(ctx core.BuildContext) core.Widget {
                return widgets.Column{
                    Children: []core.Widget{
                        widgets.Text{Content: "Page not found"},
                        widgets.Text{Content: settings.Name},
                        theme.ButtonOf(ctx, "Go Home", func() {
                            navigation.NavigatorOf(ctx).PopUntil(func(r navigation.Route) bool {
                                return r.Settings().Name == "/"
                            })
                        }),
                    },
                }
            },
            settings,
        )
    },
}
```

## Route Guards

Use the `Redirect` callback to implement authentication guards and route protection:

```go
// Auth state controller
type AuthState struct {
    core.ControllerBase
    isLoggedIn bool
}

func (a *AuthState) SetLoggedIn(loggedIn bool) {
    a.isLoggedIn = loggedIn
    a.NotifyListeners() // Triggers redirect re-evaluation
}

var authState = &AuthState{}

func App() core.Widget {
    return navigation.Navigator{
        InitialRoute:      "/",
        IsRoot:            true,
        RefreshListenable: authState, // Re-evaluate redirects when auth changes
        Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
            isProtected := strings.HasPrefix(ctx.ToPath, "/dashboard") ||
                          strings.HasPrefix(ctx.ToPath, "/settings")

            if isProtected && !authState.isLoggedIn {
                // Redirect to login, preserving the intended destination
                return navigation.RedirectWithArgs("/login", map[string]any{
                    "returnTo": ctx.ToPath,
                })
            }

            if ctx.ToPath == "/login" && authState.isLoggedIn {
                // Already logged in, go to dashboard
                return navigation.RedirectTo("/dashboard")
            }

            return navigation.NoRedirect()
        },
        OnGenerateRoute: generateRoute,
    }
}
```

### Redirect Helpers

| Function | Description |
|----------|-------------|
| `NoRedirect()` | Allow navigation to proceed normally |
| `RedirectTo(path)` | Redirect to a different path (replaces current route) |
| `RedirectWithArgs(path, args)` | Redirect with arguments preserved |

### RefreshListenable

The `RefreshListenable` field accepts any `core.Listenable`. When the listenable notifies, the navigator re-evaluates whether the current route should be redirected. This is useful for:

- Auth state changes (logout redirects to login)
- Permission changes (user loses access to a route)
- Feature flags (route becomes unavailable)

## Path Parameters

Extract dynamic values from URL paths using RouteSettings:

```go
// Route settings now include Params and Query
type RouteSettings struct {
    Name      string
    Arguments any
    Params    map[string]string    // Path params like {"id": "123"}
    Query     map[string][]string  // Query params
}

// Convenience methods
settings.Param("id")           // Get path parameter
settings.QueryValue("search")  // Get first query value
settings.QueryValues("tags")   // Get all query values
```

### Using PathPattern

For manual path matching with parameters:

```go
pattern := navigation.NewPathPattern("/products/:id")

params, ok := pattern.Match("/products/123")
// params = {"id": "123"}, ok = true

params, ok = pattern.Match("/products/hello%20world")
// params = {"id": "hello world"}, ok = true (percent-decoded)
```

### Wildcard Parameters

Capture the rest of the path:

```go
pattern := navigation.NewPathPattern("/files/*path")

params, ok := pattern.Match("/files/docs/readme.md")
// params = {"path": "docs/readme.md"}, ok = true
```

### Path Matching Options

```go
// Case-insensitive matching
pattern := navigation.NewPathPattern("/Products/:id",
    navigation.WithCaseSensitivity(navigation.CaseInsensitive),
)

// Strict trailing slash
pattern := navigation.NewPathPattern("/products/:id",
    navigation.WithTrailingSlash(navigation.TrailingSlashStrict),
)
```

### Parsing URLs

Parse full URLs with query strings:

```go
path, query := navigation.ParsePath("/search?q=drift&page=2")
// path = "/search"
// query = {"q": ["drift"], "page": ["2"]}
```

## Declarative Router

For larger applications, use the declarative `Router` API for cleaner route configuration:

```go
func App() core.Widget {
    return navigation.Router{
        InitialPath: "/",
        Routes: []navigation.RouteConfigurer{
            navigation.RouteConfig{
                Path:    "/",
                Builder: buildHome,
            },
            navigation.RouteConfig{
                Path:    "/products",
                Builder: buildProductList,
                Routes: []navigation.RouteConfigurer{
                    navigation.RouteConfig{
                        Path:    "/:id", // Nested: /products/:id
                        Builder: buildProductDetail,
                    },
                },
            },
            navigation.RouteConfig{
                Path:    "/settings",
                Builder: buildSettings,
            },
        },
        ErrorBuilder: buildNotFound,
        Redirect:     authRedirect,
    }
}

func buildProductDetail(ctx core.BuildContext, settings navigation.RouteSettings) core.Widget {
    productID := settings.Param("id")
    return ProductDetailPage{ID: productID}
}
```

### RouterState

Access the router for path-based navigation:

```go
func handleTap(ctx core.BuildContext) {
    router := navigation.RouterOf(ctx)
    if router == nil {
        return
    }

    // Navigate to a path
    router.Go("/products/123", nil)

    // Replace current route
    router.Replace("/settings", nil)
}
```

### Shell Routes

Wrap routes in a persistent layout (tabs, sidebars, etc.):

```go
navigation.Router{
    Routes: []navigation.RouteConfigurer{
        navigation.ShellRoute{
            Builder: func(ctx core.BuildContext, child core.Widget) core.Widget {
                return widgets.Column{
                    Children: []core.Widget{
                        MyNavigationBar{},
                        widgets.Expanded{Child: child},
                    },
                }
            },
            Routes: []navigation.RouteConfigurer{
                navigation.RouteConfig{Path: "/home", Builder: buildHome},
                navigation.RouteConfig{Path: "/profile", Builder: buildProfile},
            },
        },
        // Routes outside the shell
        navigation.RouteConfig{Path: "/login", Builder: buildLogin},
    },
}
```

## Deep Linking

Handle URLs from outside your app using `DeepLinkController`.

### Setup

```go
type appState struct {
    core.StateBase
    deepLinkController *navigation.DeepLinkController
}

func (s *appState) InitState() {
    // Create controller with a route mapper function
    s.deepLinkController = navigation.NewDeepLinkController(
        // Route mapper: converts deep links to navigation routes
        func(link platform.DeepLink) (navigation.DeepLinkRoute, bool) {
            switch {
            case strings.HasPrefix(link.Path, "/product/"):
                id := strings.TrimPrefix(link.Path, "/product/")
                return navigation.DeepLinkRoute{
                    Name: "/product",
                    Args: map[string]any{"id": id},
                }, true
            case strings.HasPrefix(link.Path, "/user/"):
                username := strings.TrimPrefix(link.Path, "/user/")
                return navigation.DeepLinkRoute{
                    Name: "/profile",
                    Args: map[string]any{"username": username},
                }, true
            default:
                return navigation.DeepLinkRoute{}, false
            }
        },
        // Error handler
        func(err error) {
            log.Printf("Deep link error: %v", err)
        },
    )

    // Cleanup when done
    s.OnDispose(func() {
        s.deepLinkController.Stop()
    })
}
```

The controller automatically:
- Listens for incoming deep links from the platform
- Handles the initial deep link if the app was launched via URL
- Navigates to matching routes using `RootNavigator()`

**Important:** Deep links require a root navigator. If your app uses TabScaffold at the top level, wrap it in a Router:

```go
navigation.Router{
    InitialPath: "/",
    Routes: []navigation.RouteConfigurer{
        navigation.RouteConfig{Path: "/", Builder: buildTabScaffold},
        // Deep link routes
        navigation.RouteConfig{Path: "/product/:id", Builder: buildProduct},
        navigation.RouteConfig{Path: "/profile/:username", Builder: buildProfile},
    },
}
```

## Tab Navigation

Use `TabScaffold` for bottom tab navigation with separate navigation stacks per tab:

```go
func App() core.Widget {
    return navigation.TabScaffold{
        Tabs: []navigation.Tab{
            navigation.NewTab(
                widgets.TabItem{Label: "Home", Icon: widgets.Icon{Glyph: "home"}},
                buildHomeScreen,
            ),
            navigation.NewTab(
                widgets.TabItem{Label: "Search", Icon: widgets.Icon{Glyph: "search"}},
                buildSearchScreen,
            ),
            navigation.NewTab(
                widgets.TabItem{Label: "Profile", Icon: widgets.Icon{Glyph: "person"}},
                buildProfileScreen,
            ),
        },
    }
}

func buildHomeScreen(ctx core.BuildContext) core.Widget {
    return HomeScreen{}
}

func buildSearchScreen(ctx core.BuildContext) core.Widget {
    return SearchScreen{}
}

func buildProfileScreen(ctx core.BuildContext) core.Widget {
    return ProfileScreen{}
}
```

### Active Navigator Tracking

TabScaffold automatically manages which tab's navigator is "active" for back button handling:

- Each tab has its own navigation stack
- When switching tabs, the new tab's navigator becomes active
- Back button pops from the active tab's stack
- Inactive tabs are excluded from the accessibility tree

### Tab Controller

Control the selected tab programmatically:

```go
type appState struct {
    core.StateBase
    tabController *navigation.TabController
}

func (s *appState) InitState() {
    s.tabController = navigation.NewTabController(0) // Start on first tab
}

func (s *appState) Build(ctx core.BuildContext) core.Widget {
    return navigation.TabScaffold{
        Controller: s.tabController,
        Tabs: []navigation.Tab{
            // ... tabs
        },
    }
}

// Switch tabs programmatically
func (s *appState) goToProfile() {
    s.tabController.SetIndex(2)
}
```

### Tabs with Navigation

Each tab can have its own navigation stack:

```go
navigation.Tab{
    Item:         widgets.TabItem{Label: "Home", Icon: homeIcon},
    InitialRoute: "/",
    OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
        switch settings.Name {
        case "/":
            return navigation.NewMaterialPageRoute(buildHome, settings)
        case "/details":
            return navigation.NewMaterialPageRoute(buildDetails, settings)
        }
        return nil
    },
}
```

## Platform Back Button

The Navigator automatically handles the platform back button. Use `navigation.HandleBackButton()` for standard back button handling:

```go
// In your platform-specific code, call HandleBackButton
// Returns true if a route was popped, false if at root
handled := navigation.HandleBackButton()
if !handled {
    // At root - maybe show exit confirmation or exit app
}
```

### Navigation from Outside the Widget Tree

For deep links and external navigation, use `RootNavigator()`:

```go
// Deep link handler
if nav := navigation.RootNavigator(); nav != nil {
    nav.PushNamed("/product", map[string]any{"id": productID})
}
```

For back button handling, use `HandleBackButton()` which automatically handles active tab navigation:

```go
handled := navigation.HandleBackButton()
```

## Nested Navigators

Use multiple navigators for complex flows. Only the root navigator should have `IsRoot: true`:

```go
// Main app navigator (root)
navigation.Navigator{
    InitialRoute: "/",
    IsRoot:       true, // Only the root navigator sets this
    OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
        switch settings.Name {
        case "/":
            return navigation.NewMaterialPageRoute(buildMainTabs, settings)
        case "/onboarding":
            // Onboarding has its own nested navigator
            return navigation.NewMaterialPageRoute(buildOnboarding, settings)
        }
        return nil
    },
}

// Onboarding flow with its own navigator (nested, not root)
func buildOnboarding(ctx core.BuildContext) core.Widget {
    return navigation.Navigator{
        InitialRoute: "/welcome",
        // IsRoot: false (default) - nested navigators don't register globally
        OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
            switch settings.Name {
            case "/welcome":
                return navigation.NewMaterialPageRoute(buildWelcome, settings)
            case "/setup":
                return navigation.NewMaterialPageRoute(buildSetup, settings)
            case "/complete":
                return navigation.NewMaterialPageRoute(buildComplete, settings)
            }
            return nil
        },
    }
}
```

### Back Button with Nested Navigators

For nested navigators outside TabScaffold, back button handling uses the root navigator by default. The nested navigator handles its own internal navigation via `NavigatorOf(ctx)`:

```go
func buildOnboardingStep(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: "Step 1"},
            theme.ButtonOf(ctx, "Next", func() {
                // Use NavigatorOf for internal navigation within the nested navigator
                navigation.NavigatorOf(ctx).PushNamed("/setup", nil)
            }),
            theme.ButtonOf(ctx, "Skip", func() {
                // Pop the entire onboarding flow from the root navigator
                navigation.RootNavigator().Pop(nil)
            }),
        },
    }
}
```

## Next Steps

- [Gestures](/docs/guides/gestures) - Handle touch input
- [Accessibility](/docs/guides/accessibility) - Make your app accessible
- [API Reference](/docs/api/navigation) - Navigation API documentation
