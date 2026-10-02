// Package runtime is the Go API of the Drift permissions plugin
// (github.com/go-drift/drift/plugins/permissions/plugin in drift.yaml):
// permissions no other plugin provides, for apps whose own native code or
// SDKs use them.
//
//	import permissions "github.com/go-drift/drift/plugins/permissions/runtime"
//
//	go func() {
//	    status, err := permissions.Contacts.Request(ctx)
//	    ...
//	}()
//
// Each permission works only when its section is in the plugin's config;
// otherwise its calls fail with platform.ErrPermissionNotRegistered.
// Request waits for the user, so call it from a goroutine.
package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-drift/drift/pkg/platform"
)

var (
	// Contacts is read access to the user's contacts.
	Contacts platform.Permission = configured{platform.NewPermission("contacts"), "contacts"}
	// Calendar is read access to the user's calendar events (full access
	// on iOS 17+; write-only access reads as PermissionLimited).
	Calendar platform.Permission = configured{platform.NewPermission("calendar"), "calendar"}
	// Microphone is microphone access.
	Microphone platform.Permission = configured{platform.NewPermission("microphone"), "microphone"}
	// Photos is read access to the photo library (selected photos on iOS
	// read as PermissionLimited).
	Photos platform.Permission = configured{platform.NewPermission("photos"), "photos"}
)

// configured names the config section a permission needs when native code
// did not register it.
type configured struct {
	platform.Permission
	section string
}

func (p configured) Status() (platform.PermissionResult, error) {
	status, err := p.Permission.Status()
	return status, p.explain(err)
}

func (p configured) Request(ctx context.Context) (platform.PermissionResult, error) {
	status, err := p.Permission.Request(ctx)
	return status, p.explain(err)
}

func (p configured) ShouldShowRationale() (bool, error) {
	show, err := p.Permission.ShouldShowRationale()
	return show, p.explain(err)
}

func (p configured) explain(err error) error {
	if errors.Is(err, platform.ErrPermissionNotRegistered) {
		return fmt.Errorf("%s permission needs a %s: section in the permissions plugin config: %w", p.section, p.section, err)
	}
	return err
}
