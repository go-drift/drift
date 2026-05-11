package platform

// CalendarService provides calendar access.
type CalendarService struct {
	// Permission for calendar access.
	Permission Permission
}

// Calendar is the singleton calendar service.
var Calendar = &CalendarService{
	Permission: newPerm("calendar"),
}
