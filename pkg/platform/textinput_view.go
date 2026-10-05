package platform

import (
	"fmt"
	"sync"
)

// TextInputViewConfig defines styling passed to native text view.
type TextInputViewConfig struct {
	// Text styling (native view handles rendering)
	FontFamily       string
	FontSize         float64
	FontWeight       int    // 400=regular, 700=bold
	TextColor        uint32 // ARGB
	PlaceholderColor uint32 // ARGB
	TextAlignment    int    // 0=left, 1=center, 2=right

	// Behavior
	Multiline      bool
	MaxLines       int
	Obscure        bool
	Autocorrect    bool
	KeyboardType   KeyboardType
	InputAction    TextInputAction
	Capitalization TextCapitalization

	// Padding inside native view
	PaddingLeft   float64
	PaddingTop    float64
	PaddingRight  float64
	PaddingBottom float64

	// Placeholder text
	Placeholder string
}

// TextInputViewClient receives callbacks from native text input view.
type TextInputViewClient interface {
	// OnTextChanged is called when text or selection changes.
	OnTextChanged(text string, selectionBase, selectionExtent int)

	// OnAction is called when keyboard action button is pressed.
	OnAction(action TextInputAction)

	// OnFocusChanged is called when focus state changes.
	OnFocusChanged(focused bool)
}

// TextInputView is a platform view for text input.
type TextInputView struct {
	PlatformViewBase
	config  TextInputViewConfig
	client  TextInputViewClient
	text    string
	selBase int
	selExt  int
	focused bool
	mu      sync.RWMutex
}

// NewTextInputView creates a new text input platform view.
func NewTextInputView(viewID int64, config TextInputViewConfig, client TextInputViewClient) *TextInputView {
	return &TextInputView{
		PlatformViewBase: NewPlatformViewBase(viewID, "textinput"),
		config:           config,
		client:           client,
		selBase:          0,
		selExt:           0,
	}
}

// SetClient sets the callback client for this view.
func (v *TextInputView) SetClient(client TextInputViewClient) {
	v.mu.Lock()
	v.client = client
	v.mu.Unlock()
}

// Create initializes the native view.
func (v *TextInputView) Create(params map[string]any) error {
	// View creation is handled by the registry
	return nil
}

// Dispose cleans up the native view.
func (v *TextInputView) Dispose() {
	// Cleanup handled by registry
}

// SetText updates the text content from Go side.
func (v *TextInputView) SetText(text string) {
	v.mu.Lock()
	v.text = text
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setText", map[string]any{
		"text": text,
	})
}

// SetSelection updates the cursor/selection position.
func (v *TextInputView) SetSelection(base, extent int) {
	v.mu.Lock()
	v.selBase = base
	v.selExt = extent
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setSelection", map[string]any{
		"selectionBase":   base,
		"selectionExtent": extent,
	})
}

// SetValue updates both text and selection atomically.
func (v *TextInputView) SetValue(value TextEditingValue) {
	v.mu.Lock()
	v.text = value.Text
	v.selBase = value.Selection.BaseOffset
	v.selExt = value.Selection.ExtentOffset
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setValue", map[string]any{
		"text":            value.Text,
		"selectionBase":   value.Selection.BaseOffset,
		"selectionExtent": value.Selection.ExtentOffset,
	})
}

// Text returns the current text.
func (v *TextInputView) Text() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.text
}

// Selection returns the current selection.
func (v *TextInputView) Selection() (base, extent int) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.selBase, v.selExt
}

// Snapshot returns the current text and selection under a single lock,
// ensuring a consistent read when both are needed together.
func (v *TextInputView) Snapshot() (text string, selBase, selExt int) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.text, v.selBase, v.selExt
}

// Focus requests keyboard focus for the text input.
func (v *TextInputView) Focus() {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "focus", nil)
}

// Blur dismisses the keyboard.
func (v *TextInputView) Blur() {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "blur", nil)
}

// IsFocused returns whether the view has focus.
func (v *TextInputView) IsFocused() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.focused
}

// UpdateConfig updates the view configuration.
func (v *TextInputView) UpdateConfig(config TextInputViewConfig) {
	v.mu.Lock()
	v.config = config
	v.mu.Unlock()

	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "updateConfig", map[string]any{
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
	})
}

