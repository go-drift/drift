---
id: rich-text
title: RichText
---

# RichText

Displays a tree of styled text spans where each span can have its own color, weight, size, decoration, and other typographic properties. Unlike [Text](/docs/catalog/display/text), which applies a single style to the entire string, RichText lets you mix styles inline.

## Basic Usage

Build a span tree with `graphics.Span` (leaf) and `graphics.Spans` (container), then pass it as `Content`:

```go
widgets.RichText{
    Content: graphics.Spans(
        graphics.Span("Hello "),
        graphics.Span("World").Bold(),
    ),
}
```

Child spans inherit style fields from their parent for any field left at its zero value. Set a field on a parent span to apply it to all children by default, then override in individual children as needed.

## Themed Constructor

`theme.RichTextOf` creates a RichText with the current theme's text color and body font size set on the widget-level `Style` (not the root span's style). Wrapping is enabled by default:

```go
theme.RichTextOf(ctx,
    graphics.Span("Normal text, "),
    graphics.Span("bold text, ").Bold(),
    graphics.Span("and colored text.").Color(colors.Primary),
)
```

To override layout properties, chain `With*` methods on the returned widget:

```go
theme.RichTextOf(ctx, spans...).WithAlign(graphics.TextAlignCenter).WithMaxLines(3)
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Content` | `graphics.TextSpan` | Root of the styled span tree |
| `Style` | `graphics.SpanStyle` | Widget-level default style; spans inherit these values for any zero-valued fields |
| `Wrap` | `bool` | Enable line wrapping within the available width |
| `MaxLines` | `int` | Maximum number of visible lines (0 = unlimited) |
| `Align` | `graphics.TextAlign` | Horizontal text alignment (only visible when wrapping) |

## Widget Methods

All methods use value receivers and return copies, so calls can be chained:

```go
widgets.RichText{
    Content: graphics.Spans(
        graphics.Span("Hello "),
        graphics.Span("World").Bold(),
    ),
}.WithStyle(graphics.SpanStyle{Color: colors.OnSurface, FontSize: 16})
```

| Method | Description |
|--------|-------------|
| `WithStyle(style)` | Set widget-level default style (lowest priority, inherited by all spans) |
| `WithWrap(bool)` | Enable or disable text wrapping |
| `WithMaxLines(n)` | Set maximum visible line count |
| `WithAlign(align)` | Set horizontal text alignment |

## Span Builder Methods

Every builder method returns a copy, so calls can be chained freely:

```go
graphics.Span("styled").Bold().Italic().Color(0xFFFF0000).Size(20)
```

| Method | Description |
|--------|-------------|
| `Bold()` | Set font weight to bold |
| `Italic()` | Set font style to italic |
| `Weight(w)` | Set a specific font weight |
| `Size(s)` | Set font size in logical pixels |
| `Color(c)` | Set text color |
| `Family(name)` | Set font family |
| `Underline()` | Add underline decoration |
| `Overline()` | Add overline decoration |
| `Strikethrough()` | Add line-through decoration |
| `DecorationColor(c)` | Set decoration line color (inherited by children; defaults to text color when unset) |
| `DecorationStyle(s)` | Set decoration line style (solid, double, dotted, dashed, wavy) |
| `LetterSpacing(v)` | Set spacing between characters |
| `WordSpacing(v)` | Set spacing between words |
| `Height(v)` | Set line height multiplier |
| `Background(c)` | Set background highlight color |
| `WithChildren(...)` | Attach child spans |

### Clearing Inherited Values

When a parent sets a style, children inherit it. Use the `No*` methods to explicitly reset an inherited value:

| Method | Description |
|--------|-------------|
| `NoDecoration()` | Remove inherited decoration |
| `NoDecorationColor()` | Reset decoration color to use text color |
| `NoLetterSpacing()` | Reset letter spacing to zero |
| `NoWordSpacing()` | Reset word spacing to zero |
| `NoHeight()` | Reset line height multiplier |
| `NoBackground()` | Remove inherited background color |

## Style Inheritance

Spans form a tree. Each child merges its style with the parent's resolved style: zero-valued fields inherit, non-zero fields override.

```go
graphics.TextSpan{
    Style: graphics.SpanStyle{Color: colors.Primary, FontSize: 18},
    Children: []graphics.TextSpan{
        graphics.Span("Inherits primary color and 18px. "),
        graphics.Span("Overrides to bold. ").Bold(),
        graphics.Span("Overrides color. ").Color(colors.Error),
    },
}
```

## Text Decorations

```go
theme.RichTextOf(ctx,
    graphics.Span("underline ").Underline(),
    graphics.Span("overline ").Overline(),
    graphics.Span("strikethrough ").Strikethrough(),
    graphics.Span("wavy ").Underline().
        DecorationColor(colors.Error).
        DecorationStyle(graphics.TextDecorationStyleWavy),
)
```

## Background Highlights

```go
theme.RichTextOf(ctx,
    graphics.Span("highlighted").Background(colors.PrimaryContainer),
    graphics.Span(" normal "),
    graphics.Span("also highlighted").Background(colors.SecondaryContainer),
)
```

## Related

- [Text](/docs/catalog/display/text) for single-style text
- [Theming](/docs/guides/theming) for typography configuration
- [Testing](/docs/guides/testing) for finding RichText widgets with `ByText` and `ByTextContaining`
