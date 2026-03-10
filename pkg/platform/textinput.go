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

var (
	focusedTarget   any   // The render object that currently has focus
	focusedViewID   int64 // The view ID of the currently focused text input
	hasFocusedInput bool  // Whether there's an active focused text input
	focusMu         sync.Mutex
)

// SetFocusedTarget sets the render object that currently has keyboard focus.
func SetFocusedTarget(target any) {
	focusMu.Lock()
	focusedTarget = target
	focusMu.Unlock()
}

// GetFocusedTarget returns the render object that currently has keyboard focus.
func GetFocusedTarget() any {
	focusMu.Lock()
	defer focusMu.Unlock()
	return focusedTarget
}

// SetFocusedInput marks a text input view as focused.
func SetFocusedInput(viewID int64, focused bool) {
	focusMu.Lock()
	if focused {
		focusedViewID = viewID
		hasFocusedInput = true
	} else if focusedViewID == viewID {
		focusedViewID = 0
		hasFocusedInput = false
	}
	focusMu.Unlock()
}

// HasFocus returns true if there is currently a focused text input.
func HasFocus() bool {
	focusMu.Lock()
	defer focusMu.Unlock()
	return hasFocusedInput
}

// UnfocusAll dismisses the keyboard and clears focus for all text inputs.
func UnfocusAll() {
	focusMu.Lock()
	target := focusedTarget
	viewID := focusedViewID
	focusedTarget = nil
	focusedViewID = 0
	hasFocusedInput = false
	focusMu.Unlock()

	// Blur the currently focused text input view
	if viewID != 0 {
		registry := GetPlatformViewRegistry()
		if view := registry.GetView(viewID); view != nil {
			if textInput, ok := view.(*TextInputView); ok {
				textInput.Blur()
			}
		}
	}

	// Clear any other references
	_ = target
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
