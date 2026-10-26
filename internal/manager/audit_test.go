package manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRecordsSuccessfulAdminMutations(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "merit.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetAdmin("admin", "password"); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	request := func(method, path, body string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authenticated {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.newSession()})
		}
		res := httptest.NewRecorder()
		s.Handler().ServeHTTP(res, req)
		return res
	}
	created := request(http.MethodPost, ts.URL+"/panel/test/api/nodes", `{"name":"edge-a"}`, true)
	if created.Code != http.StatusOK {
		t.Fatalf("create node status = %d: %s", created.Code, created.Body.String())
	}
	var node struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &node); err != nil {
		t.Fatal(err)
	}
	if got := request(http.MethodPost, "/panel/test/api/nodes/"+node.ID+"/ports", `{"port":80}`, true); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid port status = %d, want %d", got.Code, http.StatusBadRequest)
	}
	if got := request(http.MethodGet, "/panel/test/api/audit?limit=10", "", false); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated audit status = %d, want %d", got.Code, http.StatusUnauthorized)
	}
	res := request(http.MethodGet, "/panel/test/api/audit?limit=10", "", true)
	if res.Code != http.StatusOK {
		t.Fatalf("audit status = %d", res.Code)
	}
	var entries []struct {
		Actor  string `json:"actor"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Actor != "admin" || entries[0].Action != "新增节点" {
		t.Fatalf("unexpected audit entries: %+v", entries)
	}
}
