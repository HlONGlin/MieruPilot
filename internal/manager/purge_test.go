package manager

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"merit/internal/model"
)

func newPurgeServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func deleteNodeRequest(t *testing.T, s *Server, id, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/panel/test/api/nodes/"+id+query, nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.newSession()})
	res := httptest.NewRecorder()
	s.Handler().ServeHTTP(res, req)
	return res
}

func postAgentResult(t *testing.T, s *Server, key string, res model.TaskResult) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(res)
	req := httptest.NewRequest(http.MethodPost, "/panel/test/api/agent/result", bytes.NewReader(raw))
	req.Header.Set("X-API-Key", key)
	out := httptest.NewRecorder()
	s.Handler().ServeHTTP(out, req)
	return out
}

func TestDeleteRegisteredNodeQueuesPurgeAndWaitsForAck(t *testing.T) {
	s := newPurgeServer(t)
	n := &model.Node{ID: "node", Name: "n", APIKey: "key", Registered: true, LastSeen: time.Now()}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}

	res := deleteNodeRequest(t, s, n.ID, "")
	if res.Code != http.StatusAccepted {
		t.Fatalf("delete status = %d, want %d: %s", res.Code, http.StatusAccepted, res.Body.String())
	}
	if got := s.store.Node(n.ID); got == nil {
		t.Fatal("node record removed before remote cleanup was acknowledged")
	} else if !got.Deleting {
		t.Fatal("node was not marked as deleting")
	}

	tasks := s.pollTasks(n.ID, 0)
	if len(tasks) != 1 || tasks[0].Kind != model.TaskPurge {
		t.Fatalf("tasks = %+v, want one purge task", tasks)
	}

	// A failed purge keeps both the node and the retryable task.
	if out := postAgentResult(t, s, n.APIKey, model.TaskResult{TaskID: tasks[0].ID, OK: false, Message: "mita busy"}); out.Code != http.StatusOK {
		t.Fatalf("failed result status = %d", out.Code)
	}
	if s.store.Node(n.ID) == nil {
		t.Fatal("node removed after failed purge")
	}
	if got := s.pollTasks(n.ID, 0); len(got) != 1 || got[0].ID != tasks[0].ID {
		t.Fatal("failed purge task was not kept for retry")
	}

	if out := postAgentResult(t, s, n.APIKey, model.TaskResult{TaskID: tasks[0].ID, OK: true, Message: "clean"}); out.Code != http.StatusOK {
		t.Fatalf("successful result status = %d", out.Code)
	}
	if s.store.Node(n.ID) != nil {
		t.Fatal("node record was not removed after successful purge")
	}
}

func TestDeleteUnregisteredNodeRemovesRecordImmediately(t *testing.T) {
	s := newPurgeServer(t)
	n := &model.Node{ID: "node", Name: "n", APIKey: "key"}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	res := deleteNodeRequest(t, s, n.ID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d", res.Code, http.StatusOK)
	}
	if s.store.Node(n.ID) != nil {
		t.Fatal("unregistered node record was not removed")
	}
}

func TestForceDeleteRegisteredNodeSkipsPurge(t *testing.T) {
	s := newPurgeServer(t)
	n := &model.Node{ID: "node", Name: "n", APIKey: "key", Registered: true, LastSeen: time.Now()}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	res := deleteNodeRequest(t, s, n.ID, "?force=1")
	if res.Code != http.StatusOK {
		t.Fatalf("force delete status = %d, want %d", res.Code, http.StatusOK)
	}
	if s.store.Node(n.ID) != nil {
		t.Fatal("force delete left the node record behind")
	}
}

func TestSyncCannotReplaceQueuedPurge(t *testing.T) {
	s := newPurgeServer(t)
	n := &model.Node{ID: "node", Name: "n", APIKey: "key", Registered: true, LastSeen: time.Now()}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	if res := deleteNodeRequest(t, s, n.ID, ""); res.Code != http.StatusAccepted {
		t.Fatalf("delete status = %d", res.Code)
	}
	deleting := s.store.Node(n.ID)
	s.enqueueSync(deleting)
	tasks := s.pollTasks(n.ID, 0)
	if len(tasks) != 1 || tasks[0].Kind != model.TaskPurge {
		t.Fatalf("tasks = %+v, want only the purge task", tasks)
	}
}
