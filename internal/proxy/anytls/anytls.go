// Package anytls provides AnyTLS installation and configuration.
package anytls

import (
	"fmt"
	"path/filepath"

	"merit/internal/model"
)

const (
	DefaultVersion = "v0.2.0"
	ServiceName    = "anytls"
)

// GenerateConfig generates configuration for AnyTLS server.
// AnyTLS uses a simple config format
func GenerateConfig(port *model.Port) string {
	return fmt.Sprintf(`{
  "listen": "0.0.0.0:%d",
  "password": "%s"
}`, port.Port, port.Password)
}

// ServerArgs returns command-line arguments for AnyTLS server.
// AnyTLS typical usage: anytls server -l 0.0.0.0:port -p password
func ServerArgs(port *model.Port, configPath string) []string {
	return []string{
		"server",
		"-l", fmt.Sprintf("0.0.0.0:%d", port.Port),
		"-p", port.Password,
	}
}

// SimpleLink generates a share link for AnyTLS.
// Format: anytls://password@host:port
func SimpleLink(n *model.Node, p *model.Port) string {
	host := n.Address
	if host == "" {
		host = n.SeenIP
	}

	return fmt.Sprintf("anytls://%s@%s:%d", p.Password, host, p.Port)
}

// ConfigPath returns the config file path for a port instance.
func ConfigPath(portID string) string {
	return filepath.Join("/etc/merit/anytls", portID+".json")
}
