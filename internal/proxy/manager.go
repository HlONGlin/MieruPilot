// Package proxy provides a unified interface for managing different proxy types.
package proxy

import (
	"merit/internal/model"
	"merit/internal/proxy/anytls"
	"merit/internal/proxy/naive"
	"merit/internal/proxy/shadowtls"
)

// Manager provides unified proxy management operations.
type Manager struct{}

// NewManager creates a new proxy manager.
func NewManager() *Manager {
	return &Manager{}
}

// ShareLinkParams contains parameters for generating share links.
type ShareLinkParams struct {
	Host          string
	Port          int
	Username      string
	Password      string
	Domain        string
	HandshakeAddr string
}

// GenerateShareLink generates a share link for the given proxy type.
func (m *Manager) GenerateShareLink(proxyType string, params ShareLinkParams) string {
	port := &model.Port{
		ProxyType:     proxyType,
		Port:          params.Port,
		Username:      params.Username,
		Password:      params.Password,
		Domain:        params.Domain,
		HandshakeAddr: params.HandshakeAddr,
	}
	node := &model.Node{Address: params.Host}
	return GenerateShareLink(node, port)
}

// GetProxyType returns the proxy type for a port, defaulting to mieru for backward compatibility.
func GetProxyType(p *model.Port) string {
	return model.NormalizeProxyType(p.ProxyType)
}

// GenerateShareLink generates a share link for the given node and port. NaïveProxy
// needs both the username and the password, AnyTLS and ShadowTLS only the
// password, so each protocol renders its own URI with proper escaping.
func GenerateShareLink(n *model.Node, p *model.Port) string {
	switch GetProxyType(p) {
	case model.ProxyTypeNaive:
		return naive.SimpleLink(n, p)
	case model.ProxyTypeAnyTLS:
		return anytls.SimpleLink(n, p)
	case model.ProxyTypeShadowTLS:
		return shadowtls.SimpleLink(n, p)
	default:
		// mieru links are built by the mieru package to avoid a cycle.
		return ""
	}
}

// ValidatePort validates port configuration based on proxy type.
func ValidatePort(p *model.Port) error {
	switch GetProxyType(p) {
	case model.ProxyTypeNaive:
		return naive.Validate(p)
	case model.ProxyTypeAnyTLS:
		return anytls.Validate(p)
	case model.ProxyTypeShadowTLS:
		return shadowtls.Validate(p)
	default:
		return nil
	}
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
