// Package naive provides NaïveProxy installation and configuration.
package naive

import (
	"fmt"
	"path/filepath"
	"strings"

	"merit/internal/model"
)

const (
	DefaultVersion = "v131.0.6778.86-2"
	ServiceName    = "naive-proxy"
)

// GenerateConfig generates Caddy configuration for NaïveProxy.
func GenerateConfig(port *model.Port, nodeAddress string) string {
	var b strings.Builder

	domain := port.Domain
	if domain == "" {
		domain = nodeAddress
	}

	// Caddy JSON configuration for NaïveProxy
	b.WriteString("{\n")
	b.WriteString("  \"apps\": {\n")
	b.WriteString("    \"http\": {\n")
	b.WriteString("      \"servers\": {\n")
	b.WriteString("        \"srv0\": {\n")
	b.WriteString(fmt.Sprintf("          \"listen\": [\":%d\"],\n", port.Port))
	b.WriteString("          \"routes\": [{\n")
	b.WriteString("            \"handle\": [{\n")
	b.WriteString("              \"handler\": \"forward_proxy\",\n")
	b.WriteString("              \"hide_ip\": true,\n")
	b.WriteString("              \"hide_via\": true,\n")
	b.WriteString("              \"auth_user_deprecated\": \"" + port.Username + "\",\n")
	b.WriteString("              \"auth_pass_deprecated\": \"" + port.Password + "\",\n")
	b.WriteString("              \"probe_resistance\": {}\n")
	b.WriteString("            }]\n")
	b.WriteString("          }]\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("    },\n")

	// TLS configuration
	b.WriteString("    \"tls\": {\n")
	b.WriteString("      \"certificates\": {\n")
	b.WriteString("        \"automate\": [\"" + domain + "\"]\n")
	b.WriteString("      },\n")
	b.WriteString("      \"automation\": {\n")
	b.WriteString("        \"policies\": [{\n")

	if port.AutoACME {
		// Use ACME automatic certificate
		b.WriteString("          \"subjects\": [\"" + domain + "\"],\n")
		b.WriteString("          \"issuers\": [{\n")
		b.WriteString("            \"module\": \"acme\"\n")
		b.WriteString("          }]\n")
	} else if port.TLSCert != "" && port.TLSKey != "" {
		// Use manual certificate
		b.WriteString("          \"subjects\": [\"" + domain + "\"],\n")
		b.WriteString("          \"issuers\": [{\n")
		b.WriteString("            \"module\": \"internal\"\n")
		b.WriteString("          }]\n")
	}

	b.WriteString("        }]\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")

	return b.String()
}

// SimpleLink generates a share link for NaïveProxy.
// Format: naive+https://username:password@domain:port
func SimpleLink(n *model.Node, p *model.Port) string {
	domain := p.Domain
	if domain == "" {
		domain = n.Address
		if domain == "" {
			domain = n.SeenIP
		}
	}

	return fmt.Sprintf("naive+https://%s:%s@%s:%d",
		p.Username, p.Password, domain, p.Port)
}

// ConfigPath returns the config file path for a port instance.
func ConfigPath(portID string) string {
	return filepath.Join("/etc/merit/naive", portID+".json")
}

// CertPath returns certificate paths for a port instance.
func CertPath(portID string) (certFile, keyFile string) {
	certDir := filepath.Join("/etc/merit/naive/certs", portID)
	return filepath.Join(certDir, "cert.pem"), filepath.Join(certDir, "key.pem")
}
