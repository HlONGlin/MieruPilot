// Package updater provides automatic update checking and downloading.
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	// GitHubRepo is the repository for checking updates.
	GitHubRepo = "HlONGlin/MieruPilot"
	// CheckInterval is how often to check for updates.
	CheckInterval = 24 * time.Hour
)

// Release represents a GitHub release.
type Release struct {
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	CreatedAt  time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Assets     []Asset   `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for the latest release from GitHub.
func CheckUpdate(currentVersion string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	// Skip draft and prerelease versions
	if release.Draft || release.Prerelease {
		return nil, nil
	}

	// Compare versions
	if !isNewer(release.TagName, currentVersion) {
		return nil, nil
	}

	return &release, nil
}

// isNewer checks if newVer is newer than current.
// Simple comparison - assumes semantic versioning like v1.0.0
func isNewer(newVer, current string) bool {
	newVer = strings.TrimPrefix(newVer, "v")
	current = strings.TrimPrefix(current, "v")
	return newVer > current
}

// FindAsset finds the appropriate binary asset for the current platform.
func (r *Release) FindAsset(component string) *Asset {
	// component is "manager" or "agent"
	suffix := fmt.Sprintf("%s-%s-%s", component, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}

	for i := range r.Assets {
		if strings.Contains(r.Assets[i].Name, suffix) {
			return &r.Assets[i]
		}
	}
	return nil
}

// DownloadAsset downloads an asset to the given writer.
func DownloadAsset(asset *Asset, w io.Writer) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("failed to save asset: %w", err)
	}

	return nil
}
