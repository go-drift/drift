package platform

import "github.com/go-drift/drift/pkg/graphics"

// StatusBarStyle indicates the status bar icon color scheme.
type StatusBarStyle string

const (
	StatusBarStyleDefault StatusBarStyle = "default"
	StatusBarStyleLight   StatusBarStyle = "light"
	StatusBarStyleDark    StatusBarStyle = "dark"
)

// SystemUIStyle describes system bar and window styling.
type SystemUIStyle struct {
	StatusBarHidden bool
	StatusBarStyle  StatusBarStyle
	TitleBarHidden  bool             // Android only (no-op on iOS)
	BackgroundColor *graphics.Color // Android only (no-op on iOS)
	Transparent     bool             // Android only (no-op on iOS)
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
		"titleBarHidden":  style.TitleBarHidden,
		"transparent":     style.Transparent,
	}
	if style.BackgroundColor != nil {
		args["backgroundColor"] = uint32(*style.BackgroundColor)
	}

	_, err := systemUIChannel.Invoke("setStyle", args)
	return err
}
