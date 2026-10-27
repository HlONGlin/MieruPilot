package agent

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"merit/internal/model"
	"merit/internal/proxy/anytls"
	"merit/internal/proxy/naive"
	"merit/internal/proxy/shadowtls"
)

// proxyUnitDir is where the per-port proxy units are written.
const proxyUnitDir = "/etc/systemd/system"

// Unit name prefixes. They double as the marker used to find instances that have
// to be cleaned up later.
const (
	naiveUnitPrefix     = "naive-proxy-"
	anytlsUnitPrefix    = "anytls-"
	shadowtlsUnitPrefix = "shadowtls-"
	// shadowsocksBackendPrefix names the loopback backend of a managed ShadowTLS
	// port. It is not a proxy type of its own, so parseProxyUnitName ignores it.
	shadowsocksBackendPrefix = "shadowsocks-backend-"
)

// safePortID turns a port ID into a token that is safe inside a systemd unit name
// and a file path.
func safePortID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

// proxyServiceName returns the systemd unit name (without ".service") of a proxy
// instance, or "" for mieru ports, which are handled by the mita instances.
func (a *Agent) proxyServiceName(port *model.Port) string {
	switch model.NormalizeProxyType(port.ProxyType) {
	case model.ProxyTypeNaive:
		return naiveUnitPrefix + safePortID(port.ID)
	case model.ProxyTypeAnyTLS:
		return anytlsUnitPrefix + safePortID(port.ID)
	case model.ProxyTypeShadowTLS:
		return shadowtlsUnitPrefix + safePortID(port.ID)
	default:
		return ""
	}
}

// parseProxyUnitName splits a proxy unit name into its proxy type and port ID.
func parseProxyUnitName(unit string) (proxyType, portID string, ok bool) {
	for _, candidate := range []struct {
		prefix string
		typ    string
	}{
		{naiveUnitPrefix, model.ProxyTypeNaive},
		{anytlsUnitPrefix, model.ProxyTypeAnyTLS},
		{shadowtlsUnitPrefix, model.ProxyTypeShadowTLS},
	} {
		if strings.HasPrefix(unit, candidate.prefix) {
			return candidate.typ, strings.TrimPrefix(unit, candidate.prefix), true
		}
	}
	return "", "", false
}

// startProxyInstance starts a proxy instance based on its type.
func (a *Agent) startProxyInstance(port *model.Port) error {
	switch model.NormalizeProxyType(port.ProxyType) {
	case model.ProxyTypeMieru:
		return nil // handled by the mita instance logic
	case model.ProxyTypeNaive:
		return a.startNaiveInstance(port)
	case model.ProxyTypeAnyTLS:
		return a.startAnyTLSInstance(port)
	case model.ProxyTypeShadowTLS:
		return a.startShadowTLSInstance(port)
	default:
		return fmt.Errorf("不支持的代理类型: %s", port.ProxyType)
	}
}

// stopProxyInstance stops a proxy instance and removes its unit and files.
func (a *Agent) stopProxyInstance(port *model.Port) error {
	serviceName := a.proxyServiceName(port)
	if serviceName == "" {
		return nil // mieru ports are stopped by the mita instance logic
	}
	proxyType, portID := model.NormalizeProxyType(port.ProxyType), safePortID(port.ID)
	if err := a.removeProxyInstance(serviceName, proxyType, portID); err != nil {
		return err
	}
	if proxyType == model.ProxyTypeShadowTLS {
		return a.removeManagedBackend(portID)
	}
	return nil
}

// removeProxyInstance stops a service, deletes its unit and the per-port files
// that hold credentials.
func (a *Agent) removeProxyInstance(serviceName, proxyType, portID string) error {
	unitPath := filepath.Join(a.systemdDir(), serviceName+".service")
	if _, err := os.Stat(unitPath); err == nil {
		if out, err := a.run("systemctl", "disable", "--now", serviceName+".service"); err != nil {
			// A unit that is already gone is fine; a still running one is not.
			if active, _ := a.serviceActive(serviceName); active {
				return fmt.Errorf("停止 %s 失败: %v %s", serviceName, err, strings.TrimSpace(out))
			}
		}
		if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除 %s 失败: %w", serviceName, err)
		}
	}
	_ = os.Remove(unitPath + ".applied")
	a.removeProxyInstanceFiles(proxyType, portID)
	return nil
}

// removeProxyInstanceFiles deletes the generated configuration and credentials of
// one proxy instance.
func (a *Agent) removeProxyInstanceFiles(proxyType, portID string) {
	switch proxyType {
	case model.ProxyTypeNaive:
		_ = os.Remove(naive.ConfigPath(portID))
		certFile, keyFile := naive.CertPath(portID)
		_ = os.Remove(certFile)
		_ = os.Remove(keyFile)
		_ = os.Remove(filepath.Dir(certFile))
	case model.ProxyTypeAnyTLS:
		_ = os.Remove(anytls.ConfigPath(portID))
	case model.ProxyTypeShadowTLS:
		_ = os.Remove(shadowtls.ConfigPath(portID))
	}
}

