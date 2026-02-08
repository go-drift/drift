package platform

import (
	"sync"
)

// ActivityIndicatorSize represents the size of the activity indicator.
type ActivityIndicatorSize int

const (
	// ActivityIndicatorSizeMedium is a medium spinner.
	ActivityIndicatorSizeMedium ActivityIndicatorSize = iota
	// ActivityIndicatorSizeSmall is a small spinner.
	ActivityIndicatorSizeSmall
	// ActivityIndicatorSizeLarge is a large spinner.
	ActivityIndicatorSizeLarge
)

// ActivityIndicatorViewConfig defines styling passed to native activity indicator.
type ActivityIndicatorViewConfig struct {
	// Animating controls whether the indicator is spinning.
	Animating bool

	// Size is the indicator size (Small, Medium, Large).
	Size ActivityIndicatorSize

	// Color is the spinner color (ARGB).
	Color uint32
}

// ActivityIndicatorView is a platform view for native activity indicator.
type ActivityIndicatorView struct {
	basePlatformView
	config    ActivityIndicatorViewConfig
	animating bool
	mu        sync.RWMutex
}

// NewActivityIndicatorView creates a new activity indicator platform view.
func NewActivityIndicatorView(viewID int64, config ActivityIndicatorViewConfig) *ActivityIndicatorView {
	return &ActivityIndicatorView{
		basePlatformView: basePlatformView{
			viewID:   viewID,
			viewType: "activity_indicator",
		},
		config:    config,
		animating: config.Animating,
	}
}

// Create initializes the native view.
func (v *ActivityIndicatorView) Create(params map[string]any) error {
	return nil
}

// Dispose cleans up the native view.
func (v *ActivityIndicatorView) Dispose() {
	// Cleanup handled by registry
}

// SetAnimating starts or stops the animation.
func (v *ActivityIndicatorView) SetAnimating(animating bool) {
	v.mu.Lock()
	v.animating = animating
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setAnimating", map[string]any{
		"animating": animating,
	})
}

// IsAnimating returns whether the indicator is currently animating.
func (v *ActivityIndicatorView) IsAnimating() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.animating
}

// UpdateConfig updates the view configuration.
func (v *ActivityIndicatorView) UpdateConfig(config ActivityIndicatorViewConfig) {
	v.mu.Lock()
	v.config = config
	v.animating = config.Animating
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "updateConfig", map[string]any{
		"animating": config.Animating,
		"size":      int(config.Size),
		"color":     config.Color,
	})
}

// activityIndicatorViewFactory creates activity indicator platform views.
type activityIndicatorViewFactory struct{}

func (f *activityIndicatorViewFactory) ViewType() string {
	return "activity_indicator"
}

func (f *activityIndicatorViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	config := ActivityIndicatorViewConfig{
		Animating: true, // Default to animating
		Size:      ActivityIndicatorSizeMedium,
	}

	if v, ok := params["animating"].(bool); ok {
		config.Animating = v
	}
	if v, ok := toInt(params["size"]); ok {
		config.Size = ActivityIndicatorSize(v)
	}
	if v, ok := toUint32(params["color"]); ok {
		config.Color = v
	}

	view := NewActivityIndicatorView(viewID, config)
	return view, nil
}

// RegisterActivityIndicatorViewFactory registers the activity indicator view factory.
func RegisterActivityIndicatorViewFactory() {
	GetPlatformViewRegistry().RegisterFactory(&activityIndicatorViewFactory{})
}

func init() {
	RegisterActivityIndicatorViewFactory()
}
