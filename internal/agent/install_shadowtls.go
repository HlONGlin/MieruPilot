package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"merit/internal/proxy/shadowtls"
)

// ensureShadowTLS installs the ShadowTLS v3 server when it is not present yet.
func (a *Agent) ensureShadowTLS() (bool, error) {
	if a.shadowTLSInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 ShadowTLS，开始自动安装...")

	url, filename, err := shadowtls.DownloadURL(runtimeArch())
	if err != nil {
		return false, err
	}
	url = mirrorURL(a.cfg.MitaMirror, url)

	// Upstream publishes bare binaries (no archive, no file extension).
	binary := filepath.Join(os.TempDir(), filename)
	if err := download(url, binary); err != nil {
		return false, fmt.Errorf("下载 ShadowTLS 失败: %w", err)
	}
	defer os.Remove(binary)

	if err := installExecutable(binary, shadowtls.BinaryPath); err != nil {
		return false, fmt.Errorf("安装 ShadowTLS 失败: %w", err)
	}

	fmt.Println("ShadowTLS 安装完成")
	return true, nil
}

func (a *Agent) shadowTLSInstalled() bool {
	return executableInstalled(shadowtls.BinaryPath)
}
