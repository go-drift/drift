package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// TextFormField is a form-aware text input that wraps [TextField] and integrates
// with [Form] for validation, save, and reset operations.
//
// TextFormField automatically registers with the nearest ancestor [Form] widget
// and participates in form-wide validation, save, and reset operations. It manages
// its own internal controller if none is provided.
//
// Validation behavior:
//   - When Autovalidate is true on the field, or on the parent Form, the Validator
//     function is called whenever the field value changes after user interaction.
//   - Disabled fields skip validation entirely.
//   - Call FormState.Validate() to validate all fields at once (e.g., on submit).
//
// Controller vs InitialValue:
//   - If Controller is provided, it is the source of truth and InitialValue is ignored.
//   - If no Controller is provided, TextFormField creates an internal controller
//     initialized with InitialValue.
//
// Example:
//
//	Form{
//	    Child: Column{
//	        Children: []core.Widget{
//	            TextFormField{
//	                Label:       "Username",
//	                Placeholder: "Enter username",
//	                Validator: func(value string) string {
//	                    if len(value) < 3 {
//	                        return "Username must be at least 3 characters"
//	                    }
//	                    return ""
//	                },
//	                OnSaved: func(value string) {
//	                    // Called when FormState.Save() is invoked
//	                },
//	            },
//	        },
//	    },
//	}
type TextFormField struct {
	// TextField provides styling defaults. If set, its styling properties are used
	// as a base, with individual properties below taking precedence if non-zero.
	// This enables: TextFormField{TextField: theme.TextFieldOf(ctx, nil), ...}
	TextField TextField

	// Controller manages the text content and selection.
	// If provided, it is the source of truth and InitialValue is ignored.
	Controller *platform.TextEditingController

	// InitialValue is the field's starting value when no Controller is provided.
	InitialValue string

	// Validator returns an error message or empty string if valid.
	Validator func(string) string

	// OnSaved is called when the form is saved.
	OnSaved func(string)

	// OnChanged is called when the field value changes.
	OnChanged func(string)

	// Autovalidate enables validation when the value changes.
	Autovalidate bool

	// Label is shown above the field.
	Label string

	// Placeholder text shown when empty.
	Placeholder string

	// HelperText is shown below the field when no error.
	HelperText string

	// KeyboardType specifies the keyboard to show.
	KeyboardType platform.KeyboardType

	// InputAction specifies the keyboard action button.
	InputAction platform.TextInputAction

	// Obscure hides the text (for passwords).
	Obscure bool

	// Autocorrect enables auto-correction.
	Autocorrect bool

	// OnSubmitted is called when the user submits.
	OnSubmitted func(string)

	// OnEditingComplete is called when editing is complete.
	OnEditingComplete func()

	// Disabled controls whether the field rejects input and validation.
	Disabled bool

	// Width of the text field (0 = expand to fill).
	Width float64

	// Height of the text field.
	Height float64

	// Padding inside the text field.
	Padding layout.EdgeInsets

	// BackgroundColor of the text field.
	BackgroundColor graphics.Color

	// BorderColor of the text field.
	BorderColor graphics.Color

	// FocusColor of the text field outline.
	FocusColor graphics.Color

	// BorderRadius for rounded corners.
	BorderRadius float64

	// Style for the text.
	Style graphics.TextStyle

	// PlaceholderColor for the placeholder text.
	PlaceholderColor graphics.Color

	// LabelStyle for the label text above the field.
	LabelStyle graphics.TextStyle

	// HelperStyle for helper/error text below the field.
	HelperStyle graphics.TextStyle

	// ErrorColor for error text and border when validation fails.
	ErrorColor graphics.Color
}

// WithLabel sets the label text shown above the field.
func (t TextFormField) WithLabel(label string) TextFormField {
	t.Label = label
	return t
}

// WithPlaceholder sets the placeholder text shown when the field is empty.
func (t TextFormField) WithPlaceholder(placeholder string) TextFormField {
	t.Placeholder = placeholder
	return t
}

// WithHelperText sets the helper text shown below the field.
func (t TextFormField) WithHelperText(helper string) TextFormField {
	t.HelperText = helper
	return t
}

// WithValidator sets the validation function.
func (t TextFormField) WithValidator(validator func(string) string) TextFormField {
	t.Validator = validator
	return t
}

// WithOnSaved sets the callback invoked when the form is saved.
func (t TextFormField) WithOnSaved(onSaved func(string)) TextFormField {
	t.OnSaved = onSaved
	return t
}

// WithOnChanged sets the callback invoked when the field value changes.
func (t TextFormField) WithOnChanged(onChanged func(string)) TextFormField {
	t.OnChanged = onChanged
	return t
}

