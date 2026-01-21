// Package semantics provides accessibility semantics support for Drift.
package semantics

// SemanticsRole indicates the role of a semantics node for accessibility purposes.
type SemanticsRole int

const (
	// SemanticsRoleNone indicates no specific role.
	SemanticsRoleNone SemanticsRole = iota

	// SemanticsRoleButton indicates a clickable button.
	SemanticsRoleButton

	// SemanticsRoleCheckbox indicates a checkbox control.
	SemanticsRoleCheckbox

	// SemanticsRoleRadio indicates a radio button.
	SemanticsRoleRadio

	// SemanticsRoleSwitch indicates a toggle switch.
	SemanticsRoleSwitch

	// SemanticsRoleTextField indicates a text input field.
	SemanticsRoleTextField

	// SemanticsRoleLink indicates a hyperlink.
	SemanticsRoleLink

	// SemanticsRoleImage indicates an image.
	SemanticsRoleImage

	// SemanticsRoleSlider indicates a slider control.
	SemanticsRoleSlider

	// SemanticsRoleProgressIndicator indicates a progress indicator.
	SemanticsRoleProgressIndicator

	// SemanticsRoleTab indicates a tab control.
	SemanticsRoleTab

	// SemanticsRoleTabBar indicates a tab bar container.
	SemanticsRoleTabBar

	// SemanticsRoleList indicates a list container.
	SemanticsRoleList

	// SemanticsRoleListItem indicates a list item.
	SemanticsRoleListItem

	// SemanticsRoleScrollView indicates a scrollable container.
	SemanticsRoleScrollView

	// SemanticsRoleHeader indicates a header or title.
	SemanticsRoleHeader

	// SemanticsRoleAlert indicates an alert or dialog.
	SemanticsRoleAlert

	// SemanticsRoleMenu indicates a menu.
	SemanticsRoleMenu

	// SemanticsRoleMenuItem indicates a menu item.
	SemanticsRoleMenuItem

	// SemanticsRolePopup indicates a popup.
	SemanticsRolePopup
)

// String returns a human-readable name for the role.
func (r SemanticsRole) String() string {
	switch r {
	case SemanticsRoleNone:
		return "none"
	case SemanticsRoleButton:
		return "button"
	case SemanticsRoleCheckbox:
		return "checkbox"
	case SemanticsRoleRadio:
		return "radio"
	case SemanticsRoleSwitch:
		return "switch"
	case SemanticsRoleTextField:
		return "textField"
	case SemanticsRoleLink:
		return "link"
	case SemanticsRoleImage:
		return "image"
	case SemanticsRoleSlider:
		return "slider"
	case SemanticsRoleProgressIndicator:
		return "progressIndicator"
	case SemanticsRoleTab:
		return "tab"
	case SemanticsRoleTabBar:
		return "tabBar"
	case SemanticsRoleList:
		return "list"
	case SemanticsRoleListItem:
		return "listItem"
	case SemanticsRoleScrollView:
		return "scrollView"
	case SemanticsRoleHeader:
		return "header"
	case SemanticsRoleAlert:
		return "alert"
	case SemanticsRoleMenu:
		return "menu"
	case SemanticsRoleMenuItem:
		return "menuItem"
	case SemanticsRolePopup:
		return "popup"
	default:
		return "unknown"
	}
}

// SemanticsFlag represents boolean state flags for a semantics node.
type SemanticsFlag uint64

const (
	// SemanticsHasCheckedState indicates the node has a checked state.
	SemanticsHasCheckedState SemanticsFlag = 1 << iota

	// SemanticsIsChecked indicates the node is currently checked.
	SemanticsIsChecked

	// SemanticsHasSelectedState indicates the node has a selected state.
	SemanticsHasSelectedState

	// SemanticsIsSelected indicates the node is currently selected.
	SemanticsIsSelected

	// SemanticsHasEnabledState indicates the node has an enabled state.
	SemanticsHasEnabledState

	// SemanticsIsEnabled indicates the node is currently enabled.
	SemanticsIsEnabled

	// SemanticsIsFocusable indicates the node can receive focus.
	SemanticsIsFocusable

	// SemanticsIsFocused indicates the node currently has focus.
	SemanticsIsFocused

	// SemanticsIsButton indicates the node behaves as a button.
	SemanticsIsButton

	// SemanticsIsTextField indicates the node is a text field.
	SemanticsIsTextField

	// SemanticsIsReadOnly indicates the node is read-only.
	SemanticsIsReadOnly

	// SemanticsIsObscured indicates the node content is obscured (e.g., password).
	SemanticsIsObscured

	// SemanticsIsMultiline indicates the text field is multiline.
	SemanticsIsMultiline

	// SemanticsIsSlider indicates the node is a slider.
	SemanticsIsSlider

	// SemanticsIsLiveRegion indicates the node content updates should be announced.
	SemanticsIsLiveRegion

	// SemanticsHasToggledState indicates the node has a toggled state.
	SemanticsHasToggledState

	// SemanticsIsToggled indicates the node is currently toggled on.
	SemanticsIsToggled

	// SemanticsHasImplicitScrolling indicates the node has implicit scrolling.
	SemanticsHasImplicitScrolling

	// SemanticsIsHidden indicates the node is hidden from accessibility.
	SemanticsIsHidden

	// SemanticsIsHeader indicates the node is a header.
	SemanticsIsHeader

	// SemanticsIsImage indicates the node is an image.
	SemanticsIsImage

	// SemanticsNamesRoute indicates the node names a navigation route.
	SemanticsNamesRoute

	// SemanticsScopesRoute indicates the node defines a navigation route scope.
	SemanticsScopesRoute

	// SemanticsIsInMutuallyExclusiveGroup indicates exclusive selection (radio group).
	SemanticsIsInMutuallyExclusiveGroup

	// SemanticsHasExpandedState indicates the node has expanded/collapsed state.
	SemanticsHasExpandedState

	// SemanticsIsExpanded indicates the node is currently expanded.
	SemanticsIsExpanded
)

