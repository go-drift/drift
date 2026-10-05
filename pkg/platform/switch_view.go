package platform

import (
	"fmt"
	"sync"
)

// SwitchViewConfig defines styling passed to native switch view.
type SwitchViewConfig struct {
	// OnTintColor is the track color when the switch is on (ARGB).
	OnTintColor uint32

	// ThumbTintColor is the thumb/knob color (ARGB).
	ThumbTintColor uint32
}

// SwitchViewClient receives callbacks from native switch view.
type SwitchViewClient interface {
	// OnValueChanged is called when the switch value changes.
	OnValueChanged(value bool)
}

// SwitchView is a platform view for native switch control.
type SwitchView struct {
	PlatformViewBase
	config SwitchViewConfig
	client SwitchViewClient
	value  bool
	mu     sync.RWMutex
}

// NewSwitchView creates a new switch platform view.
func NewSwitchView(viewID int64, config SwitchViewConfig, client SwitchViewClient) *SwitchView {
	return &SwitchView{
		PlatformViewBase: NewPlatformViewBase(viewID, "switch"),
		config:           config,
		client:           client,
	}
}

// SetClient sets the callback client for this view.
func (v *SwitchView) SetClient(client SwitchViewClient) {
	v.mu.Lock()
	v.client = client
	v.mu.Unlock()
}

// Create initializes the native view.
func (v *SwitchView) Create(params map[string]any) error {
	return nil
}

// Dispose cleans up the native view.
func (v *SwitchView) Dispose() {
	// Cleanup handled by registry
}

// SetValue updates the switch value from Go side.
func (v *SwitchView) SetValue(value bool) {
	v.mu.Lock()
	v.value = value
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setValue", map[string]any{
		"value": value,
	})
}

// Value returns the current value.
func (v *SwitchView) Value() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.value
}

// UpdateConfig updates the view configuration.
func (v *SwitchView) UpdateConfig(config SwitchViewConfig) {
	v.mu.Lock()
	v.config = config
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "updateConfig", map[string]any{
		"onTintColor":    config.OnTintColor,
		"thumbTintColor": config.ThumbTintColor,
	})
}

// handleValueChanged processes value change events from native.
func (v *SwitchView) handleValueChanged(value bool) {
	v.mu.Lock()
	v.value = value
	client := v.client
	v.mu.Unlock()

	if client != nil {
		Dispatch(func() {
			client.OnValueChanged(value)
		})
	}
}

// switchViewFactory creates switch platform views.
type switchViewFactory struct{}

func (f *switchViewFactory) ViewType() string {
	return "switch"
}

func (f *switchViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	config := SwitchViewConfig{}

	if v, ok := toUint32(params["onTintColor"]); ok {
		config.OnTintColor = v
	}
	if v, ok := toUint32(params["thumbTintColor"]); ok {
		config.ThumbTintColor = v
	}

	view := NewSwitchView(viewID, config, nil)

	// Set initial value
	if v, ok := params["value"].(bool); ok {
		view.value = v
	}

	return view, nil
}

// RegisterSwitchViewFactory registers the switch view factory.
func RegisterSwitchViewFactory() {
	GetPlatformViewRegistry().RegisterFactory(&switchViewFactory{})
}

func init() {
	RegisterSwitchViewFactory()
}

// HandleEvent implements PlatformView: value changes from the native
// switch.
func (v *SwitchView) HandleEvent(method string, args map[string]any) error {
	if method != "onSwitchChanged" {
		return fmt.Errorf("%s: %w", method, ErrMethodNotFound)
	}
	value, err := requireBool(method, args, "value")
	if err != nil {
		return err
	}
	v.handleValueChanged(value)
	return nil
}
