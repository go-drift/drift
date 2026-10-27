package plugin

import (
	"slices"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func TestBuild(t *testing.T) {
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/notifications/plugin", Plugin).Build(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	ops := ctx.Ops()
	has := func(match func(protocol.Op) bool) bool { return slices.ContainsFunc(ops, match) }

	if !has(func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpAndroidManifestAddPermission)
		return ok && o.Name == "android.permission.POST_NOTIFICATIONS"
	}) {
		t.Error("POST_NOTIFICATIONS not declared")
	}
	// Without it canScheduleExactAlarms is always false and the app never
	// appears in Settings, Alarms & reminders.
	if !has(func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpAndroidManifestAddPermission)
		return ok && o.Name == "android.permission.SCHEDULE_EXACT_ALARM"
	}) {
		t.Error("SCHEDULE_EXACT_ALARM not declared")
	}
	if !has(func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpAndroidManifestAddComponent)
		return ok && o.Tag() == "receiver" && o.Name() == androidPackage+".DriftNotificationReceiver"
	}) {
		t.Error("alarm receiver not declared")
	}
	for _, class := range []string{"DriftNotificationsPlugin"} {
		if !has(func(op protocol.Op) bool { o, ok := op.(*protocol.OpIOSPlugin); return ok && o.Class == class }) {
			t.Errorf("iOS plugin class %s not registered", class)
		}
	}
	if !has(func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpAndroidPlugin)
		return ok && o.Class == androidPackage+".DriftNotificationsPlugin"
	}) {
		t.Error("Android plugin class not registered")
	}
}
