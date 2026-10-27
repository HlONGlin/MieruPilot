package shadowsocks

import (
	"reflect"
	"testing"
)

func TestAssetName(t *testing.T) {
	tests := map[string]string{
		"amd64": "shadowsocks-v1.18.2.x86_64-unknown-linux-musl.tar.xz",
		"arm64": "shadowsocks-v1.18.2.aarch64-unknown-linux-musl.tar.xz",
		"arm":   "shadowsocks-v1.18.2.armv7-unknown-linux-musleabihf.tar.xz",
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
	url, name, err := DownloadURL("amd64")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://github.com/shadowsocks/shadowsocks-rust/releases/download/v1.18.2/" + name
	if url != want {
		t.Errorf("DownloadURL() = %q, want %q", url, want)
	}
}

func TestServerArgs(t *testing.T) {
	got := ServerArgs("127.0.0.1:50000", "se cret", "aes-256-gcm")
	want := []string{"-s", "127.0.0.1:50000", "-m", "aes-256-gcm", "-k", "se cret"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ServerArgs() = %#v, want %#v", got, want)
	}
}

func TestLoopbackPort(t *testing.T) {
	for _, tc := range []struct {
		addr    string
		want    int
		wantErr bool
	}{
		{"127.0.0.1:50000", 50000, false},
		{"[::1]:50001", 50001, false},
		{"0.0.0.0:50000", 0, true},
		{"203.0.113.5:50000", 0, true},
		{"localhost:50000", 0, true},
		{"127.0.0.1", 0, true},
		{"127.0.0.1:0", 0, true},
		{"127.0.0.1:70000", 0, true},
	} {
		got, err := LoopbackPort(tc.addr)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("LoopbackPort(%q) = %d, %v; want %d, err=%v", tc.addr, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name, addr, password, cipher string
		wantErr                      bool
	}{
		{"valid", "127.0.0.1:50000", "pw", "aes-256-gcm", false},
		{"public address", "0.0.0.0:50000", "pw", "aes-256-gcm", true},
		{"empty password", "127.0.0.1:50000", "", "aes-256-gcm", true},
		{"unsupported cipher", "127.0.0.1:50000", "pw", "2022-blake3-aes-256-gcm", true},
		{"empty cipher", "127.0.0.1:50000", "pw", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.addr, tc.password, tc.cipher); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
