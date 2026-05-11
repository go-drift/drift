package platform

import (
	"context"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

// DeepLink describes an incoming deep link.
type DeepLink struct {
	URL       string
	Source    string
	Timestamp time.Time
}

// DeepLinkService provides deep link event access.
type DeepLinkService struct {
	state *deepLinkServiceState
	links *Stream[DeepLink]
}

// DeepLinks is the singleton deep link service.
var DeepLinks *DeepLinkService

func init() {
	state := newDeepLinkService()
	DeepLinks = &DeepLinkService{
		state: state,
		links: NewStream("drift/deeplinks/events", state.events, parseDeepLinkWithError),
	}
}

type deepLinkServiceState struct {
	channel *MethodChannel
	events  *EventChannel
}

func newDeepLinkService() *deepLinkServiceState {
	return &deepLinkServiceState{
		channel: NewMethodChannel("drift/deeplinks"),
		events:  NewEventChannel("drift/deeplinks/events"),
	}
}

// GetInitial returns the launch deep link, if available.
func (d *DeepLinkService) GetInitial() (*DeepLink, error) {
	result, err := d.state.channel.Invoke(context.Background(), "getInitial", nil)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	link, err := parseDeepLinkWithError(result)
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// Links returns a stream of deep link events.
func (d *DeepLinkService) Links() *Stream[DeepLink] {
	return d.links
}

func parseDeepLinkWithError(data any) (DeepLink, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return DeepLink{}, &errors.ParseError{
			Channel:  "drift/deeplinks/events",
			DataType: "DeepLink",
			Got:      data,
		}
	}
	url := parseString(m["url"])
	if url == "" {
		return DeepLink{}, &errors.ParseError{
			Channel:  "drift/deeplinks/events",
			DataType: "DeepLink",
			Got:      data,
		}
	}
	return DeepLink{
		URL:       url,
		Source:    parseString(m["source"]),
		Timestamp: parseTime(m["timestamp"]),
	}, nil
}
