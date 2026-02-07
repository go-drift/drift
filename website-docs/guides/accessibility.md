---
id: accessibility
title: Accessibility
sidebar_position: 13
---

# Accessibility

Drift provides comprehensive accessibility support for TalkBack (Android) and VoiceOver (iOS).

## Overview

Most built-in widgets (Button, Checkbox, Switch, TextField, TabBar) automatically provide semantics. For custom interactive elements, use semantic helper functions.

## Semantic Helpers

### Tappable

Creates an accessible tappable element:

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
widgets.SemanticLabel("User avatar", avatarWidget)
```

### SemanticImage

Marks a widget as an image with a description:

```go
widgets.SemanticImage("Bar chart showing monthly sales", chartWidget)
```

### SemanticHeading

Marks text as a heading for navigation:

```go
// Page title (level 1)
widgets.SemanticHeading(1, widgets.Text{Content: "Welcome"})

// Section heading (level 2)
widgets.SemanticHeading(2, widgets.Text{Content: "Recent Orders"})
```

### SemanticGroup

Groups related widgets into a single accessibility unit:

```go
// Price with currency - announced as "Price: $99.99"
widgets.SemanticGroup(
    widgets.Row{
        Children: []core.Widget{
            widgets.Text{Content: "Price: "},
            widgets.Text{Content: "$99.99"},
        },
    },
)
```

### SemanticLiveRegion

Marks content that updates dynamically:

```go
widgets.SemanticLiveRegion(
    widgets.Text{Content: statusMessage},
)
```

### Decorative

Hides purely visual elements from screen readers:

```go
widgets.Decorative(
    widgets.Container{Height: 1, Color: colors.Divider},
)
```

## The Semantics Widget

For advanced cases, use the `Semantics` widget directly:

```go
widgets.Semantics{
    Label:   "Submit form",
    Value:   "3 items selected",
    Hint:    "Double tap to submit",
    Role:    semantics.SemanticsRoleButton,
    Flags:   semantics.SemanticsIsEnabled | semantics.SemanticsIsButton,
    OnTap:   func() { /* handle tap */ },
    Child: myWidget,
}
```

### Semantic Roles

| Role | Use |
|------|-----|
| `SemanticsRoleButton` | Clickable button |
| `SemanticsRoleCheckbox` | Checkable item |
| `SemanticsRoleSwitch` | Toggle switch |
| `SemanticsRoleTextField` | Text input |
| `SemanticsRoleImage` | Image content |
| `SemanticsRoleSlider` | Adjustable slider |
| `SemanticsRoleLink` | Hyperlink |
| `SemanticsRoleHeader` | Heading text |

## Sliders and Adjustable Controls

```go
// Helper to get pointer to float64
func ptr(v float64) *float64 { return &v }

volume := widgets.Semantics{
    Label:        "Volume",
    Value:        fmt.Sprintf("%d%%", currentVolume),
    Role:         semantics.SemanticsRoleSlider,
    CurrentValue: ptr(float64(currentVolume)),
    MinValue:     ptr(0),
    MaxValue:     ptr(100),
    OnIncrease:   func() { setVolume(currentVolume + 10) },
    OnDecrease:   func() { setVolume(currentVolume - 10) },
    Child:  slider,
}
```

## Contrast Validation

```go
import "github.com/go-drift/drift/pkg/validation"

// Check contrast ratio
ratio := validation.ContrastRatio(textColor, backgroundColor)

// Verify WCAG compliance
if validation.MeetsWCAGAA(ratio, false) { // false = normal text
    // Meets AA standard (4.5:1)
}
```

## Best Practices

1. **Use built-in widgets** - Button, Checkbox, Switch have accessibility built-in

2. **Use semantic helpers** - For custom elements, use `Tappable` instead of raw `GestureDetector`

3. **Provide meaningful labels**:
   ```go
   // Bad
   widgets.Button{Label: "X", OnTap: closeDialog}

   // Good
   widgets.SemanticLabel("Close dialog",
       widgets.Button{Label: "X", OnTap: closeDialog},
   )
   ```

4. **Group related content** - Use `SemanticGroup` for related elements

5. **Mark decorative elements** - Use `Decorative` for visual-only content

6. **Use headings** - Help screen reader users navigate with `SemanticHeading`

7. **Ensure touch target size** - Minimum 48x48 dp for interactive elements

## Testing

1. Enable screen reader:
   - Android: Settings > Accessibility > TalkBack
   - iOS: Settings > Accessibility > VoiceOver

2. Verify all interactive elements are reachable

3. Check that labels are descriptive

4. Run `validation.LintSemanticsTree(root)` in tests to check for issues

## Next Steps

- [API Reference](/docs/api/accessibility) - Accessibility API documentation
- [API Reference](/docs/api/validation) - Validation API documentation
