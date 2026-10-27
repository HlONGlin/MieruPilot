package manager

import (
	"net/http/httptest"
	"testing"

	"merit/internal/clash"
	"merit/internal/model"
)

func addShadowTLS(t *testing.T, s *Server, request func(string, string, any) *httptest.ResponseRecorder, port int, extra map[string]any) *model.Port {
	t.Helper()
	body := map[string]any{"port": port, "proxyType": "shadowtls", "protocol": "TCP"}
	for k, v := range extra {
		body[k] = v
	}
	res := request("POST", "", body)
	if res.Code != 200 {
		t.Fatalf("add port %d: %d %s", port, res.Code, res.Body.String())
	}
	for _, p := range s.store.Node("node").Ports {
		if p.Port == port {
			return p
		}
	}
	t.Fatalf("port %d not stored", port)
	return nil
}

func TestShadowTLSEmptyBackendIsManaged(t *testing.T) {
	s, request := protocolTestServer(t)
	p := addShadowTLS(t, s, request, 443, nil)
	if !p.BackendManaged {
		t.Fatal("empty backend should be managed by the Manager")
	}
	if p.BackendAddr != "127.0.0.1:50000" {
		t.Errorf("BackendAddr = %q, want the first free loopback port", p.BackendAddr)
	}
	if p.BackendPassword == "" || p.BackendCipher != "aes-256-gcm" {
		t.Errorf("credentials were not generated: %q %q", p.BackendPassword, p.BackendCipher)
	}

	second := addShadowTLS(t, s, request, 8443, nil)
	if second.BackendAddr == p.BackendAddr || second.BackendPassword == p.BackendPassword {
		t.Errorf("two ports share a backend: %q / %q", p.BackendAddr, second.BackendAddr)
	}
}

func TestShadowTLSManagedBackendIsStable(t *testing.T) {
	s, request := protocolTestServer(t)
	p := addShadowTLS(t, s, request, 443, nil)
	addr, password := p.BackendAddr, p.BackendPassword

	// A label edit, and an edit that echoes the stored address back (as the panel
	// does), must neither re-allocate nor change the credentials.
	for _, body := range []map[string]any{
		{"label": "renamed"},
		{"label": "again", "backendAddr": addr, "backendPassword": password, "backendCipher": "aes-256-gcm"},
	} {
		if res := request("PATCH", "/"+p.ID, body); res.Code != 200 {
			t.Fatalf("patch: %d %s", res.Code, res.Body.String())
		}
		got := s.store.Node("node").FindPort(p.ID)
		if !got.BackendManaged || got.BackendAddr != addr || got.BackendPassword != password {
			t.Fatalf("managed backend changed after %v: %+v", body, got)
		}
	}
}

func TestShadowTLSManualBackendIsKept(t *testing.T) {
	s, request := protocolTestServer(t)
	p := addShadowTLS(t, s, request, 443, map[string]any{
		"backendAddr": "127.0.0.1:8388", "backendPassword": "mine", "backendCipher": "chacha20-ietf-poly1305",
	})
	if p.BackendManaged || p.BackendAddr != "127.0.0.1:8388" || p.BackendPassword != "mine" || p.BackendCipher != "chacha20-ietf-poly1305" {
		t.Fatalf("manual backend was altered: %+v", p)
	}
	// An unrelated edit must not turn it into a managed one.
	if res := request("PATCH", "/"+p.ID, map[string]any{"label": "x"}); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if got := s.store.Node("node").FindPort(p.ID); got.BackendManaged || got.BackendAddr != "127.0.0.1:8388" {
		t.Fatalf("manual backend changed on unrelated edit: %+v", got)
	}
}

func TestShadowTLSSwitchBetweenModes(t *testing.T) {
	s, request := protocolTestServer(t)
	p := addShadowTLS(t, s, request, 443, nil)

	// managed -> manual
	res := request("PATCH", "/"+p.ID, map[string]any{
		"backendAddr": "127.0.0.1:8388", "backendPassword": "mine", "backendCipher": "aes-128-gcm",
	})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	got := s.store.Node("node").FindPort(p.ID)
	if got.BackendManaged || got.BackendAddr != "127.0.0.1:8388" || got.BackendPassword != "mine" {
		t.Fatalf("switch to manual failed: %+v", got)
	}

	// manual -> managed: the hand-run server's credentials must not be reused.
	res = request("PATCH", "/"+p.ID, map[string]any{"backendAddr": ""})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	got = s.store.Node("node").FindPort(p.ID)
	if !got.BackendManaged || got.BackendAddr != "127.0.0.1:50000" || got.BackendPassword == "mine" || got.BackendCipher != "aes-256-gcm" {
		t.Fatalf("switch to managed failed: %+v", got)
	}
}

func TestShadowTLSManagedBackendAvoidsUsedPorts(t *testing.T) {
	s, request := protocolTestServer(t)
	// A listen port inside the backend range and a manual backend both occupy slots.
	addShadowTLS(t, s, request, 50000, map[string]any{"backendAddr": "127.0.0.1:50001", "backendPassword": "x"})
	p := addShadowTLS(t, s, request, 443, nil)
	if p.BackendAddr != "127.0.0.1:50002" {
		t.Errorf("BackendAddr = %q, want 127.0.0.1:50002", p.BackendAddr)
	}
}

func TestShadowTLSRejectsUnsafeManagedInput(t *testing.T) {
	s, request := protocolTestServer(t)
	p := addShadowTLS(t, s, request, 443, nil)
	res := request("PATCH", "/"+p.ID, map[string]any{"backendCipher": "2022-blake3-aes-256-gcm"})
	if res.Code != 400 {
		t.Fatalf("unsupported managed cipher accepted: %d", res.Code)
	}
	if got := s.store.Node("node").FindPort(p.ID); got.BackendCipher != "aes-256-gcm" {
		t.Errorf("rejected patch still modified the port: %q", got.BackendCipher)
	}
}

func TestManagedShadowTLSAppearsInSubscription(t *testing.T) {
	s, request := protocolTestServer(t)
	p := addShadowTLS(t, s, request, 443, nil)
	n := s.store.Node("node")
	n.Address = "203.0.113.9"
	entries := clash.BuildEntries(n)
	if len(entries) != 1 {
		t.Fatalf("managed ShadowTLS port missing from the subscription: %+v", entries)
	}
	if entries[0].BackendPassword != p.BackendPassword || entries[0].BackendCipher != p.BackendCipher {
		t.Errorf("subscription credentials differ from the node's: %+v vs %+v", entries[0], p)
	}
}
