---
id: error-handling
title: Error Handling
sidebar_position: 14
---

# Error Handling

Error boundaries catch panics and display fallback UI instead of crashing your app.

## Debug vs Production Behavior

**Debug mode** (`core.DebugMode = true`): Uncaught panics anywhere in the app automatically display a full-screen `DebugErrorScreen` with detailed error information and stack traces. This helps during development.

**Production mode** (`core.DebugMode = false`): Uncaught panics crash the app. Use `ErrorBoundary` to catch panics and show graceful fallback UI.

## Basic Usage

Import the Drift errors package with an alias to avoid conflict with the standard library:

```go
import (
    "log"

    "github.com/go-drift/drift/pkg/core"
    drifterrors "github.com/go-drift/drift/pkg/errors"
    "github.com/go-drift/drift/pkg/widgets"
)
```

Then wrap widgets with an error boundary:

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

ErrorBoundary catches panics during:
- **Build**: widget `Build()` methods
- **Layout**: render object layout
- **Paint**: render object painting
- **HitTest**: hit testing for pointer events

## Scoped Error Handling

Wrap specific subtrees to isolate failures while keeping the rest of the app running:

```go
widgets.Column{
    Children: []core.Widget{
        HeaderWidget{},  // Keeps working
        widgets.ErrorBoundary{
            Child: RiskyWidget{},  // Isolated failure
            FallbackBuilder: func(err *drifterrors.BoundaryError) core.Widget {
                return widgets.Text{Content: "Failed to load"}
            },
        },
        FooterWidget{},  // Keeps working
    },
}
```

## Global Error Handling (Production)

Wrap your entire app to provide custom error UI in production:

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

## Programmatic Control

Access the boundary's state from descendant widgets:

```go
state := widgets.ErrorBoundaryOf(ctx)
if state != nil && state.HasError() {
    state.Reset()  // Clear error and retry rendering
}
```

## Error Widgets

Drift provides built-in error display widgets:

| Widget | Purpose |
|--------|---------|
| `ErrorWidget` | Inline error display (default fallback) |
| `DebugErrorScreen` | Full-screen error with stack trace (debug mode) |

## When to Use Error Boundaries

- **Production apps**: Wrap your root widget to prevent crashes
- **Third-party widgets**: Isolate untrusted code
- **Complex subtrees**: Contain failures to specific sections
- **External data dependencies**: Handle network/parsing failures gracefully

## Next Steps

- [Debugging](/docs/guides/debugging) - Diagnostics and performance tools
- [Testing](/docs/guides/testing) - Widget testing framework
