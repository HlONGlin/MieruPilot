package manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"merit/internal/model"
)

func TestFirstSetupAndLogin(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "merit.json")
	s, err := New(Config{DataPath: dataPath, AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	getMe := func() map[string]any {
		res, err := http.Get(ts.URL + "/panel/test/api/me")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var got map[string]any
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := getMe(); got["setup"] != true {
		t.Fatalf("expected setup=true, got %#v", got)
	}

	body := strings.NewReader(`{"username":"admin","password":"password"}`)
	res, err := http.Post(ts.URL+"/panel/test/api/setup", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("setup status = %d", res.StatusCode)
	}
	if got := getMe(); got["setup"] != false {
		t.Fatalf("expected setup=false, got %#v", got)
	}
}

func TestInstallCommandIncludesKey(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "merit.json"), AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "node-1", Name: "test", APIKey: "secret-key", CreatedAt: time.Now()}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/panel/test/api/nodes/node-1/install", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: s.newSession()})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var got struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want := "/install.sh?key=secret-key"
	if !strings.Contains(got.Command, want) {
		t.Fatalf("install command %q does not contain %q", got.Command, want)
	}
}
