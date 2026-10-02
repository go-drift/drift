package platform

import "context"

// StatusBarStyle is the colour of the status bar's icons and text.
type StatusBarStyle string

const (
	// StatusBarStyleDefault follows the system theme: dark icons in light
	// mode, light icons in dark mode.
	StatusBarStyleDefault StatusBarStyle = "default"
	// StatusBarStyleLight is light icons, for dark content.
	StatusBarStyleLight StatusBarStyle = "light"
	// StatusBarStyleDark is dark icons, for light content.
	StatusBarStyleDark StatusBarStyle = "dark"
)

// SystemUIStyle describes the system bars. Drift always draws behind them,
// on both platforms; inset content with SafeArea.
type SystemUIStyle struct {
	StatusBarHidden bool
	StatusBarStyle  StatusBarStyle
}

var systemUIChannel = NewMethodChannel("drift/system_ui")

// SetSystemUI updates the system UI appearance.
func SetSystemUI(style SystemUIStyle) error {
	statusStyle := style.StatusBarStyle
	if statusStyle == "" {
		statusStyle = StatusBarStyleDefault
	}

	args := map[string]any{
		"statusBarHidden": style.StatusBarHidden,
		"statusBarStyle":  string(statusStyle),
	}

	_, err := systemUIChannel.Invoke(context.Background(), "setStyle", args)
	return err
}
