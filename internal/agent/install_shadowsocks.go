package agent

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"merit/internal/model"
	"merit/internal/proxy/shadowsocks"
)

// backendServiceName returns the unit name of the managed shadowsocks backend of
// one ShadowTLS port.
func backendServiceName(portID string) string {
	return shadowsocksBackendPrefix + portID
}

// parseBackendUnitName extracts the port ID from a managed backend unit name.
func parseBackendUnitName(unit string) (portID string, ok bool) {
	if !strings.HasPrefix(unit, shadowsocksBackendPrefix) {
		return "", false
	}
	return strings.TrimPrefix(unit, shadowsocksBackendPrefix), true
}

// ensureShadowsocks installs the shadowsocks server when it is not present yet.
func (a *Agent) ensureShadowsocks() (bool, error) {
	if shadowsocks.IsInstalled() {
		return true, nil
	}
	fmt.Println("未检测到 Shadowsocks 服务端，开始自动安装...")

	url, filename, err := shadowsocks.DownloadURL(runtimeArch())
	if err != nil {
		return false, err
	}
	url = mirrorURL(a.cfg.MitaMirror, url)

	archive := filepath.Join(os.TempDir(), filename)
	if err := download(url, archive); err != nil {
		return false, fmt.Errorf("下载 Shadowsocks 失败: %w", err)
	}
	defer os.Remove(archive)

	workDir, err := os.MkdirTemp("", "merit-shadowsocks-*")
	if err != nil {
		return false, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := extractArchive(archive, workDir); err != nil {
		return false, fmt.Errorf("解压 Shadowsocks 失败: %w", err)
	}
	binary, err := findBinary(workDir, shadowsocks.BinaryName)
	if err != nil {
		return false, err
	}
	if err := installExecutable(binary, shadowsocks.BinaryPath); err != nil {
		return false, fmt.Errorf("安装 Shadowsocks 失败: %w", err)
	}

	fmt.Println("Shadowsocks 安装完成")
	return true, nil
}

// prepareBackendBinary makes sure ssserver is available. Tests replace it.
func (a *Agent) prepareBackendBinary() error {
	if a.ensureBackendBinary != nil {
		return a.ensureBackendBinary()
	}
	if _, err := a.ensureShadowsocks(); err != nil {
		return fmt.Errorf("确保 Shadowsocks 已安装失败: %w", err)
	}
	return nil
}

// backendAddrFree reports whether addr can be bound right now. Tests replace it.
func (a *Agent) backendAddrFree(addr string) bool {
	if a.backendPortFree != nil {
		return a.backendPortFree(addr)
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// deployManagedBackend writes and starts the loopback shadowsocks server of a
// ShadowTLS port. Every value comes from the Manager; the agent generates none.
func (a *Agent) deployManagedBackend(port *model.Port, portID string) error {
	if err := shadowsocks.Validate(port.BackendAddr, port.BackendPassword, port.BackendCipher); err != nil {
		return fmt.Errorf("托管后端配置无效: %w", err)
	}
	if err := a.prepareBackendBinary(); err != nil {
		return err
	}

	serviceName := backendServiceName(portID)
	// A port that our own unit already listens on is fine; any other listener
	// would make the backend fail to start with an unhelpful systemd error.
	unitPath := filepath.Join(a.systemdDir(), serviceName+".service")
	if current, err := os.ReadFile(unitPath); err != nil || !strings.Contains(string(current), port.BackendAddr) {
		if !a.backendAddrFree(port.BackendAddr) {
			return fmt.Errorf("后端地址 %s 已被其他程序占用，请在面板中重新生成该端口的后端", port.BackendAddr)
		}
	}

	backendPort, _ := shadowsocks.LoopbackPort(port.BackendAddr)
	execStart := systemdExecStart(shadowsocks.BinaryPath,
		shadowsocks.ServerArgs(port.BackendAddr, port.BackendPassword, port.BackendCipher)...)
	return a.writeProxyUnit(serviceName, "Shadowsocks backend of "+shadowtlsUnitPrefix+portID,
		backendPort, execStart, nil, "")
}

// removeManagedBackend stops and deletes the managed backend of a port, if any.
func (a *Agent) removeManagedBackend(portID string) error {
	serviceName := backendServiceName(portID)
	unitPath := filepath.Join(a.systemdDir(), serviceName+".service")
	if _, err := os.Stat(unitPath); err != nil {
		return nil
	}
	if err := a.removeProxyInstance(serviceName, "", portID); err != nil {
		return err
	}
	if out, err := a.run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("清理后端后 systemd 重载失败: %v %s", err, strings.TrimSpace(out))
	}
	return nil
}
