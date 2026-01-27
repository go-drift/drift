package core

import (
	"reflect"

	"github.com/go-drift/drift/pkg/layout"
)

// dependOnAllAspects is a sentinel value indicating a widget depends on all changes,
// not just specific aspects. Used when DependOnInherited is called with nil aspect.
var dependOnAllAspects = &struct{}{}

// InheritedElement is the element that hosts an [InheritedWidget] and manages
// the dependency tracking for descendant widgets.
//
// When a descendant calls [BuildContext.DependOnInherited], it registers as a
// dependent of this element. When the InheritedWidget is updated and
// [InheritedWidget.UpdateShouldNotify] returns true, all registered dependents
// are notified and scheduled for rebuild.
//
// # Aspect-Based Tracking
//
// InheritedElement supports granular dependency tracking via aspects. When a
// dependent registers with a specific aspect (non-nil), it's stored in that
// dependent's aspect set. On update, [InheritedWidget.UpdateShouldNotifyDependent]
// is called for each dependent to determine if it should rebuild based on its
// registered aspects.
//
// Note: Aspect sets only grow during an element's lifetime. If a widget stops
// depending on an aspect across rebuilds, the old aspect remains registered.
// This may cause extra rebuilds but is safe (over-notification, not under).
type InheritedElement struct {
	elementBase
	child      Element
	dependents map[Element]map[any]struct{} // aspects per dependent
}

// NewInheritedElement creates an InheritedElement for the given widget.
func NewInheritedElement(widget InheritedWidget, owner *BuildOwner) *InheritedElement {
	element := &InheritedElement{
		dependents: make(map[Element]map[any]struct{}),
	}
	element.widget = widget
	element.buildOwner = owner
	element.setSelf(element)
	return element
}

func (e *InheritedElement) Mount(parent Element, slot any) {
	e.MountWithSelf(parent, slot, e)
}

// MountWithSelf allows a wrapper element to specify itself as the parent for children.
func (e *InheritedElement) MountWithSelf(parent Element, slot any, self Element) {
	e.parent = parent
	e.slot = slot
	e.self = self // Use provided self for parent references
	if parent != nil {
		e.depth = parent.Depth() + 1
	}
	e.renderParent = e.findRenderParent()
	e.mounted = true
	e.dirty = true
	e.rebuildWithSelf(self)
}

func (e *InheritedElement) Update(newWidget Widget) {
	oldWidget := e.widget.(InheritedWidget)
	e.widget = newWidget
	newInherited := newWidget.(InheritedWidget)

	// UpdateShouldNotify acts as a coarse-grained gate. If it returns false,
	// no dependents are notified. For aspect-based filtering to work, implementations
	// should return true from UpdateShouldNotify when *any* aspect might have changed,
	// then use UpdateShouldNotifyDependent for fine-grained per-dependent filtering.
	if !newInherited.UpdateShouldNotify(oldWidget) {
		e.MarkNeedsBuild()
		return
	}

	// Check each dependent's aspects for granular notification
	for dependent, aspects := range e.dependents {
		// Check for sentinel indicating "all changes" dependency
		if _, dependsOnAll := aspects[dependOnAllAspects]; dependsOnAll {
			notifyDependent(dependent)
			continue
		}
		// Empty aspects (shouldn't happen with sentinel, but defensive)
		// or aspect-specific check
		if len(aspects) == 0 || newInherited.UpdateShouldNotifyDependent(oldWidget, aspects) {
			notifyDependent(dependent)
		}
	}

	e.MarkNeedsBuild()
}

func (e *InheritedElement) Unmount() {
	e.mounted = false
	if e.child != nil {
		e.child.Unmount()
		e.child = nil
	}
	e.dependents = nil
}

func (e *InheritedElement) RebuildIfNeeded() {
	e.RebuildIfNeededWithSelf(e)
}

// RebuildIfNeededWithSelf allows a wrapper element to specify itself as the parent.
func (e *InheritedElement) RebuildIfNeededWithSelf(self Element) {
	e.rebuildWithSelf(self)
}

func (e *InheritedElement) rebuildWithSelf(self Element) {
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false
	inherited := e.widget.(InheritedWidget)
	childWidget := inherited.Child()
	e.child = updateChild(e.child, childWidget, self, e.buildOwner, nil)
}

func (e *InheritedElement) VisitChildren(visitor func(Element) bool) {
	if e.child != nil {
		visitor(e.child)
	}
}

