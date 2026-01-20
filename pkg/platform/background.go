package platform

import "time"

var backgroundService = newBackgroundService()

// TaskType defines the type of background task.
type TaskType string

const (
	TaskTypeOneTime  TaskType = "one_time"
	TaskTypePeriodic TaskType = "periodic"
	TaskTypeFetch    TaskType = "fetch"
)

// TaskConstraints defines constraints for when a task should run.
type TaskConstraints struct {
	RequiresNetwork        bool
	RequiresUnmeteredNetwork bool
	RequiresCharging       bool
	RequiresIdle           bool
	RequiresStorageNotLow  bool
	RequiresBatteryNotLow  bool
}

// TaskRequest describes a background task to schedule.
type TaskRequest struct {
	ID             string
	TaskType       TaskType
	Tag            string
	Constraints    TaskConstraints
	InitialDelay   time.Duration
	RepeatInterval time.Duration
	Data           map[string]any
}

// BackgroundEvent represents a background task event.
type BackgroundEvent struct {
	TaskID    string
	EventType string
	Data      map[string]any
	Timestamp time.Time
}

// ScheduleBackgroundTask schedules a background task.
func ScheduleBackgroundTask(request TaskRequest) error {
	return backgroundService.scheduleTask(request)
}

// CancelBackgroundTask cancels a scheduled background task.
func CancelBackgroundTask(id string) error {
	return backgroundService.cancelTask(id)
}

// CancelAllBackgroundTasks cancels all scheduled background tasks.
func CancelAllBackgroundTasks() error {
	return backgroundService.cancelAllTasks()
}

// CancelBackgroundTasksByTag cancels all tasks with the given tag.
func CancelBackgroundTasksByTag(tag string) error {
	return backgroundService.cancelTasksByTag(tag)
}

// BackgroundTaskEvents returns a channel that receives task events.
func BackgroundTaskEvents() <-chan BackgroundEvent {
	return backgroundService.eventChannel()
}

// CompleteBackgroundTask signals completion of a background task.
func CompleteBackgroundTask(id string, success bool) error {
	return backgroundService.completeTask(id, success)
}

// IsBackgroundRefreshAvailable checks if background refresh is available.
func IsBackgroundRefreshAvailable() (bool, error) {
	return backgroundService.isBackgroundRefreshAvailable()
}

type backgroundServiceState struct {
	channel *MethodChannel
	events  *EventChannel
	eventCh chan BackgroundEvent
}

func newBackgroundService() *backgroundServiceState {
	service := &backgroundServiceState{
		channel: NewMethodChannel("drift/background"),
		events:  NewEventChannel("drift/background/events"),
		eventCh: make(chan BackgroundEvent, 4),
	}

	service.events.Listen(EventHandler{OnEvent: func(data any) {
		if event, ok := parseBackgroundEvent(data); ok {
			service.eventCh <- event
		}
	}})

	return service
}

func (s *backgroundServiceState) scheduleTask(request TaskRequest) error {
	taskType := string(request.TaskType)
	if taskType == "" {
		taskType = string(TaskTypeOneTime)
	}

	_, err := s.channel.Invoke("scheduleTask", map[string]any{
		"id":              request.ID,
		"taskType":        taskType,
		"tag":             request.Tag,
		"initialDelayMs":  request.InitialDelay.Milliseconds(),
		"repeatIntervalMs": request.RepeatInterval.Milliseconds(),
		"data":            request.Data,
		"constraints": map[string]any{
			"requiresNetwork":          request.Constraints.RequiresNetwork,
			"requiresUnmeteredNetwork": request.Constraints.RequiresUnmeteredNetwork,
			"requiresCharging":         request.Constraints.RequiresCharging,
			"requiresIdle":             request.Constraints.RequiresIdle,
			"requiresStorageNotLow":    request.Constraints.RequiresStorageNotLow,
			"requiresBatteryNotLow":    request.Constraints.RequiresBatteryNotLow,
		},
	})
	return err
}

func (s *backgroundServiceState) cancelTask(id string) error {
	_, err := s.channel.Invoke("cancelTask", map[string]any{
		"id": id,
	})
	return err
}

func (s *backgroundServiceState) cancelAllTasks() error {
	_, err := s.channel.Invoke("cancelAllTasks", nil)
	return err
}

func (s *backgroundServiceState) cancelTasksByTag(tag string) error {
	_, err := s.channel.Invoke("cancelTasksByTag", map[string]any{
		"tag": tag,
	})
	return err
}

func (s *backgroundServiceState) completeTask(id string, success bool) error {
	_, err := s.channel.Invoke("completeTask", map[string]any{
		"id":      id,
		"success": success,
	})
	return err
}

func (s *backgroundServiceState) isBackgroundRefreshAvailable() (bool, error) {
	result, err := s.channel.Invoke("isBackgroundRefreshAvailable", nil)
	if err != nil {
		return false, err
	}
	if m, ok := result.(map[string]any); ok {
		return parseBool(m["available"]), nil
	}
	return false, nil
}

func (s *backgroundServiceState) eventChannel() <-chan BackgroundEvent {
	return s.eventCh
}

func parseBackgroundEvent(data any) (BackgroundEvent, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return BackgroundEvent{}, false
	}
	return BackgroundEvent{
		TaskID:    parseString(m["taskId"]),
		EventType: parseString(m["eventType"]),
		Data:      parseMap(m["data"]),
		Timestamp: parseTime(m["timestamp"]),
	}, true
}
