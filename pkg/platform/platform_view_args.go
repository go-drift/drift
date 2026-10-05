package platform

import (
	drifterrors "github.com/go-drift/drift/pkg/errors"
)

const platformViewsChannel = "drift/platform_views"

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
