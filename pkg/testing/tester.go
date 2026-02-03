package testing

import (
	"errors"
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

const (
	// DefaultTestWidth is the default logical width for the test surface.
	DefaultTestWidth = 800
	// DefaultTestHeight is the default logical height for the test surface.
	DefaultTestHeight = 600
	// DefaultScale is the default device pixel ratio.
	DefaultScale = 1.0
)

// ErrSettleTimeout is returned when PumpAndSettle exceeds its timeout.
var ErrSettleTimeout = errors.New("PumpAndSettle timed out: framework did not settle")

// WidgetTester provides isolated widget testing without real rendering.
// It drives the same build, layout, and paint phases as the engine but
// uses a fake clock and recording canvas instead of the platform layer.
type WidgetTester struct {
	buildOwner *core.BuildOwner
	root       core.Element
	rootRender layout.RenderObject
	clock      *FakeClock
	prevClock  animation.Clock
	size       graphics.Size
	scale      float64
	theme      *theme.AppThemeData
	dispatches []func()
	pointers   map[int]*pointerState
	recorder   *graphics.PictureRecorder
}

// NewWidgetTester creates a tester with default test environment.
// Call Cleanup() when done, or use NewWidgetTesterWithT() instead.
func NewWidgetTester() *WidgetTester {
	clk := NewFakeClock()
	t := &WidgetTester{
		buildOwner: core.NewBuildOwner(),
		clock:      clk,
		size:       graphics.Size{Width: DefaultTestWidth, Height: DefaultTestHeight},
		scale:      DefaultScale,
		theme:      theme.NewAppThemeData(theme.TargetPlatformMaterial, theme.BrightnessLight).Copy(),
		pointers:   make(map[int]*pointerState),
		recorder:   &graphics.PictureRecorder{},
	}
	t.prevClock = animation.SetClock(clk)
	// Register this tester's dispatch function with the platform package
	// so that platform.Dispatch works during tests
	platform.RegisterDispatch(t.Dispatch)
	return t
}

// NewWidgetTesterWithT creates a tester that auto-cleans up via t.Cleanup().
// This is the recommended constructor for tests.
func NewWidgetTesterWithT(t *testing.T) *WidgetTester {
	tester := NewWidgetTester()
	t.Cleanup(tester.Cleanup)
	return tester
}

// Cleanup restores global state (animation clock). Must be called if
// not using NewWidgetTesterWithT.
func (t *WidgetTester) Cleanup() {
	if t.root != nil {
		t.root.Unmount()
		t.root = nil
		t.rootRender = nil
	}
	animation.SetClock(t.prevClock)
}

// SetSize sets the logical surface size. Must be called before PumpWidget.
func (t *WidgetTester) SetSize(size graphics.Size) {
	t.size = size
}

// SetScale sets the device pixel ratio. Must be called before PumpWidget.
func (t *WidgetTester) SetScale(scale float64) {
	t.scale = scale
}

// SetTheme replaces the theme data. Must be called before PumpWidget.
func (t *WidgetTester) SetTheme(td *theme.AppThemeData) {
	t.theme = td
}

// Clock returns the fake clock for advancing time in tests.
func (t *WidgetTester) Clock() *FakeClock {
	return t.clock
}

// PumpWidget mounts (or remounts) a widget and runs one full frame.
func (t *WidgetTester) PumpWidget(widget core.Widget) error {
	// Unmount previous tree
	if t.root != nil {
		t.root.Unmount()
		t.root = nil
		t.rootRender = nil
	}

	// Wrap in test scaffold: DeviceScale → AppTheme → user widget
	wrapped := widgets.DeviceScale{
		Scale: t.scale,
		Child: theme.AppTheme{
			Data:  t.theme,
			Child: widget,
		},
	}

	// Mount new tree
	t.root = core.MountRoot(wrapped, t.buildOwner)
	if renderElement, ok := t.root.(interface{ RenderObject() layout.RenderObject }); ok {
		t.rootRender = renderElement.RenderObject()
	}

	// Schedule initial layout/paint
	if t.rootRender != nil {
		pipeline := t.buildOwner.Pipeline()
		pipeline.ScheduleLayout(t.rootRender)
		pipeline.SchedulePaint(t.rootRender)
	}

	return t.Pump()
}

// Pump runs a single frame cycle: dispatches, tickers, build, layout, paint.
func (t *WidgetTester) Pump() error {
	// 1. Drain dispatch queue
	dispatches := t.dispatches
	t.dispatches = nil
	for _, fn := range dispatches {
		fn()
	}

	// 2. Step ballistics and tickers
	widgets.StepBallistics()
	animation.StepTickers()

	// 3. Flush build
	t.buildOwner.FlushBuild()

	// 4. Flush layout
	if t.rootRender != nil {
		pipeline := t.buildOwner.Pipeline()
		constraints := layout.Tight(t.size)
		pipeline.FlushLayoutForRoot(t.rootRender, constraints)

		// 5. Flush paint
		pipeline.FlushPaint()
	}

	return nil
}

// PumpAndSettle runs frames until the framework is idle or the timeout
// is reached. Each frame advances the fake clock by frameDuration (16ms).
// Returns ErrSettleTimeout if the framework does not settle within timeout.
func (t *WidgetTester) PumpAndSettle(timeout time.Duration) error {
	const frameDuration = 16 * time.Millisecond
	var elapsed time.Duration
	for elapsed < timeout {
		if err := t.Pump(); err != nil {
			return err
		}
		if !t.needsWork() {
			return nil
		}
		t.clock.Advance(frameDuration)
		elapsed += frameDuration
	}
	return ErrSettleTimeout
}

// needsWork returns true if the framework has pending work.
func (t *WidgetTester) needsWork() bool {
	return t.buildOwner.NeedsWork() ||
		animation.HasActiveTickers() ||
		widgets.HasActiveBallistics() ||
		len(t.dispatches) > 0
}

// Dispatch queues a callback for the next frame, mirroring engine.Dispatch.
func (t *WidgetTester) Dispatch(fn func()) {
	t.dispatches = append(t.dispatches, fn)
}

// RootElement returns the root element of the mounted tree.
func (t *WidgetTester) RootElement() core.Element {
	return t.root
}

// RootRenderObject returns the root render object of the mounted tree.
func (t *WidgetTester) RootRenderObject() layout.RenderObject {
	return t.rootRender
}

// Find evaluates a finder against the current element tree.
func (t *WidgetTester) Find(finder Finder) FinderResult {
	if t.root == nil {
		return FinderResult{finder: finder}
	}
	return FinderResult{
		elements: finder.Evaluate(t.root),
		finder:   finder,
	}
}

// extractRenderObject walks from an element to find its render object.
func extractRenderObject(e core.Element) layout.RenderObject {
	if e == nil {
		return nil
	}
	if ro, ok := e.(interface{ RenderObject() layout.RenderObject }); ok {
		return ro.RenderObject()
	}
	return nil
}
