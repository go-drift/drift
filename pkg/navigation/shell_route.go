package navigation

import "github.com/go-drift/drift/pkg/core"

// ShellRoute wraps child routes in a persistent layout.
// Use this for layouts like tabs, drawers, or persistent navigation bars
// that should remain visible while the inner content changes.
//
// Example:
//
//	ShellRoute{
//	    Builder: func(ctx core.BuildContext, child core.Widget) core.Widget {
//	        return widgets.Column{
//	            Children: []core.Widget{
//	                MyNavigationBar{},
//	                widgets.Expanded{Child: child},
//	            },
//	        }
//	    },
//	    Routes: []navigation.RouteConfigurer{
//	        navigation.RouteConfig{Path: "/home", Builder: homePage},
//	        navigation.RouteConfig{Path: "/settings", Builder: settingsPage},
//	    },
//	}
type ShellRoute struct {
	// Builder creates the shell layout.
	// The child parameter is the current route's widget.
	Builder func(ctx core.BuildContext, child core.Widget) core.Widget

	// Routes defines the routes that are wrapped by this shell.
	Routes []RouteConfigurer
}

func (ShellRoute) routeConfig() {}
