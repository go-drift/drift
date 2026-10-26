package platform

import (
	"fmt"
	"sync"

	"github.com/go-drift/drift/pkg/core"
)

// WebPage is what a web view shows.
type WebPage struct {
	// URL is the page's URL, "" before the first load.
	URL string
	// Loading reports whether the page is still loading.
	Loading bool
}

// WebViewController provides control over a native web browser view.
// The controller creates its platform view eagerly, so its methods work
// right after construction.
//
// Create with [NewWebViewController] and manage its lifetime with
// [core.UseDisposable]:
//
//	s.web, err = platform.NewWebViewController()
//	...
//	core.UseDisposable(&s.StateBase, s.web)
//	core.UseListenable(&s.StateBase, s.web.Page())
//	s.web.Load("https://example.com")
//
// Pass the controller to a [widgets.NativeWebView] widget to embed the native
// surface in the widget tree. The controller publishes the page as state,
// [WebViewController.Page], and load failures as the
// [WebViewController.Errors] stream; subscribe to Errors before Load so no
// failure is missed.
//
// All methods are safe for concurrent use.
type WebViewController struct {
	mu     sync.RWMutex
	viewID int64 // guarded by mu

	page     *core.Signal[WebPage]
	readPage *core.Derived[WebPage]
	errors   *Emitter[error]
}

// NewWebViewController creates a web view controller and its native web
// view. It fails if the native view cannot be created.
func NewWebViewController() (*WebViewController, error) {
	page := core.NewSignal(WebPage{})
	c := &WebViewController{
		page:     page,
		readPage: core.NewDerived(page.Value, page),
		errors:   NewEmitter[error]("platform.WebViewController"),
	}
	view, err := GetPlatformViewRegistry().Create("native_webview", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("web view: create: %w", err)
	}
	webView, ok := view.(*nativeWebView)
	if !ok {
		GetPlatformViewRegistry().Dispose(view.ViewID())
		return nil, fmt.Errorf("web view: create: unexpected view type %T", view)
	}
	webView.setController(c)
	c.viewID = webView.ViewID()
	return c, nil
}

// Page is the page the web view shows: its URL, and whether it is still
// loading. It changes on the UI thread, so a widget can rebuild on it with
// [core.UseListenable]. Each load sets Loading, then clears it when the page
// finishes.
func (c *WebViewController) Page() *core.Derived[WebPage] {
	return c.readPage
}

// Errors streams load failures, each matching [ErrWebViewNetwork],
// [ErrWebViewSSL] or [ErrWebViewLoadFailed] under [errors.Is]. Like any
// stream, its handlers run off the UI thread: update widgets inside
// drift.Dispatch.
func (c *WebViewController) Errors() *Stream[error] {
	return c.errors.Stream()
}

// setPage sets the page on the UI thread, where its listeners may rebuild
// widgets. Without a UI thread (tests), it sets it at once.
func (c *WebViewController) setPage(page WebPage) {
	set := func() { c.page.Set(page) }
	if !Dispatch(set) {
		set()
	}
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
	c.viewID = 0
	c.mu.Unlock()
	if id != 0 {
		GetPlatformViewRegistry().Dispose(id)
	}
}
