package manager

import (
	"encoding/json"
	"merit/internal/clash"
	"merit/internal/model"
	"merit/internal/proxy"
	"net/url"
	"strings"
	"testing"
)

func TestDirectClientTransports(t *testing.T) {
	n := &model.Node{Address: "192.0.2.1"}
	p := &model.Port{Port: 23112, ProxyType: "anytls", Password: "secret", Domain: "www.cloudflare.com", TLSFingerprint: "firefox", Enabled: true}
	cfg, err := clientConfig(n, p)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg)
	for _, want := range []string{"firefox", "www.cloudflare.com", "utls"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("JSON missing %s", want)
		}
	}
	u, _ := url.Parse(proxy.GenerateShareLink(n, p))
	if u.Query().Get("sni") != p.Domain || u.Query().Get("fp") != "firefox" {
		t.Fatal("URI lost TLS settings")
	}
	n.Ports = []*model.Port{p}
	yaml := clash.GenerateYAML(clash.BuildEntries(n))
	if !strings.Contains(yaml, "client-fingerprint: firefox") || !strings.Contains(yaml, "sni: www.cloudflare.com") {
		t.Fatal("YAML lost TLS settings")
	}
	p = &model.Port{Port: 443, ProxyType: "naive", Domain: "proxy.example.com", Username: "u", Password: "p", NaiveTransport: "quic"}
	cfg, err = clientConfig(n, p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["proxy"] != "quic://u:p@proxy.example.com:443" {
		t.Fatal("wrong QUIC JSON scheme")
	}
	if !strings.HasPrefix(proxy.GenerateShareLink(n, p), "naive+quic://") {
		t.Fatal("wrong QUIC URI scheme")
	}
	p.NaiveTransport = ""
	cfg, _ = clientConfig(n, p)
	if !strings.HasPrefix(cfg["proxy"].(string), "https://") {
		t.Fatal("legacy HTTPS changed")
	}
}

func TestDirectClientFieldsEditValidation(t *testing.T) {
	s, request := protocolTestServer(t)
	res := request("POST", "", map[string]any{"port": 23112, "proxyType": "anytls", "domain": "www.cloudflare.com", "tlsFingerprint": "firefox"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	id := s.store.Node("node").Ports[0].ID
	res = request("PATCH", "/"+id, map[string]any{"tlsFingerprint": "invalid"})
	if res.Code != 400 {
		t.Fatal("accepted invalid fingerprint")
	}
	if s.store.Node("node").FindPort(id).TLSFingerprint != "firefox" {
		t.Fatal("invalid edit changed data")
	}
	res = request("POST", "", map[string]any{"port": 443, "proxyType": "naive", "domain": "proxy.example.com", "autoACME": true, "naiveTransport": "quic"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	p := s.store.Node("node").Ports[1]
	if p.NaiveTransport != "quic" {
		t.Fatal("QUIC choice not persisted")
	}
	res = request("PATCH", "/"+p.ID, map[string]any{"naiveTransport": "http"})
	if res.Code != 400 {
		t.Fatal("accepted invalid transport")
	}
	if s.store.Node("node").FindPort(p.ID).NaiveTransport != "quic" {
		t.Fatal("invalid edit changed transport")
	}
}
