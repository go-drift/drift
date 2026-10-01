package plugin

import (
	"encoding/json"
	"fmt"
	"slices"

	"howett.net/plist"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// Config is the drift.yaml `config:` block of the Firebase plugin.
type Config struct {
	IOS     *Platform `yaml:"ios,omitempty"`
	Android *Platform `yaml:"android,omitempty"`
}

// Platform configures Firebase for one platform.
type Platform struct {
	// ConfigFile is the Firebase app config downloaded from the console:
	// GoogleService-Info.plist (iOS) or google-services.json (Android).
	ConfigFile string `yaml:"config_file" drift:"required,asset"`
}

// serviceInfo is the part of GoogleService-Info.plist the plugin checks.
type serviceInfo struct {
	BundleID    string `plist:"BUNDLE_ID"`
	GoogleAppID string `plist:"GOOGLE_APP_ID"`
	SenderID    string `plist:"GCM_SENDER_ID"`
}

// resolveIOS reads GoogleService-Info.plist and checks it is for this app
// and can do messaging.
func resolveIOS(ctx *driftplugin.BuildCtx, path string) ([]byte, error) {
	data, err := ctx.ResolveAsset(path)
	if err != nil {
		return nil, fmt.Errorf("read ios.config_file %q: %w", path, err)
	}
	var info serviceInfo
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("ios.config_file %q is not a GoogleService-Info.plist: %w", path, err)
	}
	if info.BundleID != ctx.AppID() {
		return nil, fmt.Errorf("ios.config_file %q is for bundle ID %q, but app.id is %q; register the iOS app in Firebase with %q and download its GoogleService-Info.plist",
			path, info.BundleID, ctx.AppID(), ctx.AppID())
	}
	if info.GoogleAppID == "" || info.SenderID == "" {
		return nil, fmt.Errorf("ios.config_file %q has no GOOGLE_APP_ID or GCM_SENDER_ID", path)
	}
	return data, nil
}

// googleServices is the part of google-services.json the plugin checks.
type googleServices struct {
	ProjectInfo struct {
		ProjectNumber string `json:"project_number"`
	} `json:"project_info"`
	Client []struct {
		ClientInfo struct {
			AndroidClientInfo struct {
				PackageName string `json:"package_name"`
			} `json:"android_client_info"`
		} `json:"client_info"`
	} `json:"client"`
}

// resolveAndroid reads google-services.json and checks it has a client for
// this app.
func resolveAndroid(ctx *driftplugin.BuildCtx, path string) ([]byte, error) {
	data, err := ctx.ResolveAsset(path)
	if err != nil {
		return nil, fmt.Errorf("read android.config_file %q: %w", path, err)
	}
	var gs googleServices
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, fmt.Errorf("android.config_file %q is not a google-services.json: %w", path, err)
	}
	if gs.ProjectInfo.ProjectNumber == "" {
		return nil, fmt.Errorf("android.config_file %q has no project_info.project_number", path)
	}
	var packages []string
	for _, c := range gs.Client {
		packages = append(packages, c.ClientInfo.AndroidClientInfo.PackageName)
	}
	if !slices.Contains(packages, ctx.AppID()) {
		return nil, fmt.Errorf("android.config_file %q has no client for package %q (it has %q); register the Android app in Firebase with %q and download google-services.json again",
			path, ctx.AppID(), packages, ctx.AppID())
	}
	return data, nil
}
