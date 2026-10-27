// Package anytls provides AnyTLS server installation and configuration.
//
// The AnyTLS server takes its settings from command line flags only; there is no
// config file and no sub-command, so the agent always generates flags.
package anytls

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"merit/internal/model"
)

const (
	// DefaultVersion is the anytls-go release the agent installs.
	DefaultVersion = "v0.0.13"
	// ServerBinaryName is the executable inside the release archive.
	ServerBinaryName = "anytls-server"
	// ServerBinaryPath is the install target.
	ServerBinaryPath = "/usr/local/bin/anytls-server"
	// ConfigDir is only used to clean up files written by older agent builds.
	ConfigDir = "/etc/merit/anytls"
)

// AssetName returns the release archive name for goarch. goreleaser builds use
// the version without the leading "v" and the Go architecture name.
func AssetName(goarch string) (string, error) {
	switch goarch {
	case "amd64", "arm64":
		return fmt.Sprintf("anytls_%s_linux_%s.zip", strings.TrimPrefix(DefaultVersion, "v"), goarch), nil
	default:
		return "", fmt.Errorf("AnyTLS 暂不支持架构 %s（官方仅发布 amd64/arm64 包）", goarch)
	}
}

// DownloadURL returns the release asset URL and file name for goarch.
func DownloadURL(goarch string) (string, string, error) {
	name, err := AssetName(goarch)
	if err != nil {
		return "", "", err
	}
	asset := fmt.Sprintf("https://github.com/anytls/anytls-go/releases/download/%s/%s", DefaultVersion, name)
	return asset, name, nil
}

// Validate checks the settings the AnyTLS server needs.
func Validate(p *model.Port) error {
	if p == nil {
		return fmt.Errorf("端口配置为空")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("端口 %d 非法", p.Port)
	}
	if strings.TrimSpace(p.Password) == "" {
		return fmt.Errorf("AnyTLS 需要密码")
	}
	return nil
}

// ListenAddr returns the listen address for a port.
// Go's wildcard TCP listener supports both address families when available.
func ListenAddr(port int) string { return fmt.Sprintf(":%d", port) }

// ServerArgs returns the AnyTLS server flags: the binary has no sub-command and
// only accepts -l (listen, defaults to 0.0.0.0:8443), -p (password) and
// -padding-scheme (optional file).
func ServerArgs(p *model.Port) []string {
	return []string{
		"-l", ListenAddr(p.Port),
		"-p", p.Password,
	}
}

// SimpleLink builds an anytls://password@host:port sharing link.
func SimpleLink(n *model.Node, p *model.Port) string {
	host := hostOf(n)

	// 无法确定地址时返回空字符串
	if host == "" {
		return ""
	}

	u := &url.URL{
		Scheme: "anytls",
		User:   url.User(p.Password),
		Host:   fmt.Sprintf("%s:%d", host, p.Port),
		// anytls-go generates an ephemeral self-signed certificate on startup.
		RawQuery: url.Values{"insecure": {"1"}, "allowInsecure": {"1"}}.Encode(),
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
