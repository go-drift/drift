---
id: error-handling-and-debugging
title: Error Handling & Debugging
sidebar_position: 1
---

# Error Handling & Debugging

## Error Boundaries

Error boundaries catch panics and display fallback UI instead of crashing your app.

### Debug vs Production Behavior

**Debug mode** (`core.DebugMode = true`): Uncaught panics anywhere in the app automatically display a full-screen `DebugErrorScreen` with detailed error information and stack traces. This helps during development.

**Production mode** (`core.DebugMode = false`): Uncaught panics crash the app. Use `ErrorBoundary` to catch panics and show graceful fallback UI.

### Basic Usage

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

### Scoped Error Handling

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

### Global Error Handling (Production)

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

### Programmatic Control

Access the boundary's state from descendant widgets:

```go
state := widgets.ErrorBoundaryOf(ctx)
if state != nil && state.HasError() {
    state.Reset()  // Clear error and retry rendering
}
```

### Error Widgets

Drift provides built-in error display widgets:

| Widget | Purpose |
|--------|---------|
| `ErrorWidget` | Inline error display (default fallback) |
| `DebugErrorScreen` | Full-screen error with stack trace (debug mode) |

### When to Use Error Boundaries

- **Production apps**: Wrap your root widget to prevent crashes
- **Third-party widgets**: Isolate untrusted code
- **Complex subtrees**: Contain failures to specific sections
- **External data dependencies**: Handle network/parsing failures gracefully

## Diagnostics HUD

Display frame rate and timing information on screen.

### Enabling Diagnostics

