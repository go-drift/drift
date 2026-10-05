package platform

import (
	"fmt"
	"sync"
)

type nativeWebViewFactory struct{}

func (nativeWebViewFactory) ViewType() string {
	return "native_webview"
}

func (nativeWebViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	return &nativeWebView{
		PlatformViewBase: NewPlatformViewBase(viewID, "native_webview"),
	}, nil
}

type nativeWebView struct {
	PlatformViewBase
	mu sync.RWMutex

	// OnPageStarted is called when a page starts loading.
	// Called on the UI thread via [Dispatch].
	OnPageStarted func(url string)

	// OnPageFinished is called when a page finishes loading.
	// Called on the UI thread via [Dispatch].
	OnPageFinished func(url string)

	// OnError is called when a loading error occurs.
	// The code parameter is one of the ErrCodeNetworkError, ErrCodeSSLError,
	// or ErrCodeLoadFailed constants. Called on the UI thread via [Dispatch].
	OnError func(code, message string)
}

func (v *nativeWebView) Create(params map[string]any) error {
	return nil
}

func (v *nativeWebView) Dispose() {}

// handlePageStarted processes page start events from native.
func (v *nativeWebView) handlePageStarted(url string) {
	v.mu.RLock()
	cb := v.OnPageStarted
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(url)
		})
	}
}

// handlePageFinished processes page finish events from native.
func (v *nativeWebView) handlePageFinished(url string) {
	v.mu.RLock()
	cb := v.OnPageFinished
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(url)
		})
	}
}

// handleError processes error events from native.
func (v *nativeWebView) handleError(code, message string) {
	v.mu.RLock()
	cb := v.OnError
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(code, message)
		})
	}
}

func init() {
	GetPlatformViewRegistry().RegisterFactory(nativeWebViewFactory{})
}

// HandleEvent implements PlatformView: page loads and errors from the
// native web view.
func (v *nativeWebView) HandleEvent(method string, args map[string]any) error {
	switch method {
	case "onPageStarted", "onPageFinished":
		url, err := requireString(method, args, "url")
		if err != nil {
			return err
		}
		if method == "onPageStarted" {
			v.handlePageStarted(url)
		} else {
			v.handlePageFinished(url)
		}
	case "onWebViewError":
		code, err := requireString(method, args, "code")
		if err != nil {
			return err
		}
		message, err := requireString(method, args, "message")
		if err != nil {
			return err
		}
		v.handleError(code, message)
	default:
		return fmt.Errorf("%s: %w", method, ErrMethodNotFound)
	}
	return nil
}
