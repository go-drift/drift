// Package engine provides the core rendering engine for the Drift framework.
//
// It manages the render pipeline: widget tree layout, painting onto
// Skia-backed surfaces, and frame scheduling for native embedders.
//
// # Embedder vs App-Facing Types
//
// This package exports types for two distinct audiences:
//
//   - Embedder types: [PointerEvent] and [PointerPhase] are raw device-pixel
//     input types used by platform embedders to deliver native pointer events.
//     These are intentionally separate from [gestures.PointerEvent], which uses
//     logical coordinates and includes delta tracking for gesture recognizers.
//     Conversion between the two happens in [HandlePointerEvent], which applies
//     device pixel ratio scaling and computes deltas.
//
//   - App-facing types: [DiagnosticsConfig], [DiagnosticsPosition], and
//     [DefaultDiagnosticsConfig] are used by applications to configure the
//     diagnostics overlay HUD. [SetBackgroundColor] is called by app code
//     to set the root background color.
package engine
