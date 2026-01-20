package core

import (
	"reflect"

	"github.com/go-drift/drift/pkg/layout"
)

// InheritedElement hosts an InheritedWidget and tracks dependents.
type InheritedElement struct {
	elementBase
	child      Element
	dependents map[Element]struct{}
}

// NewInheritedElement creates an InheritedElement for the given widget.
func NewInheritedElement(widget InheritedWidget, owner *BuildOwner) *InheritedElement {
	element := &InheritedElement{
		dependents: make(map[Element]struct{}),
	}
	element.widget = widget
	element.buildOwner = owner
	element.setSelf(element)
	return element
}

func (e *InheritedElement) Mount(parent Element, slot any) {
	e.parent = parent
	e.slot = slot
	if parent != nil {
		e.depth = parent.Depth() + 1
	}
	e.mounted = true
	e.dirty = true
	e.RebuildIfNeeded()
}

func (e *InheritedElement) Update(newWidget Widget) {
	oldWidget := e.widget.(InheritedWidget)
	e.widget = newWidget
	newInherited := newWidget.(InheritedWidget)
	if newInherited.UpdateShouldNotify(oldWidget) {
		e.notifyDependents()
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
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false
	inherited := e.widget.(InheritedWidget)
	childWidget := inherited.Child()
	e.child = updateChild(e.child, childWidget, e, e.buildOwner)
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

func (e *InheritedElement) DependOnInherited(inheritedType reflect.Type) any {
	return dependOnInheritedImpl(e, inheritedType)
}

// AddDependent registers an element as depending on this inherited widget.
func (e *InheritedElement) AddDependent(dependent Element) {
	if e.dependents == nil {
		e.dependents = make(map[Element]struct{})
	}
	e.dependents[dependent] = struct{}{}
}

// RemoveDependent unregisters an element as depending on this inherited widget.
func (e *InheritedElement) RemoveDependent(dependent Element) {
	delete(e.dependents, dependent)
}

// notifyDependents calls DidChangeDependencies on all dependents.
func (e *InheritedElement) notifyDependents() {
	for dependent := range e.dependents {
		notifyDependent(dependent)
	}
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

// dependOnInheritedImpl is the shared implementation for DependOnInherited.
// It walks up the element tree to find the nearest InheritedElement of the requested type.
func dependOnInheritedImpl(element Element, inheritedType reflect.Type) any {
	var current Element
	if base, ok := element.(interface{ parentElement() Element }); ok {
		current = base.parentElement()
	}

	for current != nil {
		if inherited, ok := current.(*InheritedElement); ok {
			widgetType := reflect.TypeOf(inherited.widget)
			if widgetType == inheritedType || (widgetType.Kind() == reflect.Ptr && widgetType.Elem() == inheritedType) {
				// Register dependency
				inherited.AddDependent(element)
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
