package platform

import (
	"sync"
)

// TextAffinity describes which side of a position the caret prefers.
type TextAffinity int

const (
	// TextAffinityUpstream - the caret is placed at the end of the previous character.
	TextAffinityUpstream TextAffinity = iota
	// TextAffinityDownstream - the caret is placed at the start of the next character.
	TextAffinityDownstream
)

// TextRange represents a range of text.
type TextRange struct {
	Start int
	End   int
}

// IsEmpty returns true if the range has zero length.
func (r TextRange) IsEmpty() bool {
	return r.Start == r.End
}

// IsValid returns true if both start and end are non-negative.
func (r TextRange) IsValid() bool {
	return r.Start >= 0 && r.End >= 0
}

// IsNormalized returns true if start <= end.
func (r TextRange) IsNormalized() bool {
	return r.Start <= r.End
}

// TextRangeEmpty is an invalid/empty text range.
var TextRangeEmpty = TextRange{Start: -1, End: -1}

// TextSelection represents the current text selection.
type TextSelection struct {
	// BaseOffset is the position where the selection started.
	BaseOffset int
	// ExtentOffset is the position where the selection ended.
	ExtentOffset int
	// Affinity indicates which direction the caret prefers.
	Affinity TextAffinity
	// IsDirectional is true if the selection has a direction.
	IsDirectional bool
}

// Start returns the smaller of BaseOffset and ExtentOffset.
func (s TextSelection) Start() int {
	if s.BaseOffset < s.ExtentOffset {
		return s.BaseOffset
	}
	return s.ExtentOffset
}

// End returns the larger of BaseOffset and ExtentOffset.
func (s TextSelection) End() int {
	if s.BaseOffset > s.ExtentOffset {
		return s.BaseOffset
	}
	return s.ExtentOffset
}

// IsCollapsed returns true if the selection has no length (just a cursor).
func (s TextSelection) IsCollapsed() bool {
	return s.BaseOffset == s.ExtentOffset
}

// IsValid returns true if both offsets are non-negative.
func (s TextSelection) IsValid() bool {
	return s.BaseOffset >= 0 && s.ExtentOffset >= 0
}

// TextSelectionCollapsed creates a collapsed selection at the given offset.
func TextSelectionCollapsed(offset int) TextSelection {
	return TextSelection{
		BaseOffset:   offset,
		ExtentOffset: offset,
		Affinity:     TextAffinityDownstream,
	}
}

// TextEditingValue represents the current text editing state.
type TextEditingValue struct {
	// Text is the current text content.
	Text string
	// Selection is the current selection within the text.
	Selection TextSelection
	// ComposingRange is the range currently being composed by IME.
	ComposingRange TextRange
}

// TextEditingValueEmpty is the default empty editing value.
var TextEditingValueEmpty = TextEditingValue{
	Selection:      TextSelectionCollapsed(0),
	ComposingRange: TextRangeEmpty,
}

// IsComposing returns true if there is an active IME composition.
func (v TextEditingValue) IsComposing() bool {
	return v.ComposingRange.IsValid() && !v.ComposingRange.IsEmpty()
}

// KeyboardType specifies the type of keyboard to show.
type KeyboardType int

const (
	KeyboardTypeText KeyboardType = iota
	KeyboardTypeNumber
	KeyboardTypePhone
	KeyboardTypeEmail
	KeyboardTypeURL
	KeyboardTypePassword
	KeyboardTypeMultiline
)

// TextInputAction specifies the action button on the keyboard.
type TextInputAction int

const (
	TextInputActionNone TextInputAction = iota
	TextInputActionDone
	TextInputActionGo
	TextInputActionNext
	TextInputActionPrevious
	TextInputActionSearch
	TextInputActionSend
	TextInputActionNewline
)

// TextCapitalization specifies text capitalization behavior.
type TextCapitalization int

const (
	TextCapitalizationNone TextCapitalization = iota
	TextCapitalizationCharacters
	TextCapitalizationWords
	TextCapitalizationSentences
)

// TextInputConfiguration configures a text input connection.
type TextInputConfiguration struct {
	KeyboardType      KeyboardType
	InputAction       TextInputAction
	Capitalization    TextCapitalization
	Autocorrect       bool
	EnableSuggestions bool
	Obscure           bool
	ActionLabel       string
}

// DefaultTextInputConfiguration returns a default text input configuration.
func DefaultTextInputConfiguration() TextInputConfiguration {
	return TextInputConfiguration{
		KeyboardType:      KeyboardTypeText,
		InputAction:       TextInputActionDone,
		Capitalization:    TextCapitalizationSentences,
		Autocorrect:       true,
		EnableSuggestions: true,
	}
}

// TextInputClient receives text input events from the platform.
type TextInputClient interface {
	// UpdateEditingValue is called when the editing value changes.
	UpdateEditingValue(value TextEditingValue)

	// PerformAction is called when the keyboard action button is pressed.
	PerformAction(action TextInputAction)

	// ConnectionClosed is called when the input connection is closed.
	ConnectionClosed()
}

