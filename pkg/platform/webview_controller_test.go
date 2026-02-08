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

func TestWebViewController_LoadURL(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	defer c.Dispose()

	if err := c.LoadURL("https://example.com"); err != nil {
		t.Errorf("LoadURL: %v", err)
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

	var gotErr string
	c.OnError = func(errMsg string) {
		gotErr = errMsg
	}

	sendWebViewEvent(t, "onWebViewError", map[string]any{
		"viewId": c.ViewID(),
		"error":  "net::ERR_NAME_NOT_RESOLVED",
	})

	if gotErr != "net::ERR_NAME_NOT_RESOLVED" {
		t.Errorf("OnError: got %q, want %q", gotErr, "net::ERR_NAME_NOT_RESOLVED")
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
		"viewId": c.ViewID(),
		"error":  "test error",
	})
}

func TestWebViewController_MethodsNoOpAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewWebViewController()
	c.Dispose()

	// All methods should be no-ops after Dispose (viewID is 0).
	if err := c.LoadURL("https://example.com"); err != nil {
		t.Errorf("LoadURL after Dispose: %v", err)
	}
	if err := c.GoBack(); err != nil {
		t.Errorf("GoBack after Dispose: %v", err)
	}
	if err := c.GoForward(); err != nil {
		t.Errorf("GoForward after Dispose: %v", err)
	}
	if err := c.Reload(); err != nil {
		t.Errorf("Reload after Dispose: %v", err)
	}
}
