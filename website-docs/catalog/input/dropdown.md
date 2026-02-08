---
id: dropdown
title: Dropdown
---

# Dropdown

A selection menu that displays a list of items.

## Basic Usage

```go
// Themed (recommended)
theme.DropdownOf(ctx, selectedPlan, []widgets.DropdownItem[string]{
    {Value: "starter", Label: "Starter"},
    {Value: "pro", Label: "Pro"},
    {Value: "enterprise", Label: "Enterprise"},
}, func(value string) {
    s.SetState(func() {
        selectedPlan = value
    })
}).WithBorderRadius(8)

// Explicit
widgets.Dropdown[string]{
    Value: selectedPlan,
    Hint:  "Select a plan",
    Items: []widgets.DropdownItem[string]{
        {Value: "starter", Label: "Starter"},
        {Value: "pro", Label: "Pro"},
        {Value: "enterprise", Label: "Enterprise"},
    },
    OnChanged: func(value string) {
        s.SetState(func() {
            selectedPlan = value
        })
    },
    BackgroundColor:   colors.Surface,
    BorderColor:       colors.Outline,
    BorderRadius:      8,
    SelectedItemColor: colors.SurfaceVariant,
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `T` | Currently selected value |
| `Hint` | `string` | Placeholder text when no value is selected |
| `Items` | `[]DropdownItem[T]` | Available options |
| `OnChanged` | `func(T)` | Called when selection changes |
| `BackgroundColor` | `color.Color` | Background color |
| `BorderColor` | `color.Color` | Border color |
| `BorderRadius` | `float64` | Corner radius |
| `TextStyle` | `graphics.TextStyle` | Text styling |
| `SelectedItemColor` | `color.Color` | Highlight color for the selected row |

## Explicit Styling Requirements

Explicit dropdowns require `BackgroundColor`, `BorderColor`, `TextStyle.Color`, and `SelectedItemColor` if you want a visible selected-row highlight. If you want defaults from the theme, prefer `theme.DropdownOf(ctx, ...)`.

## Related

- [Checkbox & Radio](/docs/catalog/input/checkbox-radio) for single-selection in a visible group
- [DatePicker & TimePicker](/docs/catalog/input/datepicker-timepicker) for date/time selection
