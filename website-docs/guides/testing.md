---
id: testing
title: Testing
sidebar_position: 16
---

# Testing

Drift includes a widget testing framework in `pkg/testing` that lets you mount widgets, drive the build/layout/paint pipeline, simulate gestures, and compare render tree snapshots without a real window or GPU.

Since the package name collides with the standard library, import it with an alias:

```go
import drifttest "github.com/go-drift/drift/pkg/testing"
```

## Setting Up a Test

Create a `WidgetTester`, configure it if needed, and mount your widget with `PumpWidget`:

```go
func TestGreeting(t *testing.T) {
    tester := drifttest.NewWidgetTesterWithT(t)
    tester.PumpWidget(MyGreeting{Name: "World"})

    result := tester.Find(drifttest.ByText("Hello, World!"))
    if !result.Exists() {
        t.Error("expected greeting text")
    }
}
```

`NewWidgetTesterWithT` registers a cleanup function via `t.Cleanup()` so global state is restored automatically.

By default the tester uses an 800x600 surface at 1x scale with a Material light theme. You can override these before calling `PumpWidget`:

```go
tester.SetSize(graphics.Size{Width: 375, Height: 812})
tester.SetScale(2.0)
tester.SetTheme(myCustomTheme)
```

## Pumping Frames

`PumpWidget` mounts the widget and runs one full frame (build, layout, paint). After making state changes you need to pump additional frames:

```go
tester.Tap(drifttest.ByText("+"))
tester.Pump() // process the rebuild triggered by setState
```

Use `PumpAndSettle` to keep pumping frames until the framework is completely idle (no dirty elements, active tickers, or queued dispatches):

```go
tester.PumpAndSettle(5 * time.Second)
```

It returns `drifttest.ErrSettleTimeout` if the framework doesn't settle within the timeout.

## Finding Widgets

Finders locate elements in the widget tree. Pass them to `tester.Find()` to get a `FinderResult`:

```go
// By widget type
tester.Find(drifttest.ByType[widgets.Text]())

// By exact text content
tester.Find(drifttest.ByText("Submit"))

// By text substring
tester.Find(drifttest.ByTextContaining("Sub"))

// By widget key
tester.Find(drifttest.ByKey("submit-btn"))

// By custom predicate
tester.Find(drifttest.ByPredicate(func(e core.Element) bool { ... }))

// Scoped: descendants of a match
tester.Find(drifttest.Descendant(parentFinder, childFinder))
```

The returned `FinderResult` provides accessors for inspecting matches:

```go
result := tester.Find(drifttest.ByType[widgets.Text]())

result.Exists()       // bool -- at least one match
result.Count()        // int
result.Widget()       // first match's widget
result.RenderObject() // first match's render object
result.All()          // []core.Element
```

## Inspecting Widgets

Cast the result from `Widget()` to access widget-specific fields:

```go
tester.PumpWidget(widgets.Text{Content: "count: 42"})

result := tester.Find(drifttest.ByType[widgets.Text]())
txt := result.Widget().(widgets.Text)
if txt.Content != "count: 42" {
    t.Errorf("expected %q, got %q", "count: 42", txt.Content)
}
```

## Simulating Gestures

The tester routes synthetic pointer events through the render tree's hit testing, matching the production dispatch path.

```go
// Tap the center of the first match
tester.Tap(drifttest.ByText("Click"))
tester.Pump()

// Tap at a specific position
tester.TapAt(graphics.Offset{X: 100, Y: 200})

// Drag a widget
tester.Drag(finder, graphics.Offset{X: 0, Y: -300})
```

Here's a full example testing a button tap:

```go
func TestButton_Tap(t *testing.T) {
    tester := drifttest.NewWidgetTesterWithT(t)

    tapped := false
    tester.PumpWidget(widgets.Button{
        Label:     "Click",
        OnTap:     func() { tapped = true },
        Color:     graphics.RGB(33, 150, 243),
        TextColor: graphics.ColorWhite,
        FontSize:  16,
        Haptic:    true,
    })

    if err := tester.Tap(drifttest.ByText("Click")); err != nil {
        t.Fatalf("Tap failed: %v", err)
    }
    tester.Pump()

    if !tapped {
        t.Error("expected button tap callback to fire")
    }
}
```

## Controlling Time

The tester injects a `FakeClock` that replaces `time.Now()` for all tickers, giving tests deterministic control over animations:

```go
func TestFadeIn(t *testing.T) {
    tester := drifttest.NewWidgetTesterWithT(t)
    tester.PumpWidget(FadeIn{Duration: 300 * time.Millisecond})

    tester.Clock().Advance(150 * time.Millisecond)
    tester.Pump()
    // assert intermediate state...

    tester.Clock().Advance(150 * time.Millisecond)
    tester.PumpAndSettle(time.Second)
    // assert final state...
}
```

## Snapshot Testing

Snapshots serialize the render tree and display list operations to JSON. They catch unintended layout or paint regressions without pixel comparison.

A snapshot contains two sections:

- **Render tree** -- every render object with its type, size, offset, and properties.
- **Display operations** -- the paint commands (`drawRect`, `drawRRect`, `translate`, `clipRect`, etc.) recorded through the canvas.

Here's what a snapshot file looks like:

```json
{
  "renderTree": {
    "id": "RenderText#0",
    "type": "RenderText",
    "size": [800, 600],
    "offset": [0, 0],
    "props": {
      "maxLines": 0,
      "text": "hello"
    }
  }
}
```

### Writing a Snapshot Test

Capture a snapshot and compare it against a golden file:

```go
func TestLoginForm_Layout(t *testing.T) {
    tester := drifttest.NewWidgetTesterWithT(t)
    tester.SetSize(graphics.Size{Width: 375, Height: 667})
    tester.PumpWidget(LoginForm{})

    snap := tester.CaptureSnapshot()
    snap.MatchesFile(t, "testdata/login_form.snapshot.json")
}
```

### Creating and Updating Snapshots

On first run, the test fails because the golden file doesn't exist. Create or update snapshot files by setting the `DRIFT_UPDATE_SNAPSHOTS` environment variable:

```bash
DRIFT_UPDATE_SNAPSHOTS=1 go test ./...
```

Subsequent runs compare against the saved file and report a diff on mismatch:

```
snapshot mismatch: testdata/login_form.snapshot.json
--- expected
+++ actual
-  "size": [375.00, 48.00],
+  "size": [375.00, 56.00],

To update: DRIFT_UPDATE_SNAPSHOTS=1 go test -run TestLoginForm_Layout
```

### Snapshot File Convention

There is no enforced directory, but the convention is to place snapshots in `testdata/` alongside the test file:

```
mypackage/
    widget.go
    widget_test.go
    testdata/
        widget.snapshot.json
```

### Asserting on Snapshot Data

You can also inspect snapshot data programmatically. For example, checking that a container paints a red rectangle:

```go
snap := tester.CaptureSnapshot()
rects := findOps(snap.DisplayOps, "drawRect")
for _, op := range rects {
    if c, ok := op.Params["color"].(string); ok && c == "0xFFFF0000" {
        // found it
    }
}
```

Or comparing two snapshots directly without golden files:

```go
before := tester.CaptureSnapshot()
// ... modify state ...
tester.Pump()
after := tester.CaptureSnapshot()

if diff := before.Diff(after); diff != "" {
    t.Errorf("unexpected change:\n%s", diff)
}
```

## Next Steps

- [Widgets](/docs/guides/widgets) - Available built-in widgets
- [Gestures](/docs/guides/gestures) - Gesture handling in detail
- [Animation](/docs/guides/animation) - Animation system
