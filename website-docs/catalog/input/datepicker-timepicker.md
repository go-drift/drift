---
id: datepicker-timepicker
title: DatePicker & TimePicker
---

# DatePicker & TimePicker

Native modal pickers for date and time selection.

## DatePicker

```go
// Themed (recommended)
theme.DatePickerOf(ctx, selectedDate, func(date time.Time) {
    s.SetState(func() { selectedDate = &date })
})

// Explicit with full styling (no theme defaults)
widgets.DatePicker{
    Value: selectedDate, // *time.Time, nil shows placeholder
    OnChanged: func(date time.Time) {
        s.SetState(func() { selectedDate = &date })
    },
    Placeholder: "Select date",
    TextStyle:   graphics.TextStyle{FontSize: 16, Color: colors.OnSurface},
    Decoration: &widgets.InputDecoration{
        LabelText:       "Birth Date",
        BorderRadius:    8,
        BorderColor:     colors.Outline,
        BackgroundColor: colors.Surface,
        HintStyle:       graphics.TextStyle{FontSize: 16, Color: colors.OnSurfaceVariant},
        LabelStyle:      graphics.TextStyle{FontSize: 14, Color: colors.OnSurfaceVariant},
    },
}
```

### DatePicker Properties

| Property | Type | Description |
|----------|------|-------------|
| `Value` | `*time.Time` | Selected date, nil shows placeholder |
| `OnChanged` | `func(time.Time)` | Called when date is selected |
| `Placeholder` | `string` | Placeholder text |
| `TextStyle` | `graphics.TextStyle` | Text styling |
| `Decoration` | `*widgets.InputDecoration` | Border, background, and label styling |

## TimePicker

```go
// Themed (recommended)
theme.TimePickerOf(ctx, selectedHour, selectedMinute, func(h, m int) {
    s.SetState(func() { selectedHour, selectedMinute = h, m })
})

// Explicit with full styling (no theme defaults)
widgets.TimePicker{
    Hour:   selectedHour,
    Minute: selectedMinute,
    OnChanged: func(hour, minute int) {
        s.SetState(func() {
            selectedHour = hour
            selectedMinute = minute
        })
    },
    TextStyle: graphics.TextStyle{FontSize: 16, Color: colors.OnSurface},
    Decoration: &widgets.InputDecoration{
        LabelText:       "Appointment Time",
        BorderRadius:    8,
        BorderColor:     colors.Outline,
        BackgroundColor: colors.Surface,
        HintStyle:       graphics.TextStyle{FontSize: 16, Color: colors.OnSurfaceVariant},
        LabelStyle:      graphics.TextStyle{FontSize: 14, Color: colors.OnSurfaceVariant},
    },
}
```

### TimePicker Properties

| Property | Type | Description |
|----------|------|-------------|
| `Hour` | `int` | Selected hour (0-23) |
| `Minute` | `int` | Selected minute (0-59) |
| `OnChanged` | `func(int, int)` | Called when time is selected |
| `TextStyle` | `graphics.TextStyle` | Text styling |
| `Decoration` | `*widgets.InputDecoration` | Border, background, and label styling |

## Explicit Styling Requirements

Explicit pickers require `TextStyle` and `Decoration` colors (`BorderColor`, `BackgroundColor`, hint/label styles) for visibility. If you want defaults from the theme, prefer `theme.DatePickerOf` or `theme.TimePickerOf`.

## Related

- [Dropdown](/docs/catalog/input/dropdown) for general selection menus
- [Forms & Validation](/docs/guides/forms) for form-based input
