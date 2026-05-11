package platform

import (
	"context"
	"fmt"
	"net/url"
)

// URLLauncher provides access to the system URL launcher.
var URLLauncher = &URLLauncherService{
	channel: NewMethodChannel("drift/url_launcher"),
}

// URLLauncherService manages opening URLs in the system browser.
type URLLauncherService struct {
	channel *MethodChannel
}

// OpenURL opens the given URL in the system browser.
func (u *URLLauncherService) OpenURL(rawURL string) error {
	if err := validateURL(rawURL); err != nil {
		return err
	}
	_, err := u.channel.Invoke(context.Background(), "openURL", map[string]any{
		"url": rawURL,
	})
	return err
}

// CanOpenURL returns whether the system can open the given URL.
//
// On iOS, only schemes listed in LSApplicationQueriesSchemes (Info.plist) can
// be queried. On Android API 30+, only schemes declared in the manifest's
// <queries> block are visible to resolveActivity. Both templates include
// http, https, mailto, tel, and sms by default; other custom schemes will
// return false unless the app's manifests are updated.
func (u *URLLauncherService) CanOpenURL(rawURL string) (bool, error) {
	if err := validateURL(rawURL); err != nil {
		return false, err
	}
	result, err := u.channel.Invoke(context.Background(), "canOpenURL", map[string]any{
		"url": rawURL,
	})
	if err != nil {
		return false, err
	}

	if m, ok := result.(map[string]any); ok {
		if canOpen, ok := m["canOpen"].(bool); ok {
			return canOpen, nil
		}
	}

	return false, fmt.Errorf("url_launcher: unexpected response from canOpenURL: %v", result)
}

func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("url_launcher: empty URL")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("url_launcher: invalid URL: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("url_launcher: URL missing scheme: %q", rawURL)
	}
	return nil
}
