package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/focus"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
)

// TextInput is the lowest-level text input widget that embeds a native platform
// text field with Skia-rendered chrome (background, border, focus styling).
//
// TextInput provides direct access to all text input configuration options including
// keyboard type, input action, text capitalization, obscuring for passwords,
// autocorrect, and multiline support. It handles focus management, gesture
// recognition, and accessibility semantics automatically.
//
// Visual properties (BackgroundColor, BorderColor, etc.) fall back to built-in
// defaults when their value is zero and they have not been explicitly set via
// a WithX method. Use the WithX methods to set a value that should be used even
// when it equals zero (e.g., [TextInput.WithBorderRadius](0) for sharp corners,
// or [TextInput.WithBorderWidth](0) to remove the border).
//
// For most use cases, prefer [TextField] which adds labels, helper text, and
// theme integration, or [TextFormField] which adds form validation support.
//
// The native text field handles all text editing, selection, and IME composition.
// Skia renders the visual chrome (background, borders) while the platform view
// handles the actual text rendering and cursor.
//
// Example:
//
//	controller := platform.NewTextEditingController("")
//	TextInput{
//	    Controller:   controller,
//	    Placeholder:  "Enter text",
//	    KeyboardType: platform.KeyboardTypeText,
//	    OnChanged:    func(text string) { fmt.Println("Changed:", text) },
//	    OnSubmitted:  func(text string) { fmt.Println("Submitted:", text) },
//	}
type TextInput struct {
	// Controller manages the text content and selection.
	Controller *platform.TextEditingController

	// Style for the text.
	Style graphics.TextStyle

	// Placeholder text shown when empty.
	Placeholder string

	// KeyboardType specifies the keyboard to show.
	KeyboardType platform.KeyboardType

	// InputAction specifies the keyboard action button.
	InputAction platform.TextInputAction

	// Capitalization specifies text capitalization behavior.
	// Defaults to None. Set to TextCapitalizationSentences for standard text input.
	Capitalization platform.TextCapitalization

	// Obscure hides the text (for passwords).
	Obscure bool

	// Autocorrect enables auto-correction.
	Autocorrect bool

	// Multiline enables multiline text input.
	Multiline bool

	// MaxLines limits the number of lines (multiline only).
	MaxLines int

	// OnChanged is called when the text changes.
	OnChanged func(string)

	// OnSubmitted is called when the user submits (presses done/return).
	OnSubmitted func(string)

	// OnEditingComplete is called when editing is complete.
	OnEditingComplete func()

	// OnFocusChange is called when focus changes.
	OnFocusChange func(bool)

	// Disabled controls whether the field rejects input.
	Disabled bool

	// Width of the text field (0 = expand to fill).
	Width float64

	// Height of the text field.
	Height float64

	// Padding inside the text field.
	Padding layout.EdgeInsets

	// BackgroundColor of the text field.
	BackgroundColor graphics.Color

	// BorderColor of the text field.
	BorderColor graphics.Color

	// FocusColor of the text field outline.
	FocusColor graphics.Color

	// BorderRadius for rounded corners.
	BorderRadius float64

	// BorderWidth for the border stroke.
	BorderWidth float64

	// PlaceholderColor is the color for placeholder text.
	PlaceholderColor graphics.Color
	// overrides tracks which fields were explicitly set via WithX methods.
	overrides textInputOverrides
}

type textInputOverrides uint16

const (
	textInputOverrideBackgroundColor  textInputOverrides = 1 << iota
	textInputOverrideBorderColor
	textInputOverrideFocusColor
	textInputOverridePlaceholderColor
	textInputOverrideBorderRadius
	textInputOverrideHeight
	textInputOverridePadding
	textInputOverrideBorderWidth
)

// WithBackgroundColor returns a copy with the specified background color.
// The value is marked as explicitly set, bypassing defaults even when zero.
func (n TextInput) WithBackgroundColor(c graphics.Color) TextInput {
	n.BackgroundColor = c
	n.overrides |= textInputOverrideBackgroundColor
	return n
}

// WithBorderColor returns a copy with the specified border color.
// The value is marked as explicitly set, bypassing defaults even when zero.
func (n TextInput) WithBorderColor(c graphics.Color) TextInput {
	n.BorderColor = c
	n.overrides |= textInputOverrideBorderColor
	return n
}

// WithFocusColor returns a copy with the specified focus outline color.
// The value is marked as explicitly set, bypassing defaults even when zero.
func (n TextInput) WithFocusColor(c graphics.Color) TextInput {
	n.FocusColor = c
	n.overrides |= textInputOverrideFocusColor
	return n
}

