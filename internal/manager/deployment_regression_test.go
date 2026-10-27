package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"merit/internal/model"
)

func TestUnsupportedTrafficLimitRejectedAtomically(t *testing.T) {
	for _, proxyType := range []string{model.ProxyTypeNaive, model.ProxyTypeAnyTLS, model.ProxyTypeShadowTLS} {
		t.Run(proxyType, func(t *testing.T) {
			s, request := protocolTestServer(t)
			res := request("POST", "", map[string]any{"port": 443, "proxyType": proxyType, "protocol": "TCP", "domain": "proxy.example.com", "autoACME": true})
			if res.Code != 200 {
				t.Fatal(res.Body.String())
			}
			p := s.store.Node("node").Ports[0]
			res = request("PATCH", "/"+p.ID, map[string]any{"trafficLimit": 100, "label": "must not persist"})
			got := s.store.Node("node").FindPort(p.ID)
			if res.Code != 400 || got.TrafficLimit != 0 || got.Label != p.Label {
				t.Fatalf("unsupported quota persisted: %d %+v", res.Code, got)
			}
			_, err := s.store.Update("node", func(n *model.Node) error {
				legacy := n.FindPort(p.ID)
				legacy.TrafficLimit, legacy.TrafficUsed = 1, 2
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if res = request("PATCH", "/"+p.ID, map[string]any{"enabled": true}); res.Code != 200 || !s.store.Node("node").FindPort(p.ID).Enabled {
				t.Fatalf("legacy unsupported quota disabled port: %s", res.Body.String())
			}
			if res = request("PATCH", "/"+p.ID, map[string]any{"trafficLimit": 0}); res.Code != 200 {
				t.Fatalf("could not clear legacy quota: %s", res.Body.String())
			}
		})
	}
}

func TestEgressRejectsUnsupportedScopedRuleAtomically(t *testing.T) {
	s, request := protocolTestServer(t)
	if res := request("POST", "", map[string]any{"port": 443, "proxyType": "anytls", "protocol": "TCP"}); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	for _, ports := range [][]int{{443}, nil} {
		cfg := model.EgressConfig{Rules: []model.EgressRule{{ID: "scope", Enabled: true, Action: model.EgressDirect, Ports: ports}}}
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/egress", bytes.NewReader(raw))
		req.SetPathValue("id", "node")
		res := httptest.NewRecorder()
		s.handlePutEgress(res, req)
		if len(ports) > 0 {
			if res.Code != 400 || s.store.Node("node").Egress != nil {
				t.Fatalf("unsupported scoped egress persisted: %d %s", res.Code, res.Body.String())
			}
		} else if res.Code != 200 || s.store.Node("node").Egress == nil {
			t.Fatalf("all-Mieru rule rejected: %d %s", res.Code, res.Body.String())
		}
	}
}

func TestUnsupportedProtocolTrafficIsIgnored(t *testing.T) {
	p := &model.Port{ProxyType: model.ProxyTypeAnyTLS, Enabled: true, TrafficLimit: 1, TrafficResetAt: time.Now()}
	if updatePortTraffic(p, 100, time.Now()) || p.TrafficUsed != 0 || !p.Enabled || p.AutoDisabled {
		t.Fatalf("unsupported protocol metrics applied: %+v", p)
	}
}

func TestEmptyAddressHasFailedNodeStatus(t *testing.T) {
	result := (&Server{}).TestNode(context.Background(), &model.Node{})
	if result.Status != "failed" || result.Success || len(result.Errors) == 0 {
		t.Fatalf("invalid node status: %+v", result)
	}
}

func TestNewListenerReservesManagedShadowTLSBackend(t *testing.T) {
	s, request := protocolTestServer(t)
	backend := addShadowTLS(t, s, request, 443, nil)
	if backend.BackendAddr != "127.0.0.1:50000" {
		t.Fatal(backend.BackendAddr)
	}
	res := request("POST", "", map[string]any{"port": 50000, "proxyType": "anytls", "protocol": "TCP"})
	if res.Code != 400 || len(s.store.Node("node").Ports) != 1 {
		t.Fatalf("conflicting listener persisted: %d %s", res.Code, res.Body.String())
	}
	res = request("POST", "", map[string]any{"port": 50000, "proxyType": "mieru", "protocol": "UDP"})
	if res.Code != 200 {
		t.Fatalf("UDP does not conflict with TCP backend: %d %s", res.Code, res.Body.String())
	}
}

func TestEnableRejectsLegacyBackendConflictAtomically(t *testing.T) {
	s, request := protocolTestServer(t)
	addShadowTLS(t, s, request, 443, nil)
	_, err := s.store.Update("node", func(n *model.Node) error {
		n.Ports = append(n.Ports, &model.Port{ID: "legacy", Port: 50000, ProxyType: model.ProxyTypeAnyTLS, Protocol: model.ProtocolTCP, Password: "p"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	res := request("PATCH", "/legacy", map[string]any{"enabled": true})
	if res.Code != 400 || s.store.Node("node").FindPort("legacy").Enabled {
		t.Fatalf("legacy conflict enabled: %d %s", res.Code, res.Body.String())
	}
}

func TestUDPOnlyPortIsUnknownWithoutTCPFailure(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	p := &model.Port{ID: "udp", Port: conn.LocalAddr().(*net.UDPAddr).Port, Protocol: model.ProtocolUDP, ProxyType: model.ProxyTypeMieru, Enabled: true}
	n := &model.Node{Address: "127.0.0.1", Ports: []*model.Port{p}}
	s := &Server{}
	result := s.TestPort(context.Background(), n, p)
	if result.Status != "unknown" || result.TCPChecked || result.UDPReachable || result.ProtocolVerified || len(result.Errors) != 0 {
		t.Fatalf("UDP was misrepresented: %+v", result)
	}
	nodeResult := s.TestNode(context.Background(), n)
	if nodeResult.Status != "unknown" || nodeResult.Success || len(nodeResult.Errors) != 0 {
		t.Fatalf("UDP node incorrectly failed or passed: %+v", nodeResult)
	}
}

func TestTCPReachabilityDoesNotImplyProtocolOrServiceStatus(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	p := &model.Port{ID: "tcp", Port: l.Addr().(*net.TCPAddr).Port, Protocol: model.ProtocolTCP, ProxyType: model.ProxyTypeAnyTLS, Enabled: true}
	n := &model.Node{Address: "127.0.0.1", Ports: []*model.Port{p}}
	s := &Server{}
	result := s.TestPort(context.Background(), n, p)
	if result.Status != "reachable" || !result.TCPConnectable || result.ServiceRunning || result.ServiceStatusKnown || result.ProtocolVerified {
		t.Fatalf("generic TCP listener promoted to running proxy: %+v", result)
	}
	if nodeResult := s.TestNode(context.Background(), n); !nodeResult.Success || len(nodeResult.Errors) != 0 {
		t.Fatalf("unrelated SSH/HTTP ports blocked node test: %+v", nodeResult)
	}
	n.Registered = true
	n.LastSeen = time.Now()
	p.InstanceRunning = true
	if result := s.TestPort(context.Background(), n, p); !result.ServiceStatusKnown || !result.ServiceRunning {
		t.Fatalf("Agent status not used: %+v", result)
	}
}

func TestUDPProbeRequiresResponse(t *testing.T) {
	for _, respond := range []bool{false, true} {
		l, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		if respond {
			go func() {
				var b [16]byte
				n, addr, err := l.ReadFrom(b[:])
				if err == nil {
					_, _ = l.WriteTo(b[:n], addr)
				}
			}()
		}
		got := (&Server{}).testUDPPort(ctx, "127.0.0.1", l.LocalAddr().(*net.UDPAddr).Port)
		cancel()
		_ = l.Close()
		if got != respond {
			t.Fatalf("UDP response=%v reported reachable=%v", respond, got)
		}
	}
}

func TestIPv6ProbeAddressAndTCPListener(t *testing.T) {
	for _, host := range []string{"::1", "[::1]"} {
		if got := probeAddress(host, 443); got != "[::1]:443" {
			t.Fatal(got)
		}
	}
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 unavailable")
	}
	defer l.Close()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	portNumber, _ := strconv.Atoi(port)
	if !(&Server{}).testTCPPort(context.Background(), "::1", portNumber) {
		t.Fatal("IPv6 TCP probe failed")
	}
}
