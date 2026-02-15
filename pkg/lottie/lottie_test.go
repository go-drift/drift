package lottie

import (
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

func TestLoadBytes_ReturnsError(t *testing.T) {
	_, err := LoadBytes([]byte(`{"v":"5.7.1"}`))
	if err == nil {
		t.Fatal("expected error from LoadBytes")
	}
}

func TestLoad_ReturnsError(t *testing.T) {
	_, err := Load(strings.NewReader(`{}`))
	if err == nil {
		t.Fatal("expected error from Load")
	}
}

func TestLoadFile_ReturnsError(t *testing.T) {
	_, err := LoadFile("nonexistent.json")
	if err == nil {
		t.Fatal("expected error from LoadFile")
	}
}

func TestNilAnimation_Duration(t *testing.T) {
	var a *Animation
	if d := a.Duration(); d != 0 {
		t.Fatalf("expected zero duration for nil animation, got %v", d)
	}
}

func TestNilAnimation_Size(t *testing.T) {
	var a *Animation
	s := a.Size()
	if s != (graphics.Size{}) {
		t.Fatalf("expected zero size for nil animation, got %v", s)
	}
}

func TestNilAnimation_Destroy(t *testing.T) {
	var a *Animation
	a.Destroy() // should not panic
}
