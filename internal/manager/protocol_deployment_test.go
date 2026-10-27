package manager

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"merit/internal/model"
)

func protocolTestServer(t *testing.T) (*Server, func(string, string, any) *httptest.ResponseRecorder) {
	t.Helper()
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.AddNode(&model.Node{ID: "node", APIKey: "key"}); err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: s.newSession()}
	return s, func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/panel/test/api/nodes/node/ports"+path, bytes.NewReader(raw))
		req.AddCookie(cookie)
		res := httptest.NewRecorder()
		s.Handler().ServeHTTP(res, req)
		return res
	}
}

func TestProtocolListenPorts(t *testing.T) {
	for _, tc := range []struct {
		name, proxyType, protocol string
		port, status              int
	}{
		{"naive acme", model.ProxyTypeNaive, "TCP", 443, 200},
		{"naive acme wrong port", model.ProxyTypeNaive, "TCP", 8443, 400},
		{"mieru privileged", model.ProxyTypeMieru, "TCP", 443, 400},
		{"anytls privileged", model.ProxyTypeAnyTLS, "TCP", 443, 200},
		{"anytls udp", model.ProxyTypeAnyTLS, "UDP", 8443, 400},
		{"mieru udp", model.ProxyTypeMieru, "UDP", 23456, 200},
		{"invalid zero", model.ProxyTypeAnyTLS, "TCP", 0, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, request := protocolTestServer(t)
			res := request("POST", "", map[string]any{
				"port": tc.port, "proxyType": tc.proxyType, "protocol": tc.protocol,
				"domain": "test.example.com", "autoACME": true,
			})
			if res.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", res.Code, tc.status, res.Body.String())
			}
		})
	}
}

func TestNaiveCertificatePatchIsAtomicAndValidated(t *testing.T) {
	s, request := protocolTestServer(t)
	cert, key, err := generateTestCert(90)
	if err != nil {
		t.Fatal(err)
	}
	res := request("POST", "", map[string]any{
		"port": 8443, "proxyType": "naive", "domain": "test.example.com",
		"tlsCert": cert, "tlsKey": key,
	})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	port := s.store.Node("node").Ports[0]
	newCert, newKey, err := generateTestCert(120)
	if err != nil {
		t.Fatal(err)
	}
	newCert, newKey = strings.TrimSpace(newCert), strings.TrimSpace(newKey)
	res = request("PATCH", "/"+port.ID, map[string]any{"tlsCert": newCert, "tlsKey": newKey})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	updated := s.store.Node("node").FindPort(port.ID)
	if updated.TLSCert != newCert || updated.TLSKey != newKey {
		t.Fatal("certificate PATCH was not persisted")
	}
	for _, invalid := range []map[string]any{
		{"tlsCert": "invalid", "tlsKey": "invalid"},
		{"tlsCert": cert, "tlsKey": newKey},
		{"domain": "wrong.example.org"},
		{"autoACME": true},
	} {
		res = request("PATCH", "/"+port.ID, invalid)
		if res.Code != 400 {
			t.Fatalf("invalid patch accepted: %s", res.Body.String())
		}
		updated = s.store.Node("node").FindPort(port.ID)
		if updated.TLSCert != newCert || updated.AutoACME || updated.Domain != "test.example.com" {
			t.Fatal("invalid patch modified the previous certificate")
		}
	}
}

func TestLegacyAgentPartialFailureRetainsTask(t *testing.T) {
	s, _ := protocolTestServer(t)
	n := s.store.Node("node")
	s.enqueueSync(n)
	task := s.pollTasks(n.ID, 0)[0]
	raw, _ := json.Marshal(model.TaskResult{TaskID: task.ID, OK: true,
		PortResults: []model.PortInstanceResult{{PortID: "failed", Port: 8443, OK: false, Error: "download failed"}},
	})
	req := httptest.NewRequest("POST", "/panel/test/api/agent/result", bytes.NewReader(raw))
	req.Header.Set("X-API-Key", "key")
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != 200 || len(s.pollTasks(n.ID, 0)) != 1 {
		t.Fatalf("failed task was lost: %s", res.Body.String())
	}
	if len(s.store.PendingTasks()[n.ID]) != 1 {
		t.Fatal("retry was not persisted across manager restarts")
	}
}
