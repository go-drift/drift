package plugin

import (
	"strings"
	"testing"
)

const fakeAPIKey = "AIzaSyFakeKeyForDriftTests0000000000000"

func TestAPIKeyFormat(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want bool
	}{
		{fakeAPIKey, true},
		{"A" + strings.Repeat("-", 19) + strings.Repeat("_", 19), true},
		{"AIzaTestKey", false},                                    // too short
		{fakeAPIKey + "0", false},                                 // too long
		{"B" + fakeAPIKey[1:], false},                             // must start with A
		{fakeAPIKey[:38] + "=", false},                            // not base64 url-safe
		{fakeAPIKey[:38] + ".", false},                            // not base64 url-safe
		{fakeAPIKey[:38] + "é", false},                            // ASCII, as on Android (see apiKeyFormat)
		{" " + fakeAPIKey[1:], false},                             // no leading space
		{"", false},                                               // missing
		{fakeAPIKey[:38] + "\n", false},                           // no trailing newline
		{strings.ToLower(fakeAPIKey[:1]) + fakeAPIKey[1:], false}, // lowercase a
	} {
		if got := apiKeyFormat.MatchString(tc.key); got != tc.want {
			t.Errorf("apiKeyFormat(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestValidIOSAppID(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"1:123456789012:ios:0123456789abcdef", true},
		{"1:123456789012:ios:0123456789ABCDEF", true},
		{"1:123456789012:ios:0x1f", true}, // NSScanner scanHexLongLong takes 0x
		{"1:+123:ios:1f", true},           // NSScanner scanInteger takes a sign
		{"2:anything", true},              // unknown versions pass unchecked
		{"01:anything", true},             // "01" is not the known version "1"
		{"", false},
		{"1", false},                                       // no ":" after the version
		{":123:ios:1f", false},                             // no version
		{"x:123:ios:1f", false},                            // version not decimal
		{"1:123456789012:android:0123456789abcdef", false}, // Android app ID
		{"1:123456789012:ios:", false},                     // no bundle ID hash
		{"1:123456789012:ios:xyz", false},                  // hash not hexadecimal
		{"1:123456789012:ios:1f:extra", false},             // trailing component
		{"1::ios:1f", false},                               // no project number
		{"1:drift-test:ios:1f", false},                     // project number not an integer
		{"1:123456789012:web:0123456789abcdef", false},     // not ios
		{"1:123456789012:ios:0123456789abcdef ", false},    // trailing space
	} {
		if got := validIOSAppID(tc.id); got != tc.want {
			t.Errorf("validIOSAppID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

// A config file Firebase would reject at startup fails the build, naming
// the file and the key.
func TestBuildRejectsConfigFirebaseRejects(t *testing.T) {
	const plist, json = "firebase/GoogleService-Info.plist", "firebase/google-services.json"
	for _, tc := range []struct {
		name     string
		platform string
		replace  []string
		want     []string
	}{
		{"short API_KEY", "ios", []string{fakeAPIKey, "AIzaTestKey"}, []string{plist, "API_KEY"}},
		{"no API_KEY", "ios", []string{"<key>API_KEY</key>", "<key>OTHER_KEY</key>"}, []string{plist, "API_KEY"}},
		{"Android GOOGLE_APP_ID", "ios", []string{":ios:", ":android:"}, []string{plist, "GOOGLE_APP_ID"}},
		{"no GOOGLE_APP_ID", "ios", []string{"<key>GOOGLE_APP_ID</key>", "<key>OTHER_KEY</key>"}, []string{plist, "GOOGLE_APP_ID"}},
		{"no PROJECT_ID", "ios", []string{"<key>PROJECT_ID</key>", "<key>OTHER_KEY</key>"}, []string{plist, "PROJECT_ID"}},
		{"no GCM_SENDER_ID", "ios", []string{"<key>GCM_SENDER_ID</key>", "<key>OTHER_KEY</key>"}, []string{plist, "GCM_SENDER_ID"}},
		{"short api_key", "android", []string{fakeAPIKey, "AIzaTestKey"}, []string{json, "client[0].api_key"}},
		{"no current_key", "android", []string{`"current_key"`, `"other_key"`}, []string{json, "client[0].api_key"}},
		{"mobilesdk_app_id without :", "android", []string{"1:123456789012:android:0123456789abcdef", "0123456789abcdef"}, []string{json, "mobilesdk_app_id"}},
		{"no project_id", "android", []string{`"project_id"`, `"other_id"`}, []string{json, "project_info.project_id"}},
		{"no project_info", "android", []string{`"project_info"`, `"other"`}, []string{json, "project_info.project_number"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := build(t, tc.platform, fullConfig, tc.replace...)
			if err == nil {
				t.Fatal("Build accepted the config file")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to name %q", err, want)
				}
			}
		})
	}
}

// Valid configs beyond the testdata's shape pass.
func TestBuildAcceptsConfigFirebaseAccepts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform string
		replace  []string
	}{
		// The Gradle plugin skips api_key entries without a current_key.
		{"api_key entry without current_key first", "android", []string{`[{"current_key"`, `[{}, {"current_key"`}},
		{"unknown GOOGLE_APP_ID version", "ios", []string{"1:123456789012:ios:", "2:123456789012:ios:"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := build(t, tc.platform, fullConfig, tc.replace...); err != nil {
				t.Errorf("Build: %v", err)
			}
		})
	}
}
