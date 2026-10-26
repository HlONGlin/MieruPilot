package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"merit/internal/model"
	"merit/internal/proxy/anytls"
	"merit/internal/proxy/naive"
	"merit/internal/proxy/shadowtls"
)

// startProxyInstance starts a proxy instance based on its type.
func (a *Agent) startProxyInstance(port *model.Port) error {
	proxyType := port.ProxyType
	if proxyType == "" {
		proxyType = model.ProxyTypeMieru
	}

	switch proxyType {
	case model.ProxyTypeMieru:
		return nil // Handled by existing mita logic
	case model.ProxyTypeNaive:
		return a.startNaiveInstance(port)
	case model.ProxyTypeAnyTLS:
		return a.startAnyTLSInstance(port)
	case model.ProxyTypeShadowTLS:
		return a.startShadowTLSInstance(port)
	default:
		return fmt.Errorf("unsupported proxy type: %s", proxyType)
	}
}

// stopProxyInstance stops a proxy instance based on its type.
func (a *Agent) stopProxyInstance(port *model.Port) error {
	proxyType := port.ProxyType
	if proxyType == "" {
		proxyType = model.ProxyTypeMieru
	}

	serviceName := a.proxyServiceName(port)

	switch proxyType {
	case model.ProxyTypeMieru:
		return nil // Handled by existing mita logic
	default:
		_, err := a.run("systemctl", "stop", serviceName)
		return err
	}
}

// proxyServiceName returns the systemd service name for a proxy instance.
func (a *Agent) proxyServiceName(port *model.Port) string {
	proxyType := port.ProxyType
	if proxyType == "" {
		proxyType = model.ProxyTypeMieru
	}

	switch proxyType {
	case model.ProxyTypeNaive:
		return fmt.Sprintf("naive-proxy-%s", port.ID)
	case model.ProxyTypeAnyTLS:
		return fmt.Sprintf("anytls-%s", port.ID)
	case model.ProxyTypeShadowTLS:
		return fmt.Sprintf("shadowtls-%s", port.ID)
	default:
		return ""
	}
}

// startNaiveInstance starts a NaïveProxy instance.
func (a *Agent) startNaiveInstance(port *model.Port) error {
	if _, err := a.ensureNaive(); err != nil {
		return fmt.Errorf("ensure naive failed: %w", err)
	}

	configPath := naive.ConfigPath(port.ID)
	config := naive.GenerateConfig(port, a.getNodeAddress())

	// Write config file
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return err
	}

	// Write certificates if provided
	if !port.AutoACME && port.TLSCert != "" && port.TLSKey != "" {
		certFile, keyFile := naive.CertPath(port.ID)
		if err := os.MkdirAll(filepath.Dir(certFile), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(certFile, []byte(port.TLSCert), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(keyFile, []byte(port.TLSKey), 0600); err != nil {
			return err
		}
	}

	// Create systemd service
	serviceName := a.proxyServiceName(port)
	serviceContent := fmt.Sprintf(`[Unit]
Description=NaiveProxy Instance %s
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/caddy-naive run --config %s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, port.ID, configPath)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}

	_, _ = a.run("systemctl", "daemon-reload")
	_, err := a.run("systemctl", "enable", "--now", serviceName)
	return err
}

// startAnyTLSInstance starts an AnyTLS instance.
func (a *Agent) startAnyTLSInstance(port *model.Port) error {
	if _, err := a.ensureAnyTLS(); err != nil {
		return fmt.Errorf("ensure anytls failed: %w", err)
	}

	configPath := anytls.ConfigPath(port.ID)
	config := anytls.GenerateConfig(port)

	// Write config file
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return err
	}

	// Create systemd service
	serviceName := a.proxyServiceName(port)
	args := anytls.ServerArgs(port, configPath)
	serviceContent := fmt.Sprintf(`[Unit]
Description=AnyTLS Instance %s
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/anytls %s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, port.ID, strings.Join(args, " "))

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}

	_, _ = a.run("systemctl", "daemon-reload")
	_, err := a.run("systemctl", "enable", "--now", serviceName)
	return err
}

// startShadowTLSInstance starts a ShadowTLS v3 instance.
func (a *Agent) startShadowTLSInstance(port *model.Port) error {
	if _, err := a.ensureShadowTLS(); err != nil {
		return fmt.Errorf("ensure shadowtls failed: %w", err)
	}

	configPath := shadowtls.ConfigPath(port.ID)
	config := shadowtls.GenerateConfig(port)

	// Write config file
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return err
	}

	// Create systemd service
	serviceName := a.proxyServiceName(port)
	args := shadowtls.ServerArgs(port, configPath)
	serviceContent := fmt.Sprintf(`[Unit]
Description=ShadowTLS Instance %s
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/shadowtls %s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, port.ID, strings.Join(args, " "))

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}

	_, _ = a.run("systemctl", "daemon-reload")
	_, err := a.run("systemctl", "enable", "--now", serviceName)
	return err
}

// getNodeAddress returns the node's public address.
func (a *Agent) getNodeAddress() string {
	// This should be provided by the manager or detected
	// For now, return empty and let it be handled by IP detection
	return ""
}

// isProxyInstanceRunning checks if a proxy instance is running.
func (a *Agent) isProxyInstanceRunning(port *model.Port) bool {
	serviceName := a.proxyServiceName(port)
	if serviceName == "" {
		return false
	}

	out, err := exec.Command("systemctl", "is-active", serviceName).Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(out)) == "active"
}
