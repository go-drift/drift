package main

import (
	"strings"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
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
			Child:        formContent{parent: s},
		},

		widgets.VSpace(24),

		// Selection controls
		sectionTitle("Selection Controls", colors),
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			theme.CheckboxOf(ctx, s.acceptTerms.Get(), func(value bool) {
				s.acceptTerms.Set(value)
			}),
			widgets.HSpace(10),
			widgets.Text{Content: "Accept terms of service", Style: labelStyle(colors)},
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
			widgets.Text{Content: "Native Switch", Style: labelStyle(colors)},
		),
		widgets.VSpace(12),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			theme.ToggleOf(ctx, s.enableAlerts.Get(), func(value bool) {
				s.enableAlerts.Set(value)
			}),
			widgets.HSpace(10),
			widgets.Text{Content: "Skia Toggle", Style: labelStyle(colors)},
		),
		widgets.VSpace(16),
		widgets.Text{Content: "Contact preference", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			theme.RadioOf(ctx, "email", s.contactMethod.Get(), func(value string) {
				s.contactMethod.Set(value)
			}),
			widgets.HSpace(10),
			widgets.Text{Content: "Email", Style: labelStyle(colors)},
		),
		widgets.VSpace(6),
		widgets.RowOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentCenter,
			widgets.MainAxisSizeMin,

			theme.RadioOf(ctx, "sms", s.contactMethod.Get(), func(value string) {
				s.contactMethod.Set(value)
			}),
			widgets.HSpace(10),
			widgets.Text{Content: "SMS", Style: labelStyle(colors)},
		),
		widgets.VSpace(16),
		widgets.Text{Content: "Plan", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.DropdownOf(ctx, s.planSelection.Get(), []widgets.DropdownItem[string]{
			{Value: "starter", Label: "Starter"},
			{Value: "pro", Label: "Pro"},
			{Value: "enterprise", Label: "Enterprise"},
		}, func(value string) {
			s.planSelection.Set(value)
		}).WithHint("Select a plan"),
		widgets.VSpace(24),

		// Date & Time Pickers
		sectionTitle("Date & Time Pickers", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Select a date using the native picker", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.DatePickerOf(ctx, s.selectedDate.Get(), func(date time.Time) {
			s.selectedDate.Set(&date)
		}),
		widgets.VSpace(16),
		widgets.Text{Content: "Select a time using the native picker", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.TimePickerOf(ctx, s.selectedHour.Get(), s.selectedMin.Get(), func(hour, minute int) {
			s.selectedHour.Set(hour)
			s.selectedMin.Set(minute)
		}),
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
		theme.TextFormFieldOf(ctx).
			WithLabel("Username").
			WithPlaceholder("Enter username").
			WithHelperText("Letters and numbers only").
			WithValidator(func(value string) string {
				if value == "" {
					return "Username is required"
				}
				if len(value) < 3 {
					return "Username must be at least 3 characters"
				}
				return ""
			}).
			WithOnSaved(func(value string) {
				f.parent.data.Username = value
			}),
		widgets.VSpace(16),

		// Email field with validation
		theme.TextFormFieldOf(ctx).
			WithLabel("Email").
			WithPlaceholder("you@example.com").
			WithValidator(func(value string) string {
				if value == "" {
					return "Email is required"
				}
				if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
					return "Please enter a valid email"
				}
				return ""
			}).
			WithOnSaved(func(value string) {
				f.parent.data.Email = value
			}),
		widgets.VSpace(16),

		// Password field with validation
		theme.TextFormFieldOf(ctx).
			WithLabel("Password").
			WithPlaceholder("Enter password").
			WithHelperText("Minimum 8 characters").
			WithObscure(true).
			WithValidator(func(value string) string {
				if value == "" {
					return "Password is required"
				}
				if len(value) < 8 {
					return "Password must be at least 8 characters"
				}
				return ""
			}).
			WithOnSaved(func(value string) {
				f.parent.data.Password = value
			}),
		widgets.VSpace(24),

		// Buttons
		theme.ButtonOf(ctx, "Submit", func() {
			if form != nil {
				f.parent.handleSubmit(form)
			}
		}),
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Reset", func() {
			if form != nil {
				f.parent.handleReset(form)
			}
		}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
		widgets.VSpace(16),

		// Status display
		widgets.Container{
			Color: colors.SurfaceVariant,
			Child: widgets.PaddingAll(12,
				widgets.Text{
					Content: f.parent.statusText.Get(),
					Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 14,
					},
				},
			),
		},
	)
}
