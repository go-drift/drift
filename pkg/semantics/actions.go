package semantics

import "maps"

// SemanticsAction represents an accessibility action that can be performed on a node.
type SemanticsAction uint64

const (
	// SemanticsActionTap performs the primary action (tap/click).
	SemanticsActionTap SemanticsAction = 1 << iota

	// SemanticsActionLongPress performs a long press action.
	SemanticsActionLongPress

	// SemanticsActionScrollLeft scrolls the content left.
	SemanticsActionScrollLeft

	// SemanticsActionScrollRight scrolls the content right.
	SemanticsActionScrollRight

	// SemanticsActionScrollUp scrolls the content up.
	SemanticsActionScrollUp

	// SemanticsActionScrollDown scrolls the content down.
	SemanticsActionScrollDown

	// SemanticsActionIncrease increases the value (e.g., slider).
	SemanticsActionIncrease

	// SemanticsActionDecrease decreases the value (e.g., slider).
	SemanticsActionDecrease

	// SemanticsActionShowOnScreen scrolls this node into view.
	SemanticsActionShowOnScreen

	// SemanticsActionMoveCursorForwardByCharacter moves text cursor forward.
	SemanticsActionMoveCursorForwardByCharacter

	// SemanticsActionMoveCursorBackwardByCharacter moves text cursor backward.
	SemanticsActionMoveCursorBackwardByCharacter

	// SemanticsActionMoveCursorForwardByWord moves text cursor forward by word.
	SemanticsActionMoveCursorForwardByWord

	// SemanticsActionMoveCursorBackwardByWord moves text cursor backward by word.
	SemanticsActionMoveCursorBackwardByWord

	// SemanticsActionSetSelection sets text selection range.
	SemanticsActionSetSelection

	// SemanticsActionSetText sets the text content.
	SemanticsActionSetText

	// SemanticsActionCopy copies selected content.
	SemanticsActionCopy

	// SemanticsActionCut cuts selected content.
	SemanticsActionCut

	// SemanticsActionPaste pastes clipboard content.
	SemanticsActionPaste

	// SemanticsActionFocus requests accessibility focus.
	SemanticsActionFocus

	// SemanticsActionUnfocus removes accessibility focus.
	SemanticsActionUnfocus

	// SemanticsActionDismiss dismisses a dismissible element (e.g., dialog).
	SemanticsActionDismiss

	// SemanticsActionCustomAction performs a custom action by ID.
	SemanticsActionCustomAction
)

// String returns a human-readable name for the action.
func (a SemanticsAction) String() string {
	switch a {
	case SemanticsActionTap:
		return "tap"
	case SemanticsActionLongPress:
		return "longPress"
	case SemanticsActionScrollLeft:
		return "scrollLeft"
	case SemanticsActionScrollRight:
		return "scrollRight"
	case SemanticsActionScrollUp:
		return "scrollUp"
	case SemanticsActionScrollDown:
		return "scrollDown"
	case SemanticsActionIncrease:
		return "increase"
	case SemanticsActionDecrease:
		return "decrease"
	case SemanticsActionShowOnScreen:
		return "showOnScreen"
	case SemanticsActionMoveCursorForwardByCharacter:
		return "moveCursorForwardByCharacter"
	case SemanticsActionMoveCursorBackwardByCharacter:
		return "moveCursorBackwardByCharacter"
	case SemanticsActionMoveCursorForwardByWord:
		return "moveCursorForwardByWord"
	case SemanticsActionMoveCursorBackwardByWord:
		return "moveCursorBackwardByWord"
	case SemanticsActionSetSelection:
		return "setSelection"
	case SemanticsActionSetText:
		return "setText"
	case SemanticsActionCopy:
		return "copy"
	case SemanticsActionCut:
		return "cut"
	case SemanticsActionPaste:
		return "paste"
	case SemanticsActionFocus:
		return "focus"
	case SemanticsActionUnfocus:
		return "unfocus"
	case SemanticsActionDismiss:
		return "dismiss"
	case SemanticsActionCustomAction:
		return "customAction"
	default:
		return "unknown"
	}
}

// ActionHandler is a function that handles a semantics action.
// The args parameter contains action-specific arguments (e.g., selection range for SetSelection).
type ActionHandler func(args any)

// SemanticsActions holds action handlers for a semantics node.
type SemanticsActions struct {
	handlers map[SemanticsAction]ActionHandler
}

// NewSemanticsActions creates a new SemanticsActions instance.
func NewSemanticsActions() *SemanticsActions {
	return &SemanticsActions{
		handlers: make(map[SemanticsAction]ActionHandler),
	}
}

// SetHandler registers a handler for the given action.
func (a *SemanticsActions) SetHandler(action SemanticsAction, handler ActionHandler) {
	if a.handlers == nil {
		a.handlers = make(map[SemanticsAction]ActionHandler)
	}
	a.handlers[action] = handler
}

// GetHandler returns the handler for the given action, or nil if none.
func (a *SemanticsActions) GetHandler(action SemanticsAction) ActionHandler {
	if a == nil || a.handlers == nil {
		return nil
	}
	return a.handlers[action]
}

// HasAction reports whether the node has a handler for the given action.
func (a *SemanticsActions) HasAction(action SemanticsAction) bool {
	return a.GetHandler(action) != nil
}

// PerformAction executes the handler for the given action if it exists.
// Returns true if the action was handled.
func (a *SemanticsActions) PerformAction(action SemanticsAction, args any) bool {
	handler := a.GetHandler(action)
	if handler == nil {
		return false
	}
	handler(args)
	return true
}

// SupportedActions returns a bitmask of all supported actions.
func (a *SemanticsActions) SupportedActions() SemanticsAction {
	if a == nil || a.handlers == nil {
		return 0
	}
	var result SemanticsAction
	for action := range a.handlers {
		result |= action
	}
	return result
}

// Merge combines another SemanticsActions into this one.
// Handlers from other override existing handlers for the same action.
func (a *SemanticsActions) Merge(other *SemanticsActions) {
	if other == nil || other.handlers == nil {
		return
	}
	if a.handlers == nil {
		a.handlers = make(map[SemanticsAction]ActionHandler)
	}
	maps.Copy(a.handlers, other.handlers)
}

// Clear removes all handlers.
func (a *SemanticsActions) Clear() {
	a.handlers = nil
}

// IsEmpty reports whether there are no action handlers.
func (a *SemanticsActions) IsEmpty() bool {
	return a == nil || len(a.handlers) == 0
}

// SetSelectionArgs contains arguments for the SetSelection action.
type SetSelectionArgs struct {
	Base   int
	Extent int
}

// SetTextArgs contains arguments for the SetText action.
type SetTextArgs struct {
	Text string
}

// CustomActionArgs contains arguments for custom actions.
type CustomActionArgs struct {
	ActionID int64
}

// MoveCursorArgs contains arguments for cursor movement actions.
type MoveCursorArgs struct {
	ExtendSelection bool
}
