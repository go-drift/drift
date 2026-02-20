---
id: error-boundary
title: Error Boundary
---

# Error Boundary

Catches panics during build, layout, paint, and hit-testing, displaying fallback UI instead of crashing the app.

## Basic Usage

```go
widgets.ErrorBoundary{
    Child: riskyWidget,
    FallbackBuilder: func(err *drifterrors.BoundaryError) core.Widget {
        return widgets.Text{Content: "Something went wrong"}
    },
    OnError: func(err *drifterrors.BoundaryError) {
        log.Printf("Widget error: %v", err)
    },
}
```

## Properties

| Property | Type | Description |
|----------|------|-------------|
| `Child` | `core.Widget` | The widget tree to protect |
| `FallbackBuilder` | `func(*BoundaryError) Widget` | Builds fallback UI when an error is caught |
| `OnError` | `func(*BoundaryError)` | Called when an error is caught |

## Error Widgets

| Widget | Purpose |
|--------|---------|
| `ErrorWidget` | Inline error display (default fallback) |
| `DebugErrorScreen` | Full-screen error with stack trace (debug mode) |

## Programmatic Control

Access the boundary's state from descendant widgets:

```go
state := widgets.ErrorBoundaryOf(ctx)
if state != nil && state.HasError() {
    state.Reset()  // Clear error and retry rendering
}
```

## Common Patterns

### Scoped Error Handling

Wrap specific subtrees to isolate failures:

```go
widgets.Column{
    Children: []core.Widget{
        HeaderWidget{},  // Keeps working
        widgets.ErrorBoundary{
            Child: RiskyWidget{},
            FallbackBuilder: func(err *drifterrors.BoundaryError) core.Widget {
                return widgets.Text{Content: "Failed to load"}
            },
        },
        FooterWidget{},  // Keeps working
    },
}
```

### Global Error Handling

Wrap the entire app for production:

```go
func main() {
    drift.NewApp(widgets.ErrorBoundary{
        Child: MyApp{},
        FallbackBuilder: func(err *drifterrors.BoundaryError) core.Widget {
            return MyCustomErrorScreen{Error: err}
        },
    }).Run()
}
```

## Related

- [Error Handling & Debugging](/docs/guides/error-handling-and-debugging) for debug vs production behavior and best practices
- [Progress Indicators](/docs/catalog/feedback/progress-indicators) for loading states
