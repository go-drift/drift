package platform

// NotificationsService holds the notification permission. Drift provides
// it because the plugins that show notifications share it: local
// notifications (plugins/notifications) and push (plugins/firebase). On
// Android those plugins declare POST_NOTIFICATIONS; with neither in
// drift.yaml, its calls fail with ErrPermissionNotDeclared.
type NotificationsService struct {
	// Permission for showing notifications. Implements
	// NotificationPermission for iOS-specific options.
	Permission NotificationPermission
}

// Notifications is the singleton notifications service.
var Notifications = &NotificationsService{Permission: newNotificationPerm()}
