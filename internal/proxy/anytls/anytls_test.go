package anytls

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"merit/internal/model"
)

func TestAssetName(t *testing.T) {
	name, err := AssetName("amd64")
	if err != nil {
		t.Fatalf("AssetName(amd64) error: %v", err)
	}
	if name != "anytls_0.0.13_linux_amd64.zip" {
		t.Errorf("AssetName(amd64) = %q, want anytls_0.0.13_linux_amd64.zip", name)
	}

	name, err = AssetName("arm64")
	if err != nil {
		t.Fatalf("AssetName(arm64) error: %v", err)
	}
	if name != "anytls_0.0.13_linux_arm64.zip" {
		t.Errorf("AssetName(arm64) = %q, want anytls_0.0.13_linux_arm64.zip", name)
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
	want := "https://github.com/anytls/anytls-go/releases/download/v0.0.13/" + name
	if asset != want {
		t.Errorf("DownloadURL() = %q, want %q", asset, want)
	}
	if !strings.HasSuffix(name, ".zip") {
		t.Errorf("DownloadURL() name = %q, want a .zip asset", name)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		port    *model.Port
		wantErr bool
	}{
		{name: "valid", port: &model.Port{Port: 8443, Password: "secret"}},
		{name: "missing password", port: &model.Port{Port: 8443}, wantErr: true},
		{name: "invalid port", port: &model.Port{Port: 0, Password: "secret"}, wantErr: true},
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

func TestServerArgs(t *testing.T) {
	port := &model.Port{Port: 8443, Password: "se cret"}

	got := ServerArgs(port)
	want := []string{"-l", ":8443", "-p", "se cret"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ServerArgs() = %#v, want %#v", got, want)
	}

	// The server binary has no sub-command; a leading "server" token would be
	// rejected by its flag parser.
	if got[0] == "server" {
		t.Error("ServerArgs() must not pass a sub-command")
	}
}

func TestSimpleLink(t *testing.T) {
	node := &model.Node{Address: "192.0.2.7"}
	port := &model.Port{Port: 8443, Password: "p@ss word/?#"}

	link := SimpleLink(node, port)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("SimpleLink() = %q is not a valid URL: %v", link, err)
	}
	if parsed.Scheme != "anytls" {
		t.Errorf("scheme = %q, want anytls", parsed.Scheme)
	}
	if parsed.Query().Get("insecure") != "1" || parsed.Query().Get("allowInsecure") != "1" {
		t.Fatal("share link must match the managed server's self-signed certificate")
	}
	if parsed.Host != "192.0.2.7:8443" {
		t.Errorf("host = %q, want 192.0.2.7:8443", parsed.Host)
	}
	if parsed.User == nil || parsed.User.Username() != port.Password {
		t.Errorf("userinfo = %v, want the password as user", parsed.User)
	}
	if strings.Contains(link, "p@ss word/?#") {
		t.Errorf("link %q contains an unescaped password", link)
	}
}
