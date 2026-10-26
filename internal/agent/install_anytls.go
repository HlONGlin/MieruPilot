package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"merit/internal/proxy/anytls"
)

// ensureAnyTLS installs AnyTLS when not present.
func (a *Agent) ensureAnyTLS() (bool, error) {
	if a.anyTLSInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 AnyTLS，开始自动安装...")

	url, filename, err := a.anyTLSDownloadURL()
	if err != nil {
		return false, err
	}

	dest := filepath.Join(os.TempDir(), filename)
	if err := download(url, dest); err != nil {
		return false, fmt.Errorf("下载 AnyTLS 失败: %w", err)
	}
	defer os.Remove(dest)

	// Install to /usr/local/bin
	targetPath := "/usr/local/bin/anytls"
	if err := os.Rename(dest, targetPath); err != nil {
		return false, fmt.Errorf("安装失败: %w", err)
	}

	if err := os.Chmod(targetPath, 0755); err != nil {
		return false, fmt.Errorf("设置权限失败: %w", err)
	}

	// Create config directory
	if err := os.MkdirAll("/etc/merit/anytls", 0755); err != nil {
		return false, fmt.Errorf("创建配置目录失败: %w", err)
	}

	fmt.Println("AnyTLS 安装完成")
	return true, nil
}

func (a *Agent) anyTLSInstalled() bool {
	_, err := exec.LookPath("anytls")
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/local/bin/anytls")
	return err == nil
}

func (a *Agent) anyTLSDownloadURL() (string, string, error) {
	ver := anytls.DefaultVersion
	arch := runtime.GOARCH

	var archStr string
	switch arch {
	case "amd64":
		archStr = "x86_64"
	case "arm64":
		archStr = "aarch64"
	default:
		return "", "", fmt.Errorf("不支持的架构: %s", arch)
	}

	filename := fmt.Sprintf("anytls-%s-linux-%s", ver, archStr)
	url := fmt.Sprintf("https://github.com/anytls/anytls/releases/download/%s/%s", ver, filename)

	if a.cfg.MitaMirror != "" {
		url = a.cfg.MitaMirror + "/" + url
	}

	return url, filename, nil
}
