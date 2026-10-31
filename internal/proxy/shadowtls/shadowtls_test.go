package shadowtls

import (
	"net/url"
	"reflect"
	"testing"

	"merit/internal/model"
	"merit/internal/proxy/shadowsocks"
)

func TestAssetName(t *testing.T) {
	tests := map[string]string{
		"amd64": "shadow-tls-x86_64-unknown-linux-musl",
		"arm64": "shadow-tls-aarch64-unknown-linux-musl",
		"arm":   "shadow-tls-armv7-unknown-linux-musleabihf",
	}
	for arch, want := range tests {
		got, err := AssetName(arch)
		if err != nil {
			t.Fatalf("AssetName(%s) error: %v", arch, err)
		}
		if got != want {
			t.Errorf("AssetName(%s) = %q, want %q", arch, got, want)
		}
	}

	if _, err := AssetName("386"); err == nil {
		t.Error("AssetName(386) unexpectedly succeeded")
	}
}

func TestDownloadURL(t *testing.T) {
	asset, name, err := DownloadURL("amd64")
	if err != nil {
		t.Fatalf("DownloadURL() error: %v", err)
	}
	want := "https://github.com/ihciah/shadow-tls/releases/download/v0.2.25/" + name
	if asset != want {
		t.Errorf("DownloadURL() = %q, want %q", asset, want)
	}
	// Upstream publishes bare binaries; nothing has to be unpacked.
	if name != "shadow-tls-x86_64-unknown-linux-musl" {
		t.Errorf("DownloadURL() name = %q", name)
	}
}

func TestValidate(t *testing.T) {
	valid := func() *model.Port {
		return &model.Port{
			Port:          443,
			Password:      "secret",
			HandshakeAddr: "cloudflare.com:443",
			BackendAddr:   "127.0.0.1:8388",
		}
	}

	tests := []struct {
		name    string
		port    *model.Port
		wantErr bool
	}{
		{name: "valid", port: valid()},
		{name: "missing backend", port: func() *model.Port { p := valid(); p.BackendAddr = ""; return p }(), wantErr: true},
		{name: "missing handshake", port: func() *model.Port { p := valid(); p.HandshakeAddr = ""; return p }(), wantErr: true},
		{name: "missing password", port: func() *model.Port { p := valid(); p.Password = ""; return p }(), wantErr: true},
		{name: "invalid port", port: func() *model.Port { p := valid(); p.Port = 0; return p }(), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServerArgsOrder(t *testing.T) {
	port := &model.Port{
		Port:          443,
		Password:      "se cret",
		HandshakeAddr: "cloudflare.com:443",
		BackendAddr:   "127.0.0.1:8388",
	}

	got := ServerArgs(port)
	want := []string{
		"--v3",
		"server",
		"--listen", "0.0.0.0:443",
		"--server", "127.0.0.1:8388",
		"--tls", "cloudflare.com:443",
		"--password", "se cret",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ServerArgs() = %#v, want %#v", got, want)
	}

	// --v3 is a global option and is only accepted before the sub-command.
	v3Index, serverIndex := -1, -1
	for i, arg := range got {
		switch arg {
		case "--v3":
			v3Index = i
		case "server":
			serverIndex = i
		}
	}
	if v3Index < 0 || serverIndex < 0 || v3Index > serverIndex {
		t.Errorf("ServerArgs() = %#v, want --v3 before the server sub-command", got)
	}
}

func TestHandshakeDefault(t *testing.T) {
	if got := Handshake(&model.Port{}); got != DefaultHandshake {
		t.Errorf("Handshake() = %q, want %q", got, DefaultHandshake)
	}
	if got := Handshake(&model.Port{HandshakeAddr: "www.bing.com:443"}); got != "www.bing.com:443" {
		t.Errorf("Handshake() = %q, want the configured address", got)
	}
	if got := Handshake(nil); got != DefaultHandshake {
		t.Errorf("Handshake(nil) = %q, want %q", got, DefaultHandshake)
	}
}

func TestSimpleLink(t *testing.T) {
	node := &model.Node{Address: "192.0.2.7"}
	port := &model.Port{
		Port:            443,
		Password:        "tls secret",
		HandshakeAddr:   "cloudflare.com:443",
		BackendAddr:     "127.0.0.1:8388",
		BackendPassword: "ss secret",
		BackendCipher:   "chacha20-ietf-poly1305",
		Label:           "我的节点",
	}

	link := SimpleLink(node, port)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("SimpleLink() = %q is not a valid URL: %v", link, err)
	}
	if parsed.Scheme != "ss" {
		t.Errorf("scheme = %q, want ss", parsed.Scheme)
	}
	if parsed.Host != "192.0.2.7:443" {
		t.Errorf("host = %q, want 192.0.2.7:443", parsed.Host)
	}
	if parsed.User == nil || parsed.User.Username() != port.BackendCipher {
		t.Errorf("userinfo username = %v, want the backend cipher", parsed.User)
	}
	if password, ok := parsed.User.Password(); !ok || password != port.BackendPassword {
		t.Errorf("userinfo password = %q, want the backend password", password)
	}
	query := parsed.Query()
	if got, want := query.Get("plugin"), "shadow-tls;host=cloudflare.com;password=tls secret;version=3"; got != want {
		t.Errorf("plugin = %q, want %q", got, want)
	}
	if parsed.Fragment != "我的节点" {
		t.Errorf("fragment = %q, want the label", parsed.Fragment)
	}
}

func TestSimpleLinkRequiresBackendPassword(t *testing.T) {
	node := &model.Node{Address: "192.0.2.7"}
	port := &model.Port{Port: 443, Password: "tls-secret", HandshakeAddr: "cloudflare.com:443"}
	if got := SimpleLink(node, port); got != "" {
		t.Errorf("SimpleLink() = %q, want empty without backend credentials", got)
	}
}

func TestSimpleLinkDefaultsBackendCipherAndSupportsIPv6(t *testing.T) {
	node := &model.Node{Address: "2001:db8::7"}
	port := &model.Port{
		Port:            443,
		Password:        "tls/p@ss?#",
		HandshakeAddr:   "www.example.com:443",
		BackendPassword: "ss/p@ss?#",
	}

	link := SimpleLink(node, port)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("SimpleLink() = %q is not a valid URL: %v", link, err)
	}
	if parsed.Host != "[2001:db8::7]:443" {
		t.Errorf("host = %q, want bracketed IPv6 host", parsed.Host)
	}
	if parsed.User.Username() != shadowsocks.DefaultCipher {
		t.Errorf("cipher = %q, want %q", parsed.User.Username(), shadowsocks.DefaultCipher)
	}
	if got, want := parsed.Query().Get("plugin"), "shadow-tls;host=www.example.com;password=tls/p@ss?#;version=3"; got != want {
		t.Errorf("plugin = %q, want %q", got, want)
	}
}
