package fetch

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	// GitHubRepo is the repository for Drift releases.
	GitHubRepo = "go-drift/drift"

	// GitHubAPILatestRelease is the endpoint for fetching the latest release.
	GitHubAPILatestRelease = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"

	// GitHubReleaseDownloadBase is the base URL for release downloads.
	GitHubReleaseDownloadBase = "https://github.com/" + GitHubRepo + "/releases/download"
)

// Manifest represents the manifest.json file in a release.
type Manifest struct {
	NDKVersion string            `json:"ndk_version,omitempty"`
	Android    *PlatformManifest `json:"android,omitempty"`
	IOS        *PlatformManifest `json:"ios,omitempty"`
}

// PlatformManifest contains checksum information for a platform.
type PlatformManifest struct {
	SHA256 string `json:"sha256"`
}

// releaseResponse is the GitHub API response for a release.
type releaseResponse struct {
	TagName string `json:"tag_name"`
}

// FetchLatestRelease fetches the latest release tag from GitHub.
func FetchLatestRelease(ctx context.Context, d *Downloader) (string, error) {
	body, err := d.DownloadJSON(ctx, GitHubAPILatestRelease)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}

	var resp releaseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse release response: %w", err)
	}

	if resp.TagName == "" {
		return "", fmt.Errorf("no tag_name in release response")
	}

	return resp.TagName, nil
}

// FetchManifest downloads and parses the manifest.json for a release.
func FetchManifest(ctx context.Context, d *Downloader, version string) (*Manifest, error) {
	url := ManifestURL(version)
	body, err := d.DownloadJSON(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &m, nil
}

// ManifestURL returns the URL for the manifest.json of a release.
func ManifestURL(version string) string {
	return fmt.Sprintf("%s/%s/manifest.json", GitHubReleaseDownloadBase, version)
}

// TarballURL returns the download URL for a platform tarball.
func TarballURL(version, platform string) string {
	return fmt.Sprintf("%s/%s/drift-%s-%s.tar.gz", GitHubReleaseDownloadBase, version, version, platform)
}
