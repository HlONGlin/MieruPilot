package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"merit/internal/proxy/shadowtls"
)

// ensureShadowTLS installs ShadowTLS v3 when not present.
func (a *Agent) ensureShadowTLS() (bool, error) {
	if a.shadowTLSInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 ShadowTLS，开始自动安装...")

	url, filename, err := a.shadowTLSDownloadURL()
	if err != nil {
		return false, err
	}

	dest := filepath.Join(os.TempDir(), filename)
	if err := download(url, dest); err != nil {
		return false, fmt.Errorf("下载 ShadowTLS 失败: %w", err)
	}
	defer os.Remove(dest)

	// Install to /usr/local/bin
	targetPath := "/usr/local/bin/shadowtls"
	if err := os.Rename(dest, targetPath); err != nil {
		return false, fmt.Errorf("安装失败: %w", err)
	}

	if err := os.Chmod(targetPath, 0755); err != nil {
		return false, fmt.Errorf("设置权限失败: %w", err)
	}

	// Create config directory
	if err := os.MkdirAll("/etc/merit/shadowtls", 0755); err != nil {
		return false, fmt.Errorf("创建配置目录失败: %w", err)
	}

	fmt.Println("ShadowTLS 安装完成")
	return true, nil
}

func (a *Agent) shadowTLSInstalled() bool {
	_, err := exec.LookPath("shadowtls")
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/local/bin/shadowtls")
	return err == nil
}

func (a *Agent) shadowTLSDownloadURL() (string, string, error) {
	ver := shadowtls.DefaultVersion
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

	filename := fmt.Sprintf("shadowtls-%s-unknown-linux-musl", archStr)
	url := fmt.Sprintf("https://github.com/ihciah/shadow-tls/releases/download/%s/%s", ver, filename)

	if a.cfg.MitaMirror != "" {
		url = a.cfg.MitaMirror + "/" + url
	}

	return url, filename, nil
}
