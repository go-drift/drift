package main

import (
	"github.com/go-drift/drift/pkg/core"
)

// Demo represents a showcase demo page.
type Demo struct {
	Route    string
	Title    string
	Subtitle string
	Category string
	Icon     string // SVG icon filename (e.g., "icon-sun.svg")
	Builder  func(ctx core.BuildContext) core.Widget
}

// Category constants for demo organization (6 categories).
const (
	CategoryTheming = "theming"
	CategoryLayout  = "layout"
	CategoryWidgets = "widgets"
	CategoryMotion  = "motion"
	CategoryMedia   = "media"
	CategorySystem  = "system"
)

// CategoryInfo describes a demo category for the home page.
type CategoryInfo struct {
	ID          string // Category ID for matching demos
	Route       string
	Title       string
	Description string
}

// categories defines the main demo categories (6 categories for 3x2 grid).
var categories = []CategoryInfo{
	{CategoryTheming, "/theming-hub", "Theming", "Colors, typography, themes, styles"},
	{CategoryLayout, "/layout-hub", "Layout", "Row, Column, Stack, scroll layout"},
	{CategoryWidgets, "/widgets-hub", "Widgets", "Buttons, forms, menus, media"},
	{CategoryMotion, "/motion-hub", "Motion", "Gestures, animation, effects"},
	{CategoryMedia, "/media-hub", "Media", "Camera, web content, images"},
	{CategorySystem, "/system-hub", "System", "Permissions, storage, sharing"},
}

// demos is the registry of all showcase demo pages.
// Add new demos here to automatically update navigation and routing.
var demos = []Demo{
	// Theming demos
	{"/theming", "Color System", "Theme colors and palettes", CategoryTheming, "icon-sun.svg", nil}, // Special: needs state
	{"/decorations", "Decorations", "Rounded corners, borders, gradients", CategoryTheming, "icon-box.svg", buildDecorationsPage},

	// Layout demos
	{"/layouts", "Layouts", "Row/Column/Stack composition", CategoryLayout, "icon-grid.svg", buildLayoutsPage},
	{"/wrap", "Wrap", "Flowing layouts that wrap", CategoryLayout, "icon-wrap.svg", buildWrapPage},
	{"/positioning", "Positioning", "Center, Align, Expanded, SizedBox", CategoryLayout, "icon-position.svg", buildPositioningPage},

	// Widgets demos
	{"/buttons", "Buttons", "Tappable buttons with haptics", CategoryWidgets, "icon-button.svg", buildButtonsPage},
	{"/forms", "Forms", "Text input and selection controls", CategoryWidgets, "icon-form.svg", buildFormsPage},
	{"/progress", "Progress", "Loading and progress indicators", CategoryWidgets, "icon-progress.svg", buildProgressPage},
	{"/images", "Images", "PNG, JPG, and SVG rendering", CategoryWidgets, "icon-image.svg", buildImagesPage},
	{"/rich-text", "Rich Text", "Inline styled text spans", CategoryWidgets, "icon-rich-text.svg", buildRichTextPage},

	// Motion demos
	{"/gestures", "Gestures", "Drag gestures with axis locking", CategoryMotion, "icon-gesture.svg", buildGesturesPage},
	{"/animations", "Animations", "Implicit animations for smooth UI", CategoryMotion, "icon-motion.svg", buildAnimationsPage},
	{"/lottie", "Lottie", "Lottie animations via Skia Skottie", CategoryMotion, "icon-motion.svg", buildLottiePage},
	{"/scroll", "Scrolling", "Scrollable lists with physics", CategoryLayout, "icon-scroll.svg", buildScrollPage},
	{"/tabs", "Tabs", "Bottom tab navigation", CategoryWidgets, "icon-navigation.svg", buildTabsPage},
	{"/overlays", "Overlays", "Modals, dialogs, and toasts", CategoryWidgets, "icon-overlay.svg", buildOverlaysPage},
	{"/bottom-sheets", "Bottom Sheets", "Drag-to-dismiss sheets", CategoryWidgets, "icon-sheet.svg", buildBottomSheetsPage},

	// Media demos
	{"/camera", "Camera", "Photo capture and gallery access", CategoryMedia, "icon-camera.svg", buildCameraPage},
	{"/webview", "WebView", "Embedded browser view", CategoryMedia, "icon-globe.svg", buildWebViewPage},
	{"/video-player", "Video Player", "Native video playback with controls", CategoryMedia, "icon-play.svg", buildVideoPlayerPage},
	{"/audio-player", "Audio Player", "Headless audio with custom UI", CategoryMedia, "icon-music.svg", buildAudioPlayerPage},

	// System demos
	{"/permissions", "Other Permissions", "Contacts, calendar, storage access", CategorySystem, "icon-shield.svg", buildPermissionsPage},
	{"/location", "Location", "GPS and location services", CategorySystem, "icon-location.svg", buildLocationPage},
	{"/notifications", "Notifications", "Push and local notifications", CategorySystem, "icon-bell.svg", buildNotificationsPage},
	{"/storage", "Storage", "File picker and directories", CategorySystem, "icon-folder.svg", buildStoragePage},
	{"/preferences", "Preferences", "Simple key-value storage", CategorySystem, "icon-settings.svg", buildPreferencesPage},
	{"/secure-storage", "Secure Storage", "Keychain and encrypted data", CategorySystem, "icon-lock.svg", buildSecureStoragePage},
	{"/share", "Share", "Share content with other apps", CategorySystem, "icon-share.svg", buildSharePage},
	{"/clipboard", "Clipboard", "Copy and paste text", CategorySystem, "icon-clipboard.svg", buildClipboardPage},
}

// demosForCategory returns all demos in a given category.
func demosForCategory(category string) []Demo {
	var result []Demo
	for _, demo := range demos {
		if demo.Category == category {
			result = append(result, demo)
		}
	}
	return result
}
