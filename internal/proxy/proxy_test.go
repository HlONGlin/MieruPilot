package proxy

import (
	"testing"

	"merit/internal/model"
)

func TestGetProxyType(t *testing.T) {
	tests := []struct {
		name     string
		port     *model.Port
		expected string
	}{
		{
			name:     "default to mieru when empty",
			port:     &model.Port{},
			expected: model.ProxyTypeMieru,
		},
		{
			name:     "explicit mieru",
			port:     &model.Port{ProxyType: model.ProxyTypeMieru},
			expected: model.ProxyTypeMieru,
		},
		{
			name:     "naive proxy",
			port:     &model.Port{ProxyType: model.ProxyTypeNaive},
			expected: model.ProxyTypeNaive,
		},
		{
			name:     "anytls",
			port:     &model.Port{ProxyType: model.ProxyTypeAnyTLS},
			expected: model.ProxyTypeAnyTLS,
		},
		{
			name:     "shadowtls",
			port:     &model.Port{ProxyType: model.ProxyTypeShadowTLS},
			expected: model.ProxyTypeShadowTLS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetProxyType(tt.port)
			if result != tt.expected {
				t.Errorf("GetProxyType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    *model.Port
		wantErr bool
	}{
		{
			name: "mieru port valid",
			port: &model.Port{
				ProxyType: model.ProxyTypeMieru,
				Port:      8080,
			},
			wantErr: false,
		},
		{
			name: "naive with auto acme valid",
			port: &model.Port{
				ProxyType: model.ProxyTypeNaive,
				Port:      443,
				Domain:    "example.com",
				AutoACME:  true,
			},
			wantErr: false,
		},
		{
			name: "naive with manual cert valid",
			port: &model.Port{
				ProxyType: model.ProxyTypeNaive,
				Port:      443,
				Domain:    "example.com",
				TLSCert:   "cert content",
				TLSKey:    "key content",
			},
			wantErr: false,
		},
		{
			name: "naive without domain invalid",
			port: &model.Port{
				ProxyType: model.ProxyTypeNaive,
				Port:      443,
			},
			wantErr: true,
		},
		{
			name: "naive without cert invalid",
			port: &model.Port{
				ProxyType: model.ProxyTypeNaive,
				Port:      443,
				Domain:    "example.com",
				AutoACME:  false,
			},
			wantErr: true,
		},
		{
			name: "shadowtls with handshake valid",
			port: &model.Port{
				ProxyType:     model.ProxyTypeShadowTLS,
				Port:          443,
				HandshakeAddr: "cloudflare.com:443",
			},
			wantErr: false,
		},
		{
			name: "shadowtls without handshake invalid",
			port: &model.Port{
				ProxyType: model.ProxyTypeShadowTLS,
				Port:      443,
			},
			wantErr: true,
		},
		{
			name: "anytls valid",
			port: &model.Port{
				ProxyType: model.ProxyTypeAnyTLS,
				Port:      8443,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateShareLink(t *testing.T) {
	tests := []struct {
		name string
		node *model.Node
		port *model.Port
	}{
		{
			name: "naive share link",
			node: &model.Node{Address: "example.com"},
			port: &model.Port{
				ProxyType: model.ProxyTypeNaive,
				Port:      443,
				Domain:    "example.com",
				Username:  "user",
				Password:  "pass",
			},
		},
		{
			name: "anytls share link",
			node: &model.Node{Address: "192.168.1.1"},
			port: &model.Port{
				ProxyType: model.ProxyTypeAnyTLS,
				Port:      8443,
				Password:  "secret",
			},
		},
		{
			name: "shadowtls share link",
			node: &model.Node{Address: "192.168.1.1"},
			port: &model.Port{
				ProxyType:     model.ProxyTypeShadowTLS,
				Port:          443,
				Password:      "secret",
				HandshakeAddr: "cloudflare.com:443",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := GenerateShareLink(tt.node, tt.port)
			if link == "" {
				t.Errorf("GenerateShareLink() returned empty string")
			}
			t.Logf("Share link: %s", link)
		})
	}
}

func TestSupportedProxyTypes(t *testing.T) {
	types := SupportedProxyTypes()

	expected := map[string]bool{
		model.ProxyTypeMieru:     true,
		model.ProxyTypeNaive:     true,
		model.ProxyTypeAnyTLS:    true,
		model.ProxyTypeShadowTLS: true,
	}

	if len(types) != len(expected) {
		t.Errorf("SupportedProxyTypes() returned %d types, want %d", len(types), len(expected))
	}

	for _, typ := range types {
		if !expected[typ] {
			t.Errorf("Unexpected proxy type: %s", typ)
		}
	}
}
