package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ensureNaive installs NaïveProxy (Caddy with forwardproxy) when not present.
func (a *Agent) ensureNaive() (bool, error) {
	if a.naiveInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 NaïveProxy，开始自动安装...")

	// NaïveProxy is implemented as Caddy with forwardproxy plugin
	// We need to download the prebuilt binary or build from source
	url, err := a.naiveDownloadURL()
	if err != nil {
		return false, err
	}

	dest := filepath.Join(os.TempDir(), "caddy-naive")
	if err := download(url, dest); err != nil {
		return false, fmt.Errorf("下载 NaïveProxy 失败: %w", err)
	}
	defer os.Remove(dest)

	// Install to /usr/local/bin
	targetPath := "/usr/local/bin/caddy-naive"
	if err := os.Rename(dest, targetPath); err != nil {
		return false, fmt.Errorf("安装失败: %w", err)
	}

	if err := os.Chmod(targetPath, 0755); err != nil {
		return false, fmt.Errorf("设置权限失败: %w", err)
	}

	// Create config directory
	if err := os.MkdirAll("/etc/merit/naive", 0755); err != nil {
		return false, fmt.Errorf("创建配置目录失败: %w", err)
	}

	fmt.Println("NaïveProxy 安装完成")
	return true, nil
}

func (a *Agent) naiveInstalled() bool {
	_, err := exec.LookPath("caddy-naive")
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/local/bin/caddy-naive")
	return err == nil
}

func (a *Agent) naiveDownloadURL() (string, error) {
	// Caddy with forwardproxy builds are available from various sources
	// For simplicity, use a known build or instruct users to build manually
	arch := runtime.GOARCH
	var archStr string
	switch arch {
	case "amd64":
		archStr = "amd64"
	case "arm64":
		archStr = "arm64"
	default:
		return "", fmt.Errorf("不支持的架构: %s", arch)
	}

	// Use xcaddy or pre-built binaries
	// TODO: This URL should point to actual NaïveProxy/Caddy builds
	url := fmt.Sprintf("https://github.com/caddyserver/caddy/releases/latest/download/caddy_linux_%s", archStr)

	if a.cfg.MitaMirror != "" {
		url = a.cfg.MitaMirror + "/" + url
	}

	return url, nil
}
