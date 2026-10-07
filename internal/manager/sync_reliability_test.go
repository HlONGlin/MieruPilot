package manager

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"merit/internal/model"
)

func TestFailedTaskWithoutPortResultsIsNotLost(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "node", APIKey: "key"}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	s.enqueueSync(n)
	tasks := s.pollTasks(n.ID, 0)
	if len(tasks) != 1 {
		t.Fatal("missing sync task")
	}
	post := func(ok bool) {
		raw, _ := json.Marshal(model.TaskResult{TaskID: tasks[0].ID, OK: ok, Message: "startup result"})
		req := httptest.NewRequest(http.MethodPost, "/panel/test/api/agent/result", bytes.NewReader(raw))
		req.Header.Set("X-API-Key", n.APIKey)
		res := httptest.NewRecorder()
		s.Handler().ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status %d", res.Code)
		}
	}
	post(false)
	if len(s.pollTasks(n.ID, 0)) != 1 {
		t.Fatal("failed sync was removed")
	}
	post(true)
	if len(s.pollTasks(n.ID, 0)) != 0 {
		t.Fatal("successful sync was not acknowledged")
	}
}

func TestSupersededResultCannotOverwritePortState(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "node", APIKey: "key", Ports: []*model.Port{{ID: "port", Port: 1234, Enabled: true, InstanceError: "等待新配置"}}}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	s.enqueueSync(n)
	old := s.pollTasks(n.ID, 0)[0]
	s.enqueueSync(n)
	latest := s.pollTasks(n.ID, 0)[0]
	if old.ID == latest.ID {
		t.Fatal("identical config submissions must have distinct task IDs")
	}
	raw, _ := json.Marshal(model.TaskResult{TaskID: old.ID, OK: true, PortResults: []model.PortInstanceResult{{PortID: "port", OK: true, Running: true}}})
	req := httptest.NewRequest(http.MethodPost, "/panel/test/api/agent/result", bytes.NewReader(raw))
	req.Header.Set("X-API-Key", "key")
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatal(res.Body.String())
	}
	port := s.store.Node(n.ID).FindPort("port")
	if port.InstanceRunning || port.InstanceError != "等待新配置" {
		t.Fatalf("old result overwrote new state: %+v", port)
	}
	if tasks := s.pollTasks(n.ID, 0); len(tasks) != 1 || tasks[0].ID != latest.ID {
		t.Fatal("old result removed latest task")
	}
}
