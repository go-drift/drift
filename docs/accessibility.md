# Accessibility in Drift

This guide covers building accessible Drift applications that work with TalkBack (Android) and VoiceOver (iOS).

## Overview

Drift provides a comprehensive accessibility system built on three main components:

1. **Semantics Tree** - A parallel tree structure that describes UI elements for assistive technologies
2. **Platform Bridges** - Native integrations for Android and iOS accessibility APIs
3. **Developer APIs** - Widgets and helpers for adding accessibility to your app

## Quick Start

Most built-in widgets (Button, Checkbox, Switch, TextField, TabBar) automatically provide semantics. For custom interactive elements, use the semantic helper functions:

```go
import "github.com/go-drift/drift/pkg/widgets"

// Make a custom card tappable and accessible
card := widgets.Tappable("Open settings", openSettings,
    widgets.Container{
        ChildWidget: widgets.Text{Content: "Settings"},
    },
)

// Add a label to an image
logo := widgets.SemanticImage("Company logo", logoWidget)

// Mark decorative elements to be skipped by screen readers
divider := widgets.Decorative(dividerLine)
```

## Semantic Helpers

Drift provides helper functions for common accessibility patterns. These are the recommended way to add accessibility to custom widgets.

### Tappable

Creates an accessible tappable element (button-like behavior):

```go
// Basic tappable
widgets.Tappable("Submit form", submitForm, myButton)

// With custom hint
widgets.TappableWithHint(
    "Delete item",
    "Double tap to permanently delete",
    deleteItem,
    deleteIcon,
)
```

### SemanticLabel

Adds an accessibility label to any widget:

```go
// Label a custom widget
widgets.SemanticLabel("User avatar", avatarWidget)

// Label a complex widget composition
widgets.SemanticLabel("Shopping cart with 3 items", cartIcon)
```

### SemanticImage

Marks a widget as an image with a description:

```go
// Informative image
widgets.SemanticImage("Bar chart showing monthly sales", chartWidget)

// Product image
widgets.SemanticImage("Red leather handbag, front view", productImage)
```

### SemanticHeading

Marks text as a heading for navigation. Screen reader users can jump between headings:

```go
// Page title (level 1)
widgets.SemanticHeading(1, widgets.Text{Content: "Welcome"})

// Section heading (level 2)
widgets.SemanticHeading(2, widgets.Text{Content: "Recent Orders"})

// Subsection (level 3)
widgets.SemanticHeading(3, widgets.Text{Content: "Order Details"})
```

### SemanticLink

Creates an accessible link:

```go
widgets.SemanticLink("Read our privacy policy", openPrivacyPolicy,
    widgets.Text{Content: "Privacy Policy", Style: linkStyle},
)
```

### SemanticGroup

Groups related widgets into a single accessibility unit. The screen reader announces all content together:

```go
// Price with currency - announced as "Price: $99.99" instead of "Price:" then "$99.99"
widgets.SemanticGroup(
    widgets.Row{
        ChildrenWidgets: []core.Widget{
            widgets.Text{Content: "Price: "},
            widgets.Text{Content: "$99.99"},
        },
    },
)

// Card with multiple text elements
widgets.SemanticGroup(
    widgets.Column{
        ChildrenWidgets: []core.Widget{
            widgets.Text{Content: "John Smith"},
            widgets.Text{Content: "Software Engineer"},
            widgets.Text{Content: "San Francisco, CA"},
        },
    },
)
```

### SemanticLiveRegion

Marks content that updates dynamically. Changes are automatically announced:

```go
// Status message that announces when updated
widgets.SemanticLiveRegion(
    widgets.Text{Content: statusMessage},
)

// Timer display
widgets.SemanticLiveRegion(timerWidget)
```

### Decorative

Hides purely visual elements from screen readers:

```go
// Decorative divider
widgets.Decorative(
    widgets.Container{Height: 1, Color: colors.Divider},
)

// Background pattern
widgets.Decorative(backgroundImage)

// Decorative icon next to text (text already conveys meaning)
widgets.Row{
    ChildrenWidgets: []core.Widget{
        widgets.Decorative(checkmarkIcon),
        widgets.Text{Content: "Task completed"},
    },
}
```

## The Semantics Widget

For advanced cases where the helpers don't fit your needs, use the `Semantics` widget directly:

