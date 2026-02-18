package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildSecureStoragePage creates a stateful widget for secure storage demos.
func buildSecureStoragePage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *secureStorageState { return &secureStorageState{} })
}

type secureStorageState struct {
	core.StateBase
	statusText      *core.ManagedState[string]
	biometricStatus *core.ManagedState[string]
	storedValue     *core.ManagedState[string]
	keyController   *platform.TextEditingController
	valueController *platform.TextEditingController
}

func (s *secureStorageState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Enter a key and value to store securely.")
	s.biometricStatus = core.NewManagedState(&s.StateBase, "Checking biometric availability...")
	s.storedValue = core.NewManagedState(&s.StateBase, "")
	s.keyController = platform.NewTextEditingController("demo_key")
	s.valueController = platform.NewTextEditingController("secret_value_123")

	// Check biometric availability on init
	go func() {
		available, err := platform.SecureStorage.IsBiometricAvailable()
		if err != nil {
			drift.Dispatch(func() {
				s.biometricStatus.Set("Error checking biometrics: " + err.Error())
			})
			return
		}

		biometricType, _ := platform.SecureStorage.GetBiometricType()
		var message string
		if available {
			message = "Biometric available: " + string(biometricType)
		} else {
			message = "Biometric not available"
		}
		drift.Dispatch(func() {
			s.biometricStatus.Set(message)
		})
	}()

	// Listen for async biometric auth results
	go func() {
		for event := range platform.SecureStorage.Listen() {
			drift.Dispatch(func() {
				if event.Success {
					if event.Value != "" {
						s.storedValue.Set(event.Value)
						s.statusText.Set("Retrieved value with biometric auth")
					} else {
						s.statusText.Set("Operation completed with biometric auth")
					}
				} else {
					s.statusText.Set("Auth failed: " + event.Error)
				}
			})
		}
	}()
}

func (s *secureStorageState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Secure Storage",
		sectionTitle("Key-Value Storage", colors),
		widgets.VSpace(12),
		widgets.Text{Content: s.biometricStatus.Get(), Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.TextFieldOf(ctx, s.keyController).
			WithLabel("Key").
			WithPlaceholder("Enter key name"),
		widgets.VSpace(12),
		theme.TextFieldOf(ctx, s.valueController).
			WithLabel("Value").
			WithPlaceholder("Enter secret value"),
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
		retrievedValueCard(s.storedValue.Get(), colors),
		widgets.VSpace(16),
		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *secureStorageState) saveValue() {
	key := s.keyController.Text()
	value := s.valueController.Text()

	if key == "" || value == "" {
		s.statusText.Set("Please enter both key and value")
		return
	}

	err := platform.SecureStorage.Set(key, value, nil)
	if err != nil {
		s.statusText.Set("Error saving: " + err.Error())
		return
	}

	s.statusText.Set("Value saved securely for key: " + key)
}

func (s *secureStorageState) getValue() {
	key := s.keyController.Text()

	if key == "" {
		s.statusText.Set("Please enter a key name")
		return
	}

	value, err := platform.SecureStorage.Get(key, nil)
	if err == platform.ErrAuthPending {
		s.statusText.Set("Authenticating...")
		return
	}
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

func (s *secureStorageState) deleteValue() {
	key := s.keyController.Text()

	if key == "" {
		s.statusText.Set("Please enter a key name")
		return
	}

	err := platform.SecureStorage.Delete(key, nil)
	if err != nil {
		s.statusText.Set("Error deleting: " + err.Error())
		return
	}

	s.storedValue.Set("")
	s.statusText.Set("Deleted key: " + key)
}

func retrievedValueCard(value string, colors theme.ColorScheme) core.Widget {
	displayValue := value
	if displayValue == "" {
		displayValue = "(no value retrieved)"
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding:      layout.EdgeInsetsAll(12),
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