// RenderObject returns the render object from the child element.
func (e *InheritedElement) RenderObject() layout.RenderObject {
	if e.child == nil {
		return nil
	}
	if child, ok := e.child.(interface{ RenderObject() layout.RenderObject }); ok {
		return child.RenderObject()
	}
	return nil
}

func (e *InheritedElement) FindAncestor(predicate func(Element) bool) Element {
	current := e.parent
	for current != nil {
		if predicate(current) {
			return current
		}
		if base, ok := current.(interface{ parentElement() Element }); ok {
			current = base.parentElement()
		} else {
			break
		}
	}
	return nil
}

func (e *InheritedElement) DependOnInherited(inheritedType reflect.Type, aspect any) any {
	return dependOnInheritedImpl(e, inheritedType, aspect)
}

func (e *InheritedElement) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return dependOnInheritedWithAspects(e, inheritedType, aspects...)
}

// AddDependent registers an element as depending on this inherited widget.
// If aspect is non-nil, it's added to the dependent's aspect set for granular tracking.
// If aspect is nil, a sentinel is added indicating the widget depends on all changes.
//
// Note: Aspect sets only grow during an element's lifetime. If a widget changes which
// aspects it depends on across rebuilds, old aspects remain registered. This may cause
// extra rebuilds but is safe (over-notification, not under-notification).
func (e *InheritedElement) AddDependent(dependent Element, aspect any) {
	if e.dependents == nil {
		e.dependents = make(map[Element]map[any]struct{})
	}

	aspects := e.dependents[dependent]
	if aspects == nil {
		aspects = make(map[any]struct{})
		e.dependents[dependent] = aspects
	}

	if aspect != nil {
		aspects[aspect] = struct{}{}
	} else {
		aspects[dependOnAllAspects] = struct{}{}
	}
}

// RemoveDependent unregisters an element as depending on this inherited widget.
func (e *InheritedElement) RemoveDependent(dependent Element) {
	delete(e.dependents, dependent)
}

// notifyDependent triggers DidChangeDependencies on the dependent element.
func notifyDependent(element Element) {
	// For StatefulElement, call DidChangeDependencies on the state
	if stateful, ok := element.(*StatefulElement); ok {
		if stateful.state != nil {
			stateful.state.DidChangeDependencies()
		}
		stateful.MarkNeedsBuild()
		return
	}
	// For other elements, just mark needs build
	element.MarkNeedsBuild()
}

// dependOnInheritedWithAspects registers multiple aspects in a single tree walk.
// This is more efficient than calling DependOnInherited multiple times.
func dependOnInheritedWithAspects(element Element, inheritedType reflect.Type, aspects ...any) any {
	var current Element
	if base, ok := element.(interface{ parentElement() Element }); ok {
		current = base.parentElement()
	}

	for current != nil {
		if inherited, ok := current.(*InheritedElement); ok {
			widgetType := reflect.TypeOf(inherited.widget)
			if widgetType == inheritedType || (widgetType.Kind() == reflect.Ptr && widgetType.Elem() == inheritedType) {
				// Register all aspects in one call
				for _, aspect := range aspects {
					inherited.AddDependent(element, aspect)
				}
				return inherited.widget
			}
		}
		if base, ok := current.(interface{ parentElement() Element }); ok {
			current = base.parentElement()
		} else {
			break
		}
	}
	return nil
}

// dependOnInheritedImpl is the shared implementation for DependOnInherited.
// It walks up the element tree to find the nearest InheritedElement of the requested type.
// The aspect parameter enables granular dependency tracking for selective rebuilds.
func dependOnInheritedImpl(element Element, inheritedType reflect.Type, aspect any) any {
	var current Element
	if base, ok := element.(interface{ parentElement() Element }); ok {
		current = base.parentElement()
	}

	for current != nil {
		if inherited, ok := current.(*InheritedElement); ok {
			widgetType := reflect.TypeOf(inherited.widget)
			if widgetType == inheritedType || (widgetType.Kind() == reflect.Ptr && widgetType.Elem() == inheritedType) {
				// Register dependency with optional aspect
				inherited.AddDependent(element, aspect)
				return inherited.widget
			}
		}
		if base, ok := current.(interface{ parentElement() Element }); ok {
			current = base.parentElement()
		} else {
			break
		}
	}
	return nil
}
