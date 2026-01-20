package platform

// MethodCall represents a method invocation from Go to native or vice versa.
type MethodCall struct {
	// ID is a unique identifier for correlating responses with requests.
	ID int64 `json:"id"`

	// Method is the name of the method to invoke.
	Method string `json:"method"`

	// Args contains the method arguments.
	Args any `json:"args,omitempty"`
}

// MethodResponse represents the result of a method call.
type MethodResponse struct {
	// ID matches the MethodCall.ID this is responding to.
	ID int64 `json:"id"`

	// Result contains the successful result (nil if error).
	Result any `json:"result,omitempty"`

	// Error contains error details if the call failed.
	Error *ChannelError `json:"error,omitempty"`
}

// IsError returns true if this response represents an error.
func (r *MethodResponse) IsError() bool {
	return r.Error != nil
}

// Event represents an event sent from native to Go via an EventChannel.
type Event struct {
	// Name identifies the event type.
	Name string `json:"event"`

	// Data contains the event payload.
	Data any `json:"data,omitempty"`
}

// EventError represents an error that occurred in an event stream.
type EventError struct {
	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable error description.
	Message string `json:"message"`

	// Details contains additional error information.
	Details any `json:"details,omitempty"`
}

// EndOfStream is sent when an event stream terminates normally.
type EndOfStream struct{}
