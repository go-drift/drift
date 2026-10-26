package platform

import (
	"errors"
	"strings"
	"testing"
)

func TestWebViewController_Lifecycle(t *testing.T) {
	setupTestBridge(t)

	c, err := NewWebViewController()
	if err != nil {
		t.Fatal(err)
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

	c := newWebView(t)

	if c.ViewID() == 0 {
		t.Error("expected non-zero ViewID from controller")
	}
}

func TestWebViewController_Load(t *testing.T) {
	setupTestBridge(t)

	c := newWebView(t)

	if err := c.Load("https://example.com"); err != nil {
		t.Errorf("Load: %v", err)
	}
}

func TestWebViewController_NavigationMethods(t *testing.T) {
	setupTestBridge(t)

	c := newWebView(t)

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

// newWebView creates a controller, failing the test if it cannot.
func newWebView(t *testing.T) *WebViewController {
	t.Helper()
	c, err := NewWebViewController()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Dispose)
	return c
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

// Each load sets Loading, then clears it when the page finishes.
func TestWebViewController_Page(t *testing.T) {
	setupTestBridge(t)
	c := newWebView(t)

	if got := c.Page().Value(); got != (WebPage{}) {
		t.Errorf("before any load: page = %+v", got)
	}
	var seen []WebPage
	c.Page().AddListener(func() { seen = append(seen, c.Page().Value()) })
	sendWebViewEvent(t, "onPageStarted", map[string]any{"viewId": c.ViewID(), "url": "https://example.com"})
	sendWebViewEvent(t, "onPageFinished", map[string]any{"viewId": c.ViewID(), "url": "https://example.com/"})

	want := []WebPage{{URL: "https://example.com", Loading: true}, {URL: "https://example.com/"}}
	if len(seen) != len(want) || seen[0] != want[0] || seen[1] != want[1] {
		t.Errorf("pages = %+v, want %+v", seen, want)
	}
}

// Load failures stream as errors matching the sentinels, with the
// platform's message.
func TestWebViewController_Errors(t *testing.T) {
	setupTestBridge(t)
	c := newWebView(t)

	var got []error
	c.Errors().Listen(StreamHandler[error]{OnEvent: func(err error) { got = append(got, err) }})
	for _, code := range []string{"network_error", "ssl_error", "load_failed"} {
		sendWebViewEvent(t, "onWebViewError", map[string]any{"viewId": c.ViewID(), "code": code, "message": "net::" + code})
	}

	want := []error{ErrWebViewNetwork, ErrWebViewSSL, ErrWebViewLoadFailed}
	if len(got) != len(want) {
		t.Fatalf("errors = %v", got)
	}
	for i := range want {
		if !errors.Is(got[i], want[i]) || !strings.Contains(got[i].Error(), "net::") {
			t.Errorf("error %d = %v, want %v with the native message", i, got[i], want[i])
		}
	}
}

// A native web view that cannot be created fails the constructor.
func TestNewWebViewController_Fails(t *testing.T) {
	setupTestBridge(t)
	SetNativeBridge(failingBridge{NewChannelError("native_error", "no WebView provider")})
	c, err := NewWebViewController()
	if c != nil || err == nil || !strings.Contains(err.Error(), "no WebView provider") {
		t.Errorf("NewWebViewController = %v, %v; want the native error", c, err)
	}
}

func TestWebViewController_MethodsReturnErrDisposedAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := newWebView(t)
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