// WithAutovalidate enables validation on every value change.
func (t TextFormField) WithAutovalidate(autovalidate bool) TextFormField {
	t.Autovalidate = autovalidate
	return t
}

// WithInitialValue sets the initial value when no controller is provided.
func (t TextFormField) WithInitialValue(value string) TextFormField {
	t.InitialValue = value
	return t
}

// WithController sets the text editing controller.
func (t TextFormField) WithController(controller *platform.TextEditingController) TextFormField {
	t.Controller = controller
	return t
}

// WithObscure sets whether the text is obscured (for passwords).
func (t TextFormField) WithObscure(obscure bool) TextFormField {
	t.Obscure = obscure
	return t
}

// WithDisabled sets whether the field is disabled.
func (t TextFormField) WithDisabled(disabled bool) TextFormField {
	t.Disabled = disabled
	return t
}

// CreateElement creates the element for the stateful widget.
func (t TextFormField) CreateElement() core.Element {
	return core.NewStatefulElement(t, nil)
}

// Key returns the widget key.
func (t TextFormField) Key() any {
	return nil
}

// CreateState creates the state for this widget.
func (t TextFormField) CreateState() core.State {
	return &textFormFieldState{}
}

// textFormFieldState manages form field state and implements formFieldState interface.
type textFormFieldState struct {
	formFieldStateBase
	controller        *platform.TextEditingController // Internal controller if not provided
	currentController *platform.TextEditingController // The controller we're currently listening to
	unsubscribe       func()                          // Unsubscribe from controller listener
	initialText       string                          // Captured once in InitState (or when controller changes)
	value             string                          // Current value
	resetting         bool                            // True during Reset() to suppress listener
}

// SetElement stores the element for rebuilds.
func (s *textFormFieldState) SetElement(element *core.StatefulElement) {
	s.formFieldStateBase.setElement(element)
}

// InitState initializes the field value from the widget.
func (s *textFormFieldState) InitState() {
	w := s.element.Widget().(TextFormField)

	if w.Controller != nil {
		// User-provided controller: capture initial text once
		s.initialText = w.Controller.Text()
		s.value = s.initialText
		s.subscribeToController(w.Controller)
	} else {
		// No controller: create internal one with InitialValue
		s.controller = platform.NewTextEditingController(w.InitialValue)
		s.initialText = w.InitialValue
		s.value = w.InitialValue
	}
}

// subscribeToController sets up listener for the given controller.
func (s *textFormFieldState) subscribeToController(ctrl *platform.TextEditingController) {
	// Unsubscribe from previous controller if any
	if s.unsubscribe != nil {
		s.unsubscribe()
		s.unsubscribe = nil
	}
	s.currentController = ctrl

	if ctrl == nil {
		return
	}

	// Add listener and store unsubscribe function
	s.unsubscribe = ctrl.AddListener(func() {
		// Guard against callbacks after dispose or during reset
		if s.element == nil || s.resetting {
			return
		}
		currentText := ctrl.Text()
		if s.value != currentText {
			s.didChange(currentText)
		}
	})
}

// Build renders the TextField with error state from validation.
func (s *textFormFieldState) Build(ctx core.BuildContext) core.Widget {
	s.registerWithForm(FormOf(ctx))
	w := s.element.Widget().(TextFormField)

	// Use provided controller or internal one
	controller := w.Controller
	if controller == nil {
		controller = s.controller
	}

	// Start with provided TextField as base (for theme defaults)
	tf := w.TextField

	// Always set controller and form-managed fields
	tf.Controller = controller
	tf.ErrorText = s.errorText
	tf.OnChanged = func(text string) { s.didChange(text) }

	// Override content fields if set
	if w.Label != "" {
		tf.Label = w.Label
	}
	if w.Placeholder != "" {
		tf.Placeholder = w.Placeholder
	}
	if w.HelperText != "" {
		tf.HelperText = w.HelperText
	}

	// Override behavior fields
	if w.KeyboardType != 0 {
		tf.KeyboardType = w.KeyboardType
	}
	if w.InputAction != 0 {
		tf.InputAction = w.InputAction
	}
	if w.Obscure {
		tf.Obscure = true
	}
	if w.Autocorrect {
		tf.Autocorrect = true
	}
	if w.OnSubmitted != nil {
		tf.OnSubmitted = w.OnSubmitted
	}
	if w.OnEditingComplete != nil {
		tf.OnEditingComplete = w.OnEditingComplete
	}
	if w.Disabled {
		tf.Disabled = true
	}

	// Override styling fields if explicitly set
	if w.Width != 0 {
		tf.Width = w.Width
	}
	if w.Height != 0 {
		tf.Height = w.Height
	}
	if w.Padding != (layout.EdgeInsets{}) {
		tf.Padding = w.Padding
	}
	if w.BackgroundColor != 0 {
		tf.BackgroundColor = w.BackgroundColor
	}
	if w.BorderColor != 0 {
		tf.BorderColor = w.BorderColor
	}
	if w.FocusColor != 0 {
		tf.FocusColor = w.FocusColor
	}
	if w.BorderRadius != 0 {
		tf.BorderRadius = w.BorderRadius
	}
	if w.Style != (graphics.TextStyle{}) {
		tf.Style = w.Style
	}
	if w.PlaceholderColor != 0 {
		tf.PlaceholderColor = w.PlaceholderColor
	}
	if w.LabelStyle != (graphics.TextStyle{}) {
		tf.LabelStyle = w.LabelStyle
	}
	if w.HelperStyle != (graphics.TextStyle{}) {
		tf.HelperStyle = w.HelperStyle
	}
	if w.ErrorColor != 0 {
		tf.ErrorColor = w.ErrorColor
	}

	return tf
}

