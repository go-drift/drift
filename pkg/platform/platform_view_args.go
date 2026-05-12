package platform

import (
	"errors"
	"fmt"

	drifterrors "github.com/go-drift/drift/pkg/errors"
)

const platformViewsChannel = "drift/platform_views"

var (
	errViewNotFound     = errors.New("platform view not found")
	errViewTypeMismatch = errors.New("platform view type mismatch")
)

// lookupView resolves a viewID to a typed PlatformView.
// errViewNotFound is tolerable (a native event may arrive after the Go side
// has disposed the view); errViewTypeMismatch is a wire-contract bug worth
// surfacing.
func lookupView[T PlatformView](r *PlatformViewRegistry, viewID int64) (T, error) {
	r.mu.RLock()
	v := r.views[viewID]
	r.mu.RUnlock()
	var zero T
	if v == nil {
		return zero, fmt.Errorf("viewID=%d: %w", viewID, errViewNotFound)
	}
	typed, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("viewID=%d got %T: %w", viewID, v, errViewTypeMismatch)
	}
	return typed, nil
}

// reportPlatformViewArg routes a parse error through the project-wide
// errors.Report pipeline tagged with the platform_views channel, then returns
// the error unchanged so handlers can `return nil, reportPlatformViewArg(...)`
// in one line.
func reportPlatformViewArg(op string, err error) error {
	drifterrors.Report(&drifterrors.DriftError{
		Op:      "platform_view." + op,
		Kind:    drifterrors.KindParsing,
		Channel: platformViewsChannel,
		Err:     err,
	})
	return err
}
