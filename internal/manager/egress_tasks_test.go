package manager

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"merit/internal/model"
)

func TestNodeEgressTestRoundTrip(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "node", Name: "node", APIKey: "key", Registered: true, LastSeen: time.Now(),
		Egress: &model.EgressConfig{Proxies: []model.EgressProxy{{ID: "proxy", Name: "landing", Host: "example.com", Port: 1080, Password: "secret", Enabled: true}}}}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	session := s.newSession()
	path := "/panel/test/api/nodes/node/egress/proxy/test"
	request := func(method string, body []byte, agent bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		if agent {
			req.URL.Path = "/panel/test/api/agent/result"
			req.Header.Set("X-API-Key", "key")
		} else {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
		}
		res := httptest.NewRecorder()
		s.Handler().ServeHTTP(res, req)
		return res
	}
	started := request(http.MethodPost, nil, false)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", started.Code, started.Body.String())
	}
	if strings.Contains(started.Body.String(), "secret") {
		t.Fatal("test response leaked credentials")
	}
	var result model.EgressTestResult
	if err := json.Unmarshal(started.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	s.enqueueSync(n)
	tasks := s.pollTasks(n.ID, 0)
	if len(tasks) != 2 {
		t.Fatalf("sync discarded test task: %+v", tasks)
	}
	reply, _ := json.Marshal(model.TaskResult{TaskID: result.TaskID, OK: false, Message: "authentication failed", EgressTest: &model.EgressTestResult{ProxyID: "proxy"}})
	if got := request(http.MethodPost, reply, true); got.Code != http.StatusOK {
		t.Fatal(got.Body.String())
	}
	completed := request(http.MethodGet, nil, false)
	if err := json.Unmarshal(completed.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "completed" || result.OK || result.Message != "authentication failed" || result.Source != "agent" {
		t.Fatalf("unexpected result: %+v", result)
	}
	remaining := s.pollTasks(n.ID, 0)
	if len(remaining) != 1 || remaining[0].Kind != model.TaskSync {
		t.Fatal("failed test should finish without consuming sync task")
	}
}