// startNaiveInstance starts one NaïveProxy (Caddy) instance.
func (a *Agent) startNaiveInstance(port *model.Port) error {
	if err := naive.Validate(port); err != nil {
		return err
	}
	if _, err := a.ensureNaive(); err != nil {
		return fmt.Errorf("确保 NaïveProxy 已安装失败: %w", err)
	}

	portID := safePortID(port.ID)
	// Certificates have to exist before Caddy starts, because the config loads
	// them from disk instead of letting Caddy issue self-signed ones.
	if !port.AutoACME {
		certFile, keyFile := naive.CertPath(portID)
		if err := os.MkdirAll(filepath.Dir(certFile), 0o700); err != nil {
			return fmt.Errorf("创建证书目录失败: %w", err)
		}
		if err := writeFileAtomic(certFile, []byte(port.TLSCert), 0o600); err != nil {
			return fmt.Errorf("写入证书失败: %w", err)
		}
		if err := writeFileAtomic(keyFile, []byte(port.TLSKey), 0o600); err != nil {
			return fmt.Errorf("写入私钥失败: %w", err)
		}
	}

	configPort := *port
	configPort.ID = portID
	config, err := naive.GenerateConfig(&configPort)
	if err != nil {
		return err
	}
	configPath := naive.ConfigPath(portID)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return err
	}
	if err := writeFileAtomic(configPath, []byte(config), 0o640); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}

	serviceName := a.proxyServiceName(port)
	execStart := systemdExecStart(naive.BinaryPath, "run", "--config", configPath)
	// Include certificate contents so renewing a manual certificate reloads it.
	state := configDigest([]byte(config + "\x00" + port.TLSCert + "\x00" + port.TLSKey))
	return a.writeProxyUnit(serviceName, "NaiveProxy instance "+serviceName, port.Port, execStart, nil, state)
}

// startAnyTLSInstance starts one AnyTLS instance.
func (a *Agent) startAnyTLSInstance(port *model.Port) error {
	if err := anytls.Validate(port); err != nil {
		return err
	}
	if _, err := a.ensureAnyTLS(); err != nil {
		return fmt.Errorf("确保 AnyTLS 已安装失败: %w", err)
	}

	serviceName := a.proxyServiceName(port)
	execStart := systemdExecStart(anytls.ServerBinaryPath, anytls.ServerArgs(port)...)
	// AnyTLS keeps no state on disk, so the unit is the whole instance.
	return a.writeProxyUnit(serviceName, "AnyTLS instance "+serviceName, port.Port, execStart, nil, "")
}

// startShadowTLSInstance starts one ShadowTLS v3 instance. When the Manager marks
// the backend as managed, the loopback shadowsocks server is deployed first and
// the ShadowTLS unit is ordered after it. A manual backend is never touched.
func (a *Agent) startShadowTLSInstance(port *model.Port) error {
	if err := shadowtls.Validate(port); err != nil {
		return err
	}
	if _, err := a.ensureShadowTLS(); err != nil {
		return fmt.Errorf("确保 ShadowTLS 已安装失败: %w", err)
	}

	portID := safePortID(port.ID)
	var after []string
	if port.BackendManaged {
		if err := a.deployManagedBackend(port, portID); err != nil {
			return fmt.Errorf("部署 ShadowTLS 托管后端失败: %w", err)
		}
		after = append(after, backendServiceName(portID)+".service")
	} else if err := a.removeManagedBackend(portID); err != nil {
		// Switched to a manual backend: the old managed one must not linger.
		return err
	}

	serviceName := a.proxyServiceName(port)
	args := shadowtls.ServerArgs(port)
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--listen" {
			args[i+1] = shadowTLSListenAddr(port.Port)
			break
		}
	}
	execStart := systemdExecStart(shadowtls.BinaryPath, args...)
	// MONOIO_FORCE_LEGACY_DRIVER keeps shadow-tls working on hosts whose kernel or
	// container runtime does not allow io_uring.
	return a.writeProxyUnitAfter(serviceName, "ShadowTLS instance "+serviceName, port.Port, execStart,
		[]string{"MONOIO_FORCE_LEGACY_DRIVER=1"}, "", after)
}

