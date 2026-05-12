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

func TestAccessibility_HandleSetEnabled(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		// Reset the semantics binding's enabled state via the handler.
		_, err := Accessibility.handleSetEnabled(map[string]any{"enabled": true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !semantics.GetSemanticsBinding().IsEnabled() {
			t.Fatal("binding not enabled after handleSetEnabled(true)")
		}
		// Restore.
		Accessibility.handleSetEnabled(map[string]any{"enabled": false})
	})

	t.Run("args not map", func(t *testing.T) {
		h := captureErrorReports(t)
		_, err := Accessibility.handleSetEnabled("garbage")
		if err == nil {
			t.Fatal("expected error")
		}
		assertAccessibilityArgReport(t, h, "accessibility.handleSetEnabled", accessibilityMethodChannel)
	})

	t.Run("missing enabled", func(t *testing.T) {
		h := captureErrorReports(t)
		_, err := Accessibility.handleSetEnabled(map[string]any{})
		if err == nil {
			t.Fatal("expected error")
		}
		assertAccessibilityArgReport(t, h, "accessibility.handleSetEnabled", accessibilityMethodChannel)
	})

	t.Run("wrong-type enabled", func(t *testing.T) {
		h := captureErrorReports(t)
		_, err := Accessibility.handleSetEnabled(map[string]any{"enabled": "true"})
		if err == nil {
			t.Fatal("expected error")
		}
		assertAccessibilityArgReport(t, h, "accessibility.handleSetEnabled", accessibilityMethodChannel)
	})
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
