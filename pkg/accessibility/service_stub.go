//go:build !android && !darwin && !ios
// +build !android,!darwin,!ios

package accessibility

import (
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// Service is a no-op on non-mobile platforms.
type Service struct{}

// NewService creates a new accessibility service (no-op on non-mobile).
func NewService() *Service {
	return &Service{}
}

// Initialize is a no-op on non-mobile platforms.
func (s *Service) Initialize() {}

// SetDeviceScale is a no-op on non-mobile platforms.
func (s *Service) SetDeviceScale(scale float64) {}

// IsEnabled always returns false on non-mobile platforms.
func (s *Service) IsEnabled() bool {
	return false
}

// SetEnabled is a no-op on non-mobile platforms.
func (s *Service) SetEnabled(enabled bool) {}

// FlushSemantics is a no-op on non-mobile platforms.
func (s *Service) FlushSemantics(rootRender layout.RenderObject) {}

// Owner returns nil on non-mobile platforms.
func (s *Service) Owner() *semantics.SemanticsOwner {
	return nil
}

// Binding returns nil on non-mobile platforms.
func (s *Service) Binding() *semantics.SemanticsBinding {
	return nil
}
