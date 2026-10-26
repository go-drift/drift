package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

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

// The Firebase SDKs check config file values when the app starts, and a
// value they reject stops Firebase (on iOS, with an uncaught exception at
// launch). The build checks those values as the SDKs do, so such a file
// fails the build instead, and a file the Firebase console issued passes.

// apiKeyFormat is the API key format Firebase Installations requires:
// Utils.API_KEY_FORMAT on Android (`\AA[\w-]{38}\z`, with Java's ASCII \w)
// and +[FIRInstallations validateAPIKey:] on iOS (39 characters, a leading
// A, alphanumerics, - and _; iOS also takes non-ASCII alphanumerics, which
// no Google API key contains).
//
// firebase-android-sdk: firebase-installations/src/main/java/com/google/firebase/installations/Utils.java
// firebase-ios-sdk: FirebaseInstallations/Source/Library/FIRInstallations.m
var apiKeyFormat = regexp.MustCompile(`^A[0-9A-Za-z_-]{38}$`)

const apiKeyRule = "39 characters from A-Z, a-z, 0-9, - and _, starting with A"

// iosAppIDV1Format is a version 1 GOOGLE_APP_ID after its "1:", as
// +[FIRApp validateAppIDFormat:withVersion:] scans it: an integer project
// number, "ios", and a hexadecimal bundle ID hash.
var iosAppIDV1Format = regexp.MustCompile(`^[+-]?[0-9]+:ios:(0[xX])?[0-9A-Fa-f]+$`)

// validIOSAppID mirrors +[FIRApp validateAppID:], which makes
// FirebaseApp.configure() raise when it fails: decimal version digits and a
// ":", then for version 1 iosAppIDV1Format. Other versions pass unchecked.
//
// firebase-ios-sdk: FirebaseCore/Sources/FIRApp.m
func validIOSAppID(id string) bool {
	version, rest, ok := strings.Cut(id, ":")
	if !ok || version == "" || strings.TrimFunc(version, unicode.IsDigit) != "" {
		return false
	}
	return version != "1" || iosAppIDV1Format.MatchString(rest)
}

// serviceInfo is the part of GoogleService-Info.plist the plugin checks.
type serviceInfo struct {
	BundleID    string `plist:"BUNDLE_ID"`
	GoogleAppID string `plist:"GOOGLE_APP_ID"`
	ProjectID   string `plist:"PROJECT_ID"`
	APIKey      string `plist:"API_KEY"`
	SenderID    string `plist:"GCM_SENDER_ID"`
}

// validate checks what Firebase raises on at launch: FirebaseApp.configure()
// on GOOGLE_APP_ID, Firebase Installations (+[FIRInstallations
// validateAppOptions:appName:]) on PROJECT_ID and API_KEY, and Firebase
// Messaging (FIRMessaging.m) on GCM_SENDER_ID.
func (s serviceInfo) validate() error {
	switch {
	case !validIOSAppID(s.GoogleAppID):
		return fmt.Errorf("GOOGLE_APP_ID %q is not a Firebase iOS app ID (1:<project number>:ios:<hex>)", s.GoogleAppID)
	case s.ProjectID == "":
		return errors.New("PROJECT_ID is missing")
	case !apiKeyFormat.MatchString(s.APIKey):
		return errors.New("API_KEY is not a Firebase API key (" + apiKeyRule + ")")
	case s.SenderID == "":
		return errors.New("GCM_SENDER_ID is missing")
	}
	return nil
}

// resolveIOS reads GoogleService-Info.plist and checks it is for this app
// and lets Firebase start.
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
	if err := info.validate(); err != nil {
		return nil, fmt.Errorf("ios.config_file %q: %w; download GoogleService-Info.plist again from the Firebase console", path, err)
	}
	return data, nil
}

// googleServices is the part of google-services.json the plugin checks.
type googleServices struct {
	ProjectInfo struct {
		ProjectNumber string `json:"project_number"`
		ProjectID     string `json:"project_id"`
	} `json:"project_info"`
	Client []googleServicesClient `json:"client"`
}

// googleServicesClient is one app of a google-services.json.
type googleServicesClient struct {
	ClientInfo struct {
		MobileSDKAppID    string `json:"mobilesdk_app_id"`
		AndroidClientInfo struct {
			PackageName string `json:"package_name"`
		} `json:"android_client_info"`
	} `json:"client_info"`
	APIKey []struct {
		CurrentKey *string `json:"current_key"`
	} `json:"api_key"`
}

// validate checks client i (the app's) for what Firebase Installations'
// preConditionChecks require of the resources the google-services Gradle
// plugin generates: google_app_id (mobilesdk_app_id) containing ":",
// project_id, and google_api_key (the first api_key with a current_key) in
// apiKeyFormat.
//
// firebase-android-sdk: firebase-installations/src/main/java/com/google/firebase/installations/FirebaseInstallations.java
// play-services-plugins: google-services-plugin/src/main/kotlin/com/google/gms/googleservices/GoogleServicesTask.kt
func (gs googleServices) validate(i int) error {
	c := gs.Client[i]
	if !strings.Contains(c.ClientInfo.MobileSDKAppID, ":") {
		return fmt.Errorf("client[%d].client_info.mobilesdk_app_id %q is not a Firebase app ID", i, c.ClientInfo.MobileSDKAppID)
	}
	if gs.ProjectInfo.ProjectID == "" {
		return errors.New("project_info.project_id is missing")
	}
	if !apiKeyFormat.MatchString(c.apiKey()) {
		return fmt.Errorf("client[%d].api_key current_key is not a Firebase API key (%s)", i, apiKeyRule)
	}
	return nil
}

// apiKey is the key the Gradle plugin makes google_api_key: the first
// api_key entry's current_key, skipping entries without one; "" if none.
func (c googleServicesClient) apiKey() string {
	for _, k := range c.APIKey {
		if k.CurrentKey != nil {
			return *k.CurrentKey
		}
	}
	return ""
}

// resolveAndroid reads google-services.json and checks it has a client for
// this app that lets Firebase start.
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
	// The Gradle plugin takes the first client for the package.
	client := slices.Index(packages, ctx.AppID())
	if client < 0 {
		return nil, fmt.Errorf("android.config_file %q has no client for package %q (it has %q); register the Android app in Firebase with %q and download google-services.json again",
			path, ctx.AppID(), packages, ctx.AppID())
	}
	if err := gs.validate(client); err != nil {
		return nil, fmt.Errorf("android.config_file %q: %w; download google-services.json again from the Firebase console", path, err)
	}
	return data, nil
}
