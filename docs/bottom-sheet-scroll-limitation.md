# Bottom Sheet Scrolling Model

## Summary

The bottom sheet implementation now uses a dedicated render layout that sizes the
content viewport based on the *current* sheet extent. This fixes the previous
scroll limitation where the ListView viewport was pinned to the initial snap height.

## Key Behavior

- Snap points are defined as fractions of **available height** (screen minus safe area insets).
- The sheet extent is stored in pixels and drives layout directly.
- The content viewport height is computed as:

```
contentViewport = currentExtent - handleHeight
```

- The sheet background extends into the bottom safe area, but content does not.
- Dragging and snapping are independent of content size.

## Why This Fixes Scrolling

Scroll views compute their max scroll extent based on their viewport height.
By making the viewport height equal to the *current* sheet extent, the scroll
range grows when the sheet expands. This means:

- At 50% snap, the list scrolls within 50% viewport.
- At 100% snap, the list scrolls within full viewport.

No flex wrappers, Expanded/Flexible hacks, or forced constraints are required.

## Related Files

- `pkg/widgets/bottom_sheet.go`
- `pkg/widgets/bottom_sheet_layout.go`
- `pkg/widgets/bottom_sheet_drag.go`
