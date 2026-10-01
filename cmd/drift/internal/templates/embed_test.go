package templates

import (
	"strings"
	"testing"
)

func TestIOSPlatformView_OnViewCreatedSentAfterInterceptorAttach(t *testing.T) {
	content, err := ReadFile("ios/PlatformView.swift")
	if err != nil {
		t.Fatalf("ReadFile(ios/PlatformView.swift) failed: %v", err)
	}

	src := string(content)

	attachIdx := strings.Index(src, "host.insertSubview(interceptor, belowSubview: overlay)")
	if attachIdx == -1 {
		t.Fatal("expected host.insertSubview(interceptor, belowSubview: overlay) in ios/PlatformView.swift")
	}

	onCreatedIdx := strings.Index(src, `"method": "onViewCreated"`)
	if onCreatedIdx == -1 {
		t.Fatal(`expected "method": "onViewCreated" in ios/PlatformView.swift`)
	}

	if onCreatedIdx < attachIdx {
		t.Fatalf("onViewCreated appears before interceptor attachment (onViewCreated=%d, attach=%d)", onCreatedIdx, attachIdx)
	}
}
