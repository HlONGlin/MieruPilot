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
			// mihomo has no NaiveProxy outbound, so the port must not be emitted.
			name: "naive proxy skipped",
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
			wantLen: 0,
		},
		{
			name: "shadowtls without backend password skipped",
			node: &model.Node{
				ID:      "test-node",
				Address: "1.2.3.4",
				Ports: []*model.Port{
					{
						ID:            "port1",
						Port:          8443,
						Password:      "stls",
						BackendAddr:   "127.0.0.1:8388",
						HandshakeAddr: "cloudflare.com:443",
						Enabled:       true,
						ProxyType:     model.ProxyTypeShadowTLS,
					},
				},
			},
			wantLen: 0,
		},
		{
			name: "shadowtls with backend credentials",
			node: &model.Node{
				ID:      "test-node",
				Address: "1.2.3.4",
				Ports: []*model.Port{
					{
						ID:              "port1",
						Port:            8443,
						Password:        "stls",
						BackendAddr:     "127.0.0.1:8388",
						BackendPassword: "ss-secret",
						Enabled:         true,
						ProxyType:       model.ProxyTypeShadowTLS,
					},
				},
			},
			wantLen:  1,
			wantType: model.ProxyTypeShadowTLS,
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
			wantLen: 2,
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

func TestUnsupportedPorts(t *testing.T) {
	n := &model.Node{
		ID:      "test-node",
		Name:    "Test Node",
		Address: "1.2.3.4",
		Ports: []*model.Port{
			{ID: "p1", Port: 443, Enabled: true, ProxyType: model.ProxyTypeNaive, Domain: "example.com"},
			{ID: "p2", Port: 8443, Enabled: true, ProxyType: model.ProxyTypeShadowTLS, Password: "stls"},
			{ID: "p3", Port: 8444, Enabled: true, ProxyType: model.ProxyTypeShadowTLS, Password: "stls", BackendPassword: "ss"},
			{ID: "p4", Port: 8080, Enabled: true, ProxyType: model.ProxyTypeMieru},
			{ID: "p5", Port: 443, Enabled: false, ProxyType: model.ProxyTypeNaive},
		},
	}

	notes := UnsupportedPorts(n)
	if len(notes) != 2 {
		t.Fatalf("UnsupportedPorts() got %d notes, want 2: %v", len(notes), notes)
	}
	if !strings.Contains(notes[0], "NaïveProxy") {
		t.Errorf("first note should mention NaïveProxy, got %q", notes[0])
	}
	if !strings.Contains(notes[1], "后端 shadowsocks 密码") {
		t.Errorf("second note should mention the missing backend password, got %q", notes[1])
	}
}

func TestGenerateYAMLWithNotes(t *testing.T) {
	yaml := GenerateYAMLWithNotes(nil, []string{"node-port (NaïveProxy)：mihomo 不支持该协议"})

	if !strings.Contains(yaml, "# 未包含: node-port (NaïveProxy)") {
		t.Errorf("GenerateYAMLWithNotes() missing note comment:\n%s", yaml)
	}
	if strings.Index(yaml, "# 未包含") > strings.Index(yaml, "mixed-port") {
		t.Errorf("note comment must be written before the configuration body:\n%s", yaml)
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
		"type: anytls",
		"server: 1.2.3.4",
		"port: 8443",
		"password: mypassword",
		"client-fingerprint: chrome",
		"udp: true",
	}

	for _, r := range required {
		if !strings.Contains(result, r) {
			t.Errorf("renderProxyEntry() missing: %s", r)
		}
	}
	if strings.Contains(result, "type: trojan") {
		t.Errorf("AnyTLS must not be rendered as trojan:\n%s", result)
	}
}

func TestRenderProxyEntry_ShadowTLS(t *testing.T) {
	entry := Entry{
		Name:            "test-shadowtls",
		ProxyType:       model.ProxyTypeShadowTLS,
		Server:          "1.2.3.4",
		Port:            8080,
		Password:        "stls-password",
		HandshakeAddr:   "cloudflare.com:443",
		SNI:             "example.com",
		BackendPassword: "ss-secret",
		BackendCipher:   DefaultShadowTLSBackendCipher,
	}

	result := renderProxyEntry(entry)

	required := []string{
		"name: test-shadowtls",
		"type: ss",
		"server: 1.2.3.4",
		"port: 8080",
		// The shadowsocks layer uses the backend credentials ...
		"password: ss-secret",
		"cipher: aes-256-gcm",
		"plugin: shadow-tls",
		// ... while the ShadowTLS password belongs to the plugin.
		"password: stls-password",
		"version: 3",
		"host: example.com",
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
	if strings.Contains(result, "cloudflare.com:443") || strings.Contains(result, "      sni:") {
		t.Errorf("renderProxyEntry() leaked the relay address or an unsupported SNI key")
	}
}

func TestManagedTLSSubscriptions(t *testing.T) {
	n := &model.Node{Address: "192.0.2.1", Ports: []*model.Port{
		{ID: "a", Port: 8443, ProxyType: model.ProxyTypeAnyTLS, Password: "anytls-secret", Enabled: true},
		{ID: "s", Port: 9443, ProxyType: model.ProxyTypeShadowTLS, Password: "tls-secret", BackendPassword: "ss-secret", Enabled: true},
	}}
	entries := BuildEntries(n)
	if len(entries) != 2 {
		t.Fatalf("expected two entries, got %d", len(entries))
	}
	anyTLS := renderProxyEntry(entries[0])
	if !strings.Contains(anyTLS, "skip-cert-verify: true") {
		t.Fatal("managed AnyTLS uses a self-signed certificate")
	}
	shadowTLS := renderProxyEntry(entries[1])
	if !strings.Contains(shadowTLS, "host: cloudflare.com\n") || strings.Contains(shadowTLS, "cloudflare.com:443") {
		t.Fatalf("invalid default ShadowTLS SNI: %s", shadowTLS)
	}
	if !strings.Contains(shadowTLS, "udp: false") {
		t.Fatal("the managed ShadowTLS tunnel cannot carry UDP datagrams")
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
