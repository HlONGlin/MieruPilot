// Package shadowsocks installs and runs the loopback shadowsocks server that backs
// a Manager-allocated ShadowTLS port.
//
// The Manager owns the backend address, password and cipher so that client
// subscriptions and the node always agree on them. This package only describes
// how to fetch and start the server.
package shadowsocks

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	// DefaultVersion is the shadowsocks-rust release the agent installs.
	DefaultVersion = "v1.18.2"
	// BinaryName is the server executable name inside the release archive.
	BinaryName = "ssserver"
	// BinaryPath is the install target.
	BinaryPath = "/usr/local/bin/ssserver"
	// DefaultCipher is used when a managed backend has no cipher yet.
	DefaultCipher = "aes-256-gcm"
	// BackendPortMin and BackendPortMax bound the loopback ports the Manager hands
	// out to managed backends.
	BackendPortMin = 50000
	BackendPortMax = 59999
)

// ManagedCiphers are the classic AEAD ciphers accepted for a managed backend.
// The 2022 ciphers need base64 keys of a fixed length and are not supported.
var ManagedCiphers = []string{"aes-128-gcm", "aes-256-gcm", "chacha20-ietf-poly1305"}

// IsManagedCipher reports whether cipher can be used for a managed backend.
func IsManagedCipher(cipher string) bool {
	for _, c := range ManagedCiphers {
		if c == cipher {
			return true
		}
	}
	return false
}

// AssetName returns the release archive name for goarch. Upstream names its
// assets shadowsocks-<version>.<target>.tar.xz.
func AssetName(goarch string) (string, error) {
	var target string
	switch goarch {
	case "amd64":
		target = "x86_64-unknown-linux-musl"
	case "arm64":
		target = "aarch64-unknown-linux-musl"
	case "arm":
		target = "armv7-unknown-linux-musleabihf"
	default:
		return "", fmt.Errorf("Shadowsocks 暂不支持架构 %s", goarch)
	}
	return fmt.Sprintf("shadowsocks-%s.%s.tar.xz", DefaultVersion, target), nil
}

// DownloadURL returns the release asset URL and file name for goarch.
func DownloadURL(goarch string) (string, string, error) {
	name, err := AssetName(goarch)
	if err != nil {
		return "", "", err
	}
	url := fmt.Sprintf("https://github.com/shadowsocks/shadowsocks-rust/releases/download/%s/%s", DefaultVersion, name)
	return url, name, nil
}

// IsInstalled reports whether the server binary is available.
func IsInstalled() bool {
	info, err := os.Stat(BinaryPath)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0 &&
		(runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0)
}

// ServerArgs returns the ssserver command line. UDP relay stays off because the
// ShadowTLS tunnel in front of it is TCP only.
func ServerArgs(listenAddr, password, cipher string) []string {
	return []string{"-s", listenAddr, "-m", cipher, "-k", password}
}

// LoopbackPort parses a managed backend address and returns its port. The
// address must be a literal loopback IP so the backend is never exposed.
func LoopbackPort(addr string) (int, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return 0, fmt.Errorf("后端地址 %q 格式应为 127.0.0.1:端口", addr)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return 0, fmt.Errorf("托管后端必须监听本机回环地址，当前为 %q", host)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("后端端口 %q 非法", portText)
	}
	return port, nil
}

// Validate checks the settings of a managed backend.
func Validate(addr, password, cipher string) error {
	if _, err := LoopbackPort(addr); err != nil {
		return err
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("后端密码为空")
	}
	if !IsManagedCipher(cipher) {
		return fmt.Errorf("托管后端不支持加密方式 %q，可选 %s", cipher, strings.Join(ManagedCiphers, "、"))
	}
	return nil
}
