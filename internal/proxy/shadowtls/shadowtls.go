// Package shadowtls provides ShadowTLS v3 server installation and configuration.
//
// ShadowTLS only obfuscates the TLS handshake: it relays decrypted traffic to a
// backend data server (typically shadowsocks), which has to run separately. The
// backend address is therefore a mandatory part of a usable deployment.
package shadowtls

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"merit/internal/model"
	"merit/internal/proxy/shadowsocks"
)

const (
	// DefaultVersion is the shadow-tls release the agent installs.
	DefaultVersion = "v0.2.25"
	// BinaryName is the installed executable name.
	BinaryName = "shadow-tls"
	// BinaryPath is the install target.
	BinaryPath = "/usr/local/bin/shadow-tls"
	// ConfigDir is only used to clean up files written by older agent builds.
	ConfigDir = "/etc/merit/shadowtls"
	// DefaultHandshake is used when a port has no handshake address.
	DefaultHandshake = "cloudflare.com:443"
)

// AssetName returns the release asset name for goarch. Upstream publishes bare
// binaries with a Rust target triple and no file extension.
func AssetName(goarch string) (string, error) {
	var triple string
	switch goarch {
	case "amd64":
		triple = "x86_64"
	case "arm64":
		triple = "aarch64"
	case "arm":
		return "shadow-tls-armv7-unknown-linux-musleabihf", nil
	default:
		return "", fmt.Errorf("ShadowTLS 暂不支持架构 %s", goarch)
	}
	return fmt.Sprintf("shadow-tls-%s-unknown-linux-musl", triple), nil
}

// DownloadURL returns the release asset URL and file name for goarch.
func DownloadURL(goarch string) (string, string, error) {
	name, err := AssetName(goarch)
	if err != nil {
		return "", "", err
	}
	asset := fmt.Sprintf("https://github.com/ihciah/shadow-tls/releases/download/%s/%s", DefaultVersion, name)
	return asset, name, nil
}

// Handshake returns the TLS handshake address of a port.
func Handshake(p *model.Port) string {
	if p == nil {
		return DefaultHandshake
	}
	if h := strings.TrimSpace(p.HandshakeAddr); h != "" {
		return h
	}
	return DefaultHandshake
}

// HandshakeHost extracts the DNS name used for SNI, not the relay's TCP address.
func HandshakeHost(address string) string {
	if host, _, err := net.SplitHostPort(strings.TrimSpace(address)); err == nil {
		return host
	}
	return strings.TrimSpace(address)
}

// Validate checks the settings a ShadowTLS server needs. A backend data server is
// mandatory: without it ShadowTLS would accept handshakes and drop all traffic.
func Validate(p *model.Port) error {
	if p == nil {
		return fmt.Errorf("端口配置为空")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("端口 %d 非法", p.Port)
	}
	if strings.TrimSpace(p.Password) == "" {
		return fmt.Errorf("ShadowTLS 需要密码")
	}
	if strings.TrimSpace(p.HandshakeAddr) == "" {
		return fmt.Errorf("ShadowTLS 需要 TLS 握手地址（如 cloudflare.com:443）")
	}
	if strings.TrimSpace(p.BackendAddr) == "" {
		return fmt.Errorf("ShadowTLS 需要后端数据服务地址（如 127.0.0.1:8388 上的 shadowsocks）")
	}
	return nil
}

// ListenAddr returns the listen address for a port.
func ListenAddr(port int) string { return fmt.Sprintf("0.0.0.0:%d", port) }

// ServerArgs returns the shadow-tls command line. The v3 flag is a global option
// and must precede the "server" sub-command.
func ServerArgs(p *model.Port) []string {
	return []string{
		"--v3",
		"server",
		"--listen", ListenAddr(p.Port),
		"--server", strings.TrimSpace(p.BackendAddr),
		"--tls", Handshake(p),
		"--password", p.Password,
	}
}

// SimpleLink builds an ss:// URI using the shadow-tls plugin. The SS userinfo
// carries the backend cipher/password, while the plugin carries the ShadowTLS
// handshake host, TLS password, and protocol version.
func SimpleLink(n *model.Node, p *model.Port) string {
	if p == nil || strings.TrimSpace(p.BackendPassword) == "" {
		return ""
	}
	host := hostOf(n)

	// 无法确定地址时返回空字符串
	if host == "" {
		return ""
	}

	cipher := strings.TrimSpace(p.BackendCipher)
	if cipher == "" {
		cipher = shadowsocks.DefaultCipher
	}
	plugin := strings.Join([]string{
		"shadow-tls",
		"host=" + HandshakeHost(Handshake(p)),
		"password=" + strings.TrimSpace(p.Password),
		"version=3",
	}, ";")
	u := &url.URL{
		Scheme:   "ss",
		User:     url.UserPassword(cipher, p.BackendPassword),
		Host:     fmt.Sprintf("%s:%d", host, p.Port),
		RawQuery: url.Values{"plugin": []string{plugin}}.Encode(),
	}
	if label := strings.TrimSpace(p.Label); label != "" {
		u.Fragment = label
	}
	return u.String()
}

func hostOf(n *model.Node) string {
	if n == nil {
		return ""
	}
	host := strings.TrimSpace(n.Address)
	if host == "" {
		host = strings.TrimSpace(n.Domain)
	}
	if host == "" {
		host = strings.TrimSpace(n.SeenIP)
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host
}

// ConfigPath returns the legacy config path written by older agent builds. It is
// kept so instances created before this version are cleaned up correctly.
func ConfigPath(portID string) string {
	return filepath.Join(ConfigDir, portID+".json")
}
