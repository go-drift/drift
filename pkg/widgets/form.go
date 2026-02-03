package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// Form is a container widget that groups form fields and provides coordinated
// validation, save, and reset operations.
//
// Form works with form field widgets that implement the formFieldState interface,
// such as [TextFormField] and [FormField]. These fields automatically register
// with the nearest ancestor Form when built.
//
// Use [FormOf] to obtain the [FormState] from a build context, then call its
// methods to interact with the form:
//   - Validate() validates all registered fields and returns true if all pass
//   - Save() calls OnSaved on all registered fields
//   - Reset() resets all fields to their initial values
//
// Autovalidate behavior:
//   - When Autovalidate is true, individual fields validate themselves when their
//     value changes (after user interaction).
//   - This does NOT validate untouched fields, avoiding premature error display.
//   - Call Validate() explicitly to validate all fields (e.g., on form submission).
//
// Example:
//
//	var formState *widgets.FormState
//
//	Form{
//	    Autovalidate: true,
//	    OnChanged: func() {
//	        // Called when any field changes
//	    },
//	    Child: Column{
//	        Children: []core.Widget{
//	            TextFormField{Label: "Email", Validator: validateEmail},
//	            TextFormField{Label: "Password", Obscure: true},
//	            Button{
//	                Child: Text{Content: "Submit"},
//	                OnPressed: func() {
//	                    if formState.Validate() {
//	                        formState.Save()
//	                    }
//	                },
//	            },
//	        },
//	    },
//	}
type Form struct {
	// Child is the form content.
	Child core.Widget
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

// FormState manages the state of a [Form] widget and provides methods to
// interact with all registered form fields.
//
// Obtain a FormState using [FormOf] from within a build context, or by storing
// a reference when building the form.
//
// Methods:
//   - Validate() bool: Validates all fields and returns true if all pass.
//   - Save(): Calls OnSaved on all fields (typically after successful validation).
//   - Reset(): Resets all fields to their initial values and clears errors.
//
// FormState tracks a generation counter that increments on validation, reset,
// and field changes, triggering rebuilds of dependent widgets.
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
	return formScope{state: s, generation: s.generation, child: w.Child}
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

// FormOf returns the [FormState] of the nearest ancestor [Form] widget,
// or nil if there is no Form ancestor.
//
// Form fields like [TextFormField] use this internally to register with their
// parent form. You can also use it to obtain the FormState for calling
// Validate, Save, or Reset.
//
// Example:
//
//	func (s *myWidgetState) Build(ctx core.BuildContext) core.Widget {
//	    formState := widgets.FormOf(ctx)
//	    return Button{
//	        Child: Text{Content: "Submit"},
//	        OnPressed: func() {
//	            if formState != nil && formState.Validate() {
//	                formState.Save()
//	            }
//	        },
//	    }
//	}
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

type formFieldStateBase struct {
	element        *core.StatefulElement
	errorText      string
	hasInteracted  bool
	registeredForm *FormState
}

func (s *formFieldStateBase) setElement(element *core.StatefulElement) {
	s.element = element
}

func (s *formFieldStateBase) setState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *formFieldStateBase) registerWithForm(form *FormState, owner formFieldState) {
	if form == s.registeredForm {
		return
	}
	if s.registeredForm != nil {
		s.registeredForm.UnregisterField(owner)
	}
	s.registeredForm = form
	if form != nil {
		form.RegisterField(owner)
	}
}

func (s *formFieldStateBase) unregisterFromForm(owner formFieldState) {
	if s.registeredForm != nil {
		s.registeredForm.UnregisterField(owner)
	}
}

func (s *formFieldStateBase) didChange(autovalidate bool, onChanged func(), validate func() bool) {
	s.hasInteracted = true
	if onChanged != nil {
		onChanged()
	}
	if s.registeredForm != nil {
		s.registeredForm.NotifyChanged()
	}

	// Validate this field if form or field autovalidate is enabled.
	// Form.autovalidate enables per-field validation on change, not form-wide validation
	// (which would show errors on untouched fields). Use Form.Validate() explicitly
	// to validate all fields (e.g., on submit).
	if (s.registeredForm != nil && s.registeredForm.autovalidate) || autovalidate {
		validate()
		return
	}

	s.setState(func() {})
}

