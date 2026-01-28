package main

import (
	"strings"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildFormsPage creates a stateful widget for the forms demo.
func buildFormsPage(ctx core.BuildContext) core.Widget {
	return formsPage{}
}

type formsPage struct{}

func (f formsPage) CreateElement() core.Element {
	return core.NewStatefulElement(f, nil)
}

func (f formsPage) Key() any {
	return nil
}

func (f formsPage) CreateState() core.State {
	return &formsState{}
}

// formData holds the collected form values after validation.
type formData struct {
	Username string
	Email    string
	Password string
}

// formsState demonstrates Form and TextFormField with validation.
type formsState struct {
	core.StateBase
	data          formData
	statusText    *core.ManagedState[string]
	acceptTerms   *core.ManagedState[bool]
	enableAlerts  *core.ManagedState[bool]
	contactMethod *core.ManagedState[string]
	planSelection *core.ManagedState[string]

	// Date & Time picker state
	selectedDate *core.ManagedState[*time.Time]
	selectedHour *core.ManagedState[int]
	selectedMin  *core.ManagedState[int]

	// Progress indicator state
	progressValue *core.ManagedState[float64]
	isLoading     *core.ManagedState[bool]
}

func (s *formsState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Fill in the form and submit")
	s.acceptTerms = core.NewManagedState(&s.StateBase, false)
	s.enableAlerts = core.NewManagedState(&s.StateBase, true)
	s.contactMethod = core.NewManagedState(&s.StateBase, "email")
	s.planSelection = core.NewManagedState(&s.StateBase, "")

	// Initialize date/time state
	s.selectedDate = core.NewManagedState[*time.Time](&s.StateBase, nil)
	s.selectedHour = core.NewManagedState(&s.StateBase, 9)
	s.selectedMin = core.NewManagedState(&s.StateBase, 0)

	// Initialize progress state
	s.progressValue = core.NewManagedState(&s.StateBase, 0.35)
	s.isLoading = core.NewManagedState(&s.StateBase, false)
}

func (s *formsState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Forms",
		// Form validation section
		sectionTitle("Form Validation", colors),
		widgets.VSpace(12),

		// Form wraps the fields and provides validation/save/reset
		widgets.Form{
			Autovalidate: true,
			ChildWidget:  formContent{parent: s},
		},

		widgets.VSpace(24),

		// Selection controls (unchanged)
		sectionTitle("Selection Controls", colors),
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.Checkbox{
				Value: s.acceptTerms.Get(),
				OnChanged: func(value bool) {
					s.acceptTerms.Set(value)
				},
			},
			widgets.HSpace(10),
			widgets.TextOf("Accept terms of service", labelStyle(colors)),
		),
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.Switch{
				OnTintColor: colors.Primary,
				Value:       s.enableAlerts.Get(),
				OnChanged: func(value bool) {
					s.enableAlerts.Set(value)
				},
			},
			widgets.HSpace(10),
			widgets.TextOf("Native Switch", labelStyle(colors)),
		),
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.Toggle{
				Value: s.enableAlerts.Get(),
				OnChanged: func(value bool) {
					s.enableAlerts.Set(value)
				},
			},
			widgets.HSpace(10),
			widgets.TextOf("Skia Toggle", labelStyle(colors)),
		),
		widgets.VSpace(16),
		widgets.TextOf("Contact preference", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.Radio[string]{
				Value:      "email",
				GroupValue: s.contactMethod.Get(),
				OnChanged: func(value string) {
					s.contactMethod.Set(value)
				},
			},
			widgets.HSpace(10),
			widgets.TextOf("Email", labelStyle(colors)),
		),
		widgets.VSpace(6),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.Radio[string]{
				Value:      "sms",
				GroupValue: s.contactMethod.Get(),
				OnChanged: func(value string) {
					s.contactMethod.Set(value)
				},
			},
			widgets.HSpace(10),
			widgets.TextOf("SMS", labelStyle(colors)),
		),
		widgets.VSpace(16),
		widgets.TextOf("Plan", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.Dropdown[string]{
			Value: s.planSelection.Get(),
			Hint:  "Select a plan",
			Items: []widgets.DropdownItem[string]{
				{Value: "starter", Label: "Starter"},
				{Value: "pro", Label: "Pro"},
				{Value: "enterprise", Label: "Enterprise"},
			},
			OnChanged: func(value string) {
				s.planSelection.Set(value)
			},
			BorderRadius: 8,
		},
		widgets.VSpace(24),

		// Date & Time Pickers
		sectionTitle("Date & Time Pickers", colors),
		widgets.VSpace(12),
		widgets.TextOf("Select a date using the native picker", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.DatePicker{
			Value: s.selectedDate.Get(),
			OnChanged: func(date time.Time) {
				s.selectedDate.Set(&date)
			},
			Placeholder: "Select date",
			Decoration: &widgets.InputDecoration{
				LabelText:    "Birth Date",
				HintText:     "Tap to select",
				BorderRadius: 8,
			},
		},
		widgets.VSpace(16),
		widgets.TextOf("Select a time using the native picker", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.TimePicker{
			Hour:   s.selectedHour.Get(),
			Minute: s.selectedMin.Get(),
			OnChanged: func(hour, minute int) {
				s.selectedHour.Set(hour)
				s.selectedMin.Set(minute)
			},
			Decoration: &widgets.InputDecoration{
				LabelText:    "Appointment Time",
				HintText:     "Tap to select",
				BorderRadius: 8,
			},
		},
		widgets.VSpace(24),

		// Progress Indicators
		sectionTitle("Progress Indicators", colors),
		widgets.VSpace(12),

		// Native Activity Indicator
		widgets.TextOf("Native Activity Indicator", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.ActivityIndicator{
				Animating: s.isLoading.Get(),
				Size:      widgets.ActivityIndicatorSizeSmall,
			},
			widgets.HSpace(16),
			widgets.ActivityIndicator{
				Animating: s.isLoading.Get(),
				Size:      widgets.ActivityIndicatorSizeMedium,
			},
			widgets.HSpace(16),
			widgets.ActivityIndicator{
				Animating: s.isLoading.Get(),
				Size:      widgets.ActivityIndicatorSizeLarge,
				Color:     colors.Primary,
			},
		),
		widgets.VSpace(16),

		// Circular Progress Indicators (indeterminate only when loading)
		widgets.TextOf("Circular Progress (toggle loading to animate)", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.CircularProgressIndicator{
				Value: s.indeterminateValue(),
				Size:  24,
			},
			widgets.HSpace(16),
			widgets.CircularProgressIndicator{
				Value: s.indeterminateValue(),
				Size:  36,
				Color: colors.Secondary,
			},
			widgets.HSpace(16),
			widgets.CircularProgressIndicator{
				Value: s.indeterminateValue(),
				Size:  48,
				Color: colors.Tertiary,
			},
		),
		widgets.VSpace(16),
		widgets.TextOf("Circular Progress (Determinate: "+itoa(int(s.progressValue.Get()*100))+"%)", labelStyle(colors)),
		widgets.VSpace(8),
		s.buildDeterminateCircular(colors),
		widgets.VSpace(16),

		// Linear Progress Indicators (indeterminate only when loading)
		widgets.TextOf("Linear Progress (toggle loading to animate)", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.LinearProgressIndicator{
			Value: s.indeterminateValue(),
		},
		widgets.VSpace(16),
		widgets.TextOf("Linear Progress (Determinate: "+itoa(int(s.progressValue.Get()*100))+"%)", labelStyle(colors)),
		widgets.VSpace(8),
		s.buildDeterminateLinear(colors),
		widgets.VSpace(16),

		// Progress control buttons
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			widgets.ButtonOf("-10%", func() {
				v := s.progressValue.Get() - 0.1
				if v < 0 {
					v = 0
				}
				s.progressValue.Set(v)
			}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			widgets.HSpace(8),
			widgets.ButtonOf("+10%", func() {
				v := s.progressValue.Get() + 0.1
				if v > 1 {
					v = 1
				}
				s.progressValue.Set(v)
			}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			widgets.HSpace(16),
			widgets.ButtonOf("Toggle Loading", func() {
				s.isLoading.Set(!s.isLoading.Get())
			}).WithColor(colors.Primary, colors.OnPrimary),
		),
		widgets.VSpace(40),
	)
}

func (s *formsState) handleSubmit(form *widgets.FormState) {
	if !form.Validate() {
		platform.Haptics.Impact(platform.HapticError)
		s.statusText.Set("Please fix the errors above")
		return
	}

	form.Save()
	platform.Haptics.Impact(platform.HapticSuccess)
	s.statusText.Set("Submitted: " + s.data.Username + " (" + s.data.Email + ")")
}

func (s *formsState) handleReset(form *widgets.FormState) {
	form.Reset()
	s.data = formData{}
	s.acceptTerms.Set(false)
	s.enableAlerts.Set(true)
	s.contactMethod.Set("email")
	s.planSelection.Set("")
	s.statusText.Set("Form reset")
}

// formContent is a separate widget so it can access FormOf(ctx).
type formContent struct {
	parent *formsState
}

func (f formContent) CreateElement() core.Element {
	return core.NewStatelessElement(f, nil)
}

func (f formContent) Key() any {
	return nil
}

func (f formContent) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	form := widgets.FormOf(ctx)

	return widgets.ColumnOf(
		widgets.MainAxisAlignmentStart,
		widgets.CrossAxisAlignmentStretch,
		widgets.MainAxisSizeMin,

		// Username field with validation
		widgets.TextFormField{
			Label:        "Username",
			Placeholder:  "Enter username",
			KeyboardType: platform.KeyboardTypeText,
			InputAction:  platform.TextInputActionNext,
			Autocorrect:  false,
			BorderRadius: 8,
			HelperText:   "Letters and numbers only",
			Validator: func(value string) string {
				if value == "" {
					return "Username is required"
				}
				if len(value) < 3 {
					return "Username must be at least 3 characters"
				}
				return ""
			},
			OnSaved: func(value string) {
				f.parent.data.Username = value
			},
		},
		widgets.VSpace(16),

		// Email field with validation
		widgets.TextFormField{
			Label:        "Email",
			Placeholder:  "you@example.com",
			KeyboardType: platform.KeyboardTypeEmail,
			InputAction:  platform.TextInputActionNext,
			Autocorrect:  false,
			BorderRadius: 8,
			Validator: func(value string) string {
				if value == "" {
					return "Email is required"
				}
				if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
					return "Please enter a valid email"
				}
				return ""
			},
			OnSaved: func(value string) {
				f.parent.data.Email = value
			},
		},
		widgets.VSpace(16),

		// Password field with validation
		widgets.TextFormField{
			Label:        "Password",
			Placeholder:  "Enter password",
			KeyboardType: platform.KeyboardTypeText,
			InputAction:  platform.TextInputActionDone,
			Obscure:      true,
			Autocorrect:  false,
			BorderRadius: 8,
			HelperText:   "Minimum 8 characters",
			Validator: func(value string) string {
				if value == "" {
					return "Password is required"
				}
				if len(value) < 8 {
					return "Password must be at least 8 characters"
				}
				return ""
			},
			OnSaved: func(value string) {
				f.parent.data.Password = value
			},
			OnSubmitted: func(value string) {
				if form != nil {
					f.parent.handleSubmit(form)
				}
			},
		},
		widgets.VSpace(24),

		// Buttons
		widgets.ButtonOf("Submit", func() {
			if form != nil {
				f.parent.handleSubmit(form)
			}
		}).WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		widgets.ButtonOf("Reset", func() {
			if form != nil {
				f.parent.handleReset(form)
			}
		}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
		widgets.VSpace(16),

		// Status display
		widgets.Container{
			Color: colors.SurfaceVariant,
			ChildWidget: widgets.PaddingAll(12,
				widgets.TextOf(f.parent.statusText.Get(), graphics.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 14,
				}),
			),
		},
	)
}

// indeterminateValue returns nil when loading (indeterminate animation) or 0 when not (stopped).
func (s *formsState) indeterminateValue() *float64 {
	if s.isLoading.Get() {
		return nil // Indeterminate - will animate
	}
	zero := 0.0
	return &zero // Determinate at 0 - no animation
}

// buildDeterminateCircular creates a determinate circular progress indicator.
func (s *formsState) buildDeterminateCircular(colors theme.ColorScheme) core.Widget {
	progress := s.progressValue.Get()
	return widgets.CircularProgressIndicator{
		Value:       &progress,
		Size:        48,
		StrokeWidth: 5,
		Color:       colors.Primary,
		TrackColor:  colors.SurfaceVariant,
	}
}

// buildDeterminateLinear creates a determinate linear progress indicator.
func (s *formsState) buildDeterminateLinear(colors theme.ColorScheme) core.Widget {
	progress := s.progressValue.Get()
	return widgets.LinearProgressIndicator{
		Value:        &progress,
		Height:       6,
		BorderRadius: 3,
		Color:        colors.Primary,
		TrackColor:   colors.SurfaceVariant,
	}
}
