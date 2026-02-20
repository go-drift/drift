package navigation

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/platform"
)

// DeepLinkRoute describes a navigation target from a deep link.
type DeepLinkRoute struct {
	Name string
	Args any
}

// DeepLinkController listens for deep links and navigates to matching routes.
//
// Deep links are dispatched via [RootNavigator], which requires a [Router] or
// [Navigator] with IsRoot=true to be present in the widget tree. If your app
// uses [TabNavigator] at the top level, wrap it in a Router or Navigator:
//
//	navigation.Router{
//	    Routes: []navigation.ScreenRoute{
//	        {Path: "/", Screen: buildTabNavigator},
//	    },
//	}
//
// Without a root navigator, deep links will remain pending indefinitely.
type DeepLinkController struct {
	RouteForLink func(link platform.DeepLink) (DeepLinkRoute, bool)
	OnError      func(err error)

	mu             sync.Mutex
	pendingRoute   *DeepLinkRoute
	retryScheduled bool
	started        atomic.Bool
	stopCh         chan struct{}
}

// NewDeepLinkController creates a controller with the route mapper
// and immediately starts listening for deep links.
func NewDeepLinkController(routeForLink func(platform.DeepLink) (DeepLinkRoute, bool), onError func(error)) *DeepLinkController {
	controller := &DeepLinkController{
		RouteForLink: routeForLink,
		OnError:      onError,
	}
	controller.start()
	return controller
}

func (c *DeepLinkController) start() {
	if c == nil || c.RouteForLink == nil {
		return
	}
	if c.started.Swap(true) {
		return
	}
	c.stopCh = make(chan struct{})
	go func() {
		link, err := platform.DeepLinks.GetInitial(context.Background())
		if err != nil {
			c.handleError(err)
		} else if link != nil {
			c.handleLink(*link)
		}

		unsub := platform.DeepLinks.Links().Listen(func(link platform.DeepLink) {
			c.handleLink(link)
		})
		defer unsub()
		<-c.stopCh
	}()
}

// Stop stops listening for deep links.
func (c *DeepLinkController) Stop() {
	if c == nil {
		return
	}
	if !c.started.Swap(false) {
		return
	}
	if c.stopCh != nil {
		close(c.stopCh)
	}
	c.mu.Lock()
	c.pendingRoute = nil
	c.retryScheduled = false
	c.mu.Unlock()
}

func (c *DeepLinkController) handleLink(link platform.DeepLink) {
	route, ok := c.RouteForLink(link)
	if !ok {
		return
	}
	drift.Dispatch(func() {
		c.navigate(route)
	})
}

func (c *DeepLinkController) handleError(err error) {
	if c.OnError != nil {
		drift.Dispatch(func() {
			c.OnError(err)
		})
	}
}

func (c *DeepLinkController) navigate(route DeepLinkRoute) {
	if nav := RootNavigator(); nav != nil {
		nav.PushNamed(route.Name, route.Args)
		return
	}
	c.mu.Lock()
	c.pendingRoute = &route
	if c.retryScheduled {
		c.mu.Unlock()
		return
	}
	c.retryScheduled = true
	c.mu.Unlock()

	drift.Dispatch(c.flushPending)
}

func (c *DeepLinkController) flushPending() {
	c.mu.Lock()
	pending := c.pendingRoute
	c.retryScheduled = false
	c.mu.Unlock()
	if pending == nil {
		return
	}
	if nav := RootNavigator(); nav != nil {
		nav.PushNamed(pending.Name, pending.Args)
		c.mu.Lock()
		c.pendingRoute = nil
		c.mu.Unlock()
		return
	}
	c.mu.Lock()
	if c.pendingRoute != nil && !c.retryScheduled {
		c.retryScheduled = true
		c.mu.Unlock()
		drift.Dispatch(c.flushPending)
		return
	}
	c.mu.Unlock()
}
