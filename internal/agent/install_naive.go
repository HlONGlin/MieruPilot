package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"merit/internal/proxy/naive"
)

// ensureNaive installs the NaïveProxy server (Caddy + klzgrad forwardproxy) when
// it is not present yet.
func (a *Agent) ensureNaive() (bool, error) {
	if a.naiveInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 NaïveProxy（Caddy forwardproxy），开始自动安装...")

	url, filename, err := naive.DownloadURL(runtimeArch())
	if err != nil {
		return false, err
	}
	url = mirrorURL(a.cfg.MitaMirror, url)

	archive := filepath.Join(os.TempDir(), filename)
	if err := download(url, archive); err != nil {
		return false, fmt.Errorf("下载 NaïveProxy 失败: %w", err)
	}
	defer os.Remove(archive)

	workDir, err := os.MkdirTemp("", "merit-naive-*")
	if err != nil {
		return false, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := extractArchive(archive, workDir); err != nil {
		return false, fmt.Errorf("解压 NaïveProxy 失败: %w", err)
	}
	binary, err := findBinary(workDir, naive.BinaryName)
	if err != nil {
		return false, err
	}
	if err := installExecutable(binary, naive.BinaryPath); err != nil {
		return false, fmt.Errorf("安装 NaïveProxy 失败: %w", err)
	}
	if !a.naiveInstalled() {
		return false, fmt.Errorf("%s 无法运行或缺少 http.handlers.forward_proxy 模块", naive.BinaryPath)
	}
	if err := os.MkdirAll(naive.ConfigDir, 0o750); err != nil {
		return false, fmt.Errorf("创建配置目录失败: %w", err)
	}

	fmt.Println("NaïveProxy 安装完成")
	return true, nil
}

func (a *Agent) naiveInstalled() bool {
	return a.naiveInstalledAt(naive.BinaryPath)
}

func (a *Agent) naiveInstalledAt(path string) bool {
	if !executableInstalled(path) {
		return false
	}
	modules, err := a.run(path, "list-modules")
	if err != nil {
		return false
	}
	for _, module := range strings.Fields(modules) {
		if module == "http.handlers.forward_proxy" {
			return true
		}
	}
	return false
}
