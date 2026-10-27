package manager

import (
	"encoding/json"
	"merit/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShadowTLSClientIncludesBothLayers(t *testing.T) {
	n := &model.Node{Address: "2001:db8::1"}
	p := &model.Port{Port: 40900, ProxyType: "shadowtls", Password: "transport-secret", BackendPassword: "backend-secret", HandshakeAddr: "cloudflare.com:443"}
	cfg, err := clientConfig(n, p)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg)
	for _, s := range []string{"transport-secret", "backend-secret", "shadowtls-transport", "aes-256-gcm", "cloudflare.com", "2001:db8::1"} {
		if !strings.Contains(string(raw), s) {
			t.Fatalf("missing %s", s)
		}
	}
	p.BackendPassword = ""
	if _, err = clientConfig(n, p); err == nil {
		t.Fatal("accepted missing backend credentials")
	}
}

func TestNaiveClientUsesVerifiedHTTPS(t *testing.T) {
	cfg, err := clientConfig(&model.Node{Address: "192.0.2.1"}, &model.Port{Port: 443, ProxyType: "naive", Domain: "proxy.example.com", Username: "user", Password: "a@b:c", Label: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfg["proxy"].(string), "https://") || strings.Contains(cfg["proxy"].(string), "#") {
		t.Fatal(cfg)
	}
}

func TestPortEditChangesCredentialsAndRejectsConflicts(t *testing.T) {
	s, request := protocolTestServer(t)
	_, err := s.store.Update("node", func(n *model.Node) error { n.Address = "192.0.2.1"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, port := range []int{24001, 24002} {
		res := request("POST", "", map[string]any{"port": port, "proxyType": "anytls"})
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
	}
	id := s.store.Node("node").Ports[0].ID
	res := request("PATCH", "/"+id, map[string]any{"port": 24003, "password": "edited-secret", "label": "edited"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	p := s.store.Node("node").FindPort(id)
	if p.Port != 24003 || p.Password != "edited-secret" || p.Label != "edited" {
		t.Fatal("edit not persisted")
	}
	for _, patch := range []map[string]any{{"port": 24002}, {"port": 0}, {"protocol": "UDP"}, {"password": ""}} {
		res = request("PATCH", "/"+id, patch)
		if res.Code != 400 {
			t.Fatalf("invalid edit accepted: %v %s", patch, res.Body.String())
		}
		if s.store.Node("node").FindPort(id).Port != 24003 {
			t.Fatal("failed edit changed persisted state")
		}
	}
	res = request("GET", "/"+id+"/client.json", nil)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "edited-secret") {
		t.Fatal(res.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/panel/test/api/nodes/node/ports/"+id+"/client.json", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("unauthenticated export status %d", rr.Code)
	}
}

func TestPortMenuIncludesRequestedActions(t *testing.T) {
	script, err := webFS.ReadFile("web/assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"详细信息", "测试联通性", "编辑信息", "showPortDetails(id)", "testPortConnectivity(id)", "editPortInfo(id)"} {
		if !strings.Contains(string(script), label) {
			t.Fatalf("missing menu action %s", label)
		}
	}
}
