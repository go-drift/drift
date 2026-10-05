package main

import (
	"context"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
	securestorage "github.com/go-drift/drift/plugins/securestorage/runtime"
)

// secureStoragePage is the secure storage demo widget.
type secureStoragePage struct{ core.StatefulBase }

func (secureStoragePage) CreateState() core.State { return &secureStorageState{} }

// buildSecureStoragePage creates a stateful widget for secure storage demos.
func buildSecureStoragePage(_ core.BuildContext) core.Widget { return secureStoragePage{} }

type secureStorageState struct {
	core.StateBase
	statusText      *core.Signal[string]
	biometricStatus *core.Signal[string]
	storedValue     *core.Signal[string]
	keyController   *platform.TextEditingController
	valueController *platform.TextEditingController
}

func (s *secureStorageState) InitState() {
	s.statusText = core.NewSignal("Enter a key and value to store securely.")
	s.biometricStatus = core.NewSignal("Checking biometric availability...")
	s.storedValue = core.NewSignal("")
	core.UseListenable(s, s.statusText)
	core.UseListenable(s, s.biometricStatus)
	core.UseListenable(s, s.storedValue)
	s.keyController = platform.NewTextEditingController("demo_key")
	s.valueController = platform.NewTextEditingController("secret_value_123")

	// Check biometric availability on init
	go func() {
		biometric, err := securestorage.AvailableBiometric(context.Background())
		message := "Biometric available: " + string(biometric)
		switch {
		case err != nil:
			message = "Error checking biometrics: " + err.Error()
		case biometric == securestorage.BiometricNone:
			message = "Biometric not available"
		}
		drift.Dispatch(func() { s.biometricStatus.Set(message) })
	}()
}

func (s *secureStorageState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Secure Storage",
		sectionTitle("Key-Value Storage", colors),
		widgets.VSpace(12),
		widgets.Text{Content: s.biometricStatus.Value(), Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.TextFieldOf(ctx, s.keyController).
			WithLabel("Key").
			WithPlaceholder("Enter key name"),
		widgets.VSpace(12),
		theme.TextFieldOf(ctx, s.valueController).
			WithLabel("Value").
			WithPlaceholder("Enter secret value"),
		widgets.VSpace(12),
		widgets.Wrap{
			Spacing:    8,
			RunSpacing: 8,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Save", func() {
					s.saveValue(false)
				}),
				theme.ButtonOf(ctx, "Save (biometric)", func() {
					s.saveValue(true)
				}),
				theme.ButtonOf(ctx, "Get", func() {
					s.getValue()
				}).WithColor(colors.Secondary, colors.OnSecondary),
				theme.ButtonOf(ctx, "Delete", func() {
					s.deleteValue()
				}).WithColor(colors.Error, colors.OnError),
			},
		},
		widgets.VSpace(16),
		retrievedValueCard(s.storedValue.Value(), colors),
		widgets.VSpace(16),
		statusCard(s.statusText.Value(), colors),
		widgets.VSpace(40),
	)
}

func (s *secureStorageState) saveValue(biometric bool) {
	key := s.keyController.Text()
	value := s.valueController.Text()
	if key == "" || value == "" {
		s.statusText.Set("Please enter both key and value")
		return
	}
	go func() {
		err := securestorage.Set(context.Background(), key, value, &securestorage.Options{RequireBiometric: biometric})
		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error saving: " + err.Error())
				return
			}
			s.statusText.Set("Value saved securely for key: " + key)
		})
	}()
}

func (s *secureStorageState) getValue() {
	key := s.keyController.Text()
	if key == "" {
		s.statusText.Set("Please enter a key name")
		return
	}
	s.statusText.Set("Reading...")
	go func() {
		value, found, err := securestorage.Get(context.Background(), key,
			&securestorage.Options{BiometricPrompt: "Read the demo secret"})
		drift.Dispatch(func() {
			switch {
			case err != nil:
				s.statusText.Set("Error retrieving: " + err.Error())
				s.storedValue.Set("")
			case !found:
				s.statusText.Set("No value found for key: " + key)
				s.storedValue.Set("")
			default:
				s.storedValue.Set(value)
				s.statusText.Set("Retrieved value for key: " + key)
			}
		})
	}()
}

func (s *secureStorageState) deleteValue() {
	key := s.keyController.Text()
	if key == "" {
		s.statusText.Set("Please enter a key name")
		return
	}
	go func() {
		err := securestorage.Delete(context.Background(), key, nil)
		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error deleting: " + err.Error())
				return
			}
			s.storedValue.Set("")
			s.statusText.Set("Deleted key: " + key)
		})
	}()
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
		Child: widgets.Column{
			MainAxisSize: widgets.MainAxisSizeMin,
			Children: []core.Widget{
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
			},
		},
	}
}
