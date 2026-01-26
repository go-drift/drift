package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// Form groups form fields and provides validation helpers.
type Form struct {
	// ChildWidget is the form content.
	ChildWidget core.Widget
	// Autovalidate runs validators when fields change.
	Autovalidate bool
	// OnChanged is called when any field changes.
	OnChanged func()
}

func (f Form) CreateElement() core.Element {
	return core.NewStatefulElement(f, nil)
}

func (f Form) Key() any {
	return nil
}

func (f Form) CreateState() core.State {
	return &FormState{}
}

// FormState manages the state of a Form widget.
type FormState struct {
	element       *core.StatefulElement
	fields        map[formFieldState]struct{}
	generation    int
	autovalidate  bool
	onChanged     func()
	isInitialized bool
}

// SetElement stores the element for rebuilds.
func (s *FormState) SetElement(element *core.StatefulElement) {
	s.element = element
}

// InitState initializes the form state.
func (s *FormState) InitState() {
	if s.fields == nil {
		s.fields = make(map[formFieldState]struct{})
	}
}

// Build renders the form scope.
func (s *FormState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(Form)
	s.autovalidate = w.Autovalidate
	s.onChanged = w.OnChanged
	s.isInitialized = true
	return formScope{state: s, generation: s.generation, childWidget: w.ChildWidget}
}

// SetState executes fn and schedules rebuild.
func (s *FormState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

// Dispose clears registrations.
func (s *FormState) Dispose() {
	s.fields = nil
}

// DidChangeDependencies is a no-op for FormState.
func (s *FormState) DidChangeDependencies() {}

// DidUpdateWidget is a no-op for FormState.
func (s *FormState) DidUpdateWidget(oldWidget core.StatefulWidget) {}

// RegisterField registers a field with this form.
func (s *FormState) RegisterField(field formFieldState) {
	if s.fields == nil {
		s.fields = make(map[formFieldState]struct{})
	}
	s.fields[field] = struct{}{}
}

// UnregisterField unregisters a field from this form.
func (s *FormState) UnregisterField(field formFieldState) {
	if s.fields == nil {
		return
	}
	delete(s.fields, field)
}

// Validate runs validators on all fields.
func (s *FormState) Validate() bool {
	valid := true
	for field := range s.fields {
		if !field.Validate() {
			valid = false
		}
	}
	s.bumpGeneration()
	return valid
}

// Save calls OnSaved for all fields.
func (s *FormState) Save() {
	for field := range s.fields {
		field.Save()
	}
}

// Reset resets all fields to their initial values.
func (s *FormState) Reset() {
	for field := range s.fields {
		field.Reset()
	}
	s.bumpGeneration()
}

// NotifyChanged informs listeners that a field changed.
// When autovalidate is enabled, the calling field is expected to validate itself
// rather than having the form validate all fields (which would show errors on
// untouched fields). Form.Validate() can still be called explicitly to validate all.
func (s *FormState) NotifyChanged() {
	if s.onChanged != nil {
		s.onChanged()
	}
	s.bumpGeneration()
}

func (s *FormState) bumpGeneration() {
	if !s.isInitialized {
		return
	}
	s.SetState(func() {
		s.generation++
	})
}

// FormOf returns the closest FormState in the widget tree.
func FormOf(ctx core.BuildContext) *FormState {
	inherited := ctx.DependOnInherited(formScopeType, nil)
	if inherited == nil {
		return nil
	}
	if scope, ok := inherited.(formScope); ok {
		return scope.state
	}
	return nil
}

type formFieldState interface {
	Validate() bool
	Save()
	Reset()
}

type formScope struct {
	state       *FormState
	generation  int
	childWidget core.Widget
}

func (f formScope) CreateElement() core.Element {
	return core.NewInheritedElement(f, nil)
}

func (f formScope) Key() any {
	return nil
}

func (f formScope) Child() core.Widget {
	return f.childWidget
}

func (f formScope) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(formScope); ok {
		return f.generation != old.generation
	}
	return true
}

// UpdateShouldNotifyDependent returns true for any aspects since formScope
// doesn't support granular aspect tracking yet.
func (f formScope) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return f.UpdateShouldNotify(oldWidget)
}

var formScopeType = reflect.TypeOf(formScope{})

// FormField builds a field that integrates with a Form.
type FormField[T any] struct {
	// InitialValue is the field's starting value.
	InitialValue T
	// Builder renders the field using its state.
	Builder func(*FormFieldState[T]) core.Widget
	// OnSaved is called when the form is saved.
	OnSaved func(T)
	// Validator returns an error message or empty string.
	Validator func(T) string
	// OnChanged is called when the field value changes.
	OnChanged func(T)
	// Disabled controls whether the field participates in validation.
	Disabled bool
	// Autovalidate enables validation when the value changes.
	Autovalidate bool
}

