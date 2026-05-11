package platform

// MicrophoneService provides microphone access.
type MicrophoneService struct {
	// Permission for microphone access.
	Permission Permission
}

// Microphone is the singleton microphone service.
var Microphone = &MicrophoneService{
	Permission: newPerm("microphone"),
}
