//go:build android || darwin || ios

package platform

import (
	"errors"
	"testing"

	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/semantics"
)

// Note: tests in this file rely on the global Accessibility.method (etc.) being
// initialized at package init. They exercise the parsing/reporting paths only.
// Cross-test interference is avoided by serial execution (no t.Parallel) and
// by snapshot/restore of the errors.SetHandler in captureErrorReports.

func TestAccessibility_HandleStateEvent(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		Accessibility.handleStateEvent(map[string]any{"enabled": true})
		if !semantics.GetSemanticsBinding().IsEnabled() {
			t.Fatal("binding not enabled after a state event with enabled=true")
		}
		Accessibility.handleStateEvent(map[string]any{"enabled": false})
		if semantics.GetSemanticsBinding().IsEnabled() {
			t.Fatal("binding still enabled after a state event with enabled=false")
		}
	})

	for name, data := range map[string]any{
		"args not map":       "garbage",
		"missing enabled":    map[string]any{},
		"wrong-type enabled": map[string]any{"enabled": "true"},
	} {
		t.Run(name, func(t *testing.T) {
			h := captureErrorReports(t)
			Accessibility.handleStateEvent(data)
			assertAccessibilityArgReport(t, h, "accessibility.stateEvent", accessibilityStateChannel)
		})
	}
}

// assertAccessibilityArgReport mirrors platform_view's assertArgErrorReport
// but does not require the inner error to be *argError — type-mismatch
// reports from lookupView wrap sentinels rather than *argError. For
// accessibility we only emit *argError today, so we still check that.
func assertAccessibilityArgReport(t *testing.T, h *capturingHandler, op, channel string) {
	t.Helper()
	for _, e := range h.snapshot() {
		if e.Op != op || e.Channel != channel || e.Kind != drifterrors.KindParsing {
			continue
		}
		var ae *argError
		if errors.As(e.Err, &ae) {
			return
		}
	}
	t.Fatalf("no matching argError report for op=%s channel=%s; got: %+v", op, channel, h.snapshot())
}
