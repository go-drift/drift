//go:build android || darwin || ios
// +build android darwin ios

package platform

import (
	"github.com/go-drift/drift/pkg/semantics"
)

// AnnouncePoliteness indicates the urgency of an accessibility announcement.
type AnnouncePoliteness int

const (
	// AnnouncePolitenessPolite is for non-urgent announcements that don't interrupt.
	AnnouncePolitenessPolite AnnouncePoliteness = iota

	// AnnouncePolitenessAssertive is for important announcements that should interrupt.
	AnnouncePolitenessAssertive
)

// accessibilityChannel provides accessibility platform communication.
type accessibilityChannel struct {
	method       *MethodChannel
	stateEvents  *EventChannel
	actionEvents *EventChannel
}

// Accessibility is the global accessibility channel instance.
var Accessibility = &accessibilityChannel{
	method:       NewMethodChannel("drift/accessibility"),
	stateEvents:  NewEventChannel("drift/accessibility/state"),
	actionEvents: NewEventChannel("drift/accessibility/actions"),
}

func init() {
	// Set up handler for incoming method calls from the platform
	// Note: performAction comes via event channel, not method channel
	Accessibility.method.SetHandler(func(method string, args any) (any, error) {
		switch method {
		case "setAccessibilityEnabled":
			return Accessibility.handleSetEnabled(args)
		default:
			return nil, ErrMethodNotFound
		}
	})

	// Listen for accessibility state changes from platform
	Accessibility.stateEvents.Listen(EventHandler{
		OnEvent: func(data any) {
			if m, ok := data.(map[string]any); ok {
				if enabled, ok := m["enabled"].(bool); ok {
					binding := semantics.GetSemanticsBinding()
					binding.SetEnabled(enabled)
				}
			}
		},
	})

	// Listen for accessibility action events from platform
	Accessibility.actionEvents.Listen(EventHandler{
		OnEvent: func(data any) {
			if m, ok := data.(map[string]any); ok {
				nodeID, _ := toInt64(m["nodeId"])
				actionValue, _ := toInt64(m["action"])
				actionArgs := m["args"]

				action := semantics.SemanticsAction(uint64(actionValue))
				binding := semantics.GetSemanticsBinding()
				binding.HandleAction(nodeID, action, actionArgs)
			}
		},
	})
}

// SendSemanticsUpdate sends a semantics tree update to the platform.
func (c *accessibilityChannel) SendSemanticsUpdate(update semantics.SemanticsUpdate) error {
	if update.IsEmpty() {
		return nil
	}

	// Convert updates to maps for serialization
	updates := make([]map[string]any, len(update.Updates))
	for i, u := range update.Updates {
		updates[i] = u.ToMap()
	}

	args := map[string]any{
		"updates":  updates,
		"removals": update.Removals,
	}

	_, err := c.method.Invoke("updateSemantics", args)
	return err
}

// Announce sends an accessibility announcement to be spoken.
func (c *accessibilityChannel) Announce(message string, politeness AnnouncePoliteness) error {
	args := map[string]any{
		"message":    message,
		"politeness": politenessToString(politeness),
	}
	_, err := c.method.Invoke("announce", args)
	return err
}

// SetAccessibilityFocus sets accessibility focus to the node with the given ID.
func (c *accessibilityChannel) SetAccessibilityFocus(nodeID int64) error {
	args := map[string]any{
		"nodeId": nodeID,
	}
	_, err := c.method.Invoke("setAccessibilityFocus", args)
	return err
}

// ClearAccessibilityFocus clears the current accessibility focus.
func (c *accessibilityChannel) ClearAccessibilityFocus() error {
	_, err := c.method.Invoke("clearAccessibilityFocus", nil)
	return err
}

// IsAccessibilityEnabled queries whether accessibility services are enabled.
func (c *accessibilityChannel) IsAccessibilityEnabled() (bool, error) {
	result, err := c.method.Invoke("isAccessibilityEnabled", nil)
	if err != nil {
		return false, err
	}
	if m, ok := result.(map[string]any); ok {
		if enabled, ok := m["enabled"].(bool); ok {
			return enabled, nil
		}
	}
	return false, nil
}

// handleSetEnabled processes an accessibility enabled state change from the platform.
func (c *accessibilityChannel) handleSetEnabled(args any) (any, error) {
	m, ok := args.(map[string]any)
	if !ok {
		return nil, ErrInvalidArguments
	}

	enabled, _ := m["enabled"].(bool)

	// Update the semantics binding
	binding := semantics.GetSemanticsBinding()
	binding.SetEnabled(enabled)

	// If enabling, request a full update
	if enabled {
		binding.RequestFullUpdate()
	}

	return nil, nil
}

// politenessToString converts politeness to a string for serialization.
func politenessToString(p AnnouncePoliteness) string {
	switch p {
	case AnnouncePolitenessAssertive:
		return "assertive"
	default:
		return "polite"
	}
}

// InitializeAccessibility sets up the accessibility system with the semantics binding.
func InitializeAccessibility() {
	binding := semantics.GetSemanticsBinding()

	// Set the send function to use the platform channel
	binding.SetSendFunction(func(update semantics.SemanticsUpdate) error {
		return Accessibility.SendSemanticsUpdate(update)
	})

	// Set the action callback (optional, for custom handling)
	binding.SetActionCallback(nil) // Use default owner-based handling
}

