package agent

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ensureMita installs the mita package when it is not present yet.
func (a *Agent) ensureMita() (bool, error) {
	if a.mitaInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 mita，开始自动安装...")

	kind, err := packageManager()
	if err != nil {
		return false, err
	}
	url, filename, err := a.packageURL(kind)
	if err != nil {
		return false, err
	}

	dest := filepath.Join(os.TempDir(), filename)
	if err := download(url, dest); err != nil {
		return false, fmt.Errorf("下载 %s 失败: %w", url, err)
	}
	defer os.Remove(dest)

	var install string
	var args []string
	if kind == "deb" {
		install = "dpkg"
		args = []string{"-i", dest}
	} else {
		install = "rpm"
		args = []string{"-Uvh", "--force", dest}
	}
	if out, err := a.run(install, args...); err != nil {
		return false, fmt.Errorf("安装失败: %v %s", err, out)
	}

	_, _ = a.run("systemctl", "daemon-reload")
	_, _ = a.run("systemctl", "enable", "--now", "mita")

	if !a.mitaInstalled() {
		return false, fmt.Errorf("安装后仍未找到 mita 可执行文件")
	}
	fmt.Println("mita 安装完成")
	return true, nil
}

func packageManager() (string, error) {
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		if _, err := exec.LookPath("dpkg"); err == nil {
			return "deb", nil
		}
	}
	if _, err := exec.LookPath("rpm"); err == nil {
		return "rpm", nil
	}
	if _, err := exec.LookPath("dpkg"); err == nil {
		return "deb", nil
	}
	return "", fmt.Errorf("未找到 dpkg 或 rpm，暂不支持该系统")
}

func (a *Agent) packageURL(kind string) (string, string, error) {
	ver := a.cfg.MitaVersion
	var filename string
	switch kind {
	case "deb":
		switch runtimeArch() {
		case "amd64":
			filename = fmt.Sprintf("mita_%s_amd64.deb", ver)
		case "arm64":
			filename = fmt.Sprintf("mita_%s_arm64.deb", ver)
		default:
			return "", "", fmt.Errorf("不支持的架构: %s", runtimeArch())
		}
	case "rpm":
		switch runtimeArch() {
		case "amd64":
			filename = fmt.Sprintf("mita-%s-1.x86_64.rpm", ver)
		case "arm64":
			filename = fmt.Sprintf("mita-%s-1.aarch64.rpm", ver)
		default:
			return "", "", fmt.Errorf("不支持的架构: %s", runtimeArch())
		}
	}
	url := fmt.Sprintf("https://github.com/enfein/mieru/releases/download/v%s/%s", ver, filename)
	if a.cfg.MitaMirror != "" {
		url = strings.TrimRight(a.cfg.MitaMirror, "/") + "/" + url
	}
	return url, filename, nil
}

func runtimeArch() string { return runtime.GOARCH }

func download(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
