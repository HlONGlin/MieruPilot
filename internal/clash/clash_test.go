package clash

import (
	"strings"
	"testing"

	"merit/internal/model"
)

func TestBuildEntries(t *testing.T) {
	tests := []struct {
		name     string
		node     *model.Node
		wantLen  int
		wantType string
	}{
		{
			name: "mieru proxy",
			node: &model.Node{
				ID:      "test-node",
				Name:    "Test Node",
				Address: "1.2.3.4",
				Ports: []*model.Port{
					{
						ID:        "port1",
						Port:      8080,
						Protocol:  model.ProtocolTCP,
						Username:  "user1",
						Password:  "pass1",
						Enabled:   true,
						ProxyType: model.ProxyTypeMieru,
					},
				},
			},
			wantLen:  1,
			wantType: model.ProxyTypeMieru,
		},
		{
			name: "naive proxy",
			node: &model.Node{
				ID:      "test-node",
				Name:    "Test Node",
				Address: "1.2.3.4",
				Ports: []*model.Port{
					{
						ID:        "port1",
						Port:      443,
						Username:  "user1",
						Password:  "pass1",
						Domain:    "example.com",
						Enabled:   true,
						ProxyType: model.ProxyTypeNaive,
					},
				},
			},
			wantLen:  1,
			wantType: model.ProxyTypeNaive,
		},
		{
			name: "mixed proxies",
			node: &model.Node{
				ID:      "test-node",
				Name:    "Test Node",
				Address: "1.2.3.4",
				Ports: []*model.Port{
					{
						ID:        "port1",
						Port:      8080,
						Username:  "user1",
						Password:  "pass1",
						Enabled:   true,
						ProxyType: model.ProxyTypeMieru,
					},
					{
						ID:        "port2",
						Port:      443,
						Username:  "user2",
						Password:  "pass2",
						Domain:    "example.com",
						Enabled:   true,
						ProxyType: model.ProxyTypeNaive,
					},
					{
						ID:        "port3",
						Port:      8443,
						Password:  "pass3",
						Enabled:   true,
						ProxyType: model.ProxyTypeAnyTLS,
					},
				},
			},
			wantLen: 3,
		},
		{
			name: "disabled ports excluded",
			node: &model.Node{
				ID:      "test-node",
				Address: "1.2.3.4",
				Ports: []*model.Port{
					{
						ID:        "port1",
						Port:      8080,
						Username:  "user1",
						Password:  "pass1",
						Enabled:   false,
						ProxyType: model.ProxyTypeMieru,
					},
					{
						ID:        "port2",
						Port:      443,
						Username:  "user2",
						Password:  "pass2",
						Enabled:   true,
						ProxyType: model.ProxyTypeMieru,
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "no address",
			node: &model.Node{
				ID:   "test-node",
				Name: "Test Node",
				Ports: []*model.Port{
					{
						ID:       "port1",
						Port:     8080,
						Enabled:  true,
						Username: "user",
						Password: "pass",
					},
				},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := BuildEntries(tt.node)
			if len(entries) != tt.wantLen {
				t.Errorf("BuildEntries() got %d entries, want %d", len(entries), tt.wantLen)
			}
			if tt.wantType != "" && len(entries) > 0 {
				if entries[0].ProxyType != tt.wantType {
					t.Errorf("BuildEntries() got proxy type %s, want %s", entries[0].ProxyType, tt.wantType)
				}
			}
		})
	}
}

func TestGenerateYAML(t *testing.T) {
	entries := []Entry{
		{
			Name:      "test-mieru",
			ProxyType: model.ProxyTypeMieru,
			Server:    "1.2.3.4",
			Port:      8080,
			Protocol:  model.ProtocolTCP,
			Username:  "user1",
			Password:  "pass1",
		},
		{
			Name:      "test-naive",
			ProxyType: model.ProxyTypeNaive,
			Server:    "1.2.3.4",
			Port:      443,
			Username:  "user2",
			Password:  "pass2",
			Domain:    "example.com",
			TLS:       true,
		},
	}

	yaml := GenerateYAML(entries)

	// Check for required sections
	requiredSections := []string{
		"proxies:",
		"proxy-groups:",
		"rules:",
		"type: mieru",
		"type: http",
		"tls: true",
	}

	for _, section := range requiredSections {
		if !strings.Contains(yaml, section) {
			t.Errorf("GenerateYAML() missing section: %s", section)
		}
	}
}

func TestRenderProxyEntry_Mieru(t *testing.T) {
	entry := Entry{
		Name:      "test-mieru",
		ProxyType: model.ProxyTypeMieru,
		Server:    "1.2.3.4",
		Port:      8080,
		Protocol:  model.ProtocolTCP,
		Username:  "user1",
		Password:  "pass1",
	}

	result := renderProxyEntry(entry)

	required := []string{
		"name: test-mieru",
		"type: mieru",
		"server: 1.2.3.4",
		"port: 8080",
		"username: user1",
		"password: pass1",
	}

	for _, r := range required {
		if !strings.Contains(result, r) {
			t.Errorf("renderProxyEntry() missing: %s", r)
		}
	}

	// Check transport is present (as a word boundary check)
	if !strings.Contains(result, "transport:") {
		t.Errorf("renderProxyEntry() missing transport field")
	}
}

func TestRenderProxyEntry_Naive(t *testing.T) {
	entry := Entry{
		Name:      "test-naive",
		ProxyType: model.ProxyTypeNaive,
		Server:    "1.2.3.4",
		Port:      443,
		Username:  "user1",
		Password:  "pass1",
		Domain:    "example.com",
		TLS:       true,
	}

	result := renderProxyEntry(entry)

	required := []string{
		"name: test-naive",
		"type: http",
		"server: example.com",
		"port: 443",
		"username: user1",
		"password: pass1",
		"tls: true",
	}

	for _, r := range required {
		if !strings.Contains(result, r) {
			t.Errorf("renderProxyEntry() missing: %s", r)
		}
	}
}

func TestRenderProxyEntry_AnyTLS(t *testing.T) {
	entry := Entry{
		Name:      "test-anytls",
		ProxyType: model.ProxyTypeAnyTLS,
		Server:    "1.2.3.4",
		Port:      8443,
		Password:  "mypassword",
	}

	result := renderProxyEntry(entry)

	required := []string{
		"name: test-anytls",
		"type: trojan",
		"server: 1.2.3.4",
		"port: 8443",
		"password: mypassword",
		"udp: true",
	}

	for _, r := range required {
		if !strings.Contains(result, r) {
			t.Errorf("renderProxyEntry() missing: %s", r)
		}
	}
}

func TestRenderProxyEntry_ShadowTLS(t *testing.T) {
	entry := Entry{
		Name:          "test-shadowtls",
		ProxyType:     model.ProxyTypeShadowTLS,
		Server:        "1.2.3.4",
		Port:          8080,
		Password:      "mypassword",
		HandshakeAddr: "cloudflare.com:443",
		SNI:           "example.com",
	}

	result := renderProxyEntry(entry)

	required := []string{
		"name: test-shadowtls",
		"type: ss",
		"server: 1.2.3.4",
		"port: 8080",
		"password: mypassword",
		"cipher: aes-256-gcm",
		"plugin: shadow-tls",
		"version: 3",
		"sni: example.com",
	}

	for _, r := range required {
		if !strings.Contains(result, r) {
			t.Errorf("renderProxyEntry() missing: %s", r)
		}
	}

	// Check that plugin-opts and host are present (host is in plugin-opts section)
	if !strings.Contains(result, "plugin-opts:") {
		t.Errorf("renderProxyEntry() missing plugin-opts section")
	}
	if !strings.Contains(result, "cloudflare.com:443") {
		t.Errorf("renderProxyEntry() missing handshake address")
	}
}

func TestYamlString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with space", "with space"},
		{"with:colon", `"with:colon"`},
		{`with"quote`, `"with\"quote"`},
		{"with'apostrophe", `"with'apostrophe"`},
		{"with\nnewline", "\"with\\nnewline\""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := yamlString(tt.input)
			if got != tt.want {
				t.Errorf("yamlString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
