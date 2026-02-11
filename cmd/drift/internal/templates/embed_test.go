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

	addSubviewIdx := strings.Index(src, "host.addSubview(interceptor)")
	if addSubviewIdx == -1 {
		t.Fatal("expected host.addSubview(interceptor) in ios/PlatformView.swift")
	}

	onCreatedIdx := strings.Index(src, `"method": "onViewCreated"`)
	if onCreatedIdx == -1 {
		t.Fatal(`expected "method": "onViewCreated" in ios/PlatformView.swift`)
	}

	if onCreatedIdx < addSubviewIdx {
		t.Fatalf("onViewCreated appears before interceptor attachment (onViewCreated=%d, addSubview=%d)", onCreatedIdx, addSubviewIdx)
	}
}
