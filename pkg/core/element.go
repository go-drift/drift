package core

import (
	"reflect"

	"github.com/go-drift/drift/pkg/layout"
)

type elementBase struct {
	widget     Widget
	parent     Element
	depth      int
	slot       any
	buildOwner *BuildOwner
	dirty      bool
	self       Element
	mounted    bool
}

func (e *elementBase) Widget() Widget {
	return e.widget
}

func (e *elementBase) Depth() int {
	return e.depth
}

func (e *elementBase) MarkNeedsBuild() {
	if e.dirty {
		return
	}
	e.dirty = true
	if e.buildOwner != nil && e.self != nil {
		e.buildOwner.ScheduleBuild(e.self)
	}
}

func (e *elementBase) parentElement() Element {
	return e.parent
}

func (e *elementBase) setSelf(self Element) {
	e.self = self
}

func (e *elementBase) setBuildOwner(owner *BuildOwner) {
	e.buildOwner = owner
}

func (e *elementBase) isMounted() bool {
	return e.mounted
}

// StatelessElement hosts a StatelessWidget.
type StatelessElement struct {
	elementBase
	child Element
}

func NewStatelessElement(widget StatelessWidget, owner *BuildOwner) *StatelessElement {
	element := &StatelessElement{}
	element.widget = widget
	element.buildOwner = owner
	element.setSelf(element)
	return element
}

func (e *StatelessElement) Mount(parent Element, slot any) {
	e.parent = parent
	e.slot = slot
	if parent != nil {
		e.depth = parent.Depth() + 1
	}
	e.mounted = true
	e.dirty = true
	e.RebuildIfNeeded()
}

func (e *StatelessElement) Update(newWidget Widget) {
	e.widget = newWidget
	e.MarkNeedsBuild()
}

func (e *StatelessElement) Unmount() {
	e.mounted = false
	if e.child != nil {
		e.child.Unmount()
		e.child = nil
	}
}

func (e *StatelessElement) RebuildIfNeeded() {
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false
	widget := e.widget.(StatelessWidget)
	built := widget.Build(e)
	e.child = updateChild(e.child, built, e, e.buildOwner)
}

func (e *StatelessElement) VisitChildren(visitor func(Element) bool) {
	if e.child != nil {
		visitor(e.child)
	}
}

// RenderObject returns the render object from the first render-object child.
func (e *StatelessElement) RenderObject() layout.RenderObject {
	if e.child == nil {
		return nil
	}
	if child, ok := e.child.(interface{ RenderObject() layout.RenderObject }); ok {
		return child.RenderObject()
	}
	return nil
}

