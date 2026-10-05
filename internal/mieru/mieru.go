// Package mieru contains helpers to generate mita server configuration,
// mieru client share links and Clash/mihomo subscriptions.
package mieru

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"merit/internal/model"
)

// RandomID returns a 16 character random hex identifier.
func RandomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// GenerateUsername returns a random mita user name.
func GenerateUsername() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "m" + hex.EncodeToString(b)
}

const passwordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GeneratePassword returns a random URL friendly password.
func GeneratePassword() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = passwordAlphabet[int(v)%len(passwordAlphabet)]
	}
	return string(out)
}

// NewPort builds a fully populated node port with credentials.
func NewPort(port int, protocol, label string) *model.Port {
	if protocol == "" {
		protocol = model.ProtocolTCP
	}
	return &model.Port{
		ID:        RandomID(),
		Port:      port,
		Protocol:  strings.ToUpper(protocol),
		Username:  GenerateUsername(),
		Password:  GeneratePassword(),
		Label:     label,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
}

// ServerConfigJSON renders the mita server configuration file.
func ServerConfigJSON(cfg *model.DesiredConfig) ([]byte, error) {
	type users struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	type bindings struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	type serverConfig struct {
		PortBindings []bindings          `json:"portBindings"`
		Users        []users             `json:"users"`
		LoggingLevel string              `json:"loggingLevel"`
		Egress       *model.EgressConfig `json:"egress,omitempty"`
	}
	sc := serverConfig{LoggingLevel: cfg.LoggingLevel, Egress: cfg.Egress}
	if sc.LoggingLevel == "" {
		sc.LoggingLevel = "INFO"
	}
	if sc.PortBindings == nil {
		sc.PortBindings = []bindings{}
	}
	if sc.Users == nil {
		sc.Users = []users{}
	}
	for _, b := range cfg.PortBindings {
		sc.PortBindings = append(sc.PortBindings, bindings{Port: b.Port, Protocol: b.Protocol})
	}
	for _, u := range cfg.Users {
		sc.Users = append(sc.Users, users{Name: u.Name, Password: u.Password})
	}
	return json.MarshalIndent(sc, "", "    ")
}

// ServerAddress returns the address to embed in client links.
func ServerAddress(n *model.Node) string {
	addr := strings.TrimSpace(n.Address)
	if addr == "" {
		addr = strings.TrimSpace(n.SeenIP)
	}
	return addr
}

// ClientHost returns the host to put in share links (domain preferred, IPv6
// wrapped in brackets).
func ClientHost(n *model.Node) string {
	host := strings.TrimSpace(n.Domain)
	if host == "" {
		host = ServerAddress(n)
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host
}

// SimpleLink builds a native mierus:// sharing link for a single port.
// The fragment is the display name used by clients, with the port label taking
// precedence over the node name.
func SimpleLink(n *model.Node, p *model.Port) string {
	host := ClientHost(n)
	u := &url.URL{Scheme: "mierus", User: url.UserPassword(p.Username, p.Password), Host: host}
	q := url.Values{}
	q.Set("handshake-mode", model.DefaultHandshake)
	q.Set("mtu", strconv.Itoa(model.DefaultMTU))
	q.Set("multiplexing", model.DefaultMultiplexing)
	q.Set("profile", model.DefaultProfile)
	q.Add("port", strconv.Itoa(p.Port))
	q.Add("protocol", p.Protocol)
	q.Set("traffic-pattern", model.DefaultTrafficPattern)
	u.RawQuery = q.Encode()
	name := strings.TrimSpace(p.Label)
	if name == "" {
		name = strings.TrimSpace(n.Name)
	}
	if name != "" {
		u.Fragment = name
	}
	return u.String()
}

// StandardLink builds a mieru:// standard sharing link by embedding the client
// profile configuration as base64. The mieru binary's own export command is
// not available on the manager, so the simple link is the primary interface.
// This helper is kept for completeness.
func StandardLink(n *model.Node, p *model.Port) string {
	return SimpleLink(n, p)
}

// ClashEntry is one mieru proxy inside a Clash configuration.
type ClashEntry struct {
	Name     string
	Server   string
	Port     int
	Protocol string
	Username string
	Password string
}

// BuildClashEntries flattens all enabled ports of a node into clash entries.
// Nodes without a known address are skipped because they cannot be used yet.
func BuildClashEntries(n *model.Node) []ClashEntry {
	host := ClientHost(n)
	if host == "" {
		return nil
	}
	var out []ClashEntry
	name := n.Name
	if name == "" {
		name = n.ID
	}
	for _, p := range n.Ports {
		if !p.Enabled {
			continue
		}
		label := p.Label
		if label == "" {
			label = fmt.Sprintf("%d", p.Port)
		}
		out = append(out, ClashEntry{
			Name:     fmt.Sprintf("%s-%s", name, label),
			Server:   host,
			Port:     p.Port,
			Protocol: p.Protocol,
			Username: p.Username,
			Password: p.Password,
		})
	}
	return out
}

// ClashYAML renders a mihomo / Clash Meta configuration containing all entries.
func ClashYAML(entries []ClashEntry) string {
	var b strings.Builder
	b.WriteString("# Generated by merit\n")
	b.WriteString("mixed-port: 7890\n")
	b.WriteString("allow-lan: false\n")
	b.WriteString("mode: rule\n")
	b.WriteString("log-level: info\n\n")

	b.WriteString("proxies:\n")
	if len(entries) == 0 {
		b.WriteString("  []\n")
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		transport := e.Protocol
		if transport == "" {
			transport = model.ProtocolTCP
		}
		b.WriteString(fmt.Sprintf("  - name: %s\n", yamlString(e.Name)))
		b.WriteString("    type: mieru\n")
		b.WriteString(fmt.Sprintf("    server: %s\n", yamlString(e.Server)))
		b.WriteString(fmt.Sprintf("    port: %d\n", e.Port))
		b.WriteString(fmt.Sprintf("    transport: %s\n", transport))
		b.WriteString("    udp: true\n")
		b.WriteString(fmt.Sprintf("    username: %s\n", yamlString(e.Username)))
		b.WriteString(fmt.Sprintf("    password: %s\n", yamlString(e.Password)))
		b.WriteString(fmt.Sprintf("    handshake-mode: %s\n", model.DefaultHandshake))
		b.WriteString(fmt.Sprintf("    mtu: %d\n", model.DefaultMTU))
		b.WriteString(fmt.Sprintf("    multiplexing: %s\n", model.DefaultMultiplexing))
		b.WriteString(fmt.Sprintf("    traffic-pattern: %s\n", yamlString(model.DefaultTrafficPattern)))
		names = append(names, e.Name)
	}

	b.WriteString("\nproxy-groups:\n")
	b.WriteString("  - name: mieru\n")
	b.WriteString("    type: select\n")
	b.WriteString("    proxies:\n")
	if len(names) == 0 {
		b.WriteString("      - DIRECT\n")
	}
	for _, n := range names {
		b.WriteString(fmt.Sprintf("      - %s\n", yamlString(n)))
	}

	b.WriteString("\nrules:\n")
	b.WriteString("  - MATCH,mieru\n")
	return b.String()
}

func yamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
