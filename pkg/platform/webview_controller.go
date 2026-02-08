package platform

import (
	"fmt"
	"sync"

	"github.com/go-drift/drift/pkg/errors"
)

// WebViewController provides control over a native web browser view.
// The controller creates its platform view eagerly, so methods and callbacks
// work immediately after construction.
//
// Create with [NewWebViewController] and manage lifecycle with
// [core.UseController]:
//
//	s.web = core.UseController(&s.StateBase, platform.NewWebViewController)
//	s.web.OnPageFinished = func(url string) { ... }
//	s.web.Load("https://example.com")
//
// Pass the controller to a [widgets.NativeWebView] widget to embed the native
// surface in the widget tree.
//
// Set callback fields before calling [WebViewController.Load] to ensure
// no events are missed.
//
// All methods are safe for concurrent use.
type WebViewController struct {
	mu     sync.RWMutex
	view   *nativeWebView // guarded by mu
	viewID int64          // guarded by mu

	// OnPageStarted is called when a page starts loading.
	// Called on the UI thread.
	OnPageStarted func(url string)

	// OnPageFinished is called when a page finishes loading.
	// Called on the UI thread.
	OnPageFinished func(url string)

	// OnError is called when a loading error occurs.
	// The code parameter is one of [ErrCodeNetworkError], [ErrCodeSSLError],
	// or [ErrCodeLoadFailed]. Called on the UI thread.
	OnError func(code, message string)
}

// NewWebViewController creates a new web view controller.
// The underlying platform view is created eagerly so methods and callbacks
// work immediately.
func NewWebViewController() *WebViewController {
	c := &WebViewController{}

	view, err := GetPlatformViewRegistry().Create("native_webview", map[string]any{})
	if err != nil {
		errors.Report(&errors.DriftError{
			Op:  "NewWebViewController",
			Err: fmt.Errorf("failed to create webview: %w", err),
		})
		return c
	}

	webView, ok := view.(*nativeWebView)
	if !ok {
		errors.Report(&errors.DriftError{
			Op:  "NewWebViewController",
			Err: fmt.Errorf("unexpected view type: %T", view),
		})
		return c
	}

	c.view = webView
	c.viewID = webView.ViewID()

	// Wire view callbacks to controller callback fields.
	webView.OnPageStarted = func(url string) {
		if c.OnPageStarted != nil {
			c.OnPageStarted(url)
		}
	}
	webView.OnPageFinished = func(url string) {
		if c.OnPageFinished != nil {
			c.OnPageFinished(url)
		}
	}
	webView.OnError = func(code, message string) {
		if c.OnError != nil {
			c.OnError(code, message)
		}
	}

	return c
}

// ViewID returns the platform view ID, or 0 if the view was not created.
func (c *WebViewController) ViewID() int64 {
	c.mu.RLock()
	id := c.viewID
	c.mu.RUnlock()
	return id
}

// Load loads the specified URL.
func (c *WebViewController) Load(url string) error {
	c.mu.RLock()
	id := c.viewID
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := GetPlatformViewRegistry().InvokeViewMethod(id, "load", map[string]any{
		"url": url,
	})
	return err
}

// GoBack navigates back in history.
func (c *WebViewController) GoBack() error {
	c.mu.RLock()
	id := c.viewID
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := GetPlatformViewRegistry().InvokeViewMethod(id, "goBack", nil)
	return err
}

// GoForward navigates forward in history.
func (c *WebViewController) GoForward() error {
	c.mu.RLock()
	id := c.viewID
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := GetPlatformViewRegistry().InvokeViewMethod(id, "goForward", nil)
	return err
}

// Reload reloads the current page.
func (c *WebViewController) Reload() error {
	c.mu.RLock()
	id := c.viewID
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := GetPlatformViewRegistry().InvokeViewMethod(id, "reload", nil)
	return err
}

// Dispose releases the web view and its native resources. After disposal,
// this controller must not be reused. Dispose is idempotent; calling it more
// than once is safe.
func (c *WebViewController) Dispose() {
	c.mu.Lock()
	id := c.viewID
	c.view = nil
	c.viewID = 0
	c.mu.Unlock()
	if id != 0 {
		GetPlatformViewRegistry().Dispose(id)
	}
}
