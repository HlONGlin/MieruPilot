// Package proxy provides a unified interface for managing different proxy types.
package proxy

import (
	"fmt"

	"merit/internal/model"
	"merit/internal/proxy/anytls"
	"merit/internal/proxy/naive"
	"merit/internal/proxy/shadowtls"
)

// GetProxyType returns the proxy type for a port, defaulting to mieru for backward compatibility.
func GetProxyType(p *model.Port) string {
	if p.ProxyType == "" {
		return model.ProxyTypeMieru
	}
	return p.ProxyType
}

// GenerateShareLink generates a share link for the given proxy type.
func GenerateShareLink(n *model.Node, p *model.Port) string {
	proxyType := GetProxyType(p)

	switch proxyType {
	case model.ProxyTypeNaive:
		return naive.SimpleLink(n, p)
	case model.ProxyTypeAnyTLS:
		return anytls.SimpleLink(n, p)
	case model.ProxyTypeShadowTLS:
		return shadowtls.SimpleLink(n, p)
	case model.ProxyTypeMieru:
		// Import would cause circular dependency, handle in mieru package
		return ""
	default:
		return ""
	}
}

// ValidatePort validates port configuration based on proxy type.
func ValidatePort(p *model.Port) error {
	proxyType := GetProxyType(p)

	switch proxyType {
	case model.ProxyTypeNaive:
		if p.Domain == "" && !p.AutoACME {
			return fmt.Errorf("NaïveProxy requires domain for TLS certificate")
		}
		if !p.AutoACME && (p.TLSCert == "" || p.TLSKey == "") {
			return fmt.Errorf("NaïveProxy requires TLS certificate when AutoACME is disabled")
		}
	case model.ProxyTypeShadowTLS:
		if p.HandshakeAddr == "" {
			return fmt.Errorf("ShadowTLS requires handshake server address")
		}
	}

	return nil
}

// SupportedProxyTypes returns a list of all supported proxy types.
func SupportedProxyTypes() []string {
	return []string{
		model.ProxyTypeMieru,
		model.ProxyTypeNaive,
		model.ProxyTypeAnyTLS,
		model.ProxyTypeShadowTLS,
	}
}
