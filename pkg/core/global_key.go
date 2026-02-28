package core

import "sync/atomic"

var globalKeyNextID atomic.Uint64

// GlobalKey provides cross-tree access to an element's [State] and
// [BuildContext]. The type parameter S is the concrete State type returned by
// [GlobalKey.CurrentState].
//
// A GlobalKey is a value type that wraps a pointer; identity comes from the
// pointer, so two GlobalKey values wrapping the same pointer compare as equal
// via [reflect.DeepEqual] (the mechanism used by the framework's widget
// reconciliation). Two calls to [NewGlobalKey] always produce distinct keys.
//
// When a widget returns a GlobalKey from its Key() method, the framework
// registers the element in the [BuildOwner]'s global key registry on mount
// and removes it on unmount. This enables looking up the element, its state,
// or its context from anywhere in the application, regardless of tree
// position.
//
// GlobalKey works with all element types (stateless, stateful, render object,
// inherited). However, [GlobalKey.CurrentState] only returns a non-zero value
// for stateful elements whose State satisfies the type parameter S.
//
// # Thread Safety
//
// GlobalKey's accessor methods (CurrentState, CurrentElement, CurrentContext)
// read from fields that are written during mount/unmount on the UI thread.
// Call these methods from the UI thread only.
//
// # Example
//
//	var formKey = core.NewGlobalKey[*formState]()
//
//	type formWidget struct {
//	    core.StatefulBase
//	}
//
//	func (w formWidget) Key() any           { return formKey }
//	func (formWidget) CreateState() core.State { return &formState{} }
//
//	type formState struct {
//	    core.StateBase
//	    // ...
//	}
//
//	func (s *formState) Validate() bool { /* ... */ return true }
//
//	// From a sibling or parent widget, without passing references:
//	if state := formKey.CurrentState(); state != nil {
//	    state.Validate()
//	}
type GlobalKey[S State] struct {
	inner *globalKeyState[S]
}

// globalKeyState holds the mutable registration state for a GlobalKey.
// The id field ensures that reflect.DeepEqual distinguishes different keys
// even when both have nil elements.
type globalKeyState[S State] struct {
	id      uint64
	element Element
}

// NewGlobalKey creates a new GlobalKey with a unique identity. Each call
// returns a key that is distinct from every other key, so widget reconciliation
// will never match two widgets with different GlobalKey instances.
//
// Store the key in a package-level variable or a long-lived struct field so
// that the same key is used across rebuilds:
//
//	var myKey = core.NewGlobalKey[*myState]()
func NewGlobalKey[S State]() GlobalKey[S] {
	return GlobalKey[S]{inner: &globalKeyState[S]{
		id: globalKeyNextID.Add(1),
	}}
}

// CurrentState returns the State associated with this key's element, or the
// zero value of S if no element is currently mounted with this key. For
// non-stateful elements (stateless, render object, inherited) this always
// returns the zero value.
func (k GlobalKey[S]) CurrentState() S {
	if k.inner == nil || k.inner.element == nil {
		var zero S
		return zero
	}
	if se, ok := k.inner.element.(*StatefulElement); ok && se.state != nil {
		if typed, ok := se.state.(S); ok {
			return typed
		}
	}
	var zero S
	return zero
}

// CurrentElement returns the Element currently mounted with this key, or nil.
func (k GlobalKey[S]) CurrentElement() Element {
	if k.inner == nil {
		return nil
	}
	return k.inner.element
}

// CurrentContext returns the BuildContext for this key's element, or nil.
// The returned context is valid only while the element is mounted.
func (k GlobalKey[S]) CurrentContext() BuildContext {
	if k.inner == nil || k.inner.element == nil {
		return nil
	}
	if ctx, ok := k.inner.element.(BuildContext); ok {
		return ctx
	}
	return nil
}

// globalKeyRegistry is the internal interface used by the framework to register
// and unregister elements for global keys. It is unexported to prevent external
// implementations.
type globalKeyRegistry interface {
	globalKeyImpl() any
	setElement(Element)
	clearElement(Element)
}

// Compile-time check that GlobalKey satisfies globalKeyRegistry.
var _ globalKeyRegistry = GlobalKey[State]{}

func (k GlobalKey[S]) globalKeyImpl() any { return k.inner }

func (k GlobalKey[S]) setElement(e Element) {
	if k.inner != nil {
		k.inner.element = e
	}
}

func (k GlobalKey[S]) clearElement(e Element) {
	if k.inner != nil && k.inner.element == e {
		k.inner.element = nil
	}
}

// registerGlobalKeyIfNeeded checks if the widget's key implements globalKeyRegistry
// and registers it with the BuildOwner.
func registerGlobalKeyIfNeeded(widget Widget, element Element, owner *BuildOwner) {
	if widget == nil || owner == nil {
		return
	}
	key := widget.Key()
	if gk, ok := key.(globalKeyRegistry); ok {
		gk.setElement(element)
		owner.RegisterGlobalKey(gk.globalKeyImpl(), element)
	}
}

// unregisterGlobalKeyIfNeeded checks if the widget's key implements globalKeyRegistry
// and unregisters it from the BuildOwner.
func unregisterGlobalKeyIfNeeded(widget Widget, element Element, owner *BuildOwner) {
	if widget == nil || owner == nil {
		return
	}
	key := widget.Key()
	if gk, ok := key.(globalKeyRegistry); ok {
		gk.clearElement(element)
		owner.UnregisterGlobalKey(gk.globalKeyImpl(), element)
	}
}
