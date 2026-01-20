package navigation

// NavigatorObserver observes navigation events.
type NavigatorObserver interface {
	// DidPush is called when a route is pushed.
	DidPush(route, previousRoute Route)

	// DidPop is called when a route is popped.
	DidPop(route, previousRoute Route)

	// DidRemove is called when a route is removed.
	DidRemove(route, previousRoute Route)

	// DidReplace is called when a route is replaced.
	DidReplace(newRoute, oldRoute Route)
}

// BaseNavigatorObserver provides default no-op implementations.
type BaseNavigatorObserver struct{}

// DidPush is a no-op.
func (b *BaseNavigatorObserver) DidPush(route, previousRoute Route) {}

// DidPop is a no-op.
func (b *BaseNavigatorObserver) DidPop(route, previousRoute Route) {}

// DidRemove is a no-op.
func (b *BaseNavigatorObserver) DidRemove(route, previousRoute Route) {}

// DidReplace is a no-op.
func (b *BaseNavigatorObserver) DidReplace(newRoute, oldRoute Route) {}
