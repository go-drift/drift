package core

import (
	"reflect"

	"github.com/go-drift/drift/pkg/layout"
)

// LayoutBuilderWidget is implemented by widgets that defer child building to
// the layout phase. Unlike standard widgets whose Build runs before layout,
// a LayoutBuilderWidget provides a builder function that is invoked during
// the render object's PerformLayout, once the parent's constraints are known.
//
// The LayoutBuilder method returns the builder callback. The element stores
// this callback and passes it to the render object, which invokes it with
// the resolved constraints during layout.
type LayoutBuilderWidget interface {
	RenderObjectWidget
	LayoutBuilder() func(ctx BuildContext, constraints layout.Constraints) Widget
}

// LayoutBuilderElement hosts a LayoutBuilderWidget, deferring child building
// to the layout phase so the builder function receives actual constraints.
//
// This element uses a dual-trigger invalidation model:
//
//   - Layout-phase trigger: when the parent's constraints change, the render
//     object calls layoutCallback during PerformLayout and the element
//     re-invokes the builder with the new constraints.
//   - Build-phase trigger: when an inherited dependency changes or the widget
//     is updated, [LayoutBuilderElement.RebuildIfNeeded] translates the dirty
//     flag into childDirty and calls MarkNeedsLayout on the render object,
//     which schedules a new layout pass that re-invokes the builder.
//
// LayoutBuilderElement implements [renderObjectHost], allowing descendant
// RenderObjectElements to attach their render objects through it.
type LayoutBuilderElement struct {
	elementBase
	renderObject        layout.RenderObject
	child               Element
	childDirty          bool               // forces child rebuild on next layout callback
	previousConstraints layout.Constraints // skip rebuild when constraints unchanged
	hasBuilt            bool               // whether we've ever built the child
}

// NewLayoutBuilderElement creates a LayoutBuilderElement for the given widget.
// The owner may be nil during widget testing; it is set by the framework when
// the element is mounted into a live tree.
func NewLayoutBuilderElement(widget LayoutBuilderWidget, owner *BuildOwner) *LayoutBuilderElement {
	element := &LayoutBuilderElement{}
	element.widget = widget
	element.buildOwner = owner
	element.setSelf(element)
	return element
}

// Mount creates the render object, registers the layout callback on it,
// attaches to the render tree, and marks the child as needing its first build.
// Child building is deferred until the first layout pass.
func (e *LayoutBuilderElement) Mount(parent Element, slot any) {
	e.parent = parent
	e.slot = slot
	if parent != nil {
		e.depth = parent.Depth() + 1
	}
	e.mounted = true

	// Create render object
	widget := e.widget.(LayoutBuilderWidget)
	e.renderObject = widget.CreateRenderObject(e)
	if e.buildOwner != nil {
		e.renderObject.SetOwner(e.buildOwner.Pipeline())
	}

	// Set the layout callback on the render object
	if setter, ok := e.renderObject.(interface {
		SetLayoutCallback(func(layout.Constraints))
	}); ok {
		setter.SetLayoutCallback(e.layoutCallback)
	}

	// Attach to render tree
	e.attachRenderObject(slot)

	// Mark child as needing build on first layout
	e.childDirty = true
}

// Update replaces the widget, marks the child dirty, and triggers a relayout
// so the layout callback re-invokes the builder with the new widget's function.
func (e *LayoutBuilderElement) Update(newWidget Widget) {
	e.widget = newWidget
	// Mark child dirty so next layout rebuilds with the new builder
	e.childDirty = true
	// Update render object properties
	if lbw, ok := e.widget.(LayoutBuilderWidget); ok {
		lbw.UpdateRenderObject(e, e.renderObject)
	}
	// Trigger relayout so the callback fires
	e.renderObject.MarkNeedsLayout()
}

// Unmount recursively unmounts the child element and detaches the render
// object from the render tree.
func (e *LayoutBuilderElement) Unmount() {
	e.mounted = false
	if e.child != nil {
		e.child.Unmount()
		e.child = nil
	}
	e.detachRenderObject()
}

// RebuildIfNeeded handles build-phase invalidation (e.g. inherited dependency changes).
// Child building is still deferred to layout, but we must translate the dirty flag
// into childDirty + MarkNeedsLayout so the layout callback re-invokes the builder.
func (e *LayoutBuilderElement) RebuildIfNeeded() {
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false
	e.childDirty = true
	e.renderObject.MarkNeedsLayout()
}

