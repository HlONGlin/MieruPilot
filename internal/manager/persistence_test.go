package manager

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"merit/internal/model"
)

func TestManagerRecoversAndAcknowledgesPersistedTasks(t *testing.T) {
	cfg := Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "n", APIKey: "k"}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	s.enqueueSync(n)
	tasks := s.pollTasks(n.ID, 0)
	restarted, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	recovered := restarted.pollTasks(n.ID, 0)
	if len(recovered) != 1 || recovered[0].ID != tasks[0].ID {
		t.Fatal("Manager lost persisted task")
	}
	restarted.clearTask(n.ID, recovered[0].ID)
	third, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(third.pollTasks(n.ID, 0)) != 0 {
		t.Fatal("acknowledged task resurrected")
	}
}

func TestSplitFrontendAssetsUnderRandomPath(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "data.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/panel/test", "/panel/test/assets/app.js", "/panel/test/assets/style.css"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Body.Len() == 0 {
			t.Fatalf("asset %s: status %d", path, w.Code)
		}
	}
}
