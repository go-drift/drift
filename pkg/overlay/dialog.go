package overlay

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// Dialog provides Material 3 card chrome for dialog content.
//
// It reads styling from [theme.DialogThemeData] (via [theme.ThemeData.DialogThemeOf])
// and renders a [widgets.Container] with a surface color, border radius, elevation
// shadow, and padding. The default theme uses SurfaceContainerHigh as the
// background, 28px border radius, elevation 3, and 24px padding on all sides.
//
// Set Width to constrain the dialog to a specific width. When Width is zero
// the dialog shrinks to fit its content. Dialog does not enforce a maximum
// width; callers showing dialogs on wide screens should set an explicit Width
// to avoid unexpectedly large dialogs.
//
// Dialog is intentionally minimal. For different chrome, skip Dialog entirely
// and return your own [widgets.Container] from the [ShowDialog] builder:
//
//	overlay.ShowDialog(ctx, overlay.DialogOptions{
//	    Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
//	        return widgets.Container{
//	            Width: 400, Color: myColor, BorderRadius: 8,
//	            Child: myContent(dismiss),
//	        }
//	    },
//	})
//
// For a pre-built title/content/actions layout, use [AlertDialog] instead.
type Dialog struct {
	core.StatelessBase

	// Child is the dialog content widget.
	Child core.Widget
	// Width constrains the dialog to an explicit width in pixels.
	// Zero means the dialog shrinks to fit its content.
	Width float64
}

func (d Dialog) Build(ctx core.BuildContext) core.Widget {
	th := theme.ThemeOf(ctx)
	dt := th.DialogThemeOf()
	colors := th.ColorScheme

	c := widgets.Container{
		Child:        d.Child,
		Color:        dt.BackgroundColor,
		BorderRadius: dt.BorderRadius,
		Shadow:       graphics.BoxShadowElevation(dt.Elevation, colors.Shadow),
		Padding:      dt.Padding,
	}
	if d.Width > 0 {
		c.Width = d.Width
	}
	return c
}

// AlertDialog arranges a title, content, and action buttons inside a [Dialog].
//
// All fields are optional. When all fields are nil/empty the result is a blank
// card at the theme's AlertDialogWidth (default 280); this is typically a
// programming error.
//
// When present, the sections are laid out in a [widgets.Column] with
// [widgets.MainAxisSizeMin] and [widgets.CrossAxisAlignmentStart]:
//
//   - Title appears first
//   - Content appears below the title (spacing from [theme.DialogThemeData.TitleContentSpacing])
//   - Actions appear in a right-aligned [widgets.Row] (spacing above from
//     [theme.DialogThemeData.ContentActionsSpacing], horizontal gaps from
//     [theme.DialogThemeData.ActionSpacing])
//
// Width defaults to [theme.DialogThemeData.AlertDialogWidth] when zero.
//
// AlertDialog wraps its column in a [Dialog], so it inherits all theme-driven
// styling (background color, border radius, shadow, padding).
//
// For a quick alert with string title/content and themed buttons, use
// [ShowAlertDialog] instead of building AlertDialog manually.
//
// Example with custom widgets:
//
//	overlay.ShowDialog(ctx, overlay.DialogOptions{
//	    BarrierColor: graphics.RGBA(0, 0, 0, 0.5),
//	    Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
//	        textTheme := theme.ThemeOf(ctx).TextTheme
//	        return overlay.AlertDialog{
//	            Title:   theme.TextOf(ctx, "Confirm", textTheme.HeadlineSmall),
//	            Content: theme.TextOf(ctx, "Save changes?", textTheme.BodyMedium),
//	            Actions: []core.Widget{
//	                theme.ButtonOf(ctx, "Cancel", dismiss),
//	                theme.ButtonOf(ctx, "Save", func() { save(); dismiss() }),
//	            },
//	        }
//	    },
//	})
type AlertDialog struct {
	core.StatelessBase

	// Title is the heading widget, typically themed text using
	// [theme.TextOf] with HeadlineSmall.
	Title core.Widget
	// Content is the body widget, typically themed text using
	// [theme.TextOf] with BodyMedium.
	Content core.Widget
	// Actions are the dialog buttons placed in a right-aligned row.
	// Common choices include themed buttons via [theme.ButtonOf].
	Actions []core.Widget
	// Width is the dialog width in pixels. Zero defaults to
	// [theme.DialogThemeData.AlertDialogWidth] (280).
	Width float64
}

func (a AlertDialog) Build(ctx core.BuildContext) core.Widget {
	dt := theme.ThemeOf(ctx).DialogThemeOf()

	width := a.Width
	if width == 0 {
		width = dt.AlertDialogWidth
	}

	var children []core.Widget

	if a.Title != nil {
		children = append(children, a.Title)
	}

	if a.Content != nil {
		if len(children) > 0 {
			children = append(children, widgets.VSpace(dt.TitleContentSpacing))
		}
		children = append(children, a.Content)
	}

	if len(a.Actions) > 0 {
		if len(children) > 0 {
			children = append(children, widgets.VSpace(dt.ContentActionsSpacing))
		}
		var actionChildren []core.Widget
		for i, action := range a.Actions {
			if i > 0 {
				actionChildren = append(actionChildren, widgets.HSpace(dt.ActionSpacing))
			}
			actionChildren = append(actionChildren, action)
		}
		children = append(children, widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentEnd,
			MainAxisSize:      widgets.MainAxisSizeMax,
			Children:          actionChildren,
		})
	}

	return Dialog{
		Width: width,
		Child: widgets.Column{
			MainAxisSize:       widgets.MainAxisSizeMin,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			Children:           children,
		},
	}
}
