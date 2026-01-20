package platform

import "sync"

// SafeArea provides safe area inset management.
var SafeArea = &SafeAreaService{
	events: NewEventChannel("drift/safe_area/events"),
	insets: EdgeInsets{},
}

// EdgeInsets represents the safe area insets from system UI elements.
type EdgeInsets struct {
	Top, Bottom, Left, Right float64
}

// SafeAreaService manages safe area inset events.
type SafeAreaService struct {
	events   *EventChannel
	insets   EdgeInsets
	handlers []func(EdgeInsets)
	mu       sync.RWMutex
}

func init() {
	SafeArea.events.Listen(EventHandler{
		OnEvent: func(data any) {
			if m, ok := data.(map[string]any); ok {
				insets := EdgeInsets{}
				if top, ok := m["top"].(float64); ok {
					insets.Top = top
				}
				if bottom, ok := m["bottom"].(float64); ok {
					insets.Bottom = bottom
				}
				if left, ok := m["left"].(float64); ok {
					insets.Left = left
				}
				if right, ok := m["right"].(float64); ok {
					insets.Right = right
				}
				SafeArea.updateInsets(insets)
			}
		},
	})
}

// Insets returns the current safe area insets.
func (s *SafeAreaService) Insets() EdgeInsets {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.insets
}

// AddHandler registers a handler to be called on inset changes.
// Returns a function that can be called to remove the handler.
func (s *SafeAreaService) AddHandler(handler func(EdgeInsets)) func() {
	s.mu.Lock()
	s.handlers = append(s.handlers, handler)
	index := len(s.handlers) - 1
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		if index < len(s.handlers) {
			s.handlers = append(s.handlers[:index], s.handlers[index+1:]...)
		}
		s.mu.Unlock()
	}
}

// updateInsets updates the safe area insets and notifies handlers.
func (s *SafeAreaService) updateInsets(newInsets EdgeInsets) {
	s.mu.Lock()
	if s.insets == newInsets {
		s.mu.Unlock()
		return
	}
	s.insets = newInsets
	handlers := make([]func(EdgeInsets), len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.Unlock()

	for _, h := range handlers {
		h(newInsets)
	}
}
