package core

import (
	"reflect"
	"time"

	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/layout"
)

// IndexedSlot represents a child's position in a multi-child parent.
type IndexedSlot struct {
	Index           int
	PreviousSibling Element
}

type elementBase struct {
	widget       Widget
	parent       Element
	depth        int
	slot         any
	buildOwner   *BuildOwner
	dirty        bool
	self         Element
	mounted      bool
	renderParent *RenderObjectElement // nearest ancestor that owns a render object
}

func (e *elementBase) Widget() Widget {
	return e.widget
}

func (e *elementBase) Depth() int {
	return e.depth
}

func (e *elementBase) Slot() any {
	return e.slot
}

func (e *elementBase) UpdateSlot(newSlot any) {
	e.slot = newSlot
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

// findRenderParent walks up the element tree to find the nearest RenderObjectElement.
func (e *elementBase) findRenderParent() *RenderObjectElement {
	current := e.parent
	for current != nil {
		if roElement, ok := current.(*RenderObjectElement); ok {
			return roElement
		}
		if base, ok := current.(interface{ parentElement() Element }); ok {
			current = base.parentElement()
		} else {
			break
		}
	}
	return nil
}

// safeBuild executes a build function with panic recovery.
// If the build panics, it reports the error and returns an error widget.
func (e *elementBase) safeBuild(buildFn func() Widget) Widget {
	var built Widget
	var buildErr *errors.BoundaryError

	func() {
		defer func() {
			if r := recover(); r != nil {
				buildErr = &errors.BoundaryError{
					Phase:      "build",
					Widget:     reflect.TypeOf(e.widget).String(),
					Recovered:  r,
					StackTrace: errors.CaptureStack(),
					Timestamp:  time.Now(),
				}
			}
		}()
		built = buildFn()
	}()

	if buildErr != nil {
		// Report to global error handler
		errors.ReportBoundaryError(buildErr)

		// Find nearest error boundary in ancestors
		if boundary := e.findErrorBoundary(); boundary != nil {
			boundary.CaptureError(buildErr)
			// Return nil to indicate the boundary will handle display
			return nil
		}

		// Use global fallback error widget builder
		if builder := GetErrorWidgetBuilder(); builder != nil {
			if errWidget := builder(buildErr); errWidget != nil {
				return errWidget
			}
		}

		// Final fallback: return a minimal placeholder widget
		return errorPlaceholder{err: buildErr}
	}
	return built
}

// findErrorBoundary searches ancestors for an error boundary.
func (e *elementBase) findErrorBoundary() ErrorBoundaryCapture {
	current := e.parent
	for current != nil {
		if capture, ok := current.(ErrorBoundaryCapture); ok {
			return capture
		}
		if base, ok := current.(interface{ parentElement() Element }); ok {
			current = base.parentElement()
		} else {
			break
		}
	}
	return nil
}

// errorPlaceholder is a minimal fallback widget shown when build fails
// and no error widget builder is configured.
type errorPlaceholder struct {
	err *errors.BoundaryError
}

func (p errorPlaceholder) CreateElement() Element {
	return NewStatelessElement(p, nil)
}

func (p errorPlaceholder) Key() any {
	return nil
}

func (p errorPlaceholder) Build(ctx BuildContext) Widget {
	// Return nil to render nothing - the error has been reported
	return nil
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
	e.renderParent = e.findRenderParent()
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
	built := e.safeBuild(func() Widget {
		return widget.Build(e)
	})
	e.child = updateChild(e.child, built, e, e.buildOwner, nil)
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

func (e *StatelessElement) DependOnInherited(inheritedType reflect.Type, aspect any) any {
	return dependOnInheritedImpl(e, inheritedType, aspect)
}

func (e *StatelessElement) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return dependOnInheritedWithAspects(e, inheritedType, aspects...)
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
	e.renderParent = e.findRenderParent()
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
	built := e.safeBuild(func() Widget {
		return e.state.Build(e)
	})
	e.child = updateChild(e.child, built, e, e.buildOwner, nil)
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

func (e *StatefulElement) DependOnInherited(inheritedType reflect.Type, aspect any) any {
	return dependOnInheritedImpl(e, inheritedType, aspect)
}

func (e *StatefulElement) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return dependOnInheritedWithAspects(e, inheritedType, aspects...)
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

	// Create render object
	widget := e.widget.(RenderObjectWidget)
	e.renderObject = widget.CreateRenderObject(e)
	if e.buildOwner != nil {
		e.renderObject.SetOwner(e.buildOwner.Pipeline())
	}

	// Attach to render tree BEFORE building children
	e.attachRenderObject(slot)

	// Build children
	e.dirty = true
	e.RebuildIfNeeded()
}

func (e *RenderObjectElement) Update(newWidget Widget) {
	e.widget = newWidget
	e.MarkNeedsBuild()
}

func (e *RenderObjectElement) Unmount() {
	e.mounted = false

	// Unmount children first (they detach their own render objects)
	for _, child := range e.children {
		child.Unmount()
	}
	e.children = nil

	// Then detach our own render object
	e.detachRenderObject()
}

func (e *RenderObjectElement) RebuildIfNeeded() {
	if !e.dirty || !e.mounted {
		return
	}
	e.dirty = false

	widget := e.widget.(RenderObjectWidget)
	widget.UpdateRenderObject(e, e.renderObject)

	switch typed := e.widget.(type) {
	case interface{ ChildWidget() Widget }:
		childWidget := typed.ChildWidget()
		var child Element
		if len(e.children) > 0 {
			child = e.children[0]
		}
		child = updateChild(child, childWidget, e, e.buildOwner, nil)
		if child != nil {
			e.children = []Element{child}
		} else {
			e.children = nil
		}
		// NO SetChild call - attachment handled in child's Mount/Unmount

	case interface{ ChildrenWidgets() []Widget }:
		widgets := typed.ChildrenWidgets()
		e.children = updateChildren(e, e.children, widgets, e.buildOwner)
		e.rebuildChildrenRenderList()
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

func (e *RenderObjectElement) DependOnInherited(inheritedType reflect.Type, aspect any) any {
	return dependOnInheritedImpl(e, inheritedType, aspect)
}

func (e *RenderObjectElement) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return dependOnInheritedWithAspects(e, inheritedType, aspects...)
}

// RenderObject exposes the backing render object for the element.
func (e *RenderObjectElement) RenderObject() layout.RenderObject {
	return e.renderObject
}

func (e *RenderObjectElement) parentElement() Element {
	return e.parent
}

// UpdateSlot updates the slot and notifies the render parent of the move.
func (e *RenderObjectElement) UpdateSlot(newSlot any) {
	oldSlot := e.slot
	e.slot = newSlot
	if e.renderParent != nil {
		e.renderParent.moveRenderObjectChild(e.renderObject, oldSlot, newSlot)
	}
}

// moveRenderObjectChild notifies that a child moved from oldSlot to newSlot.
// This is a no-op because the caller (RebuildIfNeeded) calls rebuildChildrenRenderList()
// after all children are processed. Calling it here would use stale e.children data
// since updateChildren builds a new list that hasn't been assigned yet.
func (e *RenderObjectElement) moveRenderObjectChild(child layout.RenderObject, oldSlot, newSlot any) {
	// No-op: rebuildChildrenRenderList() is called by the parent after updateChildren completes.
	// Future optimization: could use slot information for incremental updates.
}

// attachRenderObject attaches this element's render object to the render tree.
// Called from Mount after the render object is created.
func (e *RenderObjectElement) attachRenderObject(slot any) {
	newRenderParent := e.findRenderParent()

	// Handle reparenting: detach from old parent if different
	if e.renderParent != nil && e.renderParent != newRenderParent {
		e.renderParent.removeRenderObjectChild(e.renderObject, e.slot)
	}

	e.renderParent = newRenderParent
	if e.renderParent != nil {
		e.renderParent.insertRenderObjectChild(e.renderObject, slot)
	}
}

// detachRenderObject removes this element's render object from the render tree.
// Called from Unmount before the element is unmounted.
func (e *RenderObjectElement) detachRenderObject() {
	if e.renderParent != nil {
		e.renderParent.removeRenderObjectChild(e.renderObject, e.slot)
		e.renderParent = nil
	}
}

// insertRenderObjectChild adds a child render object at the given slot.
func (e *RenderObjectElement) insertRenderObjectChild(child layout.RenderObject, slot any) {
	if child == nil {
		return
	}
	// Set parent reference
	if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
		setter.SetParent(e.renderObject)
	}
	// For single-child render objects, set the child directly
	if single, ok := e.renderObject.(interface{ SetChild(layout.RenderObject) }); ok {
		single.SetChild(child)
		return
	}
	// For multi-child: parent reference is set above; the children list will be
	// rebuilt by RebuildIfNeeded after all children are mounted and e.children
	// is fully populated.
}

// removeRenderObjectChild removes a child render object.
func (e *RenderObjectElement) removeRenderObjectChild(child layout.RenderObject, slot any) {
	if child == nil {
		return
	}
	if setter, ok := child.(interface{ SetParent(layout.RenderObject) }); ok {
		setter.SetParent(nil)
	}
	if single, ok := e.renderObject.(interface{ SetChild(layout.RenderObject) }); ok {
		single.SetChild(nil)
		return
	}
	e.rebuildChildrenRenderList()
}

// rebuildChildrenRenderList rebuilds render object children from element children.
func (e *RenderObjectElement) rebuildChildrenRenderList() {
	if multi, ok := e.renderObject.(interface{ SetChildren([]layout.RenderObject) }); ok {
		objects := make([]layout.RenderObject, 0, len(e.children))
		for _, child := range e.children {
			if roProvider, ok := child.(interface{ RenderObject() layout.RenderObject }); ok {
				if ro := roProvider.RenderObject(); ro != nil {
					objects = append(objects, ro)
				}
			}
		}
		multi.SetChildren(objects)
	}
}

func updateChild(existing Element, widget Widget, parent Element, owner *BuildOwner, slot any) Element {
	if widget == nil {
		if existing != nil {
			existing.Unmount()
		}
		return nil
	}
	if existing != nil && canUpdateWidget(existing.Widget(), widget) {
		// Update slot if changed (use DeepEqual to safely compare any types)
		if !reflect.DeepEqual(existing.Slot(), slot) {
			existing.UpdateSlot(slot)
		}
		existing.Update(widget)
		return existing
	}
	if existing != nil {
		existing.Unmount()
	}
	element := inflateWidget(widget, owner)
	element.Mount(parent, slot)
	return element
}

// updateChildren reconciles old elements with new widgets using keys.
// Implements multi-pass diffing: top sync, bottom scan, key map, final sync.
func updateChildren(
	parent Element,
	oldChildren []Element,
	newWidgets []Widget,
	owner *BuildOwner,
) []Element {
	newChildren := make([]Element, 0, len(newWidgets))

	oldStart, newStart := 0, 0
	oldEnd, newEnd := len(oldChildren), len(newWidgets)

	var prevChild Element

	// 1. Sync from top - match elements at same position
	for oldStart < oldEnd && newStart < newEnd {
		oldChild := oldChildren[oldStart]
		newWidget := newWidgets[newStart]
		if !canUpdateWidget(oldChild.Widget(), newWidget) {
			break
		}
		slot := IndexedSlot{Index: newStart, PreviousSibling: prevChild}
		child := updateChild(oldChild, newWidget, parent, owner, slot)
		newChildren = append(newChildren, child)
		prevChild = child
		oldStart++
		newStart++
	}

	// 2. Scan from bottom - find matching tail (don't process yet)
	oldEndScan, newEndScan := oldEnd, newEnd
	for oldEndScan > oldStart && newEndScan > newStart {
		oldChild := oldChildren[oldEndScan-1]
		newWidget := newWidgets[newEndScan-1]
		if !canUpdateWidget(oldChild.Widget(), newWidget) {
			break
		}
		oldEndScan--
		newEndScan--
	}

	// 3. Build key map for middle old children
	// Only comparable keys can be used in the map; non-comparable keys are treated as non-keyed.
	// NOTE: Duplicate keys silently overwrite earlier entries. If duplicate keys should be
	// invalid, add a debug log/guard here. For now this matches Flutter's behavior.
	keyedOld := make(map[any]Element)
	nonKeyedOld := make([]Element, 0)
	for i := oldStart; i < oldEndScan; i++ {
		child := oldChildren[i]
		key := child.Widget().Key()
		if key != nil && isComparable(key) {
			keyedOld[key] = child
		} else {
			nonKeyedOld = append(nonKeyedOld, child)
		}
	}

	// 4. Process middle new widgets
	nonKeyedIdx := 0
	for newStart < newEndScan {
		newWidget := newWidgets[newStart]
		key := newWidget.Key()
		var oldChild Element

		if key != nil && isComparable(key) {
			oldChild = keyedOld[key]
			delete(keyedOld, key)
		} else if nonKeyedIdx < len(nonKeyedOld) {
			// Try to reuse non-keyed children in order
			candidate := nonKeyedOld[nonKeyedIdx]
			if canUpdateWidget(candidate.Widget(), newWidget) {
				oldChild = candidate
				nonKeyedOld[nonKeyedIdx] = nil // Mark as used
			}
			nonKeyedIdx++
		}

		slot := IndexedSlot{Index: len(newChildren), PreviousSibling: prevChild}
		child := updateChild(oldChild, newWidget, parent, owner, slot)
		newChildren = append(newChildren, child)
		prevChild = child
		newStart++
	}

	// 5. Process bottom matches
	for newEndScan < newEnd {
		oldChild := oldChildren[oldEndScan]
		newWidget := newWidgets[newEndScan]
		slot := IndexedSlot{Index: len(newChildren), PreviousSibling: prevChild}
		child := updateChild(oldChild, newWidget, parent, owner, slot)
		newChildren = append(newChildren, child)
		prevChild = child
		oldEndScan++
		newEndScan++
	}

	// 6. Unmount unused old children
	for _, remaining := range keyedOld {
		remaining.Unmount()
	}
	for _, remaining := range nonKeyedOld {
		if remaining != nil {
			remaining.Unmount()
		}
	}

	return newChildren
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

// isComparable returns true if the value can be used as a map key.
// Non-comparable types (slices, maps, functions) return false.
func isComparable(v any) bool {
	if v == nil {
		return true
	}
	return reflect.TypeOf(v).Comparable()
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