// TextInputConnection manages the connection to the native IME.
type TextInputConnection struct {
	id      int64
	client  TextInputClient
	channel *MethodChannel
}

// ID returns the connection identifier.
func (c *TextInputConnection) ID() int64 {
	return c.id
}

var (
	textInputChannel     *MethodChannel
	textInputConnections = make(map[int64]*TextInputConnection)
	textInputMu          sync.Mutex
	nextConnectionID     int64
	activeConnectionID   int64
	focusedTarget        any // The render object that currently has focus
)

// UnfocusAll hides the keyboard and unfocuses all text input connections.
func UnfocusAll() {
	textInputMu.Lock()
	activeID := activeConnectionID
	activeConnectionID = 0
	focusedTarget = nil
	conn := textInputConnections[activeID]
	textInputMu.Unlock()

	if conn != nil {
		conn.Hide()
		// Notify the client so it can update its visual state
		if conn.client != nil {
			conn.client.ConnectionClosed()
		}
	}
}

// SetActiveConnection marks a connection as the currently active one.
func SetActiveConnection(id int64) {
	textInputMu.Lock()
	activeConnectionID = id
	textInputMu.Unlock()
}

// SetFocusedTarget sets the render object that currently has keyboard focus.
func SetFocusedTarget(target any) {
	textInputMu.Lock()
	focusedTarget = target
	textInputMu.Unlock()
}

// GetFocusedTarget returns the render object that currently has keyboard focus.
func GetFocusedTarget() any {
	textInputMu.Lock()
	defer textInputMu.Unlock()
	return focusedTarget
}

// HasFocus returns true if there is currently a focused text input.
func HasFocus() bool {
	textInputMu.Lock()
	defer textInputMu.Unlock()
	return activeConnectionID != 0
}

func initTextInput() {
	if textInputChannel != nil {
		return
	}

	textInputChannel = NewMethodChannel("drift/text_input")
	textInputChannel.SetHandler(handleTextInputMethod)

	// Also listen for events from native (text changes, actions, etc.)
	eventChannel := NewEventChannel("drift/text_input")
	eventChannel.Listen(EventHandler{
		OnEvent: func(data any) {
			handleTextInputEvent(data)
		},
	})
}

// handleTextInputEvent processes events from native text input.
func handleTextInputEvent(data any) {
	dataMap, ok := data.(map[string]any)
	if !ok {
		println("textinput: event data is not a map")
		return
	}

	method, _ := dataMap["method"].(string)
	if method == "" {
		println("textinput: event has no method")
		return
	}

	switch method {
	case "updateEditingState":
		handleUpdateEditingState(dataMap)
	case "performAction":
		handlePerformAction(dataMap)
	case "connectionClosed":
		handleConnectionClosed(dataMap)
	}
}

// NewTextInputConnection creates a new text input connection.
func NewTextInputConnection(client TextInputClient, config TextInputConfiguration) *TextInputConnection {
	initTextInput()

	textInputMu.Lock()
	nextConnectionID++
	id := nextConnectionID
	textInputMu.Unlock()

	conn := &TextInputConnection{
		id:      id,
		client:  client,
		channel: textInputChannel,
	}

	textInputMu.Lock()
	textInputConnections[id] = conn
	textInputMu.Unlock()

	// Notify native to create the input connection
	textInputChannel.Invoke("createConnection", map[string]any{
		"connectionId":      id,
		"keyboardType":      int(config.KeyboardType),
		"inputAction":       int(config.InputAction),
		"capitalization":    int(config.Capitalization),
		"autocorrect":       config.Autocorrect,
		"enableSuggestions": config.EnableSuggestions,
		"obscure":           config.Obscure,
		"actionLabel":       config.ActionLabel,
	})

	return conn
}

// Show shows the soft keyboard.
func (c *TextInputConnection) Show() {
	c.channel.Invoke("show", map[string]any{
		"connectionId": c.id,
	})
}

// Hide hides the soft keyboard.
func (c *TextInputConnection) Hide() {
	c.channel.Invoke("hide", map[string]any{
		"connectionId": c.id,
	})
}

// SetEditingState updates the native text state.
func (c *TextInputConnection) SetEditingState(value TextEditingValue) {
	c.channel.Invoke("setEditingState", map[string]any{
		"connectionId":    c.id,
		"text":            value.Text,
		"selectionBase":   value.Selection.BaseOffset,
		"selectionExtent": value.Selection.ExtentOffset,
		"composingStart":  value.ComposingRange.Start,
		"composingEnd":    value.ComposingRange.End,
	})
}

// Close closes this text input connection.
func (c *TextInputConnection) Close() {
	textInputMu.Lock()
	delete(textInputConnections, c.id)
	textInputMu.Unlock()

	c.channel.Invoke("closeConnection", map[string]any{
		"connectionId": c.id,
	})
}

