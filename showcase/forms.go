package main

import (
	"strings"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// formsPage is the forms demo widget.
type formsPage struct{ core.StatefulBase }

func (formsPage) CreateState() core.State { return &formsState{} }

// buildFormsPage creates a stateful widget for the forms demo.
func buildFormsPage(_ core.BuildContext) core.Widget { return formsPage{} }

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
	statusText    *core.Managed[string]
	acceptTerms   *core.Managed[bool]
	enableAlerts  *core.Managed[bool]
	contactMethod *core.Managed[string]
	planSelection *core.Managed[string]

	// Date & Time picker state
	selectedDate *core.Managed[*time.Time]
	selectedHour *core.Managed[int]
	selectedMin  *core.Managed[int]
}

func (s *formsState) InitState() {
	s.statusText = core.NewManaged(s, "Fill in the form and submit")
	s.acceptTerms = core.NewManaged(s, false)
	s.enableAlerts = core.NewManaged(s, true)
	s.contactMethod = core.NewManaged(s, "email")
	s.planSelection = core.NewManaged(s, "")

	// Initialize date/time state
	s.selectedDate = core.NewManaged[*time.Time](s, nil)
	s.selectedHour = core.NewManaged(s, 9)
	s.selectedMin = core.NewManaged(s, 0)
}

func (s *formsState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Forms",
		// Form validation section
		sectionTitle("Form Validation", colors),
		widgets.VSpace(12),

		// Form wraps the fields and provides validation/save/reset
		widgets.Form{
			Autovalidate: true,
			Child:        formContent{state: s},
		},

		widgets.VSpace(24),

		// Selection controls
		sectionTitle("Selection Controls", colors),
		widgets.VSpace(12),
		widgets.Row{
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				theme.CheckboxOf(ctx, s.acceptTerms.Value(), func(value bool) {
					s.acceptTerms.Set(value)
				}),
				widgets.HSpace(10),
				widgets.Text{Content: "Accept terms of service", Style: labelStyle(colors)},
			},
		},
		widgets.VSpace(12),
		widgets.Row{
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				widgets.Switch{
					OnTintColor: colors.Primary,
					Value:       s.enableAlerts.Value(),
					OnChanged: func(value bool) {
						s.enableAlerts.Set(value)
					},
				},
				widgets.HSpace(10),
				widgets.Text{Content: "Native Switch", Style: labelStyle(colors)},
			},
		},
		widgets.VSpace(12),
		widgets.Row{
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				theme.ToggleOf(ctx, s.enableAlerts.Value(), func(value bool) {
					s.enableAlerts.Set(value)
				}),
				widgets.HSpace(10),
				widgets.Text{Content: "Skia Toggle", Style: labelStyle(colors)},
			},
		},
		widgets.VSpace(16),
		widgets.Text{Content: "Contact preference", Style: labelStyle(colors)},
		widgets.VSpace(8),
		widgets.Row{
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				theme.RadioOf(ctx, "email", s.contactMethod.Value(), func(value string) {
					s.contactMethod.Set(value)
				}),
				widgets.HSpace(10),
				widgets.Text{Content: "Email", Style: labelStyle(colors)},
			},
		},
		widgets.VSpace(6),
		widgets.Row{
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				theme.RadioOf(ctx, "sms", s.contactMethod.Value(), func(value string) {
					s.contactMethod.Set(value)
				}),
				widgets.HSpace(10),
				widgets.Text{Content: "SMS", Style: labelStyle(colors)},
			},
		},
		widgets.VSpace(16),
		widgets.Text{Content: "Plan", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.DropdownOf(ctx, s.planSelection.Value(), []widgets.DropdownItem[string]{
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
		theme.DatePickerOf(ctx, s.selectedDate.Value(), func(date time.Time) {
			s.selectedDate.Set(&date)
		}),
		widgets.VSpace(16),
		widgets.Text{Content: "Select a time using the native picker", Style: labelStyle(colors)},
		widgets.VSpace(8),
		theme.TimePickerOf(ctx, s.selectedHour.Value(), s.selectedMin.Value(), func(hour, minute int) {
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
	core.StatelessBase
	state *formsState
}

func (f formContent) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
	form := widgets.FormOf(ctx)

	return widgets.Column{
		CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
		MainAxisSize:       widgets.MainAxisSizeMin,
		Children: []core.Widget{
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
					f.state.data.Username = value
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
					f.state.data.Email = value
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
					f.state.data.Password = value
				}),
			widgets.VSpace(24),

			// Buttons
			theme.ButtonOf(ctx, "Submit", func() {
				if form != nil {
					f.state.handleSubmit(form)
				}
			}),
			widgets.VSpace(8),
			theme.ButtonOf(ctx, "Reset", func() {
				if form != nil {
					f.state.handleReset(form)
				}
			}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
			widgets.VSpace(16),

			// Status display
			widgets.Container{
				Color:   colors.SurfaceVariant,
				Padding: layout.EdgeInsetsAll(12),
				Child: widgets.Text{
					Content: f.state.statusText.Value(),
					Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 14,
					},
				},
			},
		}}
}
