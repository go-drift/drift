package widgets

import (
	"strings"
	"unicode/utf8"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/focus"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// NativeTextField embeds a native text input field.
type NativeTextField struct {
	// Controller manages the text content and selection.
	Controller *platform.TextEditingController

	// Style for the text.
	Style rendering.TextStyle

	// Placeholder text shown when empty.
	Placeholder string

	// KeyboardType specifies the keyboard to show.
	KeyboardType platform.KeyboardType

	// InputAction specifies the keyboard action button.
	InputAction platform.TextInputAction

	// Obscure hides the text (for passwords).
	Obscure bool

	// Autocorrect enables auto-correction.
	Autocorrect bool

	// OnChanged is called when the text changes.
	OnChanged func(string)

	// OnSubmitted is called when the user submits (presses done/return).
	OnSubmitted func(string)

	// OnEditingComplete is called when editing is complete.
	OnEditingComplete func()

	// Disabled controls whether the field rejects input.
	Disabled bool

	// Width of the text field (0 = expand to fill).
	Width float64

	// Height of the text field.
	Height float64

	// Padding inside the text field.
	Padding layout.EdgeInsets

	// BackgroundColor of the text field.
	BackgroundColor rendering.Color

	// BorderColor of the text field.
	BorderColor rendering.Color

	// FocusColor of the text field outline.
	FocusColor rendering.Color

	// BorderRadius for rounded corners.
	BorderRadius float64

	// PlaceholderColor is the color for placeholder text.
	PlaceholderColor rendering.Color
}

// CreateElement creates the element for the stateful widget.
func (n NativeTextField) CreateElement() core.Element {
	return core.NewStatefulElement(n, nil)
}

// Key returns the widget key.
func (n NativeTextField) Key() any {
	return nil
}

// CreateState creates the state for this widget.
func (n NativeTextField) CreateState() core.State {
	return &nativeTextFieldState{}
}

type nativeTextFieldState struct {
	element    *core.StatefulElement
	connection *platform.TextInputConnection
	focused    bool
	viewID     int64
	focusNode  *focus.FocusNode
}

func (s *nativeTextFieldState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *nativeTextFieldState) InitState() {
	// Create and register focus node for tab navigation
	s.focusNode = &focus.FocusNode{
		CanRequestFocus: true,
		DebugLabel:      "NativeTextField",
		Rect:            s, // s implements RectProvider
		OnFocusChange: func(hasFocus bool) {
			if hasFocus && !s.focused {
				// Focus node gained focus, activate the text field
				// Pass render object as target for tap-outside-to-unfocus detection
				var target any
				if s.element != nil {
					target = s.element.RenderObject()
				}
				s.focus(target)
			}
		},
	}
	manager := focus.GetFocusManager()
	if manager.RootScope != nil {
		manager.RootScope.Children = append(manager.RootScope.Children, s.focusNode)
	}
}

func (s *nativeTextFieldState) Dispose() {
	if s.connection != nil {
		s.connection.Close()
		s.connection = nil
	}
	// Remove focus node from scope
	if s.focusNode != nil {
		manager := focus.GetFocusManager()
		if manager.RootScope != nil {
			// Clear FocusedChild if it points to this node
			if manager.RootScope.FocusedChild == s.focusNode {
				manager.RootScope.FocusedChild = nil
			}
			// Remove from children
			children := manager.RootScope.Children
			for i, child := range children {
				if child == s.focusNode {
					manager.RootScope.Children = append(children[:i], children[i+1:]...)
					break
				}
			}
		}
		s.focusNode = nil
	}
}

func (s *nativeTextFieldState) DidChangeDependencies() {}

func (s *nativeTextFieldState) DidUpdateWidget(oldWidget core.StatefulWidget) {}

// FocusRect implements focus.RectProvider for directional navigation.
func (s *nativeTextFieldState) FocusRect() focus.FocusRect {
	if s.element == nil {
		return focus.FocusRect{}
	}
	offset := core.GlobalOffsetOf(s.element)
	// Get size from the render object if available
	if ro := s.element.RenderObject(); ro != nil {
		if sizer, ok := ro.(interface{ Size() rendering.Size }); ok {
			size := sizer.Size()
			return focus.FocusRect{
				Left:   offset.X,
				Top:    offset.Y,
				Right:  offset.X + size.Width,
				Bottom: offset.Y + size.Height,
			}
		}
	}
	return focus.FocusRect{Left: offset.X, Top: offset.Y, Right: offset.X, Bottom: offset.Y}
}

func (s *nativeTextFieldState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *nativeTextFieldState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(NativeTextField)

	// Default values
	height := w.Height
	if height == 0 {
		height = 44 // Standard iOS text field height
	}

	padding := w.Padding
	if padding == (layout.EdgeInsets{}) {
		padding = layout.EdgeInsetsSymmetric(12, 8)
	}

	bgColor := w.BackgroundColor
	if bgColor == 0 {
		bgColor = rendering.ColorWhite
	}

	borderColor := w.BorderColor
	if borderColor == 0 {
		borderColor = rendering.Color(0xFFCCCCCC)
	}

	focusColor := w.FocusColor
	if focusColor == 0 {
		focusColor = rendering.Color(0xFF007AFF)
	}

	// Get current text from controller or empty string
	text := ""
	if w.Controller != nil {
		text = w.Controller.Text()
	}

	// Show placeholder if empty
	displayText := text
	textStyle := w.Style
	// Ensure text has a visible color (default to black if not set)
	if textStyle.Color == 0 {
		textStyle.Color = rendering.Color(0xFF000000)
	}
	if displayText == "" && w.Placeholder != "" {
		displayText = w.Placeholder
		placeholderColor := w.PlaceholderColor
		if placeholderColor == 0 {
			placeholderColor = rendering.Color(0xFF999999) // Default placeholder color
		}
		textStyle.Color = placeholderColor
	}
	if w.Obscure && text != "" {
		runeCount := utf8.RuneCountInString(text)
		displayText = strings.Repeat("•", runeCount)
	}

	// Build the visual representation
	return nativeTextFieldRender{
		text:         displayText,
		style:        textStyle,
		width:        w.Width,
		height:       height,
		padding:      padding,
		bgColor:      bgColor,
		borderColor:  borderColor,
		focusColor:   focusColor,
		borderRadius: w.BorderRadius,
		state:        s,
		config:       w,
	}
}

// UpdateEditingValue implements platform.TextInputClient.
func (s *nativeTextFieldState) UpdateEditingValue(value platform.TextEditingValue) {
	w := s.element.Widget().(NativeTextField)
	if w.Controller != nil {
		w.Controller.SetValue(value)
	}
	if w.OnChanged != nil {
		w.OnChanged(value.Text)
	}
	s.SetState(func() {})
}

// PerformAction implements platform.TextInputClient.
func (s *nativeTextFieldState) PerformAction(action platform.TextInputAction) {
	w := s.element.Widget().(NativeTextField)
	switch action {
	case platform.TextInputActionDone, platform.TextInputActionGo, platform.TextInputActionSearch, platform.TextInputActionSend:
		if w.OnSubmitted != nil && w.Controller != nil {
			w.OnSubmitted(w.Controller.Text())
		}
		if w.OnEditingComplete != nil {
			w.OnEditingComplete()
		}
		s.unfocus()
	case platform.TextInputActionNext:
		s.unfocus()
		focus.GetFocusManager().MoveFocus(1)
	case platform.TextInputActionPrevious:
		s.unfocus()
		focus.GetFocusManager().MoveFocus(-1)
	}
}

// ConnectionClosed implements platform.TextInputClient.
func (s *nativeTextFieldState) ConnectionClosed() {
	s.connection = nil
	s.SetState(func() {
		s.focused = false
	})
}

func (s *nativeTextFieldState) focus(target any) {
	if s.focused {
		return
	}

	w := s.element.Widget().(NativeTextField)
	if w.Disabled {
		return
	}

	// Mark as focused early to prevent re-entry from OnFocusChange callback
	s.focused = true

	// Sync with focus system - mark this node as primary focus
	if s.focusNode != nil {
		s.focusNode.RequestFocus()
	}

	// Unfocus any other active text input first
	platform.UnfocusAll()

	config := platform.TextInputConfiguration{
		KeyboardType:      w.KeyboardType,
		InputAction:       w.InputAction,
		Autocorrect:       w.Autocorrect,
		EnableSuggestions: !w.Obscure,
		Obscure:           w.Obscure,
	}

	s.connection = platform.NewTextInputConnection(s, config)

	// Mark this as the active connection and register the focused target
	platform.SetActiveConnection(s.connection.ID())
	platform.SetFocusedTarget(target)

	s.connection.Show()

	// Sync initial state
	if w.Controller != nil {
		s.connection.SetEditingState(w.Controller.Value())
	}

	// Trigger rebuild
	s.SetState(func() {})
}

func (s *nativeTextFieldState) unfocus() {
	if !s.focused {
		return
	}

	s.focused = false

	if s.connection != nil {
		s.connection.Close()
		s.connection = nil
	}

	// Trigger rebuild
	s.SetState(func() {})
}

// nativeTextFieldRender is a render widget for visual text field display.
type nativeTextFieldRender struct {
	text         string
	style        rendering.TextStyle
	width        float64
	height       float64
	padding      layout.EdgeInsets
	bgColor      rendering.Color
	borderColor  rendering.Color
	focusColor   rendering.Color
	borderRadius float64
	state        *nativeTextFieldState
	config       NativeTextField
}

func (n nativeTextFieldRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(n, nil)
}

func (n nativeTextFieldRender) Key() any {
	return nil
}

func (n nativeTextFieldRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderNativeTextField{
		text:         n.text,
		style:        n.style,
		width:        n.width,
		height:       n.height,
		padding:      n.padding,
		bgColor:      n.bgColor,
		borderColor:  n.borderColor,
		focusColor:   n.focusColor,
		borderRadius: n.borderRadius,
		state:        n.state,
	}

	r.SetSelf(r)
	return r
}

func (n nativeTextFieldRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderNativeTextField); ok {
		r.text = n.text
		r.style = n.style
		r.width = n.width
		r.height = n.height
		r.padding = n.padding
		r.bgColor = n.bgColor
		r.borderColor = n.borderColor
		r.focusColor = n.focusColor
		r.borderRadius = n.borderRadius
		r.state = n.state
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderNativeTextField struct {
	layout.RenderBoxBase
	text         string
	style        rendering.TextStyle
	width        float64
	height       float64
	padding      layout.EdgeInsets
	bgColor      rendering.Color
	borderColor  rendering.Color
	focusColor   rendering.Color
	borderRadius float64
	state        *nativeTextFieldState
	layout       *rendering.TextLayout
	tap          *gestures.TapGestureRecognizer
}

func (r *renderNativeTextField) PerformLayout() {
	constraints := r.Constraints()
	width := r.width
	if width == 0 {
		width = constraints.MaxWidth
	}
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)

	height := r.height
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)

	r.SetSize(rendering.Size{Width: width, Height: height})

	// Layout text for display
	if r.text != "" {
		manager, _ := rendering.DefaultFontManagerErr()
		if manager == nil {
			// Error already reported by DefaultFontManagerErr
			r.layout = nil
		} else {
			layout, err := rendering.LayoutText(r.text, r.style, manager)
			if err == nil {
				r.layout = layout
			} else {
				r.layout = nil
			}
		}
	} else {
		r.layout = nil
	}
}

