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
func GenerateConfig(port *model.Port) string {
	// AnyTLS uses command-line arguments, not a config file
	// Return the arguments as a pseudo-config
	return fmt.Sprintf(`{
  "listen": ":%d",
  "password": "%s",
  "protocol": "%s"
}`, port.Port, port.Password, port.Protocol)
}

// ServerArgs returns command-line arguments for AnyTLS server.
func ServerArgs(port *model.Port) []string {
	args := []string{
		"server",
		"--listen", fmt.Sprintf(":%d", port.Port),
		"--password", port.Password,
	}

	if port.Protocol == model.ProtocolUDP {
		args = append(args, "--udp")
	}

	return args
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