```go
widgets.Semantics{
    // Text content
    Label:   "Submit form",           // Primary accessibility label
    Value:   "3 items selected",      // Current value
    Hint:    "Double tap to submit",  // Usage hint
    Tooltip: "Send the form data",    // Tooltip text

    // Role and state
    Role:  semantics.SemanticsRoleButton,
    Flags: semantics.SemanticsIsEnabled | semantics.SemanticsIsButton,

    // Grouping
    Container:        true,  // Creates a semantic boundary
    MergeDescendants: true,  // Merge all descendant labels into this node

    // Actions
    OnTap:       func() { /* handle tap */ },
    OnLongPress: func() { /* handle long press */ },
    OnIncrease:  func() { /* increase value */ },
    OnDecrease:  func() { /* decrease value */ },

    ChildWidget: myWidget,
}
```

### Semantic Roles

Use roles to convey the type of element:

```go
semantics.SemanticsRoleNone       // Default, no specific role
semantics.SemanticsRoleButton     // Clickable button
semantics.SemanticsRoleCheckbox   // Checkable item
semantics.SemanticsRoleSwitch     // Toggle switch
semantics.SemanticsRoleTextField  // Text input
semantics.SemanticsRoleImage      // Image content
semantics.SemanticsRoleSlider     // Adjustable slider
semantics.SemanticsRoleTab        // Tab in a tab bar
semantics.SemanticsRoleTabList    // Tab bar container
semantics.SemanticsRoleLink       // Hyperlink
semantics.SemanticsRoleHeading    // Heading text
semantics.SemanticsRoleList       // List container
semantics.SemanticsRoleListItem   // Item in a list
semantics.SemanticsRoleScrollView // Scrollable region
```

### Semantic Flags

Flags communicate boolean states:

```go
// Capability flags
semantics.SemanticsHasCheckedState  // Can be checked
semantics.SemanticsHasEnabledState  // Can be enabled/disabled
semantics.SemanticsHasToggledState  // Can be toggled
semantics.SemanticsIsTextField      // Is a text field
semantics.SemanticsIsSlider         // Is a slider
semantics.SemanticsIsButton         // Is a button
semantics.SemanticsIsLink           // Is a link
semantics.SemanticsIsImage          // Is an image
semantics.SemanticsIsHeader         // Is a header/heading
semantics.SemanticsIsFocusable      // Can receive focus
semantics.SemanticsIsKeyboardKey    // Is a keyboard key

// State flags
semantics.SemanticsIsChecked        // Currently checked
semantics.SemanticsIsSelected       // Currently selected
semantics.SemanticsIsToggled        // Currently toggled on
semantics.SemanticsIsEnabled        // Currently enabled
semantics.SemanticsIsFocused        // Currently focused
semantics.SemanticsIsObscured       // Content is obscured (password)
semantics.SemanticsIsMultiline      // Multi-line text field
semantics.SemanticsIsReadOnly       // Read-only field
semantics.SemanticsIsHidden         // Hidden from accessibility
semantics.SemanticsIsLiveRegion     // Live region for announcements
semantics.SemanticsNamesRoute       // Names a navigation route
```

## Grouping Semantics

### MergeSemantics

Use `MergeSemantics` to combine multiple elements into one accessibility node:

```go
// Card with icon and label read as one unit
card := widgets.MergeSemantics{
    ChildWidget: widgets.Row{
        ChildrenWidgets: []core.Widget{
            widgets.Icon{Icon: icons.Star},
            widgets.Text{Content: "Favorites"},
        },
    },
}
// VoiceOver/TalkBack announces: "Favorites"
```

### ExcludeSemantics

Use `ExcludeSemantics` to hide decorative elements:

```go
// Decorative separator
separator := widgets.ExcludeSemantics{
    Excluding: true,
    ChildWidget: widgets.Container{
        Height: 1,
        Color:  colors.Divider,
    },
}
```

## Images

Images require explicit accessibility handling:

```go
// Informative image - provide description
profilePic := widgets.Image{
    Source:        avatarSource,
    SemanticLabel: "Profile photo of Jane Doe",
}

// Decorative image - exclude from accessibility
decorativePattern := widgets.Image{
    Source:               backgroundSource,
    ExcludeFromSemantics: true,
}
```

## Sliders and Adjustable Controls

For sliders, provide value information:

