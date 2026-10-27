package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"merit/internal/proxy/anytls"
)

// ensureAnyTLS installs the AnyTLS server when it is not present yet.
func (a *Agent) ensureAnyTLS() (bool, error) {
	if a.anyTLSInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 AnyTLS，开始自动安装...")

	url, filename, err := anytls.DownloadURL(runtimeArch())
	if err != nil {
		return false, err
	}
	url = mirrorURL(a.cfg.MitaMirror, url)

	archive := filepath.Join(os.TempDir(), filename)
	if err := download(url, archive); err != nil {
		return false, fmt.Errorf("下载 AnyTLS 失败: %w", err)
	}
	defer os.Remove(archive)

	workDir, err := os.MkdirTemp("", "merit-anytls-*")
	if err != nil {
		return false, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(workDir)

	// The release archive is a zip that contains both anytls-client and
	// anytls-server at its root; only the server is installed.
	if err := extractArchive(archive, workDir); err != nil {
		return false, fmt.Errorf("解压 AnyTLS 失败: %w", err)
	}
	binary, err := findBinary(workDir, anytls.ServerBinaryName)
	if err != nil {
		return false, err
	}
	if err := installExecutable(binary, anytls.ServerBinaryPath); err != nil {
		return false, fmt.Errorf("安装 AnyTLS 失败: %w", err)
	}

	fmt.Println("AnyTLS 安装完成")
	return true, nil
}

func (a *Agent) anyTLSInstalled() bool {
	return executableInstalled(anytls.ServerBinaryPath)
}
