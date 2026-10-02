package platform

import "testing"

// A tap that launched the app arrives before the app listens; it is
// delivered to the first Opens listener, once.
func TestNotificationOpensQueueUntilListened(t *testing.T) {
	t.Cleanup(ResetForTest)
	encoded, err := DefaultCodec.Encode(map[string]any{
		"id": "n1", "action": "tap", "data": map[string]any{"route": "/inbox"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleEvent("drift/notifications/opened", encoded); err != nil {
		t.Fatal(err)
	}

	var got []NotificationOpen
	unsub := Notifications.Opens().Listen(func(o NotificationOpen) { got = append(got, o) })
	defer unsub()
	if len(got) != 1 || got[0].ID != "n1" || got[0].Action != "tap" || got[0].Data["route"] != "/inbox" {
		t.Fatalf("first listener got %+v, want the queued tap n1", got)
	}

	var later []NotificationOpen
	unsubLater := Notifications.Opens().Listen(func(o NotificationOpen) { later = append(later, o) })
	defer unsubLater()
	if len(later) != 0 {
		t.Errorf("later listener got %+v, want nothing (queued taps are delivered once)", later)
	}
}
