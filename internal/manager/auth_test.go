package manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"merit/internal/model"
)

func TestLoginOnlyAfterScriptInitialization(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "merit.json")
	s, err := New(Config{DataPath: dataPath, AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetAdmin("admin", "password"); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/panel/test/api/login", "application/json", strings.NewReader(`{"username":"admin","password":"password"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", res.StatusCode, http.StatusOK)
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

func TestAgentBinaryAcceptsInstallerName(t *testing.T) {
	agentDir := t.TempDir()
	dataPath := filepath.Join(t.TempDir(), "merit.json")
	if err := os.WriteFile(filepath.Join(agentDir, "merit-agent"), []byte("agent-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{DataPath: dataPath, AgentDir: agentDir, PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/panel/test/download/agent?os=linux&arch=amd64")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("agent download status = %d", res.StatusCode)
	}
}
