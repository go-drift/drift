package platform

import "errors"

// Web view load failures, the same on Android and iOS. A
// [WebViewController]'s Errors stream delivers them as errors matching
// these under [errors.Is], with the platform's message.
var (
	// ErrWebViewNetwork indicates a network-level failure such as DNS
	// resolution, connectivity or a timeout.
	ErrWebViewNetwork = errors.New("web page could not be reached")

	// ErrWebViewSSL indicates a TLS or certificate failure, such as an
	// untrusted or expired certificate.
	ErrWebViewSSL = errors.New("web page's secure connection failed")

	// ErrWebViewLoadFailed indicates any other page load failure.
	ErrWebViewLoadFailed = errors.New("web page failed to load")
)

// webViewErrorCodes are the native web views' codes for the load failures.
var webViewErrorCodes = ErrorCodes{
	"network_error": ErrWebViewNetwork,
	"ssl_error":     ErrWebViewSSL,
	"load_failed":   ErrWebViewLoadFailed,
}
