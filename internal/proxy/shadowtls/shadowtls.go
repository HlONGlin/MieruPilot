// Package shadowtls provides ShadowTLS v3 installation and configuration.
package shadowtls

import (
	"fmt"
	"path/filepath"

	"merit/internal/model"
)

const (
	DefaultVersion = "v0.3.5"
	ServiceName    = "shadowtls"
)

// GenerateConfig generates configuration for ShadowTLS v3 server.
func GenerateConfig(port *model.Port) string {
	handshake := port.HandshakeAddr
	if handshake == "" {
		handshake = "example.com:443"
	}

	return fmt.Sprintf(`{
  "listen": ":%d",
  "server": {
    "password": "%s",
    "handshake": "%s"
  }
}`, port.Port, port.Password, handshake)
}

// ServerArgs returns command-line arguments for ShadowTLS v3 server.
func ServerArgs(port *model.Port, configPath string) []string {
	return []string{
		"server",
		"--config", configPath,
	}
}

// SimpleLink generates a share link for ShadowTLS v3.
// Format: shadowtls://password@host:port?handshake=example.com:443
func SimpleLink(n *model.Node, p *model.Port) string {
	host := n.Address
	if host == "" {
		host = n.SeenIP
	}

	handshake := p.HandshakeAddr
	if handshake == "" {
		handshake = "example.com:443"
	}

	return fmt.Sprintf("shadowtls://%s@%s:%d?handshake=%s",
		p.Password, host, p.Port, handshake)
}

// ConfigPath returns the config file path for a port instance.
func ConfigPath(portID string) string {
	return filepath.Join("/etc/merit/shadowtls", portID+".json")
}
