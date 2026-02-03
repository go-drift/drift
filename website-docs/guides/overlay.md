---
id: overlay
title: Overlays
sidebar_position: 10
---

# Overlays

Drift's overlay system provides core infrastructure for modals, dialogs, bottom sheets, tooltips, and other floating UI elements that appear above the main content.

## Overview

The overlay system consists of:

- **Overlay**: A container widget that manages a stack of overlay entries above its child
- **OverlayEntry**: A mutable handle for inserting/removing content from the overlay
- **ModalBarrier**: A semi-transparent scrim with optional tap-to-dismiss behavior
- **ModalRoute**: A route type that displays as a modal overlay with a barrier

## Getting Started with Overlay

### Accessing the Overlay

Use `OverlayOf(ctx)` to access the nearest overlay's state from within the widget tree:

```go
func showTooltip(ctx core.BuildContext) {
    overlayState := overlay.OverlayOf(ctx)
    if overlayState == nil {
        // No overlay ancestor - handle gracefully
        return
    }

    // Create entry with constructor (required for proper keying)
    entry := overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
        return MyTooltip{}
    })
    overlayState.Insert(entry, nil, nil)
}
```

### Creating Overlay Entries

Always use `NewOverlayEntry()` to create entries. This constructor assigns a unique ID for stable keying:

```go
entry := overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
    return widgets.Container{
        Color:  graphics.RGBA(0, 0, 0, 200),
        Width:  200,
        Height: 100,
        Child:  widgets.Text{Content: "I'm an overlay!"},
    }
})
```

### Inserting Entries

The `Insert` method accepts positioning parameters:

```go
// Insert at top (default)
overlayState.Insert(entry, nil, nil)

// Insert below a specific entry
overlayState.Insert(newEntry, existingEntry, nil)

// Insert above a specific entry
overlayState.Insert(newEntry, nil, existingEntry)
```

### Removing Entries

Call `Remove()` on the entry to remove it:

```go
entry.Remove()
```

This is safe to call:
- After Insert (removes from overlay)
- Before first build (cancels pending entry)
- Multiple times (no-op if already removed)

## Entry Lifecycle

An `OverlayEntry` progresses through these states:

| State | overlay | mounted | entryState | MarkNeedsBuild | Remove |
|-------|---------|---------|------------|----------------|--------|
| Created | nil | false | nil | no-op | no-op |
| After Insert() | set | false | nil | no-op | removes from overlay |
| After Build | set | true | set | triggers rebuild | removes + unmounts |
| After Remove() | nil | false | nil | no-op | no-op |

After `Remove()`, an entry can be re-inserted to any overlay.

## Rebuilding Entries

Use `MarkNeedsBuild()` to trigger a rebuild of an entry's widget:

```go
entry.MarkNeedsBuild()
```

This is a no-op if the entry is not currently mounted.

## Entry Configuration

### Opaque

The `Opaque` field controls whether hits pass through to the underlying page content:

```go
entry := overlay.NewOverlayEntry(builder)
entry.Opaque = true  // Block hits from reaching the page content
```

When `Opaque` is true:
- Hits are blocked from reaching the child (page content) below the overlay
- Other overlay entries (like barriers) can still receive hits
- Entries below are still rendered (for partial transparency effects)
- Use for modals where the page should not be interactive

This design allows modal barriers to work correctly - the barrier sits below the opaque dialog content but can still receive dismiss taps.

### MaintainState

The `MaintainState` field is reserved for future use:

```go
entry.MaintainState = true  // Reserved, currently has no effect
```

Currently all entries are always built regardless of this flag.

## Modal Barrier

`ModalBarrier` prevents interaction with widgets behind it:

```go
func buildBarrier(ctx core.BuildContext) core.Widget {
    return overlay.ModalBarrier{
        Color:         graphics.RGBA(0, 0, 0, 128),  // 50% black
        Dismissible:   true,
        OnDismiss:     func() { entry.Remove() },
        SemanticLabel: "Dismiss dialog",
    }
}
```

Properties:
- **Color**: Background color (typically semi-transparent black)
- **Dismissible**: When true, tapping the barrier triggers OnDismiss
- **OnDismiss**: Called when barrier is tapped (if Dismissible=true)
- **SemanticLabel**: Accessibility label for screen readers

The barrier always absorbs all touches, even when `Dismissible=false`.

## Modal Routes

For modals that integrate with navigation, use `ModalRoute`:

```go
func showDialog(ctx core.BuildContext) {
    nav := navigation.NavigatorOf(ctx)
    if nav == nil {
        return
    }

    route := navigation.NewModalRoute(
        func(ctx core.BuildContext) core.Widget {
            return MyDialog{}
        },
        navigation.RouteSettings{Name: "/dialog"},
    )
    route.BarrierDismissible = true
    barrierColor := graphics.RGBA(0, 0, 0, 0.5)
    route.BarrierColor = &barrierColor  // Pointer to allow nil (use default)
    route.BarrierLabel = "Close dialog"

    nav.Push(route)
}
```

