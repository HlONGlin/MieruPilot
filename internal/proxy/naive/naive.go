// Package naive provides NaïveProxy server installation and configuration.
//
// The NaïveProxy server side is Caddy built with klzgrad's forwardproxy plugin.
// Stock Caddy releases do not include that plugin, so the plugin's own release
// is used as the installation source.
package naive

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"merit/internal/model"
)

const (
	// DefaultVersion is the klzgrad/forwardproxy release that bundles a Caddy
	// build with the naive forwardproxy handler.
	DefaultVersion = "v2.11.7-naive2"
	// ArchiveName is the only asset published by that release.
	ArchiveName = "caddy-forwardproxy-naive.tar.xz"
	// BinaryName is the executable name inside the archive.
	BinaryName = "caddy"
	// BinaryPath is the install target. It is deliberately not "caddy" so a
	// distribution caddy package cannot shadow or overwrite it.
	BinaryPath = "/usr/local/bin/caddy-naive"
	// ConfigDir holds one JSON config and one certificate directory per port.
	ConfigDir = "/etc/merit/naive"
)

// DownloadURL returns the release asset URL and file name for goarch. Upstream
// only publishes a linux/amd64 build; other architectures get an actionable
// error instead of an unusable binary.
func DownloadURL(goarch string) (string, string, error) {
	if goarch != "amd64" {
		return "", "", fmt.Errorf("NaïveProxy 官方仅提供 linux/amd64 预编译包；当前架构 %s 需按 klzgrad/forwardproxy@%s 的发布流程构建（匹配 CADDY_VERSION 并应用其 Caddy 补丁），然后放置到 %s", goarch, DefaultVersion, BinaryPath)
	}
	asset := fmt.Sprintf("https://github.com/klzgrad/forwardproxy/releases/download/%s/%s", DefaultVersion, ArchiveName)
	return asset, ArchiveName, nil
}

// Validate checks the settings Caddy needs to serve one NaïveProxy port.
func Validate(p *model.Port) error {
	if p == nil {
		return fmt.Errorf("端口配置为空")
	}
	if transport := p.EffectiveNaiveTransport(); transport != "https" && transport != "quic" {
		return fmt.Errorf("NaïveProxy 客户端传输必须是 https 或 quic")
	}
	if strings.TrimSpace(p.Domain) == "" {
		return fmt.Errorf("NaïveProxy 需要域名：TLS 证书必须与客户端访问的域名匹配")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("端口 %d 非法", p.Port)
	}
	if !p.AutoACME && (strings.TrimSpace(p.TLSCert) == "" || strings.TrimSpace(p.TLSKey) == "") {
		return fmt.Errorf("未启用自动 ACME 时必须填写 TLS 证书与私钥")
	}
	if p.AutoACME && p.Port != 443 {
		return fmt.Errorf("自动 ACME 需要 443 端口完成 HTTP-01/TLS-ALPN 校验，当前端口为 %d", p.Port)
	}
	return nil
}

// GenerateConfig renders the Caddy JSON config for one NaïveProxy port.
//
// The config is produced through json.Marshal so that credentials containing
// quotes, backslashes or newlines cannot corrupt the file.
func GenerateConfig(p *model.Port) (string, error) {
	if err := Validate(p); err != nil {
		return "", err
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(p.Username + ":" + p.Password))

	handler := map[string]any{
		"handler":  "forward_proxy",
		"hide_ip":  true,
		"hide_via": true,
		// Upstream uses [][]byte and compares the decoded bytes with the Basic
		// token. json.Marshal must therefore encode the token itself once more.
		"auth_credentials": [][]byte{[]byte(credentials)},
		"probe_resistance": map[string]any{},
	}

	server := map[string]any{
		"listen":                  []string{fmt.Sprintf(":%d", p.Port)},
		"routes":                  []any{map[string]any{"handle": []any{handler}}},
		"tls_connection_policies": []any{map[string]any{}},
		// Independent instances must not compete for Caddy's HTTP redirect port.
		"automatic_https": map[string]any{"disable_redirects": true},
	}

	tlsApp := map[string]any{}
	domain := strings.TrimSpace(p.Domain)
	if p.AutoACME {
		tlsApp["certificates"] = map[string]any{"automate": []string{domain}}

		// Configure ACME issuer
		issuer := map[string]any{"module": "acme"}
		// Use Let's Encrypt staging CA when ACMEStaging is true.
		// Staging certificates are NOT trusted by browsers or clients - they are only
		// for testing the ACME flow (DNS propagation, port reachability, renewal).
		if p.ACMEStaging {
			issuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
		}

		tlsApp["automation"] = map[string]any{
			"policies": []any{map[string]any{
				"subjects": []string{domain},
				"issuers":  []any{issuer},
			}},
		}
	} else {
		certFile, keyFile := CertPath(p.ID)
		tlsApp["certificates"] = map[string]any{
			"load_files": []any{map[string]any{
				"certificate": certFile,
				"key":         keyFile,
			}},
		}
	}

	config := map[string]any{
		"admin": map[string]any{"disabled": true},
		"apps": map[string]any{
			"http": map[string]any{"servers": map[string]any{"naive": server}},
			"tls":  tlsApp,
		},
	}

	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw) + "\n", nil
}

// SimpleLink builds a naive+https sharing link. url.URL escapes the credentials
// so passwords with reserved characters stay valid URIs.
func SimpleLink(n *model.Node, p *model.Port) string {
	host := strings.TrimSpace(p.Domain)
	if host == "" && n != nil {
		host = strings.TrimSpace(n.Domain)
		if host == "" {
			host = strings.TrimSpace(n.Address)
		}
		if host == "" {
			host = strings.TrimSpace(n.SeenIP)
		}
	}

	// 无法确定地址时返回空字符串
	if host == "" {
		return ""
	}

	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}

	u := &url.URL{
		Scheme: "naive+" + p.EffectiveNaiveTransport(),
		User:   url.UserPassword(p.Username, p.Password),
		Host:   fmt.Sprintf("%s:%d", host, p.Port),
	}
	if label := strings.TrimSpace(p.Label); label != "" {
		u.Fragment = label
	}
	return u.String()
}

// ConfigPath returns the Caddy config path for a port instance. portID must
// already be sanitised by the caller.
func ConfigPath(portID string) string {
	return filepath.Join(ConfigDir, portID+".json")
}

// CertPath returns the manual certificate paths for a port instance.
func CertPath(portID string) (certFile, keyFile string) {
	dir := filepath.Join(ConfigDir, "certs", portID)
	return filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
}
