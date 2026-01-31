---
id: debugging
title: Debugging & Diagnostics
sidebar_position: 14
---

# Debugging & Diagnostics

Tools for debugging Drift applications during development.

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
| `/tree` | Render tree as JSON |
| `/debug` | Basic root render object info |

### Accessing the Server

The debug server runs inside the app on the device. To access it from your development machine:

**Android (device or emulator):**

```bash
adb forward tcp:9999 tcp:9999
curl http://localhost:9999/tree | jq .
```

**iOS Simulator:**

The simulator shares the host network, so no forwarding is needed:

```bash
curl http://localhost:9999/tree | jq .
```

**iOS Device:**

Use the device's IP address (find it in Settings > Wi-Fi):

```bash
curl http://<device-ip>:9999/tree | jq .
```

## Render Tree Inspection

The `/tree` endpoint returns the render tree, not the widget tree. Render objects handle layout and painting.

### Widget vs Element vs RenderObject

- **Widget**: Immutable configuration object
- **Element**: Mutable instance that manages widget lifecycle
- **RenderObject**: Handles layout and painting

## Performance Optimization

### RepaintBoundary

Isolate expensive subtrees from repainting:

```go
widgets.RepaintBoundary{
    ChildWidget: expensiveWidget,
}
```

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
        ChildWidget: content,
    }
}
```

### Avoiding Unnecessary Rebuilds

```go
// Check before SetState
if s.value != newValue {
    s.SetState(func() {
        s.value = newValue
    })
}
```

## Debug Mode

```go
import "github.com/go-drift/drift/pkg/core"

// Enable debug mode for detailed error screens
core.DebugMode = true
```

In debug mode, uncaught panics show `DebugErrorScreen` with stack traces instead of crashing.

## Next Steps

- [Testing](/docs/guides/testing) - Widget testing framework
- [Error Handling](/docs/guides/error-handling) - Production error handling
