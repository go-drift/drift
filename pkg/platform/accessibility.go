//go:build android || darwin || ios

package platform

import (
	"context"

	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/semantics"
)

const (
	accessibilityMethodChannel  = "drift/accessibility"
	accessibilityStateChannel   = "drift/accessibility/state"
	accessibilityActionsChannel = "drift/accessibility/actions"
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
	method:       NewMethodChannel(accessibilityMethodChannel),
	stateEvents:  NewEventChannel(accessibilityStateChannel),
	actionEvents: NewEventChannel(accessibilityActionsChannel),
}

// reportArg routes a parse error through errors.Report tagged with the given
// accessibility channel, then returns the error so callers can `return nil,
// c.reportArg(...)` in one line.
func (c *accessibilityChannel) reportArg(channel, op string, err error) error {
	drifterrors.Report(&drifterrors.DriftError{
		Op:      "accessibility." + op,
		Kind:    drifterrors.KindParsing,
		Channel: channel,
		Err:     err,
	})
	return err
}

func init() {
	initAccessibilityListeners()
	registerBuiltinInit(initAccessibilityListeners)
}

func initAccessibilityListeners() {
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
			const op = "stateEvent"
			args, err := requireMap(op, data)
			if err != nil {
				Accessibility.reportArg(accessibilityStateChannel, op, err)
				return
			}
			enabled, err := requireBool(op, args, "enabled")
			if err != nil {
				Accessibility.reportArg(accessibilityStateChannel, op, err)
				return
			}
			semantics.GetSemanticsBinding().SetEnabled(enabled)
		},
	})

	// Listen for accessibility action events from platform
	Accessibility.actionEvents.Listen(EventHandler{
		OnEvent: func(data any) {
			const op = "actionEvent"
			args, err := requireMap(op, data)
			if err != nil {
				Accessibility.reportArg(accessibilityActionsChannel, op, err)
				return
			}
			nodeID, err := requireInt64(op, args, "nodeId")
			if err != nil {
				Accessibility.reportArg(accessibilityActionsChannel, op, err)
				return
			}
			actionValue, err := requireInt64(op, args, "action")
			if err != nil {
				Accessibility.reportArg(accessibilityActionsChannel, op, err)
				return
			}
			actionArgs := args["args"] // optional, no require

			action := semantics.SemanticsAction(uint64(actionValue))
			semantics.GetSemanticsBinding().HandleAction(nodeID, action, actionArgs)
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

	_, err := c.method.Invoke(context.Background(), "updateSemantics", args)
	return err
}

// Announce sends an accessibility announcement to be spoken.
func (c *accessibilityChannel) Announce(message string, politeness AnnouncePoliteness) error {
	args := map[string]any{
		"message":    message,
		"politeness": politenessToString(politeness),
	}
	_, err := c.method.Invoke(context.Background(), "announce", args)
	return err
}

// SetAccessibilityFocus sets accessibility focus to the node with the given ID.
func (c *accessibilityChannel) SetAccessibilityFocus(nodeID int64) error {
	args := map[string]any{
		"nodeId": nodeID,
	}
	_, err := c.method.Invoke(context.Background(), "setAccessibilityFocus", args)
	return err
}

// ClearAccessibilityFocus clears the current accessibility focus.
func (c *accessibilityChannel) ClearAccessibilityFocus() error {
	_, err := c.method.Invoke(context.Background(), "clearAccessibilityFocus", nil)
	return err
}

// IsAccessibilityEnabled queries whether accessibility services are enabled.
func (c *accessibilityChannel) IsAccessibilityEnabled() (bool, error) {
	result, err := c.method.Invoke(context.Background(), "isAccessibilityEnabled", nil)
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
func (c *accessibilityChannel) handleSetEnabled(raw any) (any, error) {
	const op = "handleSetEnabled"
	args, err := requireMap(op, raw)
	if err != nil {
		return nil, c.reportArg(accessibilityMethodChannel, op, err)
	}
	enabled, err := requireBool(op, args, "enabled")
	if err != nil {
		return nil, c.reportArg(accessibilityMethodChannel, op, err)
	}

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