// handleTextInputMethod handles incoming text input events from native.
func handleTextInputMethod(method string, args any) (any, error) {
	argsMap, ok := args.(map[string]any)
	if !ok {
		return nil, ErrInvalidArguments
	}

	switch method {
	case "updateEditingState":
		return handleUpdateEditingState(argsMap)
	case "performAction":
		return handlePerformAction(argsMap)
	case "connectionClosed":
		return handleConnectionClosed(argsMap)
	default:
		return nil, ErrMethodNotFound
	}
}

func handleUpdateEditingState(args map[string]any) (any, error) {
	connID, _ := toInt64(args["connectionId"])

	textInputMu.Lock()
	conn := textInputConnections[connID]
	textInputMu.Unlock()

	if conn == nil || conn.client == nil {
		return nil, nil
	}

	text, _ := args["text"].(string)
	selBase, _ := toInt(args["selectionBase"])
	selExtent, _ := toInt(args["selectionExtent"])
	compStart, _ := toInt(args["composingStart"])
	compEnd, _ := toInt(args["composingEnd"])

	value := TextEditingValue{
		Text: text,
		Selection: TextSelection{
			BaseOffset:   selBase,
			ExtentOffset: selExtent,
		},
		ComposingRange: TextRange{
			Start: compStart,
			End:   compEnd,
		},
	}

	conn.client.UpdateEditingValue(value)
	return nil, nil
}

func handlePerformAction(args map[string]any) (any, error) {
	connID, _ := toInt64(args["connectionId"])

	textInputMu.Lock()
	conn := textInputConnections[connID]
	textInputMu.Unlock()

	if conn == nil || conn.client == nil {
		return nil, nil
	}

	action, _ := toInt(args["action"])
	conn.client.PerformAction(TextInputAction(action))
	return nil, nil
}

func handleConnectionClosed(args map[string]any) (any, error) {
	connID, _ := toInt64(args["connectionId"])

	textInputMu.Lock()
	conn := textInputConnections[connID]
	delete(textInputConnections, connID)
	textInputMu.Unlock()

	if conn != nil && conn.client != nil {
		conn.client.ConnectionClosed()
	}
	return nil, nil
}

// toInt converts various numeric types to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// toInt64 converts various numeric types to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// TextEditingController manages text input state.
type TextEditingController struct {
	value          TextEditingValue
	listeners      map[int]func()
	nextListenerID int
	mu             sync.RWMutex
}

// NewTextEditingController creates a new text editing controller with the given initial text.
func NewTextEditingController(text string) *TextEditingController {
	return &TextEditingController{
		value: TextEditingValue{
			Text:           text,
			Selection:      TextSelectionCollapsed(len(text)),
			ComposingRange: TextRangeEmpty,
		},
		listeners: make(map[int]func()),
	}
}

// Text returns the current text content.
func (c *TextEditingController) Text() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value.Text
}

// SetText sets the text content.
func (c *TextEditingController) SetText(text string) {
	c.mu.Lock()
	c.value.Text = text
	// Move selection to end if it's beyond the text length
	if c.value.Selection.BaseOffset > len(text) {
		c.value.Selection.BaseOffset = len(text)
	}
	if c.value.Selection.ExtentOffset > len(text) {
		c.value.Selection.ExtentOffset = len(text)
	}
	c.mu.Unlock()
	c.notifyListeners()
}

// Selection returns the current selection.
func (c *TextEditingController) Selection() TextSelection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value.Selection
}

// SetSelection sets the selection.
func (c *TextEditingController) SetSelection(selection TextSelection) {
	c.mu.Lock()
	c.value.Selection = selection
	c.mu.Unlock()
	c.notifyListeners()
}

// Value returns the complete editing value.
func (c *TextEditingController) Value() TextEditingValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// SetValue sets the complete editing value.
func (c *TextEditingController) SetValue(value TextEditingValue) {
	c.mu.Lock()
	c.value = value
	c.mu.Unlock()
	c.notifyListeners()
}

// Clear clears the text.
func (c *TextEditingController) Clear() {
	c.SetText("")
}

// AddListener adds a callback that is called when the value changes.
// Returns an unsubscribe function.
func (c *TextEditingController) AddListener(fn func()) func() {
	c.mu.Lock()
	id := c.nextListenerID
	c.nextListenerID++
	c.listeners[id] = fn
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		delete(c.listeners, id)
		c.mu.Unlock()
	}
}

// notifyListeners calls all registered listeners.
func (c *TextEditingController) notifyListeners() {
	c.mu.RLock()
	listeners := make([]func(), 0, len(c.listeners))
	for _, fn := range c.listeners {
		listeners = append(listeners, fn)
	}
	c.mu.RUnlock()

	for _, fn := range listeners {
		fn()
	}
}
