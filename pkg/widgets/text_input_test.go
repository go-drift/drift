package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

func TestBuildPlatformViewConfig_IdenticalWidgets(t *testing.T) {
	// DidUpdateWidget compares buildPlatformViewConfig(old) != buildPlatformViewConfig(new)
	// to skip redundant config updates. Identical widgets must produce equal configs.
	s := &textInputState{}
	w := TextInput{
		Obscure:          true,
		Style:            graphics.TextStyle{FontSize: 16, Color: graphics.RGB(0, 0, 0)},
		Placeholder:      "Password",
		PlaceholderColor: graphics.RGB(150, 150, 150),
		Padding:          layout.EdgeInsetsSymmetric(12, 8),
		KeyboardType:     platform.KeyboardTypePassword,
		InputAction:      platform.TextInputActionDone,
	}

	a := s.buildPlatformViewConfig(w)
	b := s.buildPlatformViewConfig(w)

	if a != b {
		t.Error("same widget should produce equal configs")
	}
}

func TestBuildPlatformViewConfig_DifferentObscure(t *testing.T) {
	// Password vs non-password fields differ in Obscure. The config comparison
	// must detect this so that config updates are sent when Obscure changes.
	s := &textInputState{}
	a := TextInput{Obscure: false, Style: graphics.TextStyle{FontSize: 16}}
	b := TextInput{Obscure: true, Style: graphics.TextStyle{FontSize: 16}}

	if s.buildPlatformViewConfig(a) == s.buildPlatformViewConfig(b) {
		t.Error("different Obscure values should produce different configs")
	}
}

func TestBuildPlatformViewConfig_DifferentStyle(t *testing.T) {
	s := &textInputState{}
	a := TextInput{Style: graphics.TextStyle{FontSize: 14, Color: graphics.RGB(0, 0, 0)}}
	b := TextInput{Style: graphics.TextStyle{FontSize: 16, Color: graphics.RGB(0, 0, 0)}}

	if s.buildPlatformViewConfig(a) == s.buildPlatformViewConfig(b) {
		t.Error("different font sizes should produce different configs")
	}
}

func TestBuildPlatformViewConfig_DifferentPadding(t *testing.T) {
	s := &textInputState{}
	a := TextInput{Padding: layout.EdgeInsetsAll(8)}
	b := TextInput{Padding: layout.EdgeInsetsAll(12)}

	if s.buildPlatformViewConfig(a) == s.buildPlatformViewConfig(b) {
		t.Error("different padding should produce different configs")
	}
}

func TestBuildPlatformViewConfig_ConfigUnchangedOnRebuild(t *testing.T) {
	// Simulates the scenario that causes the bug: a form rebuild creates
	// new TextInput widgets with identical config. The comparison must
	// return equal so DidUpdateWidget skips the config update, avoiding
	// a redundant setInputType that disrupts password field cursor position.
	s := &textInputState{}

	// Two independent widget instances with the same config (as would happen
	// when Form.generation bumps and the entire form rebuilds).
	old := TextInput{
		Obscure:      true,
		Autocorrect:  false,
		Style:        graphics.TextStyle{FontSize: 16, FontFamily: "Roboto", Color: graphics.RGB(0, 0, 0)},
		Placeholder:  "Enter password",
		Padding:      layout.EdgeInsetsSymmetric(12, 8),
		KeyboardType: platform.KeyboardTypePassword,
		InputAction:  platform.TextInputActionDone,
	}
	new := TextInput{
		Obscure:      true,
		Autocorrect:  false,
		Style:        graphics.TextStyle{FontSize: 16, FontFamily: "Roboto", Color: graphics.RGB(0, 0, 0)},
		Placeholder:  "Enter password",
		Padding:      layout.EdgeInsetsSymmetric(12, 8),
		KeyboardType: platform.KeyboardTypePassword,
		InputAction:  platform.TextInputActionDone,
	}

	oldConfig := s.buildPlatformViewConfig(old)
	newConfig := s.buildPlatformViewConfig(new)

	if oldConfig != newConfig {
		t.Error("rebuild with identical config should not trigger config update")
	}
}