func (e *StatelessElement) FindAncestor(predicate func(Element) bool) Element {
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

func (e *StatelessElement) DependOnInherited(inheritedType reflect.Type) any {
	return dependOnInheritedImpl(e, inheritedType)
}

// StatefulElement hosts a StatefulWidget and its State.
type StatefulElement struct {
	elementBase
	child Element
	state State
}

func NewStatefulElement(widget StatefulWidget, owner *BuildOwner) *StatefulElement {
	element := &StatefulElement{}
	element.widget = widget
	element.buildOwner = owner
	element.setSelf(element)
	return element
}

func (e *StatefulElement) Mount(parent Element, slot any) {
	e.parent = parent
	e.slot = slot
	if parent != nil {
		e.depth = parent.Depth() + 1
	}
	e.mounted = true
	widget := e.widget.(StatefulWidget)
	e.state = widget.CreateState()
	if setter, ok := e.state.(interface{ SetElement(*StatefulElement) }); ok {
		setter.SetElement(e)
	} else if setter, ok := e.state.(interface{ setElement(*StatefulElement) }); ok {
		setter.setElement(e)
	}
	e.state.InitState()
	e.dirty = true
	e.RebuildIfNeeded()
}

func (e *StatefulElement) Update(newWidget Widget) {
	oldWidget := e.widget.(StatefulWidget)
	e.widget = newWidget
	e.state.DidUpdateWidget(oldWidget)
	e.MarkNeedsBuild()
}

func (e *StatefulElement) Unmount() {
	e.mounted = false
	if e.child != nil {
		e.child.Unmount()
		e.child = nil
	}
	if e.state != nil {
		e.state.Dispose()
	}
}

func (e *StatefulElement) RebuildIfNeeded() {
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false
	built := e.state.Build(e)
	e.child = updateChild(e.child, built, e, e.buildOwner)
}

func (e *StatefulElement) VisitChildren(visitor func(Element) bool) {
	if e.child != nil {
		visitor(e.child)
	}
}

// RenderObject returns the render object from the first render-object child.
func (e *StatefulElement) RenderObject() layout.RenderObject {
	if e.child == nil {
		return nil
	}
	if child, ok := e.child.(interface{ RenderObject() layout.RenderObject }); ok {
		return child.RenderObject()
	}
	return nil
}

func (e *StatefulElement) FindAncestor(predicate func(Element) bool) Element {
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

func (e *StatefulElement) DependOnInherited(inheritedType reflect.Type) any {
	return dependOnInheritedImpl(e, inheritedType)
}

// RenderObjectElement hosts a RenderObject and optional children.
type RenderObjectElement struct {
	elementBase
	renderObject layout.RenderObject
	children     []Element
}

func NewRenderObjectElement(widget RenderObjectWidget, owner *BuildOwner) *RenderObjectElement {
	element := &RenderObjectElement{}
	element.widget = widget
	element.buildOwner = owner
	element.setSelf(element)
	return element
}

func (e *RenderObjectElement) Mount(parent Element, slot any) {
	e.parent = parent
	e.slot = slot
	if parent != nil {
		e.depth = parent.Depth() + 1
	}
	e.mounted = true
	widget := e.widget.(RenderObjectWidget)
	e.renderObject = widget.CreateRenderObject(e)
	if e.buildOwner != nil {
		e.renderObject.SetOwner(e.buildOwner.Pipeline())
	}
	e.dirty = true
	e.RebuildIfNeeded()
}

func (e *RenderObjectElement) Update(newWidget Widget) {
	e.widget = newWidget
	e.MarkNeedsBuild()
}

func (e *RenderObjectElement) Unmount() {
	e.mounted = false
	for _, child := range e.children {
		child.Unmount()
	}
	e.children = nil
}

func (e *RenderObjectElement) RebuildIfNeeded() {
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false
	widget := e.widget.(RenderObjectWidget)
	widget.UpdateRenderObject(e, e.renderObject)
	switch typed := e.widget.(type) {
	case interface{ Child() Widget }:
		childWidget := typed.Child()
		var child Element
		if len(e.children) > 0 {
			child = e.children[0]
		}
		child = updateChild(child, childWidget, e, e.buildOwner)
		if child != nil {
			e.children = []Element{child}
		} else {
			e.children = nil
		}
		if single, ok := e.renderObject.(interface{ SetChild(layout.RenderObject) }); ok {
			if child == nil {
				single.SetChild(nil)
			} else if childElement, ok := child.(interface{ RenderObject() layout.RenderObject }); ok {
				single.SetChild(childElement.RenderObject())
			}
		}
	case interface{ Children() []Widget }:
		widgets := typed.Children()
		updated := make([]Element, 0, len(widgets))
		for index, childWidget := range widgets {
			var existing Element
			if index < len(e.children) {
				existing = e.children[index]
			}
			child := updateChild(existing, childWidget, e, e.buildOwner)
			if child != nil {
				updated = append(updated, child)
			}
		}
		for i := len(widgets); i < len(e.children); i++ {
			e.children[i].Unmount()
		}
		e.children = updated
		if multi, ok := e.renderObject.(interface{ SetChildren([]layout.RenderObject) }); ok {
			objects := make([]layout.RenderObject, 0, len(updated))
			for _, child := range updated {
				if childElement, ok := child.(interface{ RenderObject() layout.RenderObject }); ok {
					objects = append(objects, childElement.RenderObject())
				}
			}
			multi.SetChildren(objects)
		}
	}
}

func (e *RenderObjectElement) VisitChildren(visitor func(Element) bool) {
	for _, child := range e.children {
		if !visitor(child) {
			return
		}
	}
}

func (e *RenderObjectElement) FindAncestor(predicate func(Element) bool) Element {
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

func (e *RenderObjectElement) DependOnInherited(inheritedType reflect.Type) any {
	return dependOnInheritedImpl(e, inheritedType)
}

// RenderObject exposes the backing render object for the element.
func (e *RenderObjectElement) RenderObject() layout.RenderObject {
	return e.renderObject
}

func (e *RenderObjectElement) parentElement() Element {
	return e.parent
}

func updateChild(existing Element, widget Widget, parent Element, owner *BuildOwner) Element {
	if widget == nil {
		if existing != nil {
			existing.Unmount()
		}
		return nil
	}
	if existing != nil && canUpdateWidget(existing.Widget(), widget) {
		existing.Update(widget)
		return existing
	}
	if existing != nil {
		existing.Unmount()
	}
	element := inflateWidget(widget, owner)
	element.Mount(parent, nil)
	return element
}

func canUpdateWidget(existing Widget, next Widget) bool {
	if existing == nil || next == nil {
		return false
	}
	if reflect.TypeOf(existing) != reflect.TypeOf(next) {
		return false
	}
	return reflect.DeepEqual(existing.Key(), next.Key())
}

func inflateWidget(widget Widget, owner *BuildOwner) Element {
	if widget == nil {
		return nil
	}
	element := widget.CreateElement()
	if setter, ok := element.(interface{ setBuildOwner(*BuildOwner) }); ok {
		setter.setBuildOwner(owner)
	}
	if setter, ok := element.(interface{ setSelf(Element) }); ok {
		setter.setSelf(element)
	}
	return element
}
