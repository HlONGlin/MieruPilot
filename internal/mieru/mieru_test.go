package mieru

import (
	"strings"
	"testing"

	"merit/internal/model"
)

func TestSimpleLink(t *testing.T) {
	n := &model.Node{Name: "hk-1", Address: "1.2.3.4"}
	p := &model.Port{Port: 23456, Protocol: model.ProtocolTCP, Username: "user1", Password: "pass1"}
	got := SimpleLink(n, p)
	for _, want := range []string{
		"mierus://user1:pass1@1.2.3.4?handshake-mode=HANDSHAKE_NO_WAIT",
		"mtu=1400",
		"multiplexing=MULTIPLEXING_OFF",
		"port=23456",
		"profile=default",
		"protocol=TCP",
		"traffic-pattern=CIXu%2BasFEAAiCAgBEAEYBCAIKgUIABCAATICCAA%3D",
		"#hk-1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("SimpleLink = %q, want %q", got, want)
		}
	}
}

func TestSimpleLinkIncludesLabel(t *testing.T) {
	n := &model.Node{Address: "1.2.3.4"}
	p := &model.Port{Port: 23456, Protocol: model.ProtocolTCP, Username: "user1", Password: "pass1", Label: "香港-1"}
	got := SimpleLink(n, p)
	if !strings.Contains(got, "#%E9%A6%99%E6%B8%AF-1") {
		t.Fatalf("SimpleLink = %q, want encoded label fragment", got)
	}
}

func TestSimpleLinkIPv6(t *testing.T) {
	n := &model.Node{Address: "2001:db8::1"}
	p := &model.Port{Port: 1234, Protocol: model.ProtocolUDP, Username: "u", Password: "p"}
	got := SimpleLink(n, p)
	if !strings.Contains(got, "@[2001:db8::1]?") {
		t.Fatalf("SimpleLink IPv6 = %q, want bracketed host", got)
	}
}

func TestSimpleLinkDomainPrecedence(t *testing.T) {
	n := &model.Node{Address: "1.2.3.4", Domain: "proxy.example.com"}
	p := &model.Port{Port: 80, Protocol: model.ProtocolTCP, Username: "u", Password: "p"}
	if !strings.Contains(SimpleLink(n, p), "@proxy.example.com?") {
		t.Fatal("domain should take precedence over address")
	}
}

func TestBuildDesiredSkipsDisabled(t *testing.T) {
	n := &model.Node{Ports: []*model.Port{
		{Port: 1, Protocol: model.ProtocolTCP, Username: "a", Password: "b", Enabled: true},
		{Port: 2, Protocol: model.ProtocolUDP, Username: "c", Password: "d", Enabled: false},
	}}
	cfg := n.BuildDesired()
	if !cfg.Enable {
		t.Fatal("expected enable")
	}
	if len(cfg.PortBindings) != 1 || cfg.PortBindings[0].Port != 1 {
		t.Fatalf("unexpected bindings: %+v", cfg.PortBindings)
	}
	if len(cfg.Users) != 1 || cfg.Users[0].Name != "a" {
		t.Fatalf("unexpected users: %+v", cfg.Users)
	}
}

func TestClashYAML(t *testing.T) {
	n := &model.Node{Name: "hk-1", Address: "1.2.3.4", Ports: []*model.Port{
		{Port: 23456, Protocol: model.ProtocolTCP, Username: "u", Password: "p", Enabled: true},
	}}
	out := ClashYAML(BuildClashEntries(n))
	for _, want := range []string{"type: mieru", "server: \"1.2.3.4\"", "port: 23456", "transport: TCP", "handshake-mode: HANDSHAKE_NO_WAIT", "mtu: 1400", "multiplexing: MULTIPLEXING_OFF", "traffic-pattern:", "MATCH,mieru"} {
		if !strings.Contains(out, want) {
			t.Fatalf("clash yaml missing %q:\n%s", want, out)
		}
	}
}

func TestClashSkipsNodeWithoutAddress(t *testing.T) {
	n := &model.Node{Name: "hk-1", Ports: []*model.Port{
		{Port: 1, Protocol: model.ProtocolTCP, Username: "u", Password: "p", Enabled: true},
	}}
	if entries := BuildClashEntries(n); len(entries) != 0 {
		t.Fatalf("expected no entries without address, got %+v", entries)
	}
}

func TestServerConfigEgressUsesMitaFormatAndWildcard(t *testing.T) {
	cfg := &model.DesiredConfig{Egress: &model.EgressConfig{
		Proxies: []model.EgressProxy{{Name: "landing-jp", Protocol: model.EgressProtocolSocks5, Host: "127.0.0.1", Port: 1080, Username: "u", Password: "p", Enabled: true}},
		Rules:   []model.EgressRule{{Name: "all", Action: model.EgressProxyAction, ProxyNames: []string{"landing-jp"}, Enabled: true}},
	}}
	out, err := ServerConfigJSON(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"\"protocol\": \"SOCKS5_PROXY_PROTOCOL\"", "\"socks5Authentication\"", "\"action\": \"PROXY\"", "\"proxyNames\"", "landing-jp", "\"domainNames\"", "\"ipRanges\"", "\"*\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("server config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\"ports\"") || strings.Contains(text, "\"enabled\"") || strings.Contains(text, "\"order\"") {
		t.Fatalf("manager-only egress fields leaked into mita config:\n%s", text)
	}
}