func (r *renderNativeTextField) Paint(ctx *layout.PaintContext) {
	size := r.Size()

	// Draw background
	bgPaint := rendering.DefaultPaint()
	bgPaint.Color = r.bgColor

	if r.borderRadius > 0 {
		rrect := rendering.RRectFromRectAndRadius(
			rendering.RectFromLTWH(0, 0, size.Width, size.Height),
			rendering.CircularRadius(r.borderRadius),
		)
		ctx.Canvas.DrawRRect(rrect, bgPaint)
	} else {
		ctx.Canvas.DrawRect(rendering.RectFromLTWH(0, 0, size.Width, size.Height), bgPaint)
	}

	// Draw border
	borderPaint := rendering.DefaultPaint()
	borderPaint.Color = r.borderColor
	borderPaint.Style = rendering.PaintStyleStroke
	borderPaint.StrokeWidth = 1

	if r.borderRadius > 0 {
		rrect := rendering.RRectFromRectAndRadius(
			rendering.RectFromLTWH(0.5, 0.5, size.Width-1, size.Height-1),
			rendering.CircularRadius(r.borderRadius),
		)
		ctx.Canvas.DrawRRect(rrect, borderPaint)
	} else {
		ctx.Canvas.DrawRect(rendering.RectFromLTWH(0.5, 0.5, size.Width-1, size.Height-1), borderPaint)
	}

	// Draw text
	if r.layout != nil {
		// Center text vertically
		textY := (size.Height - r.layout.Size.Height) / 2
		offset := rendering.Offset{X: r.padding.Left, Y: textY}
		ctx.Canvas.DrawText(r.layout, offset)
	}

	// Draw focus indicator
	if r.state != nil && r.state.focused {
		focusPaint := rendering.DefaultPaint()
		focusPaint.Color = r.focusColor
		focusPaint.Style = rendering.PaintStyleStroke
		focusPaint.StrokeWidth = 2

		if r.borderRadius > 0 {
			rrect := rendering.RRectFromRectAndRadius(
				rendering.RectFromLTWH(1, 1, size.Width-2, size.Height-2),
				rendering.CircularRadius(r.borderRadius),
			)
			ctx.Canvas.DrawRRect(rrect, focusPaint)
		} else {
			ctx.Canvas.DrawRect(rendering.RectFromLTWH(1, 1, size.Width-2, size.Height-2), focusPaint)
		}
	}
}