// WithPlaceholderColor returns a copy with the specified placeholder text color.
// The value is marked as explicitly set, bypassing defaults even when zero.
func (n TextInput) WithPlaceholderColor(c graphics.Color) TextInput {
	n.PlaceholderColor = c
	n.overrides |= textInputOverridePlaceholderColor
	return n
}

// WithBorderRadius returns a copy with the specified corner radius.
// The value is marked as explicitly set, so even zero (sharp corners)
// will be used instead of falling back to the default.
func (n TextInput) WithBorderRadius(radius float64) TextInput {
	n.BorderRadius = radius
	n.overrides |= textInputOverrideBorderRadius
	return n
}

// WithHeight returns a copy with the specified height.
// The value is marked as explicitly set, bypassing defaults even when zero.
func (n TextInput) WithHeight(height float64) TextInput {
	n.Height = height
	n.overrides |= textInputOverrideHeight
	return n
}

// WithPadding returns a copy with the specified internal padding.
// The value is marked as explicitly set, so even a zero [layout.EdgeInsets]
// will be used instead of falling back to the default.
func (n TextInput) WithPadding(padding layout.EdgeInsets) TextInput {
	n.Padding = padding
	n.overrides |= textInputOverridePadding
	return n
}

// WithBorderWidth returns a copy with the specified border stroke width.
// The value is marked as explicitly set, so even zero (no border)
// will be used instead of falling back to the default.
func (n TextInput) WithBorderWidth(width float64) TextInput {
	n.BorderWidth = width
	n.overrides |= textInputOverrideBorderWidth
	return n
}

// CreateElement creates the element for the stateful widget.
func (n TextInput) CreateElement() core.Element {
	return core.NewStatefulElement(n, nil)
}

// Key returns the widget key.
func (n TextInput) Key() any {
	return nil
}

// CreateState creates the state for this widget.
func (n TextInput) CreateState() core.State {
	return &textInputState{}
}

type textInputState struct {
	element            *core.StatefulElement
	platformView       *platform.TextInputView
	focused            bool
	focusNode          *focus.FocusNode
	updatingController bool // suppress echo during programmatic updates
}

func (s *textInputState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *textInputState) InitState() {
	// Create and register focus node for tab navigation
	s.focusNode = &focus.FocusNode{
		CanRequestFocus: true,
		DebugLabel:      "TextInput",
		Rect:            s, // s implements RectProvider
		OnFocusChange: func(hasFocus bool) {
			if hasFocus && !s.focused {
				s.focus()
			} else if !hasFocus && s.focused {
				s.unfocus()
			}
		},
	}
	manager := focus.GetFocusManager()
	if manager.RootScope != nil {
		manager.RootScope.Children = append(manager.RootScope.Children, s.focusNode)
	}
}

