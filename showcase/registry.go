package main

import (
	"github.com/go-drift/drift/pkg/core"
)

// Demo represents a showcase demo page.
type Demo struct {
	Route    string
	Title    string
	Subtitle string
	Builder  func(ctx core.BuildContext) core.Widget
}

// demos is the registry of all showcase demo pages.
// Add new demos here to automatically update navigation and routing.
var demos = []Demo{
	{"/buttons", "Buttons", "Tappable buttons with haptics", buildButtonsPage},
	{"/forms", "Forms", "Text input and form handling", buildFormsPage},
	{"/layouts", "Layouts", "Row, Column, Stack composition", buildLayoutsPage},
	{"/decorations", "Decorations", "Rounded corners and borders", buildDecorationsPage},
	{"/images", "Images", "PNG/JPG image rendering", buildImagesPage},
	{"/tabs", "Tabs", "Bottom tab bar navigation", buildTabsPage},
	{"/scroll", "Scrolling", "Scrollable lists with physics", buildScrollPage},
	{"/gestures", "Gestures", "Drag gestures with axis locking", buildGesturesPage},
	{"/webview", "WebView", "Embedded native browser view", buildWebViewPage},
	{"/notifications", "Notifications", "Permissions and local alerts", buildNotificationsPage},
	{"/animations", "Animations", "Implicit animations for smooth UI", buildAnimationsPage},
}

// demosWithTheming returns demos plus theming page (which needs isDark state).
// This is separate because theming has a different builder signature.
func themingDemo() Demo {
	return Demo{"/theming", "Theming", "Colors, typography, dark mode", nil}
}
