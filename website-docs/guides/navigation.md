---
id: navigation
title: Navigation
sidebar_position: 9
---

# Navigation

Drift provides stack-based navigation with support for named routes, deep linking, and tab navigation.

## Setting Up Routes

Use a Navigator with route generation:

```go
func App() core.Widget {
    return navigation.Navigator{
        InitialRoute: "/",
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
- Navigates to matching routes using `GlobalNavigator()`

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

The Navigator automatically handles the platform back button. Use `navigation.HandleBackButton()` or `navigation.GlobalNavigator()` for custom back button handling:

```go
// In your platform-specific code, call HandleBackButton
// Returns true if a route was popped, false if at root
handled := navigation.HandleBackButton()
if !handled {
    // At root - maybe show exit confirmation or exit app
}

// Or get the global navigator directly
if nav := navigation.GlobalNavigator(); nav != nil {
    if nav.CanPop() {
        nav.Pop(nil)
    }
}
```

## Nested Navigators

Use multiple navigators for complex flows:

```go
// Main app navigator
navigation.Navigator{
    InitialRoute: "/",
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

// Onboarding flow with its own navigator
func buildOnboarding(ctx core.BuildContext) core.Widget {
    return navigation.Navigator{
        InitialRoute: "/welcome",
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

## Next Steps

- [Gestures](/docs/guides/gestures) - Handle touch input
- [Accessibility](/docs/guides/accessibility) - Make your app accessible
- [API Reference](/docs/api/navigation) - Navigation API documentation
