package main

import (
	"github.com/go-drift/drift/pkg/core"
)

// Hub pages for each demo category.
// These display a list of demos within a category.

func buildThemingHubPage(ctx core.BuildContext) core.Widget {
	return categoryHubPage(
		ctx,
		CategoryTheming,
		"Theming",
		"Colors, styles, and theme customization. See the color system and typography in action.",
	)
}

func buildLayoutHubPage(ctx core.BuildContext) core.Widget {
	return categoryHubPage(
		ctx,
		CategoryLayout,
		"Layout",
		"Layout primitives for arranging content. Row, Column, Stack, and spacing utilities.",
	)
}

func buildWidgetsHubPage(ctx core.BuildContext) core.Widget {
	return categoryHubPage(
		ctx,
		CategoryWidgets,
		"Widgets",
		"Interactive UI components. Buttons, text inputs, forms, and media display.",
	)
}

func buildMotionHubPage(ctx core.BuildContext) core.Widget {
	return categoryHubPage(
		ctx,
		CategoryMotion,
		"Motion",
		"Animation and touch handling. Gestures, implicit animations, scrollable lists, and navigation.",
	)
}

func buildMediaHubPage(ctx core.BuildContext) core.Widget {
	return categoryHubPage(
		ctx,
		CategoryMedia,
		"Media",
		"Camera and web content. Photo capture, gallery access, and embedded browser views.",
	)
}

func buildSystemHubPage(ctx core.BuildContext) core.Widget {
	return categoryHubPage(
		ctx,
		CategorySystem,
		"System",
		"Native device integration. Location, notifications, storage, sharing, clipboard, and permissions.",
	)
}