func (f FormField[T]) CreateElement() core.Element {
	return core.NewStatefulElement(f, nil)
}

func (f FormField[T]) Key() any {
	return nil
}

func (f FormField[T]) CreateState() core.State {
	return &FormFieldState[T]{}
}

// FormFieldState stores mutable state for a FormField.
type FormFieldState[T any] struct {
	element         *core.StatefulElement
	value           T
	errorText       string
	hasInteracted   bool
	registeredForm  *FormState
	initializedOnce bool
}

// SetElement stores the element for rebuilds.
func (s *FormFieldState[T]) SetElement(element *core.StatefulElement) {
	s.element = element
}

// InitState initializes the field value from the widget.
func (s *FormFieldState[T]) InitState() {
	w := s.element.Widget().(FormField[T])
	s.value = w.InitialValue
	s.initializedOnce = true
}

// Build renders the field by calling Builder.
func (s *FormFieldState[T]) Build(ctx core.BuildContext) core.Widget {
	s.registerWithForm(FormOf(ctx))
	w := s.element.Widget().(FormField[T])
	if w.Builder == nil {
		return nil
	}
	return w.Builder(s)
}

// SetState executes fn and schedules rebuild.
func (s *FormFieldState[T]) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

// Dispose unregisters the field from the form.
func (s *FormFieldState[T]) Dispose() {
	if s.registeredForm != nil {
		s.registeredForm.UnregisterField(s)
	}
}

// DidChangeDependencies is a no-op for FormFieldState.
func (s *FormFieldState[T]) DidChangeDependencies() {}

// DidUpdateWidget updates value if the initial value changed before interaction.
func (s *FormFieldState[T]) DidUpdateWidget(oldWidget core.StatefulWidget) {
	oldField, ok := oldWidget.(FormField[T])
	if !ok {
		return
	}
	newField := s.element.Widget().(FormField[T])
	if s.hasInteracted {
		return
	}
	if !reflect.DeepEqual(oldField.InitialValue, newField.InitialValue) {
		s.value = newField.InitialValue
		if newField.Autovalidate {
			s.Validate()
		}
	}
}

// Value returns the current value.
func (s *FormFieldState[T]) Value() T {
	return s.value
}

// ErrorText returns the current error message.
func (s *FormFieldState[T]) ErrorText() string {
	return s.errorText
}

// HasError reports whether the field has an error.
func (s *FormFieldState[T]) HasError() bool {
	return s.errorText != ""
}

// DidChange updates the value and triggers validation/notifications.
func (s *FormFieldState[T]) DidChange(value T) {
	s.value = value
	s.hasInteracted = true
	w := s.element.Widget().(FormField[T])
	if w.OnChanged != nil {
		w.OnChanged(value)
	}
	if s.registeredForm != nil {
		s.registeredForm.NotifyChanged()
	}

	// Validate this field if form or field autovalidate is enabled.
	// Form.autovalidate enables per-field validation on change, not form-wide validation
	// (which would show errors on untouched fields). Use Form.Validate() explicitly
	// to validate all fields (e.g., on submit).
	if (s.registeredForm != nil && s.registeredForm.autovalidate) || w.Autovalidate {
		s.Validate()
		return
	}
	s.SetState(func() {})
}

// Validate runs the field validator.
func (s *FormFieldState[T]) Validate() bool {
	w := s.element.Widget().(FormField[T])
	if w.Disabled {
		s.errorText = ""
		return true
	}
	if w.Validator == nil {
		s.errorText = ""
		return true
	}
	message := w.Validator(s.value)
	if message == "" {
		s.errorText = ""
		s.SetState(func() {})
		return true
	}
	s.errorText = message
	s.SetState(func() {})
	return false
}

// Save triggers the OnSaved callback.
func (s *FormFieldState[T]) Save() {
	w := s.element.Widget().(FormField[T])
	if w.Disabled {
		return
	}
	if w.OnSaved != nil {
		w.OnSaved(s.value)
	}
}

// Reset returns the field to its initial value.
func (s *FormFieldState[T]) Reset() {
	w := s.element.Widget().(FormField[T])
	s.value = w.InitialValue
	s.errorText = ""
	s.hasInteracted = false
	if w.OnChanged != nil {
		w.OnChanged(s.value)
	}
	s.SetState(func() {})
}

func (s *FormFieldState[T]) registerWithForm(form *FormState) {
	if form == s.registeredForm {
		return
	}
	if s.registeredForm != nil {
		s.registeredForm.UnregisterField(s)
	}
	s.registeredForm = form
	if form != nil {
		form.RegisterField(s)
	}
}