func (r *renderNativeTextField) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

// HandlePointer implements PointerHandler for gesture recognition.
func (r *renderNativeTextField) HandlePointer(event gestures.PointerEvent) {
	if r.tap == nil {
		r.tap = gestures.NewTapGestureRecognizer(gestures.DefaultArena)
		r.tap.OnTap = func() {
			if r.state != nil {
				r.state.focus(r) // Pass self as the focused target
			}
		}
	}

	if event.Phase == gestures.PointerPhaseDown {
		r.tap.AddPointer(event)
	} else {
		r.tap.HandleEvent(event)
	}
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderNativeTextField) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleTextField

	// Set flags
	flags := semantics.SemanticsIsTextField | semantics.SemanticsIsFocusable | semantics.SemanticsHasEnabledState
	if r.state != nil && r.state.focused {
		flags = flags.Set(semantics.SemanticsIsFocused)
	}
	// Check if enabled via the state's widget config
	if r.state != nil && r.state.element != nil {
		if w, ok := r.state.element.Widget().(NativeTextField); ok {
			if !w.Disabled {
				flags = flags.Set(semantics.SemanticsIsEnabled)
			}
			if w.Obscure {
				flags = flags.Set(semantics.SemanticsIsObscured)
			}
		}
	}
	config.Properties.Flags = flags

	// Set current value (text content)
	config.Properties.Value = r.text

	// Set hint
	config.Properties.Hint = "Double tap to edit"

	// Set actions
	config.Actions = semantics.NewSemanticsActions()
	config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {
		if r.state != nil {
			r.state.focus(r)
		}
	})
	config.Actions.SetHandler(semantics.SemanticsActionFocus, func(args any) {
		if r.state != nil {
			r.state.focus(r)
		}
	})

	return true
}