// SetState executes fn and schedules rebuild.
func (s *textFormFieldState) SetState(fn func()) {
	s.formFieldStateBase.setState(fn)
}

// Dispose unregisters the field from the form and cleans up listeners.
func (s *textFormFieldState) Dispose() {
	// Unsubscribe from controller listener
	if s.unsubscribe != nil {
		s.unsubscribe()
		s.unsubscribe = nil
	}
	s.currentController = nil

	// Unregister from form
	s.formFieldStateBase.unregisterFromForm(s)
}

// DidChangeDependencies is a no-op.
func (s *textFormFieldState) DidChangeDependencies() {}

// DidUpdateWidget handles widget updates.
func (s *textFormFieldState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	oldField, ok := oldWidget.(TextFormField)
	if !ok {
		return
	}
	newField := s.element.Widget().(TextFormField)

	// Handle controller changes
	if oldField.Controller != newField.Controller {
		if newField.Controller != nil {
			// New controller provided - subscribe to it and update initialText
			s.initialText = newField.Controller.Text()
			s.value = s.initialText
			s.subscribeToController(newField.Controller)
		} else {
			// Switched from provided to no controller - create internal one
			s.unsubscribe()
			s.unsubscribe = nil
			s.currentController = nil
			s.controller = platform.NewTextEditingController(newField.InitialValue)
			s.initialText = newField.InitialValue
			s.value = newField.InitialValue
		}
	}

	// Update InitialValue if not interacted and using internal controller
	if !s.hasInteracted && newField.Controller == nil && oldField.InitialValue != newField.InitialValue {
		s.value = newField.InitialValue
		s.initialText = newField.InitialValue
		if s.controller != nil {
			s.controller.SetText(newField.InitialValue)
		}
		if newField.Autovalidate {
			s.Validate()
		}
	}
}

// Value returns the current value.
func (s *textFormFieldState) Value() string {
	return s.value
}

// ErrorText returns the current error message.
func (s *textFormFieldState) ErrorText() string {
	return s.errorText
}

// HasError reports whether the field has an error.
func (s *textFormFieldState) HasError() bool {
	return s.errorText != ""
}

// didChange updates the value and triggers validation/notifications.
func (s *textFormFieldState) didChange(value string) {
	s.value = value
	w := s.element.Widget().(TextFormField)
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

// Validate implements formFieldState. Runs the field validator.
func (s *textFormFieldState) Validate() bool {
	w := s.element.Widget().(TextFormField)
	var validator func() string
	if w.Validator != nil {
		validator = func() string {
			return w.Validator(s.value)
		}
	}
	return s.formFieldStateBase.validate(w.Disabled, validator)
}

// Save implements formFieldState. Triggers the OnSaved callback.
func (s *textFormFieldState) Save() {
	w := s.element.Widget().(TextFormField)
	if w.Disabled {
		return
	}
	if w.OnSaved != nil {
		w.OnSaved(s.value)
	}
}

// Reset implements formFieldState. Resets the field to its initial value.
func (s *textFormFieldState) Reset() {
	w := s.element.Widget().(TextFormField)

	// Set flag to suppress controller listener during reset
	s.resetting = true

	s.value = s.initialText
	s.formFieldStateBase.resetState()

	// Reset controller to initial text
	ctrl := w.Controller
	if ctrl == nil {
		ctrl = s.controller
	}
	if ctrl != nil {
		ctrl.SetText(s.initialText)
	}

	s.resetting = false

	// Notify once after all state is consistent
	if w.OnChanged != nil {
		w.OnChanged(s.value)
	}

	s.SetState(func() {})
}

func (s *textFormFieldState) registerWithForm(form *FormState) {
	s.formFieldStateBase.registerWithForm(form, s)
}
