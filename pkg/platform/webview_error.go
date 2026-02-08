package platform

// Canonical webview error codes shared by Android and iOS.
// Native implementations map platform-specific errors to these codes
// so that Go callbacks receive consistent values across platforms.
const (
	// ErrCodeNetworkError indicates a network-level failure such as
	// DNS resolution, connectivity, or timeout errors.
	ErrCodeNetworkError = "network_error"

	// ErrCodeSSLError indicates a TLS/certificate failure such as
	// untrusted certificates or expired certificates.
	ErrCodeSSLError = "ssl_error"

	// ErrCodeLoadFailed indicates a general page load failure that
	// does not fit a more specific category.
	ErrCodeLoadFailed = "load_failed"
)