// layoutCallback is invoked by the render object during PerformLayout.
// It skips rebuilding when the constraints are unchanged and no build-phase
// invalidation (childDirty) has occurred. Otherwise it invokes the builder
// function and reconciles the child element tree via updateChild.
//
// Note: calling updateChild from within the layout pass means the element tree
// mutates during layout. This is the same trade-off Flutter makes. The builder
// must not trigger MarkNeedsLayout on ancestors that have already been laid out
// in the current pass, as that could produce stale layout results.
func (e *LayoutBuilderElement) layoutCallback(constraints layout.Constraints) {
	if !e.mounted {
		return
	}
	if !e.childDirty && e.hasBuilt && constraints == e.previousConstraints {
		return
	}

	lbw := e.widget.(LayoutBuilderWidget)
	builder := lbw.LayoutBuilder()

	var built Widget
	if builder != nil {
		built = e.safeBuild(func() Widget {
			return builder(e, constraints)
		})
	}

	e.child = updateChild(e.child, built, e, e.buildOwner, nil)

	e.childDirty = false
	e.previousConstraints = constraints
	e.hasBuilt = true
}

// VisitChildren calls the visitor with the single child element, if present.
func (e *LayoutBuilderElement) VisitChildren(visitor func(Element) bool) {
	if e.child != nil {
		visitor(e.child)
	}
}

// RenderObject returns the render object owned by this element.
func (e *LayoutBuilderElement) RenderObject() layout.RenderObject {
	return e.renderObject
}

// FindAncestor walks up the element tree and returns the first ancestor
// matching the predicate, or nil if none is found.
func (e *LayoutBuilderElement) FindAncestor(predicate func(Element) bool) Element {
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

// DependOnInherited registers a dependency on the nearest ancestor
// InheritedElement of the given type and returns its current value.
func (e *LayoutBuilderElement) DependOnInherited(inheritedType reflect.Type, aspect any) any {
	return dependOnInheritedImpl(e, inheritedType, aspect)
}

// DependOnInheritedWithAspects registers a dependency on the nearest ancestor
// InheritedElement of the given type, filtered by the specified aspects.
func (e *LayoutBuilderElement) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return dependOnInheritedWithAspects(e, inheritedType, aspects...)
}

// renderObjectHost implementation: these methods allow descendant
// RenderObjectElements to attach/detach their render objects through us.

func (e *LayoutBuilderElement) insertRenderObjectChild(child layout.RenderObject, slot any) {
	if child == nil {
		return
	}
	if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
		setter.SetParent(e.renderObject)
	}
	if single, ok := e.renderObject.(interface{ SetChild(layout.RenderObject) }); ok {
		single.SetChild(child)
	}
}

func (e *LayoutBuilderElement) removeRenderObjectChild(child layout.RenderObject, slot any) {
	if child == nil {
		return
	}
	if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
		setter.SetParent(nil)
	}
	if single, ok := e.renderObject.(interface{ SetChild(layout.RenderObject) }); ok {
		single.SetChild(nil)
	}
}

func (e *LayoutBuilderElement) moveRenderObjectChild(child layout.RenderObject, oldSlot, newSlot any) {
	// No-op for single-child element.
}

// attachRenderObject attaches this element's render object to the render tree.
func (e *LayoutBuilderElement) attachRenderObject(slot any) {
	newRenderParent := e.findRenderParent()

	if e.renderParent != nil && e.renderParent != newRenderParent {
		e.renderParent.removeRenderObjectChild(e.renderObject, e.slot)
	}

	e.renderParent = newRenderParent
	if e.renderParent != nil {
		e.renderParent.insertRenderObjectChild(e.renderObject, slot)
	}
}

// detachRenderObject removes this element's render object from the render tree.
func (e *LayoutBuilderElement) detachRenderObject() {
	if e.renderParent != nil {
		e.renderParent.removeRenderObjectChild(e.renderObject, e.slot)
		e.renderParent = nil
	}
	if e.renderObject != nil {
		if disposer, ok := e.renderObject.(interface{ Dispose() }); ok {
			disposer.Dispose()
		}
	}
}

// UpdateSlot updates the slot and notifies the render parent of the move.
func (e *LayoutBuilderElement) UpdateSlot(newSlot any) {
	oldSlot := e.slot
	e.slot = newSlot
	if e.renderParent != nil {
		e.renderParent.moveRenderObjectChild(e.renderObject, oldSlot, newSlot)
	}
}