```go
volume := widgets.Semantics{
    Label:        "Volume",
    Value:        fmt.Sprintf("%d%%", currentVolume),
    Role:         semantics.SemanticsRoleSlider,
    CurrentValue: float64Ptr(float64(currentVolume)),
    MinValue:     float64Ptr(0),
    MaxValue:     float64Ptr(100),
    OnIncrease:   func() { setVolume(currentVolume + 10) },
    OnDecrease:   func() { setVolume(currentVolume - 10) },
    ChildWidget:  slider,
}

func float64Ptr(v float64) *float64 { return &v }
```

## Live Announcements

Use the accessibility package to announce dynamic changes:

```go
import "github.com/go-drift/drift/pkg/accessibility"

// Non-urgent announcement (polite)
accessibility.Announce("3 new messages", accessibility.PolitenessPolite)

// Urgent announcement (interrupts current speech)
accessibility.Announce("Error: Network disconnected", accessibility.PolitenessAssertive)
```

### Live Regions

For areas that update dynamically, mark them as live regions:

```go
// Status area that auto-announces changes
statusArea := widgets.Semantics{
    Label:       status,
    Flags:       semantics.SemanticsIsLiveRegion,
    ChildWidget: widgets.Text{Content: status},
}
```

## Focus Management

The accessibility focus is automatically synced with keyboard focus. For custom focus handling:

```go
import "github.com/go-drift/drift/pkg/accessibility"

// Move accessibility focus programmatically
manager := accessibility.GetAccessibilityFocusManager()
manager.MoveAccessibilityFocus(accessibility.TraversalDirectionNext)

// Check current focus
if focusedNode := manager.GetAccessibilityFocus(); focusedNode != nil {
    // Handle focused element
}
```

## Validation and Debugging

### Contrast Checking

Ensure text meets WCAG contrast requirements:

```go
import "github.com/go-drift/drift/pkg/validation"

// Check contrast ratio
ratio := validation.ContrastRatio(textColor, backgroundColor)

// Verify WCAG compliance
if validation.MeetsWCAGAA(ratio, false) { // false = normal text size
    // Meets AA standard (4.5:1 for normal text)
}

if validation.MeetsWCAGAAA(ratio, true) { // true = large text
    // Meets AAA standard (4.5:1 for large text)
}

// Get suggestions
result := validation.CheckContrast(textColor, bgColor, false)
fmt.Printf("Ratio: %.2f, AA: %v, AAA: %v\n", result.Ratio, result.MeetsAA, result.MeetsAAA)
```

### Accessibility Linting

Run accessibility checks on your semantics tree:

```go
import "github.com/go-drift/drift/pkg/validation"

// Lint the semantics tree
results := validation.LintSemanticsTree(semanticsRoot)

for _, result := range results {
    fmt.Printf("[%s] %s: %s\n", result.Severity, result.Rule, result.Message)
    fmt.Printf("  Suggestion: %s\n", result.Suggestion)
}

// Filter by severity
errors := validation.FilterBySeverity(results, validation.SeverityError)
if validation.HasErrors(results) {
    // Handle critical issues
}
```

Lint rules include:
- `missing-label` - Interactive element without accessibility label
- `empty-button` - Button without label
- `image-missing-alt` - Image without alt text (not marked decorative)
- `touch-target-size` - Touch target smaller than 48x48
- `missing-value` - Slider without value information
- `missing-hint` - Text field without hint

## Best Practices

### 1. Use Built-in Widgets When Possible

Built-in widgets like `Button`, `Checkbox`, `Switch`, and `TextField` have accessibility built-in:

```go
// Good - Button handles all accessibility automatically
widgets.Button{
    Label: "Submit",
    OnTap: submitForm,
}
```

### 2. Use Semantic Helpers for Custom Widgets

For custom interactive elements, use `Tappable` instead of raw `GestureDetector`:

```go
// Bad - GestureDetector has no accessibility
widgets.Tap(handleTap, myCustomCard)

// Good - Tappable includes accessibility
widgets.Tappable("Open item details", handleTap, myCustomCard)
```

### 3. Provide Meaningful Labels

```go
// Bad - relies on visual context
widgets.Button{Label: "X", OnTap: closeDialog}

// Good - use SemanticLabel to add context
widgets.SemanticLabel("Close dialog",
    widgets.Button{Label: "X", OnTap: closeDialog},
)
```

### 4. Group Related Content

