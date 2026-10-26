// Package updater provides automatic update checking and downloading.
package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// GitHubRepo is the repository for checking updates.
	GitHubRepo = "HlONGlin/UnifiedProxy"
	// CheckInterval is how often to check for updates.
	CheckInterval = 24 * time.Hour
	// releaseCacheTTL keeps the settings page usable when GitHub is temporarily
	// unavailable or slow behind a proxy.
	releaseCacheTTL      = 10 * time.Minute
	githubRequestTimeout = 5 * time.Second
)

var releaseAPIURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
var releaseRequestTimeout = githubRequestTimeout

var releaseCache struct {
	sync.Mutex
	release   *Release
	fetchedAt time.Time
}

// Release represents a GitHub release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for the latest release from GitHub.
func CheckUpdate(currentVersion string) (*Release, error) {
	release, err := LatestRelease()
	if err != nil {
		return nil, err
	}
	if release.Draft || release.Prerelease || !isNewer(release.TagName, currentVersion) {
		return nil, nil
	}
	return release, nil
}

// LatestRelease returns the latest stable GitHub release.
func LatestRelease() (*Release, error) {
	releaseCache.Lock()
	if releaseCache.release != nil && time.Since(releaseCache.fetchedAt) < releaseCacheTTL {
		release := *releaseCache.release
		releaseCache.Unlock()
		return &release, nil
	}
	releaseCache.Unlock()

	client := &http.Client{Timeout: releaseRequestTimeout}
	req, err := http.NewRequest(http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check update: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "UnifiedProxy-Updater")
	resp, err := client.Do(req)
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

	releaseCache.Lock()
	releaseCache.release = &release
	releaseCache.fetchedAt = time.Now()
	releaseCache.Unlock()

	return &release, nil
}

func resetReleaseCache() {
	releaseCache.Lock()
	releaseCache.release = nil
	releaseCache.fetchedAt = time.Time{}
	releaseCache.Unlock()
}

// isNewer checks if newVer is newer than current.
// Simple comparison - assumes semantic versioning like v1.0.0
func isNewer(newVer, current string) bool {
	newParts, okNew := versionParts(newVer)
	currentParts, okCurrent := versionParts(current)
	if !okNew || !okCurrent {
		return false
	}
	for i := 0; i < len(newParts); i++ {
		if newParts[i] != currentParts[i] {
			return newParts[i] > currentParts[i]
		}
	}
	return false
}

func versionParts(version string) ([3]int, bool) {
	var parts [3]int
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	fields := strings.SplitN(version, ".", 4)
	if len(fields) < 3 {
		return parts, false
	}
	for i := 0; i < 3; i++ {
		value := 0
		for _, c := range fields[i] {
			if c < '0' || c > '9' {
				return parts, false
			}
			value = value*10 + int(c-'0')
		}
		parts[i] = value
	}
	return parts, true
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

// DownloadAndVerify downloads a release binary and verifies its SHA256SUMS entry.
func DownloadAndVerify(release *Release, component, destination string) error {
	asset := release.FindAsset(component)
	if asset == nil {
		return fmt.Errorf("发布版本没有当前平台的 %s 文件", component)
	}
	var sums Asset
	for i := range release.Assets {
		if release.Assets[i].Name == "SHA256SUMS" {
			sums = release.Assets[i]
			break
		}
	}
	if sums.BrowserDownloadURL == "" {
		return fmt.Errorf("发布版本缺少 SHA256SUMS")
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新失败: HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".merit-manager-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	sumResp, err := client.Get(sums.BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer sumResp.Body.Close()
	sumBody, err := io.ReadAll(sumResp.Body)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(sumBody), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimLeft(fields[1], "*") == filepath.Base(asset.Name) {
			want = fields[0]
			break
		}
	}
	if want == "" || !strings.EqualFold(want, hex.EncodeToString(h.Sum(nil))) {
		return fmt.Errorf("更新文件校验失败")
	}
	if err := os.Chmod(tmpName, 0755); err != nil {
		return err
	}
	return os.Rename(tmpName, destination)
}

// ScheduleRestart asks systemd to restart the manager after the update request
// has returned. The transient unit runs outside the manager process cgroup.
func ScheduleRestart() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("当前系统不支持在线重启，请手动替换程序")
	}
	unit := fmt.Sprintf("merit-manager-update-%d", time.Now().UnixNano())
	script := "sleep 2; systemctl restart merit-manager.service"
	cmd := exec.Command("systemd-run", "--unit="+unit, "--collect", "/bin/sh", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("安排 Manager 重启失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// CurrentExecutable returns the running Manager binary path.
func CurrentExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}
