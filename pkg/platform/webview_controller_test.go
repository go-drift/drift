package platform

import "testing"

func TestWebViewController_Lifecycle(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	if c == nil {
		t.Fatal("expected non-nil controller")
	}
	if c.ViewID() == 0 {
		t.Error("expected non-zero ViewID")
	}

	c.Dispose()

	if c.ViewID() != 0 {
		t.Error("expected zero ViewID after Dispose")
	}
}

func TestWebViewController_ViewID(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	if c.ViewID() == 0 {
		t.Error("expected non-zero ViewID from controller")
	}
}

func TestWebViewController_Load(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	if err := c.Load("https://example.com"); err != nil {
		t.Errorf("Load: %v", err)
	}
}

func TestWebViewController_NavigationMethods(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	if err := c.GoBack(); err != nil {
		t.Errorf("GoBack: %v", err)
	}
	if err := c.GoForward(); err != nil {
		t.Errorf("GoForward: %v", err)
	}
	if err := c.Reload(); err != nil {
		t.Errorf("Reload: %v", err)
	}
}

// sendWebViewEvent simulates a native event arriving for a webview platform view.
func sendWebViewEvent(t *testing.T, method string, args map[string]any) {
	t.Helper()
	args["method"] = method
	data, err := DefaultCodec.Encode(args)
	if err != nil {
		t.Fatalf("encode event: %v", err)
	}
	if err := HandleEvent("drift/platform_views", data); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
}

func TestWebViewController_PageStartedCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	var gotURL string
	c.OnPageStarted = func(url string) {
		gotURL = url
	}

	sendWebViewEvent(t, "onPageStarted", map[string]any{
		"viewId": c.ViewID(),
		"url":    "https://example.com",
	})

	if gotURL != "https://example.com" {
		t.Errorf("OnPageStarted url: got %q, want %q", gotURL, "https://example.com")
	}
}

func TestWebViewController_PageFinishedCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	var gotURL string
	c.OnPageFinished = func(url string) {
		gotURL = url
	}

	sendWebViewEvent(t, "onPageFinished", map[string]any{
		"viewId": c.ViewID(),
		"url":    "https://example.com/page",
	})

	if gotURL != "https://example.com/page" {
		t.Errorf("OnPageFinished url: got %q, want %q", gotURL, "https://example.com/page")
	}
}

func TestWebViewController_ErrorCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	var gotCode, gotMessage string
	c.OnError = func(code, message string) {
		gotCode = code
		gotMessage = message
	}

	sendWebViewEvent(t, "onWebViewError", map[string]any{
		"viewId":  c.ViewID(),
		"code":    "network_error",
		"message": "net::ERR_NAME_NOT_RESOLVED",
	})

	if gotCode != "network_error" {
		t.Errorf("OnError code: got %q, want %q", gotCode, "network_error")
	}
	if gotMessage != "net::ERR_NAME_NOT_RESOLVED" {
		t.Errorf("OnError message: got %q, want %q", gotMessage, "net::ERR_NAME_NOT_RESOLVED")
	}
}

func TestWebViewController_NilCallbacksDoNotPanic(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	// No callbacks set; these should not panic.
	sendWebViewEvent(t, "onPageStarted", map[string]any{
		"viewId": c.ViewID(),
		"url":    "https://example.com",
	})
	sendWebViewEvent(t, "onPageFinished", map[string]any{
		"viewId": c.ViewID(),
		"url":    "https://example.com",
	})
	sendWebViewEvent(t, "onWebViewError", map[string]any{
		"viewId":  c.ViewID(),
		"code":    "load_failed",
		"message": "test error",
	})
}

func TestWebViewController_MethodsReturnErrDisposedAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	c.Dispose()

	// All methods should return ErrDisposed after Dispose.
	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"Load", func() error { return c.Load("https://example.com") }},
		{"GoBack", func() error { return c.GoBack() }},
		{"GoForward", func() error { return c.GoForward() }},
		{"Reload", func() error { return c.Reload() }},
	} {
		if err := tc.fn(); err != ErrDisposed {
			t.Errorf("%s after Dispose: got %v, want ErrDisposed", tc.name, err)
		}
	}
}
