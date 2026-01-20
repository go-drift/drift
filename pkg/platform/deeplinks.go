package platform

import "time"

var deepLinkService = newDeepLinkService()

// DeepLink describes an incoming deep link.
type DeepLink struct {
	URL       string
	Source    string
	Timestamp time.Time
}

// GetInitialDeepLink returns the launch deep link, if available.
func GetInitialDeepLink() (*DeepLink, error) {
	return deepLinkService.getInitial()
}

// DeepLinks streams deep link events while the app is running.
func DeepLinks() <-chan DeepLink {
	return deepLinkService.linkChannel()
}

type deepLinkServiceState struct {
	channel *MethodChannel
	events  *EventChannel
	linksCh chan DeepLink
}

func newDeepLinkService() *deepLinkServiceState {
	service := &deepLinkServiceState{
		channel: NewMethodChannel("github.com/go-drift/drift/deeplinks"),
		events:  NewEventChannel("github.com/go-drift/drift/deeplinks/events"),
		linksCh: make(chan DeepLink, 4),
	}

	service.events.Listen(EventHandler{OnEvent: func(data any) {
		if link, ok := parseDeepLink(data); ok {
			service.linksCh <- link
		}
	}})

	return service
}

func (s *deepLinkServiceState) getInitial() (*DeepLink, error) {
	result, err := s.channel.Invoke("getInitial", nil)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	link, ok := parseDeepLink(result)
	if !ok {
		return nil, ErrInvalidArguments
	}
	return &link, nil
}

func (s *deepLinkServiceState) linkChannel() <-chan DeepLink {
	return s.linksCh
}

func parseDeepLink(data any) (DeepLink, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return DeepLink{}, false
	}
	url := parseDeepLinkString(m["url"])
	if url == "" {
		return DeepLink{}, false
	}
	return DeepLink{
		URL:       url,
		Source:    parseDeepLinkString(m["source"]),
		Timestamp: parseDeepLinkTime(m["timestamp"]),
	}, true
}

func parseDeepLinkString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func parseDeepLinkTime(value any) time.Time {
	var millis int64
	switch v := value.(type) {
	case int64:
		millis = v
	case int:
		millis = int64(v)
	case int32:
		millis = int64(v)
	case float64:
		millis = int64(v)
	case float32:
		millis = int64(v)
	case uint64:
		millis = int64(v)
	case uint32:
		millis = int64(v)
	case uint:
		millis = int64(v)
	case int16:
		millis = int64(v)
	case int8:
		millis = int64(v)
	case uint16:
		millis = int64(v)
	case uint8:
		millis = int64(v)
	default:
		return time.Time{}
	}
	return time.UnixMilli(millis)
}
