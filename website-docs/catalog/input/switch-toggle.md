---
id: switch-toggle
title: Switch & Toggle
---

# Switch & Toggle

`Switch` uses native platform controls (UISwitch on iOS, SwitchCompat on Android). `Toggle` is a Drift-rendered on/off control.

## Switch

```go
// Native switch (use struct literal, no themed constructor)
widgets.Switch{
    Value:       isEnabled,
    OnTintColor: colors.Primary,
    OnChanged: func(value bool) {
        s.SetState(func() {
            isEnabled = value
        })
    },
}
```

### Switch Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `bool` | Current on/off state |
| `OnTintColor` | `color.Color` | Background color when on |
| `OnChanged` | `func(bool)` | Called when toggled |

## Toggle

```go
// Themed (recommended)
theme.ToggleOf(ctx, isEnabled, func(value bool) {
    s.SetState(func() {
        isEnabled = value
    })
})

// Explicit
widgets.Toggle{
    Value:         isEnabled,
    ActiveColor:   colors.Primary,
    InactiveColor: colors.SurfaceVariant,
    OnChanged: func(value bool) {
        s.SetState(func() {
            isEnabled = value
        })
    },
}
```

### Toggle Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `bool` | Current on/off state |
| `ActiveColor` | `color.Color` | Track color when on |
| `InactiveColor` | `color.Color` | Track color when off |
| `OnChanged` | `func(bool)` | Called when toggled |

## Switch vs Toggle

| | Switch | Toggle |
|---|---|---|
| Rendering | Native platform control | Drift-rendered |
| Themed constructor | No | `theme.ToggleOf` |
| Look and feel | Matches OS style | Consistent across platforms |

## Related

- [Checkbox & Radio](/docs/catalog/input/checkbox-radio) for other selection controls
- [Forms & Validation](/docs/guides/forms) for form-based input