// shadow-tls inherits the Linux IPV6_V6ONLY default. Select [::] only when it
// also accepts IPv4, so enabling IPv6 cannot silently drop existing IPv4 users.
func shadowTLSListenAddr(port int) string {
	v6Only, err := os.ReadFile("/proc/sys/net/ipv6/bindv6only")
	if err == nil && strings.TrimSpace(string(v6Only)) == "0" {
		if listener, err := net.Listen("tcp6", "[::1]:0"); err == nil {
			_ = listener.Close()
			return fmt.Sprintf("[::]:%d", port)
		}
	}
	return shadowtls.ListenAddr(port)
}

// writeProxyUnit writes a systemd unit, reloads systemd and starts the service.
// The port is recorded in a comment so status reports can map a unit back to its
// port without another registry.
func (a *Agent) writeProxyUnit(serviceName, description string, port int, execStart string, env []string, configState string) error {
	return a.writeProxyUnitAfter(serviceName, description, port, execStart, env, configState, nil)
}

// writeProxyUnitAfter is writeProxyUnit for a unit that must start after other
// units. Wants (not Requires) is used so a restarting backend does not take the
// ShadowTLS listener down with it; clients simply reconnect.
func (a *Agent) writeProxyUnitAfter(serviceName, description string, port int, execStart string, env []string, configState string, after []string) error {
	if serviceName == "" {
		return fmt.Errorf("无法为实例生成服务名")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "#MeritPort=%d\n", port)
	b.WriteString("[Unit]\n")
	fmt.Fprintf(&b, "Description=%s\n", description)
	b.WriteString("After=network-online.target" + joinUnits(after) + "\n")
	b.WriteString("Wants=network-online.target" + joinUnits(after) + "\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	for _, entry := range env {
		fmt.Fprintf(&b, "Environment=%s\n", systemdEscapeArg(entry))
	}
	fmt.Fprintf(&b, "ExecStart=%s\n", execStart)
	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=5\n")
	b.WriteString("StandardOutput=journal\n")
	b.WriteString("StandardError=journal\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=multi-user.target\n")

	if err := os.MkdirAll(a.systemdDir(), 0o755); err != nil {
		return err
	}
	unitPath := filepath.Join(a.systemdDir(), serviceName+".service")
	raw := []byte(b.String())
	old, readErr := os.ReadFile(unitPath)
	changed := readErr != nil || string(old) != string(raw)
	digest := configDigest(append(append([]byte{}, raw...), []byte(configState)...))
	applied, appliedErr := os.ReadFile(unitPath + ".applied")
	needsApply := changed || appliedErr != nil || strings.TrimSpace(string(applied)) != digest
	// Unit files contain proxy passwords and must not be world-readable.
	if changed {
		if err := writeFileAtomic(unitPath, raw, 0o600); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", serviceName, err)
		}
	} else if err := os.Chmod(unitPath, 0o600); err != nil {
		return err
	}
	if needsApply {
		if out, err := a.run("systemctl", "daemon-reload"); err != nil {
			return fmt.Errorf("systemd 重载失败: %v %s", err, strings.TrimSpace(out))
		}
	}
	if out, err := a.run("systemctl", "enable", serviceName+".service"); err != nil {
		return fmt.Errorf("启用 %s 失败: %v %s", serviceName, err, strings.TrimSpace(out))
	}
	active, _ := a.serviceActive(serviceName)
	if needsApply || !active {
		verb := "restart"
		if !active {
			verb = "start"
		}
		if out, err := a.run("systemctl", verb, serviceName+".service"); err != nil {
			return fmt.Errorf("%s %s 失败: %v %s", verb, serviceName, err, strings.TrimSpace(out))
		}
	}
	if active, err := a.serviceActive(serviceName); err != nil || !active {
		return fmt.Errorf("%s 启动后未处于 active 状态", serviceName)
	}
	return writeFileAtomic(unitPath+".applied", []byte(digest), 0o600)
}

// joinUnits renders extra unit names for an After=/Wants= line.
func joinUnits(units []string) string {
	if len(units) == 0 {
		return ""
	}
	return " " + strings.Join(units, " ")
}

// serviceActive reports whether a unit is currently active.
func (a *Agent) serviceActive(serviceName string) (bool, error) {
	out, err := a.run("systemctl", "is-active", serviceName+".service")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "active", nil
}

// isProxyInstanceRunning checks if a proxy instance is running.
func (a *Agent) isProxyInstanceRunning(port *model.Port) bool {
	serviceName := a.proxyServiceName(port)
	if serviceName == "" {
		return false
	}
	active, err := a.serviceActive(serviceName)
	if err != nil {
		return false
	}
	return active
}

// proxyUnitPort reads the port number recorded in a proxy unit file.
func (a *Agent) proxyUnitPort(serviceName string) int {
	data, err := os.ReadFile(filepath.Join(a.systemdDir(), serviceName+".service"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "#MeritPort=") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "#MeritPort=")))
		if err != nil {
			return 0
		}
		return port
	}
	return 0
}
