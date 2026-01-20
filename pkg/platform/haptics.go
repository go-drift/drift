package platform

// HapticsService provides haptic feedback functionality.
var Haptics = &HapticsService{
	channel: NewMethodChannel("drift/haptics"),
}

// HapticsService manages haptic feedback.
type HapticsService struct {
	channel *MethodChannel
}

// HapticFeedbackType defines the type of haptic feedback.
type HapticFeedbackType string

const (
	// HapticLight is a light impact feedback.
	HapticLight HapticFeedbackType = "light"

	// HapticMedium is a medium impact feedback.
	HapticMedium HapticFeedbackType = "medium"

	// HapticHeavy is a heavy impact feedback.
	HapticHeavy HapticFeedbackType = "heavy"

	// HapticSelection is a selection feedback (subtle tick).
	HapticSelection HapticFeedbackType = "selection"

	// HapticSuccess indicates a successful action.
	HapticSuccess HapticFeedbackType = "success"

	// HapticWarning indicates a warning.
	HapticWarning HapticFeedbackType = "warning"

	// HapticError indicates an error.
	HapticError HapticFeedbackType = "error"
)

// Impact triggers an impact haptic feedback.
func (h *HapticsService) Impact(style HapticFeedbackType) error {
	_, err := h.channel.Invoke("impact", map[string]any{
		"style": string(style),
	})
	return err
}

// LightImpact triggers a light impact feedback.
func (h *HapticsService) LightImpact() error {
	return h.Impact(HapticLight)
}

// MediumImpact triggers a medium impact feedback.
func (h *HapticsService) MediumImpact() error {
	return h.Impact(HapticMedium)
}

// HeavyImpact triggers a heavy impact feedback.
func (h *HapticsService) HeavyImpact() error {
	return h.Impact(HapticHeavy)
}

// SelectionClick triggers a selection tick feedback.
// Use this for selection changes in pickers, sliders, etc.
func (h *HapticsService) SelectionClick() error {
	return h.Impact(HapticSelection)
}

// Vibrate triggers a vibration for the specified duration in milliseconds.
func (h *HapticsService) Vibrate(durationMs int) error {
	_, err := h.channel.Invoke("vibrate", map[string]any{
		"duration": durationMs,
	})
	return err
}
