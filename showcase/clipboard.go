package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildClipboardPage creates a stateful widget for clipboard demos.
func buildClipboardPage(ctx core.BuildContext) core.Widget {
	return clipboardPage{}
}

type clipboardPage struct{}

func (c clipboardPage) CreateElement() core.Element {
	return core.NewStatefulElement(c, nil)
}

func (c clipboardPage) Key() any {
	return nil
}

func (c clipboardPage) CreateState() core.State {
	return &clipboardState{}
}

type clipboardState struct {
	core.StateBase
	statusText *core.ManagedState[string]
}

func (s *clipboardState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Tap a button to interact with the clipboard.")
}

func (s *clipboardState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Clipboard",
		sectionTitle("Copy & Paste", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Interact with the system clipboard:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			ChildrenWidgets: []core.Widget{
				theme.ButtonOf(ctx, "Copy", func() {
					s.copyText()
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Read", func() {
					s.readClipboard()
				}).WithColor(colors.Tertiary, colors.OnTertiary),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Clear", func() {
					s.clearClipboard()
				}).WithColor(colors.SurfaceContainerHigh, colors.OnSurface),
			},
		},
		widgets.VSpace(24),
		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *clipboardState) copyText() {
	err := platform.Clipboard.SetText("Hello from Drift!")
	if err != nil {
		s.statusText.Set("Error copying: " + err.Error())
		return
	}
	s.statusText.Set("Copied \"Hello from Drift!\" to clipboard")
}

func (s *clipboardState) readClipboard() {
	text, err := platform.Clipboard.GetText()
	if err != nil {
		s.statusText.Set("Error reading: " + err.Error())
		return
	}
	if text == "" {
		s.statusText.Set("Clipboard is empty")
		return
	}
	s.statusText.Set("Clipboard contents: " + text)
}

func (s *clipboardState) clearClipboard() {
	err := platform.Clipboard.Clear()
	if err != nil {
		s.statusText.Set("Error clearing: " + err.Error())
		return
	}
	s.statusText.Set("Clipboard cleared")
}
