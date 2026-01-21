package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
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

// formsState demonstrates the new StateBase pattern for reduced boilerplate.
// Previously required: SetElement, SetState, Dispose, DidChangeDependencies, DidUpdateWidget
// Now only InitState and Build need to be implemented.
type formsState struct {
	core.StateBase     // Embeds all required State interface methods
	usernameController *platform.TextEditingController
	passwordController *platform.TextEditingController
	emailController    *platform.TextEditingController
	nativeController   *platform.TextEditingController
	statusText         *core.ManagedState[string]
	acceptTerms        *core.ManagedState[bool]
	enableAlerts       *core.ManagedState[bool]
	contactMethod      *core.ManagedState[string]
	planSelection      *core.ManagedState[string]
}

func (s *formsState) InitState() {
	// Controllers are auto-disposed when state is disposed
	s.usernameController = platform.NewTextEditingController("")
	s.passwordController = platform.NewTextEditingController("")
	s.emailController = platform.NewTextEditingController("")
	s.nativeController = platform.NewTextEditingController("")

	// ManagedState values auto-trigger rebuilds when Set() is called
	s.statusText = core.NewManagedState(&s.StateBase, "Fill in the form above")
	s.acceptTerms = core.NewManagedState(&s.StateBase, false)
	s.enableAlerts = core.NewManagedState(&s.StateBase, true)
	s.contactMethod = core.NewManagedState(&s.StateBase, "email")
	s.planSelection = core.NewManagedState(&s.StateBase, "")
}

func (s *formsState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Forms",
		// Username field
		sectionTitle("Text Input", colors),
		widgets.VSpace(12),
		widgets.TextField{
			Label:        "Username",
			Controller:   s.usernameController,
			Placeholder:  "Enter username",
			KeyboardType: platform.KeyboardTypeText,
			InputAction:  platform.TextInputActionNext,
			Autocorrect:  false,
			Enabled:      true,
			BorderRadius: 8,
		},
		widgets.VSpace(16),

		// Email field
		widgets.TextField{
			Label:        "Email",
			Controller:   s.emailController,
			Placeholder:  "you@example.com",
			KeyboardType: platform.KeyboardTypeEmail,
			InputAction:  platform.TextInputActionNext,
			Autocorrect:  false,
			Enabled:      true,
			BorderRadius: 8,
		},
		widgets.VSpace(16),

		// Password field
		sectionTitle("Password Input", colors),
		widgets.VSpace(12),
		widgets.TextField{
			Label:        "Password",
			Controller:   s.passwordController,
			Placeholder:  "Enter password",
			KeyboardType: platform.KeyboardTypePassword,
			InputAction:  platform.TextInputActionDone,
			Obscure:      true,
			Enabled:      true,
			BorderRadius: 8,
			OnSubmitted: func(text string) {
				s.handleSubmit()
			},
		},
		widgets.VSpace(24),

		// Native text field
		sectionTitle("Native Text Input", colors),
		widgets.VSpace(12),
		widgets.TextOf("Native notes", labelStyle(colors)),
		widgets.VSpace(8),
		widgets.NativeTextField{
			Controller:      s.nativeController,
			Placeholder:     "Native input field",
			KeyboardType:    platform.KeyboardTypeText,
			InputAction:     platform.TextInputActionDone,
			Autocorrect:     true,
			Enabled:         true,
			Height:          48,
			BorderRadius:    8,
			BackgroundColor: colors.Surface,
			BorderColor:     colors.Outline,
			Style: rendering.TextStyle{
				Color:    colors.OnSurface,
				FontSize: 16,
			},
		},
		widgets.VSpace(24),

		// Submit button
		widgets.NewButton("Submit Form", func() {
			s.handleSubmit()
		}).WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(8),
		widgets.NewButton("Clear Form", func() {
			s.clearForm()
		}).WithColor(colors.SurfaceVariant, colors.OnSurfaceVariant),
		widgets.VSpace(16),

		// Status
		widgets.NewContainer(
			widgets.PaddingAll(12,
				widgets.TextOf(s.statusText.Get(), rendering.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 14,
				}),
			),
		).WithColor(colors.SurfaceVariant).Build(),
		widgets.VSpace(24),

		// Selection controls
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
				Value: s.enableAlerts.Get(),
				OnChanged: func(value bool) {
					s.enableAlerts.Set(value)
				},
			},
			widgets.HSpace(10),
			widgets.TextOf("Enable notifications", labelStyle(colors)),
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
		widgets.VSpace(40),
	)
}

func (s *formsState) handleSubmit() {
	username := s.usernameController.Text()
	email := s.emailController.Text()
	password := s.passwordController.Text()

	if username == "" || email == "" || password == "" {
		platform.Haptics.Impact(platform.HapticError)
		s.statusText.Set("Please fill in all fields")
		return
	}

	platform.Haptics.Impact(platform.HapticSuccess)
	s.statusText.Set("Form submitted for: " + username + " (" + email + ")")
}

func (s *formsState) clearForm() {
	s.usernameController.Clear()
	s.emailController.Clear()
	s.passwordController.Clear()
	s.nativeController.Clear()
	s.statusText.Set("Form cleared")
	s.acceptTerms.Set(false)
	s.enableAlerts.Set(true)
	s.contactMethod.Set("email")
	s.planSelection.Set("")
}

// Note: SetState, Dispose, DidChangeDependencies, and DidUpdateWidget are
// now inherited from core.StateBase - no need to implement them!