// Has checks if a specific flag is set.
func (f SemanticsFlag) Has(flag SemanticsFlag) bool {
	return f&flag != 0
}

// Set adds a flag.
func (f SemanticsFlag) Set(flag SemanticsFlag) SemanticsFlag {
	return f | flag
}

// Clear removes a flag.
func (f SemanticsFlag) Clear(flag SemanticsFlag) SemanticsFlag {
	return f &^ flag
}

// SemanticsProperties defines the semantic information for a node.
type SemanticsProperties struct {
	// Label is the primary text description of the node.
	Label string

	// Value is the current value (e.g., slider position, text content).
	Value string

	// Hint provides guidance on the action that will occur (e.g., "Double tap to activate").
	Hint string

	// Tooltip provides additional information shown on hover/long press.
	Tooltip string

	// Role defines the semantic role of the node.
	Role SemanticsRole

	// Flags contains boolean state flags.
	Flags SemanticsFlag

	// CurrentValue for slider-type controls.
	CurrentValue *float64

	// MinValue for slider-type controls.
	MinValue *float64

	// MaxValue for slider-type controls.
	MaxValue *float64

	// ScrollPosition for scrollable containers.
	ScrollPosition *float64

	// ScrollExtentMin is the minimum scroll extent.
	ScrollExtentMin *float64

	// ScrollExtentMax is the maximum scroll extent.
	ScrollExtentMax *float64

	// HeadingLevel indicates heading level (1-6, 0 for none).
	HeadingLevel int

	// TextSelection indicates selected text range (start, end indices).
	TextSelectionStart int
	TextSelectionEnd   int

	// SortKey is used for custom ordering in accessibility traversal.
	SortKey *float64

	// CustomActions is a list of custom accessibility actions.
	CustomActions []CustomSemanticsAction
}

// CustomSemanticsAction defines a custom action for accessibility.
type CustomSemanticsAction struct {
	// ID uniquely identifies this action.
	ID int64

	// Label is the human-readable description of the action.
	Label string
}

// IsEmpty reports whether the properties contain any meaningful semantic information.
func (p SemanticsProperties) IsEmpty() bool {
	return p.Label == "" &&
		p.Value == "" &&
		p.Hint == "" &&
		p.Tooltip == "" &&
		p.Role == SemanticsRoleNone &&
		p.Flags == 0 &&
		p.CurrentValue == nil &&
		p.MinValue == nil &&
		p.MaxValue == nil &&
		p.ScrollPosition == nil &&
		p.ScrollExtentMin == nil &&
		p.ScrollExtentMax == nil &&
		p.HeadingLevel == 0 &&
		len(p.CustomActions) == 0
}

// Merge combines another SemanticsProperties into this one.
// Non-empty values from other take precedence.
func (p SemanticsProperties) Merge(other SemanticsProperties) SemanticsProperties {
	result := p

	if other.Label != "" {
		result.Label = other.Label
	}
	if other.Value != "" {
		result.Value = other.Value
	}
	if other.Hint != "" {
		result.Hint = other.Hint
	}
	if other.Tooltip != "" {
		result.Tooltip = other.Tooltip
	}
	if other.Role != SemanticsRoleNone {
		result.Role = other.Role
	}

	// Flags are combined (OR'd)
	result.Flags |= other.Flags

	if other.CurrentValue != nil {
		result.CurrentValue = other.CurrentValue
	}
	if other.MinValue != nil {
		result.MinValue = other.MinValue
	}
	if other.MaxValue != nil {
		result.MaxValue = other.MaxValue
	}
	if other.ScrollPosition != nil {
		result.ScrollPosition = other.ScrollPosition
	}
	if other.ScrollExtentMin != nil {
		result.ScrollExtentMin = other.ScrollExtentMin
	}
	if other.ScrollExtentMax != nil {
		result.ScrollExtentMax = other.ScrollExtentMax
	}
	if other.HeadingLevel != 0 {
		result.HeadingLevel = other.HeadingLevel
	}
	if other.SortKey != nil {
		result.SortKey = other.SortKey
	}
	if len(other.CustomActions) > 0 {
		result.CustomActions = append(result.CustomActions, other.CustomActions...)
	}

	return result
}
