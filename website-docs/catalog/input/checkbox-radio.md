---
id: checkbox-radio
title: Checkbox & Radio
---

# Checkbox & Radio

`Checkbox` is a boolean toggle. `Radio` selects a single value from a group.

## Checkbox

```go
// Themed (recommended)
theme.CheckboxOf(ctx, isChecked, func(value bool) {
    s.SetState(func() {
        isChecked = value
    })
})

// Explicit
widgets.Checkbox{
    Value:       isChecked,
    ActiveColor: colors.Primary,
    CheckColor:  colors.OnPrimary,
    OnChanged: func(value bool) {
        s.SetState(func() {
            isChecked = value
        })
    },
}
```

### Checkbox Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `bool` | Current checked state |
| `ActiveColor` | `color.Color` | Fill color when checked |
| `CheckColor` | `color.Color` | Checkmark color |
| `OnChanged` | `func(bool)` | Called when toggled |

## Radio

```go
// Themed (recommended)
theme.RadioOf(ctx, "email", selectedOption, func(value string) {
    s.SetState(func() {
        selectedOption = value
    })
})

// Explicit
widgets.Radio[string]{
    Value:       "email",
    GroupValue:  selectedOption,
    ActiveColor: colors.Primary,
    OnChanged: func(value string) {
        s.SetState(func() {
            selectedOption = value
        })
    },
}
```

### Radio Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `T` | The value this radio button represents |
| `GroupValue` | `T` | Currently selected value in the group |
| `ActiveColor` | `color.Color` | Fill color when selected |
| `OnChanged` | `func(T)` | Called when selected |

## Common Patterns

### Radio Group

```go
widgets.ColumnOf(
    widgets.MainAxisAlignmentStart,
    widgets.CrossAxisAlignmentStart,
    widgets.MainAxisSizeMin,
    theme.RadioOf(ctx, "email", contactMethod, setContactMethod),
    theme.RadioOf(ctx, "phone", contactMethod, setContactMethod),
    theme.RadioOf(ctx, "mail", contactMethod, setContactMethod),
)
```

## Related

- [Switch & Toggle](/docs/catalog/input/switch-toggle) for on/off controls
- [Dropdown](/docs/catalog/input/dropdown) for selection from a list
