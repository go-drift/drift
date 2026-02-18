package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

func buildPreferencesPage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *preferencesState { return &preferencesState{} })
}

type preferencesState struct {
	core.StateBase
	statusText      *core.ManagedState[string]
	storedValue     *core.ManagedState[string]
	keyController   *platform.TextEditingController
	valueController *platform.TextEditingController
}

func (s *preferencesState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Enter a key and value to store.")
	s.storedValue = core.NewManagedState(&s.StateBase, "")
	s.keyController = platform.NewTextEditingController("my_key")
	s.valueController = platform.NewTextEditingController("my_value")
}

func (s *preferencesState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Preferences",
		sectionTitle("Key-Value Storage", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Simple, unencrypted storage for app preferences.", Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.TextFieldOf(ctx, s.keyController).
			WithLabel("Key").
			WithPlaceholder("Enter key name"),
		widgets.VSpace(12),
		theme.TextFieldOf(ctx, s.valueController).
			WithLabel("Value").
			WithPlaceholder("Enter value"),
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Save", func() {
					s.saveValue()
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Get", func() {
					s.getValue()
				}).WithColor(colors.Secondary, colors.OnSecondary),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Delete", func() {
					s.deleteValue()
				}).WithColor(colors.Error, colors.OnError),
			},
		},
		widgets.VSpace(16),
		prefsValueCard(s.storedValue.Get(), colors),
		widgets.VSpace(16),
		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *preferencesState) saveValue() {
	key := s.keyController.Text()
	value := s.valueController.Text()

	if key == "" || value == "" {
		s.statusText.Set("Please enter both key and value")
		return
	}

	err := platform.Preferences.Set(key, value)
	if err != nil {
		s.statusText.Set("Error saving: " + err.Error())
		return
	}

	s.statusText.Set("Saved value for key: " + key)
}

func (s *preferencesState) getValue() {
	key := s.keyController.Text()

	if key == "" {
		s.statusText.Set("Please enter a key name")
		return
	}

	value, err := platform.Preferences.Get(key)
	if err != nil {
		s.statusText.Set("Error retrieving: " + err.Error())
		s.storedValue.Set("")
		return
	}

	if value == "" {
		s.statusText.Set("No value found for key: " + key)
		s.storedValue.Set("")
		return
	}

	s.storedValue.Set(value)
	s.statusText.Set("Retrieved value for key: " + key)
}

func (s *preferencesState) deleteValue() {
	key := s.keyController.Text()

	if key == "" {
		s.statusText.Set("Please enter a key name")
		return
	}

	err := platform.Preferences.Delete(key)
	if err != nil {
		s.statusText.Set("Error deleting: " + err.Error())
		return
	}

	s.storedValue.Set("")
	s.statusText.Set("Deleted key: " + key)
}

func prefsValueCard(value string, colors theme.ColorScheme) core.Widget {
	displayValue := value
	if displayValue == "" {
		displayValue = "(no value retrieved)"
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding: layout.EdgeInsetsAll(12),
		Child: widgets.ColumnOf(
			widgets.MainAxisAlignmentStart,
			widgets.CrossAxisAlignmentStart,
			widgets.MainAxisSizeMin,
			widgets.Text{Content: "Retrieved Value:", Style: graphics.TextStyle{
				Color:      colors.OnSurfaceVariant,
				FontSize:   12,
				FontWeight: graphics.FontWeightBold,
			}},
			widgets.VSpace(4),
			widgets.Text{Content: displayValue, Style: graphics.TextStyle{
				Color:    colors.OnSurface,
				FontSize: 16,
			}},
		),
	}
}
