package widgets

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// DiagnosticsHUDDataSource provides frame timing data to the HUD.
type DiagnosticsHUDDataSource interface {
	// FPSLabel returns the current FPS display string.
	FPSLabel() string
	// SamplesInto copies frame samples into dst and returns count copied.
	SamplesInto(dst []time.Duration) int
	// SampleCount returns the number of samples available.
	SampleCount() int
	// RegisterRenderObject registers the HUD render object for targeted repaints.
	RegisterRenderObject(ro layout.RenderObject)
}

// DiagnosticsHUD displays performance metrics overlay.
type DiagnosticsHUD struct {
	// DataSource provides frame timing data.
	DataSource DiagnosticsHUDDataSource
	// TargetTime is the target frame duration for coloring the graph.
	TargetTime time.Duration
	// GraphWidth is the width of the frame graph. Defaults to 120.
	GraphWidth float64
	// GraphHeight is the height of the frame graph. Defaults to 60.
	GraphHeight float64
	// ShowFPS controls whether to display the FPS counter.
	ShowFPS bool
	// ShowFrameGraph controls whether to display the frame time graph.
	ShowFrameGraph bool
}

func (d DiagnosticsHUD) CreateElement() core.Element {
	return core.NewStatelessElement(d, nil)
}

func (d DiagnosticsHUD) Key() any {
	return nil
}

func (d DiagnosticsHUD) Build(ctx core.BuildContext) core.Widget {
	graphWidth := d.GraphWidth
	if graphWidth == 0 {
		graphWidth = 120
	}
	graphHeight := d.GraphHeight
	if graphHeight == 0 {
		graphHeight = 60
	}

	return diagnosticsHUDRender{
		dataSource:     d.DataSource,
		targetTime:     d.TargetTime,
		graphWidth:     graphWidth,
		graphHeight:    graphHeight,
		showFPS:        d.ShowFPS,
		showFrameGraph: d.ShowFrameGraph,
	}
}

type diagnosticsHUDRender struct {
	dataSource     DiagnosticsHUDDataSource
	targetTime     time.Duration
	graphWidth     float64
	graphHeight    float64
	showFPS        bool
	showFrameGraph bool
}

func (d diagnosticsHUDRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d diagnosticsHUDRender) Key() any {
	return nil
}

func (d diagnosticsHUDRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderDiagnosticsHUD{
		sampleBuffer: make([]time.Duration, 60), // Pre-allocate for typical sample count
	}
	r.SetSelf(r)
	r.update(d)
	// Register for targeted repaints
	if d.dataSource != nil {
		d.dataSource.RegisterRenderObject(r)
	}
	return r
}

