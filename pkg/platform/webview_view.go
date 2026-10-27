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
	mu         sync.RWMutex
	controller *WebViewController // guarded by mu; nil until set
}

func (v *nativeWebView) Create(params map[string]any) error {
	return nil
}

func (v *nativeWebView) Dispose() {}

// setController sets the controller the view's events go to.
func (v *nativeWebView) setController(c *WebViewController) {
	v.mu.Lock()
	v.controller = c
	v.mu.Unlock()
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
		if c := v.controllerOrNil(); c != nil {
			c.setPage(WebPage{URL: url, Loading: method == "onPageStarted"})
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
		if c := v.controllerOrNil(); c != nil {
			c.failLoad()
			c.errors.Emit(webViewErrorCodes.NewError(code, message))
		}
	default:
		return fmt.Errorf("%s: %w", method, ErrMethodNotFound)
	}
	return nil
}

func (v *nativeWebView) controllerOrNil() *WebViewController {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.controller
}