func (s *formFieldStateBase) validate(disabled bool, validator func() string) bool {
	if disabled {
		s.errorText = ""
		s.setState(func() {})
		return true
	}
	if validator == nil {
		s.errorText = ""
		s.setState(func() {})
		return true
	}
	message := validator()
	if message == "" {
		s.errorText = ""
		s.setState(func() {})
		return true
	}
	s.errorText = message
	s.setState(func() {})
	return false
}

func (s *formFieldStateBase) resetState() {
	s.errorText = ""
	s.hasInteracted = false
}

type formScope struct {
	state      *FormState
	generation int
	child      core.Widget
}

func (f formScope) CreateElement() core.Element {
	return core.NewInheritedElement(f, nil)
}

func (f formScope) Key() any {
	return nil
}

func (f formScope) ChildWidget() core.Widget {
	return f.child
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

// FormField is a generic form field widget for building custom form inputs
// that integrate with [Form] for validation, save, and reset operations.
//
// Unlike [TextFormField] which is specialized for text input, FormField[T]
// can wrap any input widget type and manage values of any type T.
//
// The Builder function receives the [FormFieldState] and should return a widget
// that displays the current value and calls DidChange when the value changes.
//
// Example (custom checkbox field):
//
//	FormField[bool]{
//	    InitialValue: false,
//	    Validator: func(checked bool) string {
//	        if !checked {
//	            return "You must accept the terms"
//	        }
//	        return ""
//	    },
//	    Builder: func(state *widgets.FormFieldState[bool]) core.Widget {
//	        return Row{
//	            Children: []core.Widget{
//	                Checkbox{
//	                    Value: state.Value(),
//	                    OnChanged: func(v bool) { state.DidChange(v) },
//	                },
//	                Text{Content: "I accept the terms"},
//	                if state.HasError() {
//	                    Text{Content: state.ErrorText(), Style: errorStyle},
//	                },
//	            },
//	        }
//	    },
//	    OnSaved: func(checked bool) {
//	        // Called when FormState.Save() is invoked
//	    },
//	}
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

// FormFieldState stores the mutable state for a [FormField] and provides methods
// for querying and updating the field value.
//
// Methods:
//   - Value() T: Returns the current field value.
//   - ErrorText() string: Returns the current validation error message, or empty string.
//   - HasError() bool: Returns true if there is a validation error.
//   - DidChange(T): Call this when the field value changes to update state and trigger validation.
//   - Validate() bool: Runs the validator and returns true if valid.
//   - Save(): Calls the OnSaved callback with the current value.
//   - Reset(): Resets to InitialValue and clears errors.
type FormFieldState[T any] struct {
	formFieldStateBase
	value           T
	initializedOnce bool
}

// SetElement stores the element for rebuilds.
func (s *FormFieldState[T]) SetElement(element *core.StatefulElement) {
	s.formFieldStateBase.setElement(element)
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
	s.formFieldStateBase.setState(fn)
}

// Dispose unregisters the field from the form.
func (s *FormFieldState[T]) Dispose() {
	s.formFieldStateBase.unregisterFromForm(s)
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
	w := s.element.Widget().(FormField[T])
	s.formFieldStateBase.didChange(
		w.Autovalidate,
		func() {
			if w.OnChanged != nil {
				w.OnChanged(value)
			}
		},
		s.Validate,
	)
}

// Validate runs the field validator.
func (s *FormFieldState[T]) Validate() bool {
	w := s.element.Widget().(FormField[T])
	var validator func() string
	if w.Validator != nil {
		validator = func() string {
			return w.Validator(s.value)
		}
	}
	return s.formFieldStateBase.validate(w.Disabled, validator)
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
	s.formFieldStateBase.resetState()
	if w.OnChanged != nil {
		w.OnChanged(s.value)
	}
	s.SetState(func() {})
}

func (s *FormFieldState[T]) registerWithForm(form *FormState) {
	s.formFieldStateBase.registerWithForm(form, s)
}