func (d diagnosticsHUDRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderDiagnosticsHUD); ok {
		r.update(d)
		// Re-register in case data source changed
		if d.dataSource != nil {
			d.dataSource.RegisterRenderObject(r)
		}
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderDiagnosticsHUD struct {
	layout.RenderBoxBase
	dataSource     DiagnosticsHUDDataSource
	targetTime     time.Duration
	graphWidth     float64
	graphHeight    float64
	showFPS        bool
	showFrameGraph bool

	// Cached state
	textLayout     *graphics.TextLayout
	cachedFPSLabel string
	sampleBuffer   []time.Duration // Reusable buffer for samples
}

func (r *renderDiagnosticsHUD) update(d diagnosticsHUDRender) {
	r.dataSource = d.dataSource
	r.targetTime = d.targetTime
	r.graphWidth = d.graphWidth
	r.graphHeight = d.graphHeight
	r.showFPS = d.showFPS
	r.showFrameGraph = d.showFrameGraph
}

// IsRepaintBoundary returns true to isolate HUD repaints from the main app.
func (r *renderDiagnosticsHUD) IsRepaintBoundary() bool {
	return true
}

func (r *renderDiagnosticsHUD) PerformLayout() {
	// Calculate size based on what's shown
	width := r.graphWidth + 16 // 8px padding on each side
	height := 8.0              // Base padding

	if r.showFPS {
		height += 18 // Text height + padding
	}
	if r.showFrameGraph {
		height += r.graphHeight + 4 // Graph + padding
	}

	constraints := r.Constraints()
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderDiagnosticsHUD) Paint(ctx *layout.PaintContext) {
	size := r.Size()

	// Draw semi-transparent background
	bgPaint := graphics.DefaultPaint()
	bgPaint.Color = graphics.RGBA(0, 0, 0, 0.71)
	bgRect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)
	bgRRect := graphics.RRectFromRectAndRadius(bgRect, graphics.CircularRadius(4))
	ctx.Canvas.DrawRRect(bgRRect, bgPaint)

	yOffset := 4.0

	// Draw FPS label if enabled
	if r.showFPS && r.dataSource != nil {
		fpsLabel := r.dataSource.FPSLabel()

		// Only recreate text layout if label changed
		if fpsLabel != r.cachedFPSLabel || r.textLayout == nil {
			r.cachedFPSLabel = fpsLabel
			textStyle := graphics.TextStyle{
				Color:      graphics.RGB(255, 255, 255),
				FontSize:   12,
				FontWeight: graphics.FontWeightBold,
			}
			manager, _ := graphics.DefaultFontManagerErr()
			if manager != nil {
				r.textLayout, _ = graphics.LayoutTextWithConstraints(fpsLabel, textStyle, manager, 0)
			}
		}

		if r.textLayout != nil {
			ctx.Canvas.DrawText(r.textLayout, graphics.Offset{X: 8, Y: yOffset})
		}
		yOffset += 18
	}

	// Draw frame graph if enabled
	if r.showFrameGraph && r.dataSource != nil {
		graphLeft := 8.0
		graphTop := yOffset
		graphWidth := r.graphWidth
		graphHeight := r.graphHeight

		// Get samples without allocation (reuse buffer)
		sampleCount := r.dataSource.SampleCount()
		if sampleCount > len(r.sampleBuffer) {
			r.sampleBuffer = make([]time.Duration, sampleCount)
		}
		numSamples := r.dataSource.SamplesInto(r.sampleBuffer)

		if numSamples > 0 {
			// Calculate bar width based on number of samples
			barWidth := graphWidth / float64(numSamples)
			if barWidth < 1 {
				// Too many samples - show only the most recent ones that fit
				barWidth = 1
				maxBars := int(graphWidth)
				if numSamples > maxBars {
					// Skip older samples by adjusting the start offset
					copy(r.sampleBuffer, r.sampleBuffer[numSamples-maxBars:numSamples])
					numSamples = maxBars
				}
			}

			// Find max frame time for scaling (cap at 4x target to avoid tiny bars)
			maxTime := r.targetTime * 4
			for i := 0; i < numSamples; i++ {
				if r.sampleBuffer[i] > maxTime {
					maxTime = r.sampleBuffer[i]
				}
			}

			// Draw bars
			for i := 0; i < numSamples; i++ {
				ft := r.sampleBuffer[i]

				// Calculate bar height (0 = bottom, graphHeight = top)
				barHeight := (float64(ft) / float64(maxTime)) * graphHeight
				if barHeight > graphHeight {
					barHeight = graphHeight
				}
				if barHeight < 1 {
					barHeight = 1
				}

				// Determine color based on frame time relative to target
				var barColor graphics.Color
				ratio := float64(ft) / float64(r.targetTime)
				if ratio <= 1.0 {
					// Green - at or below target
					barColor = graphics.RGB(76, 175, 80)
				} else if ratio <= 2.0 {
					// Yellow - 1-2x target
					barColor = graphics.RGB(255, 193, 7)
				} else {
					// Red - >2x target
					barColor = graphics.RGB(244, 67, 54)
				}

				barPaint := graphics.DefaultPaint()
				barPaint.Color = barColor

				x := graphLeft + float64(i)*barWidth
				y := graphTop + graphHeight - barHeight
				ctx.Canvas.DrawRect(graphics.RectFromLTWH(x, y, barWidth-1, barHeight), barPaint)
			}

			// Draw target line
			targetY := graphTop + graphHeight - (float64(r.targetTime)/float64(maxTime))*graphHeight
			linePaint := graphics.DefaultPaint()
			linePaint.Color = graphics.RGBA(255, 255, 255, 0.5)
			ctx.Canvas.DrawRect(graphics.RectFromLTWH(graphLeft, targetY, graphWidth, 1), linePaint)
		}
	}
}

// HitTest returns false to allow taps to pass through to the app below.
func (r *renderDiagnosticsHUD) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}