`ModalRoute` automatically:
- Creates a modal barrier entry
- Creates a content entry above the barrier
- Removes both entries when the route is popped
- Handles the case where overlay isn't ready yet (defers insertion)

## Navigator Integration

The `Navigator` widget automatically wraps its content in an `Overlay`. This means:

- Modal routes work out of the box
- Custom overlays can be added via `OverlayOf(ctx)`
- The overlay state becomes available after the first build

The navigator notifies routes when the overlay becomes available via `SetOverlay()`.

## Common Patterns

### Tooltip Overlay

```go
type tooltipState struct {
    core.StateBase
    entry *overlay.OverlayEntry
}

func (s *tooltipState) showTooltip(ctx core.BuildContext, message string) {
    overlayState := overlay.OverlayOf(ctx)
    if overlayState == nil {
        return
    }

    s.entry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
        return widgets.Positioned{
            Left: widgets.Ptr(100),
            Top:  widgets.Ptr(200),
            Child: widgets.Container{
                Padding: layout.EdgeInsetsAll(8),
                Color:   graphics.RGBA(50, 50, 50, 230),
                Child:   widgets.Text{Content: message},
            },
        }
    })
    overlayState.Insert(s.entry, nil, nil)
}

func (s *tooltipState) hideTooltip() {
    if s.entry != nil {
        s.entry.Remove()
        s.entry = nil
    }
}

func (s *tooltipState) Dispose() {
    s.hideTooltip()
    s.StateBase.Dispose()
}
```

### Stacked Overlays

Multiple overlay entries stack in order (first inserted = bottom, last inserted = top):

```go
// Toast appears below dialog
toastEntry := overlay.NewOverlayEntry(buildToast)
overlayState.Insert(toastEntry, nil, nil)

// Barrier blocks interaction with toast and page content
barrierEntry := overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
    return overlay.ModalBarrier{
        Color:       graphics.RGBA(0, 0, 0, 128),
        Dismissible: false,
    }
})
overlayState.Insert(barrierEntry, nil, nil)

// Dialog appears above barrier
dialogEntry := overlay.NewOverlayEntry(buildDialog)
dialogEntry.Opaque = true  // Block hits from reaching page content
overlayState.Insert(dialogEntry, nil, nil)
```

### Using InitialEntries

For overlays that should exist from the start:

```go
overlay.Overlay{
    InitialEntries: []*overlay.OverlayEntry{
        overlay.NewOverlayEntry(buildPersistentBanner),
    },
    Child: MainContent{},
}
```

## Build-Time Considerations

Operations during build are handled safely:

- Insertions during build are queued until after build completes
- Removals during build are also queued
- Remove cancels any pending Insert for the same entry
- `OnOverlayReady` fires after build completes to avoid re-entrancy

## Best Practices

1. **Always use NewOverlayEntry()**: This assigns unique IDs for stable keying
2. **Clean up in Dispose**: Remove entries when your widget is disposed
3. **Use Opaque for modals**: Set `Opaque=true` for modal content to block page interaction
4. **Handle missing overlay**: Always check if `OverlayOf(ctx)` returns nil
5. **Use ModalRoute for navigation**: When modals are part of navigation flow, use `ModalRoute`
6. **Use barriers with modals**: Always pair opaque content with a ModalBarrier for dismiss handling

## API Reference

### overlay.NewOverlayEntry

```go
func NewOverlayEntry(builder func(ctx core.BuildContext) core.Widget) *OverlayEntry
```

Creates an OverlayEntry with a unique ID.

### overlay.OverlayOf

```go
func OverlayOf(ctx core.BuildContext) OverlayState
```

Returns the nearest Overlay ancestor's state, or nil if no Overlay exists.

### overlay.OverlayState

```go
type OverlayState interface {
    Insert(entry *OverlayEntry, below *OverlayEntry, above *OverlayEntry)
    InsertAll(entries []*OverlayEntry, below *OverlayEntry, above *OverlayEntry)
    Rearrange(newEntries []*OverlayEntry)
}
```

### overlay.OverlayEntry

```go
type OverlayEntry struct {
    Builder       func(ctx core.BuildContext) core.Widget
    Opaque        bool
    MaintainState bool
}

func (e *OverlayEntry) Remove()
func (e *OverlayEntry) MarkNeedsBuild()
```

### overlay.ModalBarrier

```go
type ModalBarrier struct {
    Color         graphics.Color
    Dismissible   bool
    OnDismiss     func()
    SemanticLabel string
}
```

### navigation.NewModalRoute

```go
func NewModalRoute(
    builder func(ctx core.BuildContext) core.Widget,
    settings RouteSettings,
) *ModalRoute
```