// handleTextChanged processes text change events from native.
func (v *TextInputView) handleTextChanged(text string, selBase, selExt int) {
	v.mu.Lock()
	v.text = text
	v.selBase = selBase
	v.selExt = selExt
	v.mu.Unlock()

	if v.client != nil {
		v.client.OnTextChanged(text, selBase, selExt)
	}
}

// handleAction processes action events from native.
func (v *TextInputView) handleAction(action TextInputAction) {
	if v.client != nil {
		v.client.OnAction(action)
	}
}

// handleFocusChanged processes focus change events from native.
func (v *TextInputView) handleFocusChanged(focused bool) {
	v.mu.Lock()
	v.focused = focused
	v.mu.Unlock()

	if v.client != nil {
		v.client.OnFocusChanged(focused)
	}
}

// textInputViewFactory creates text input platform views.
type textInputViewFactory struct{}

func (f *textInputViewFactory) ViewType() string {
	return "textinput"
}

func (f *textInputViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	// Extract config from params
	config := TextInputViewConfig{}

	if v, ok := params["fontFamily"].(string); ok {
		config.FontFamily = v
	}
	if v, ok := toFloat64(params["fontSize"]); ok {
		config.FontSize = v
	}
	if v, ok := toInt(params["fontWeight"]); ok {
		config.FontWeight = v
	}
	if v, ok := toUint32(params["textColor"]); ok {
		config.TextColor = v
	}
	if v, ok := toUint32(params["placeholderColor"]); ok {
		config.PlaceholderColor = v
	}
	if v, ok := toInt(params["textAlignment"]); ok {
		config.TextAlignment = v
	}
	if v, ok := params["multiline"].(bool); ok {
		config.Multiline = v
	}
	if v, ok := toInt(params["maxLines"]); ok {
		config.MaxLines = v
	}
	if v, ok := params["obscure"].(bool); ok {
		config.Obscure = v
	}
	if v, ok := params["autocorrect"].(bool); ok {
		config.Autocorrect = v
	}
	if v, ok := toInt(params["keyboardType"]); ok {
		config.KeyboardType = KeyboardType(v)
	}
	if v, ok := toInt(params["inputAction"]); ok {
		config.InputAction = TextInputAction(v)
	}
	if v, ok := toInt(params["capitalization"]); ok {
		config.Capitalization = TextCapitalization(v)
	}
	if v, ok := toFloat64(params["paddingLeft"]); ok {
		config.PaddingLeft = v
	}
	if v, ok := toFloat64(params["paddingTop"]); ok {
		config.PaddingTop = v
	}
	if v, ok := toFloat64(params["paddingRight"]); ok {
		config.PaddingRight = v
	}
	if v, ok := toFloat64(params["paddingBottom"]); ok {
		config.PaddingBottom = v
	}
	if v, ok := params["placeholder"].(string); ok {
		config.Placeholder = v
	}

	// The client will be set later by the widget
	view := NewTextInputView(viewID, config, nil)
	return view, nil
}

// toFloat64 converts various numeric types to float64.
// RegisterTextInputViewFactory registers the text input view factory.
func RegisterTextInputViewFactory() {
	GetPlatformViewRegistry().RegisterFactory(&textInputViewFactory{})
}

func init() {
	RegisterTextInputViewFactory()
}

// HandleEvent implements PlatformView: text, action and focus changes from
// the native field.
func (v *TextInputView) HandleEvent(method string, args map[string]any) error {
	switch method {
	case "onTextChanged":
		text, err := requireString(method, args, "text")
		if err != nil {
			return err
		}
		selBase, err := requireInt(method, args, "selectionBase")
		if err != nil {
			return err
		}
		selExt, err := requireInt(method, args, "selectionExtent")
		if err != nil {
			return err
		}
		v.handleTextChanged(text, selBase, selExt)
	case "onAction":
		action, err := requireInt(method, args, "action")
		if err != nil {
			return err
		}
		v.handleAction(TextInputAction(action))
	case "onFocusChanged":
		focused, err := requireBool(method, args, "focused")
		if err != nil {
			return err
		}
		v.handleFocusChanged(focused)
	default:
		return fmt.Errorf("%s: %w", method, ErrMethodNotFound)
	}
	return nil
}