```go
// Bad - each item announced separately
widgets.Row{
    ChildrenWidgets: []core.Widget{
        widgets.Text{Content: "Price:"},
        widgets.Text{Content: "$99.99"},
    },
}

// Good - grouped with SemanticGroup
widgets.SemanticGroup(
    widgets.Row{
        ChildrenWidgets: []core.Widget{
            widgets.Text{Content: "Price:"},
            widgets.Text{Content: "$99.99"},
        },
    },
)
// Announces: "Price: $99.99"
```

### 5. Mark Decorative Elements

```go
// Good - decorative elements hidden from screen readers
widgets.Decorative(backgroundPattern)

// Also good - Image widget has built-in option
widgets.Image{
    Source:               decorativeBackground,
    ExcludeFromSemantics: true,
}
```

### 6. Use Headings for Structure

```go
// Good - headings help screen reader users navigate
widgets.Column{
    ChildrenWidgets: []core.Widget{
        widgets.SemanticHeading(1, widgets.Text{Content: "Settings"}),
        widgets.SemanticHeading(2, widgets.Text{Content: "Account"}),
        // account settings...
        widgets.SemanticHeading(2, widgets.Text{Content: "Privacy"}),
        // privacy settings...
    },
}
```

### 7. Ensure Touch Target Size

Minimum touch target should be 48x48 dp:

```go
// Good - adequate touch target with accessibility
widgets.Tappable("Add item", handleTap,
    widgets.Container{
        Width:     48,
        Height:    48,
        Alignment: layout.AlignmentCenter,
        ChildWidget: widgets.Icon{Icon: icons.Add, Size: 24},
    },
)
```

### 8. Announce Dynamic Changes

```go
// When content updates dynamically
func updateItemCount(count int) {
    setCount(count)
    accessibility.Announce(
        fmt.Sprintf("%d items in cart", count),
        accessibility.PolitenessPolite,
    )
}

// Or use a live region for automatic announcements
widgets.SemanticLiveRegion(
    widgets.Text{Content: fmt.Sprintf("%d items", count)},
)
```

## Platform-Specific Notes

### Android (TalkBack)

- Roles map to `AccessibilityNodeInfo` class names
- Flags map to `isCheckable`, `isChecked`, etc.
- Custom actions appear in the local context menu

### iOS (VoiceOver)

- Roles map to `UIAccessibilityTraits`
- Hints are read after a brief pause
- Increment/decrement actions use swipe up/down gestures

## Testing

1. **Enable screen reader**
   - Android: Settings > Accessibility > TalkBack
   - iOS: Settings > Accessibility > VoiceOver

2. **Navigate your app**
   - Verify all interactive elements are reachable
   - Check that labels are descriptive
   - Confirm states are announced correctly

3. **Run automated checks**
   - Use `validation.LintSemanticsTree()` in tests
   - Check contrast ratios during UI tests

4. **Test common flows**
   - Navigation and back button
   - Form submission
   - Error states
   - Loading indicators

## Quick Reference

### Semantic Helpers

| Helper | Use Case |
|--------|----------|
| `Tappable(label, onTap, child)` | Custom tappable elements (cards, list items) |
| `TappableWithHint(label, hint, onTap, child)` | Tappable with custom hint text |
| `SemanticLabel(label, child)` | Add label to any widget |
| `SemanticImage(description, child)` | Images that convey information |
| `SemanticHeading(level, child)` | Section headings (levels 1-6) |
| `SemanticLink(label, onTap, child)` | Hyperlinks |
| `SemanticGroup(child)` | Merge multiple elements into one |
| `SemanticLiveRegion(child)` | Content that updates dynamically |
| `Decorative(child)` | Hide from screen readers |

### Built-in Accessible Widgets

These widgets have accessibility built-in - no additional work needed:

- `Button` - Announced as button with label
- `Checkbox` - Announced with checked/unchecked state
- `Switch` - Announced with on/off state
- `TextField` - Text input with hint support
- `TabBar` - Tab navigation with position
- `Image` - Supports `SemanticLabel` and `ExcludeFromSemantics`
- `ScrollView` - Scroll actions for screen readers

### Common Patterns

```go
// Accessible custom button/card
widgets.Tappable("View order details", viewOrder, orderCard)

// Image with description
widgets.SemanticImage("Product: Blue sneakers", productImage)

// Grouped price display
widgets.SemanticGroup(priceWithCurrency)

// Page structure with headings
widgets.SemanticHeading(1, pageTitle)
widgets.SemanticHeading(2, sectionTitle)

// Hide decorative elements
widgets.Decorative(dividerLine)
```