func (s *textInputState) Dispose() {
	// Dispose platform view
	if s.platformView != nil {
		platform.GetPlatformViewRegistry().Dispose(s.platformView.ViewID())
		s.platformView = nil
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

func (s *textInputState) DidChangeDependencies() {}

func (s *textInputState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	// Update platform view config if needed
	if s.platformView != nil {
		w := s.element.Widget().(TextInput)
		s.updatePlatformViewConfig(w)

		// Sync controller value if it changed programmatically
		if w.Controller != nil {
			s.updatingController = true
			s.platformView.SetValue(w.Controller.Value())
			s.updatingController = false
		}
	}
}

// FocusRect implements focus.RectProvider for directional navigation.
func (s *textInputState) FocusRect() focus.FocusRect {
	if s.element == nil {
		return focus.FocusRect{}
	}
	offset := core.GlobalOffsetOf(s.element)
	if ro := s.element.RenderObject(); ro != nil {
		if sizer, ok := ro.(interface{ Size() graphics.Size }); ok {
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

func (s *textInputState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *textInputState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(TextInput)

	// Default values (only when not explicitly set via WithX)
	height := w.Height
	if w.overrides&textInputOverrideHeight == 0 && height == 0 {
		height = 44 // Standard text field height
	}

	padding := w.Padding
	if w.overrides&textInputOverridePadding == 0 && padding == (layout.EdgeInsets{}) {
		padding = layout.EdgeInsetsSymmetric(12, 8)
	}

	bgColor := w.BackgroundColor
	if w.overrides&textInputOverrideBackgroundColor == 0 && bgColor == 0 {
		bgColor = graphics.ColorWhite
	}

	borderColor := w.BorderColor
	if w.overrides&textInputOverrideBorderColor == 0 && borderColor == 0 {
		borderColor = graphics.Color(0xFFCCCCCC)
	}

	focusColor := w.FocusColor
	if w.overrides&textInputOverrideFocusColor == 0 && focusColor == 0 {
		focusColor = graphics.Color(0xFF007AFF)
	}

	borderWidth := w.BorderWidth
	if w.overrides&textInputOverrideBorderWidth == 0 && borderWidth == 0 {
		borderWidth = 1
	}

	return textInputRender{
		width:        w.Width,
		height:       height,
		padding:      padding,
		bgColor:      bgColor,
		borderColor:  borderColor,
		focusColor:   focusColor,
		borderRadius: w.BorderRadius,
		borderWidth:  borderWidth,
		state:        s,
		config:       w,
	}
}

// ensurePlatformView creates the native text input view if not already created.
func (s *textInputState) ensurePlatformView() {
	if s.platformView != nil {
		return
	}

	w := s.element.Widget().(TextInput)
	config := s.buildPlatformViewConfig(w)

	params := map[string]any{
		"fontFamily":       config.FontFamily,
		"fontSize":         config.FontSize,
		"fontWeight":       config.FontWeight,
		"textColor":        config.TextColor,
		"placeholderColor": config.PlaceholderColor,
		"textAlignment":    config.TextAlignment,
		"multiline":        config.Multiline,
		"maxLines":         config.MaxLines,
		"obscure":          config.Obscure,
		"autocorrect":      config.Autocorrect,
		"keyboardType":     int(config.KeyboardType),
		"inputAction":      int(config.InputAction),
		"capitalization":   int(config.Capitalization),
		"paddingLeft":      config.PaddingLeft,
		"paddingTop":       config.PaddingTop,
		"paddingRight":     config.PaddingRight,
		"paddingBottom":    config.PaddingBottom,
		"placeholder":      config.Placeholder,
	}

	// Include initial text if controller is set
	if w.Controller != nil {
		params["text"] = w.Controller.Text()
	}

	view, err := platform.GetPlatformViewRegistry().Create("textinput", params)
	if err != nil {
		return
	}

	textInputView, ok := view.(*platform.TextInputView)
	if !ok {
		return
	}

	s.platformView = textInputView

	// Register as client (this is done via a custom method since factory creates without client)
	// We need to set the client after creation
	s.registerAsClient()
}

// registerAsClient sets up this state as the callback receiver for the platform view.
func (s *textInputState) registerAsClient() {
	if s.platformView == nil {
		return
	}

	// Set this state as the client for callbacks
	s.platformView.SetClient(s)
}

func (s *textInputState) buildPlatformViewConfig(w TextInput) platform.TextInputViewConfig {
	// Apply default padding to match Skia chrome
	padding := w.Padding
	if w.overrides&textInputOverridePadding == 0 && padding == (layout.EdgeInsets{}) {
		padding = layout.EdgeInsetsSymmetric(12, 8)
	}

	config := platform.TextInputViewConfig{
		FontFamily:     w.Style.FontFamily,
		FontSize:       w.Style.FontSize,
		FontWeight:     int(w.Style.FontWeight),
		Multiline:      w.Multiline,
		MaxLines:       w.MaxLines,
		Obscure:        w.Obscure,
		Autocorrect:    w.Autocorrect,
		KeyboardType:   w.KeyboardType,
		InputAction:    w.InputAction,
		Capitalization: w.Capitalization,
		PaddingLeft:    padding.Left,
		PaddingTop:     padding.Top,
		PaddingRight:   padding.Right,
		PaddingBottom:  padding.Bottom,
		Placeholder:    w.Placeholder,
	}

	if config.FontSize == 0 {
		config.FontSize = 16
	}

	// Convert colors to ARGB uint32
	textColor := w.Style.Color
	if textColor == 0 {
		textColor = graphics.Color(0xFF000000) // black
	}
	config.TextColor = uint32(textColor)

	placeholderColor := w.PlaceholderColor
	if w.overrides&textInputOverridePlaceholderColor == 0 && placeholderColor == 0 {
		placeholderColor = graphics.Color(0xFF999999)
	}
	config.PlaceholderColor = uint32(placeholderColor)

	return config
}

func (s *textInputState) updatePlatformViewConfig(w TextInput) {
	if s.platformView == nil {
		return
	}
	config := s.buildPlatformViewConfig(w)
	s.platformView.UpdateConfig(config)
}

// OnTextChanged implements TextInputViewClient.
func (s *textInputState) OnTextChanged(text string, selectionBase, selectionExtent int) {
	w := s.element.Widget().(TextInput)
	if w.Controller == nil {
		return
	}

	// Don't echo back during programmatic updates
	if s.updatingController {
		return
	}

	oldText := w.Controller.Text()

	// Update controller
	w.Controller.SetValue(platform.TextEditingValue{
		Text: text,
		Selection: platform.TextSelection{
			BaseOffset:   selectionBase,
			ExtentOffset: selectionExtent,
		},
		ComposingRange: platform.TextRangeEmpty,
	})

	// Only trigger OnChanged if text actually changed
	if w.OnChanged != nil && text != oldText {
		w.OnChanged(text)
	}

	s.SetState(func() {})
}

// OnAction implements TextInputViewClient.
func (s *textInputState) OnAction(action platform.TextInputAction) {
	w := s.element.Widget().(TextInput)

	switch action {
	case platform.TextInputActionDone, platform.TextInputActionGo,
		platform.TextInputActionSearch, platform.TextInputActionSend:
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

// OnFocusChanged implements TextInputViewClient.
func (s *textInputState) OnFocusChanged(focused bool) {
	w := s.element.Widget().(TextInput)

	s.SetState(func() {
		s.focused = focused
	})

	if w.OnFocusChange != nil {
		w.OnFocusChange(focused)
	}

	if focused {
		// Sync focus node
		if s.focusNode != nil {
			s.focusNode.RequestFocus()
		}
		// Set focused target for tap-outside-to-unfocus.
		// This handles the case when native view gains focus directly
		// (e.g., user taps on EditText) rather than through our tap gesture.
		if s.element != nil {
			platform.SetFocusedTarget(s.element.RenderObject())
		}
		// Track focused input
		if s.platformView != nil {
			platform.SetFocusedInput(s.platformView.ViewID(), true)
		}
	} else {
		// Clear focused input tracking
		if s.platformView != nil {
			platform.SetFocusedInput(s.platformView.ViewID(), false)
		}
	}
}

func (s *textInputState) focus() {
	if s.focused {
		return
	}

	w := s.element.Widget().(TextInput)
	if w.Disabled {
		return
	}

	s.focused = true

	// Sync with focus system
	if s.focusNode != nil {
		s.focusNode.RequestFocus()
	}

	// Ensure platform view exists
	s.ensurePlatformView()

	// Sync controller value to native view
	if s.platformView != nil && w.Controller != nil {
		s.updatingController = true
		s.platformView.SetValue(w.Controller.Value())
		s.updatingController = false
		s.platformView.Focus()

		// Track focused input for UnfocusAll/HasFocus
		platform.SetFocusedInput(s.platformView.ViewID(), true)
	}

	// Set this as the focused target for tap-outside-to-unfocus
	if s.element != nil {
		platform.SetFocusedTarget(s.element.RenderObject())
	}

	s.SetState(func() {})
}

func (s *textInputState) unfocus() {
	if !s.focused {
		return
	}

	s.focused = false

	if s.platformView != nil {
		s.platformView.Blur()
		// Clear focused input tracking
		platform.SetFocusedInput(s.platformView.ViewID(), false)
	}

	s.SetState(func() {})
}

// textInputRender is a render widget for the text field chrome.
type textInputRender struct {
	width        float64
	height       float64
	padding      layout.EdgeInsets
	bgColor      graphics.Color
	borderColor  graphics.Color
	focusColor   graphics.Color
	borderRadius float64
	borderWidth  float64
	state        *textInputState
	config       TextInput
}

func (n textInputRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(n, nil)
}

func (n textInputRender) Key() any {
	return nil
}

func (n textInputRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderTextInput{
		width:        n.width,
		height:       n.height,
		padding:      n.padding,
		bgColor:      n.bgColor,
		borderColor:  n.borderColor,
		focusColor:   n.focusColor,
		borderRadius: n.borderRadius,
		borderWidth:  n.borderWidth,
		state:        n.state,
		config:       n.config,
	}

	r.SetSelf(r)
	return r
}

func (n textInputRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderTextInput); ok {
		r.width = n.width
		r.height = n.height
		r.padding = n.padding
		r.bgColor = n.bgColor
		r.borderColor = n.borderColor
		r.focusColor = n.focusColor
		r.borderRadius = n.borderRadius
		r.borderWidth = n.borderWidth
		r.state = n.state
		r.config = n.config
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderTextInput struct {
	layout.RenderBoxBase
	width        float64
	height       float64
	padding      layout.EdgeInsets
	bgColor      graphics.Color
	borderColor  graphics.Color
	focusColor   graphics.Color
	borderRadius float64
	borderWidth  float64
	state        *textInputState
	config       TextInput
	tap          *gestures.TapGestureRecognizer
}

func (r *renderTextInput) PerformLayout() {
	constraints := r.Constraints()
	width := r.width
	if width == 0 {
		width = constraints.MaxWidth
	}
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)

	height := r.height
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)

	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderTextInput) updatePlatformView(clipBounds *graphics.Rect) {
	if r.state == nil || r.state.element == nil {
		return
	}

	// Ensure view exists
	r.state.ensurePlatformView()

	if r.state.platformView == nil {
		return
	}

	// Get global position
	globalOffset := core.GlobalOffsetOf(r.state.element)
	size := r.Size()

	// Update native view geometry with clip bounds
	// Note: SetGeometry/applyClipBounds controls visibility based on clip state
	r.state.platformView.SetGeometry(globalOffset, size, clipBounds)
	r.state.platformView.SetEnabled(!r.config.Disabled)
}

func (r *renderTextInput) Paint(ctx *layout.PaintContext) {
	// Get clip bounds for platform view
	clip, hasClip := ctx.CurrentClipBounds()
	var clipPtr *graphics.Rect
	if hasClip {
		clipPtr = &clip
	}

	// Update platform view position each frame to animate with page transitions
	r.updatePlatformView(clipPtr)

	size := r.Size()

	// Draw background
	bgPaint := graphics.DefaultPaint()
	bgPaint.Color = r.bgColor

	if r.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(
			graphics.RectFromLTWH(0, 0, size.Width, size.Height),
			graphics.CircularRadius(r.borderRadius),
		)
		ctx.Canvas.DrawRRect(rrect, bgPaint)
	} else {
		ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, size.Width, size.Height), bgPaint)
	}

	// Draw border
	borderPaint := graphics.DefaultPaint()
	borderPaint.Style = graphics.PaintStyleStroke
	borderPaint.StrokeWidth = r.borderWidth

	// Use focus color when focused, otherwise border color
	if r.state != nil && r.state.focused {
		borderPaint.Color = r.focusColor
		borderPaint.StrokeWidth = 2 // Thicker border when focused
	} else {
		borderPaint.Color = r.borderColor
	}

	halfStroke := borderPaint.StrokeWidth / 2
	if r.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(
			graphics.RectFromLTWH(halfStroke, halfStroke, size.Width-borderPaint.StrokeWidth, size.Height-borderPaint.StrokeWidth),
			graphics.CircularRadius(r.borderRadius),
		)
		ctx.Canvas.DrawRRect(rrect, borderPaint)
	} else {
		ctx.Canvas.DrawRect(graphics.RectFromLTWH(halfStroke, halfStroke, size.Width-borderPaint.StrokeWidth, size.Height-borderPaint.StrokeWidth), borderPaint)
	}

	// Native view handles text rendering - no Skia text drawing needed
}

func (r *renderTextInput) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

// HandlePointer implements PointerHandler for gesture recognition.
func (r *renderTextInput) HandlePointer(event gestures.PointerEvent) {
	if r.tap == nil {
		r.tap = gestures.NewTapGestureRecognizer(gestures.DefaultArena)
		r.tap.OnTap = func() {
			if r.state != nil {
				r.state.focus()
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
func (r *renderTextInput) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleTextField

	// Set flags
	flags := semantics.SemanticsIsTextField | semantics.SemanticsIsFocusable | semantics.SemanticsHasEnabledState
	if r.state != nil && r.state.focused {
		flags = flags.Set(semantics.SemanticsIsFocused)
	}
	if r.state != nil && r.state.element != nil {
		if w, ok := r.state.element.Widget().(TextInput); ok {
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
	if r.state != nil && r.state.element != nil {
		if w, ok := r.state.element.Widget().(TextInput); ok && w.Controller != nil {
			config.Properties.Value = w.Controller.Text()
		}
	}

	// Set hint
	config.Properties.Hint = "Double tap to edit"

	// Set actions
	config.Actions = semantics.NewSemanticsActions()
	config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {
		if r.state != nil {
			r.state.focus()
		}
	})
	config.Actions.SetHandler(semantics.SemanticsActionFocus, func(args any) {
		if r.state != nil {
			r.state.focus()
		}
	})

	return true
}