```go
func main() {
    app := drift.NewApp(MyApp{})
    app.Diagnostics = engine.DefaultDiagnosticsConfig()
    app.Run()
}
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `ShowFPS` | Display current frame rate |
| `ShowFrameGraph` | Render frame timing visualization |
| `ShowLayoutBounds` | Draw colored borders around widget bounds |
| `Position` | HUD placement (TopLeft, TopRight, etc.) |
| `GraphSamples` | Number of frames to show in graph (default: 60) |
| `TargetFrameTime` | Expected frame duration (default: 16.67ms for 60fps) |
| `DebugServerPort` | HTTP debug server port (0 = disabled) |
| `RuntimeSampleInterval` | Runtime sample interval (default: 5s) |
| `RuntimeSampleWindow` | Runtime sample history window (default: 60s) |

Note: when `DebugServerPort` is enabled, runtime sampling is enabled by default
using the interval/window settings above.

## Debug Server

HTTP server for remote inspection.

### Enabling the Server

Enable the debug server by setting `DebugServerPort` in the diagnostics config:

```go
func main() {
    app := drift.NewApp(MyApp{})
    config := engine.DefaultDiagnosticsConfig()
    config.DebugServerPort = 9999
    app.Diagnostics = config
    app.Run()
}
```

### Endpoints

| Endpoint | Description |
|----------|-------------|
| `/health` | Server status check |
| `/render-tree` | Render tree as JSON (layout and painting) |
| `/widget-tree` | Widget/element tree as JSON (configuration and state) |
| `/frames` | Recent frame timings, counts, and flags |
| `/runtime` | Recent runtime/GC samples |
| `/jank` | Combined frames/runtime snapshot |
| `/debug` | Basic root render object info |

### Accessing the Server

The debug server runs inside the app on the device. To access it from your development machine:

**Android (device or emulator):**

```bash
adb forward tcp:9999 tcp:9999
curl http://localhost:9999/render-tree | jq .
```

**iOS Simulator:**

The simulator shares the host network, so no forwarding is needed:

```bash
curl http://localhost:9999/render-tree | jq .
```

**iOS Device:**

Use the device's IP address (find it in Settings > Wi-Fi):

```bash
curl http://<device-ip>:9999/render-tree | jq .
```

### Filtering Frame Timelines

`/frames` supports optional query params:

- `limit` (int): return only the last N samples
- `min_ms` (float): return only samples with `frameMs >= min_ms`
- `dispatch_ms` (float): return samples with `dispatchMs >= dispatch_ms`
- `animate_ms` (float): return samples with `animateMs >= animate_ms`
- `build_ms` (float): return samples with `buildMs >= build_ms`
- `layout_ms` (float): return samples with `layoutMs >= layout_ms`
- `record_ms` (float): return samples with `recordMs >= record_ms`
- `composite_ms` (float): return samples with `compositeMs >= composite_ms`
- `semantics_ms` (float): return samples with `semanticsMs >= semantics_ms`
- `flush_ms` (float): return samples with `platformFlushMs >= flush_ms`
- `trace_overhead_ms` (float): return samples with `traceOverheadMs >= trace_overhead_ms`
- `resumed` (bool): return only samples where `resumedThisFrame` is true

Examples:

```bash
curl "http://localhost:9999/frames?limit=120" | jq .
curl "http://localhost:9999/frames?min_ms=16.7" | jq .
curl "http://localhost:9999/frames?layout_ms=6&resumed=1" | jq .
```

### Runtime Samples

`/runtime` returns a ring buffer of runtime/GC snapshots.

Optional query params:

- `window` (seconds): return samples from the last N seconds
- `limit` (int): return only the last N samples

Examples:

```bash
curl "http://localhost:9999/runtime" | jq .
curl "http://localhost:9999/runtime?window=30" | jq .
curl "http://localhost:9999/runtime?limit=3" | jq .
```

Note: sampling starts when diagnostics are enabled (debug server on), so the
buffer fills over time.

### Combined Snapshot

`/jank` returns both frame samples and runtime samples in one response.

Note: `limit` applies to both frame samples and runtime samples when used with
`/jank`.

```bash
curl "http://localhost:9999/jank?min_ms=8&window=30" | jq .
```

## Tree Inspection

Drift maintains three parallel trees. The debug server exposes two of them:

### Widget vs Element vs RenderObject

- **Widget**: Immutable configuration object describing what the UI should look like
- **Element**: Mutable instance that manages widget lifecycle and holds state
- **RenderObject**: Handles layout calculations and painting

### Render Tree (`/render-tree`)

Returns the render tree with layout and painting information:

```json
{
  "type": "*layout.RenderFlex",
  "size": {"width": 400, "height": 800},
  "constraints": {"minWidth": 0, "maxWidth": 400, "minHeight": 0, "maxHeight": 800},
  "needsLayout": false,
  "needsPaint": false,
  "isRepaintBoundary": false,
  "children": [...]
}
```

### Widget Tree (`/widget-tree`)

Returns the element tree with widget configuration and state information:

```json
{
  "widgetType": "widgets.Column",
  "elementType": "*core.RenderObjectElement",
  "depth": 3,
  "needsBuild": false,
  "hasState": false,
  "children": [...]
}
```

The `hasState` field is `true` for elements backed by a `StatefulWidget`, indicating they have associated state.

## Performance Optimization

### RepaintBoundary

Isolate expensive subtrees from repainting. Each boundary gets its own cached layer -- when the subtree repaints, only that layer is re-recorded. See the [Layout guide](layout.md#repaint-boundaries-and-the-layer-tree) for details.

```go
widgets.RepaintBoundary{
    Child: expensiveWidget,
}
```

In the render tree output, `"isRepaintBoundary": true` indicates nodes with their own layer. `"needsPaint": true` means the layer will be re-recorded on the next frame.

### ListViewBuilder for Large Lists

Use virtualized lists instead of ListView:

```go
widgets.ListViewBuilder{
    ItemCount:  1000,
    ItemExtent: 60,
    ItemBuilder: func(ctx core.BuildContext, i int) core.Widget {
        return buildItem(items[i])
    },
}
```

### Theme Memoization

Cache theme data to avoid unnecessary lookups:

```go
func (s *appState) Build(ctx core.BuildContext) core.Widget {
    // Cache theme at app root
    themeData := s.cachedTheme
    return theme.Theme{
        Data:        themeData,
        Child: content,
    }
}
```

### Avoiding Unnecessary Rebuilds

See the [State Management](/docs/guides/state-management#4-minimize-rebuilds) guide for techniques to minimize unnecessary rebuilds.

## Debug Mode

```go
import "github.com/go-drift/drift/pkg/core"

// Enable debug mode for detailed error screens
core.DebugMode = true
```

In debug mode, uncaught panics show `DebugErrorScreen` with stack traces instead of crashing.

## Next Steps

- [Testing](/docs/guides/testing) - Widget testing framework
- [State Management](/docs/guides/state-management) - Managing widget state
