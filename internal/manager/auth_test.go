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

func TestPortRetryQueuesSinglePortConfig(t *testing.T) {
	server, err := New(Config{DataPath: filepath.Join(t.TempDir(), "merit.json"), AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "node-test", Name: "test", APIKey: "api-key", Ports: []*model.Port{
		{ID: "port-a", Port: 1234, Protocol: model.ProtocolTCP, Username: "user-a", Password: "pass-a", Enabled: true},
		{ID: "port-b", Port: 1235, Protocol: model.ProtocolUDP, Username: "user-b", Password: "pass-b", Enabled: true},
	}}
	if err := server.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	if err := server.enqueuePortSync(n, "port-b"); err != nil {
		t.Fatal(err)
	}
	runtime := server.nodeRuntime(n.ID)
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	var task *model.Task
	for _, pending := range runtime.pending {
		task = pending
	}
	if task == nil || task.Config == nil {
		t.Fatal("expected port sync task")
	}
	if len(task.Config.PortBindings) != 1 || task.Config.PortBindings[0].Port != 1235 {
		t.Fatalf("retry must only contain port 1235, got %+v", task.Config.PortBindings)
	}
	if len(task.Config.PortIDs) != 1 || task.Config.PortIDs[0] != "port-b" {
		t.Fatalf("retry must include target port ID, got %+v", task.Config.PortIDs)
	}
}

func TestSyncQueueKeepsLatestDesiredStateAndIndependentPortRetries(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "merit.json"), AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "queue-node", Name: "queue", APIKey: "key", Ports: []*model.Port{
		{ID: "a", Port: 1111, Protocol: model.ProtocolTCP, Username: "a", Password: "a", Enabled: true},
		{ID: "b", Port: 2222, Protocol: model.ProtocolTCP, Username: "b", Password: "b", Enabled: true},
	}}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	if err := s.enqueuePortSync(n, "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.enqueuePortSync(n, "b"); err != nil {
		t.Fatal(err)
	}
	rt := s.nodeRuntime(n.ID)
	rt.mu.Lock()
	if len(rt.pending) != 2 {
		rt.mu.Unlock()
		t.Fatalf("expected retries for both ports, got %d tasks", len(rt.pending))
	}
	rt.mu.Unlock()

	s.enqueueSync(n)
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.pending) != 1 {
		t.Fatalf("full sync should supersede partial retries; got %d tasks", len(rt.pending))
	}
	for _, task := range rt.pending {
		if task.Config.Partial || len(task.Config.PortBindings) != 2 {
			t.Fatalf("expected full desired state task, got %+v", task.Config)
		}
	}
}

func TestPortInstanceResultPersistsHealthAndSyncTimes(t *testing.T) {
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "merit.json"), AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "node-health", Name: "health", APIKey: "health-key", Ports: []*model.Port{{ID: "port-health", Port: 4567, Enabled: true}}}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Now().UTC().Truncate(time.Second)
	if _, err := s.store.Update(n.ID, func(n *model.Node) error {
		p := n.FindPort("port-health")
		p.LastSyncAttempt = checkedAt
		p.InstanceRunning = true
		p.InstanceSyncedAt = checkedAt
		p.InstanceError = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got := s.store.Node(n.ID).FindPort("port-health")
	if !got.InstanceRunning || !got.InstanceSyncedAt.Equal(checkedAt) || !got.LastSyncAttempt.Equal(checkedAt) {
		t.Fatalf("port instance health was not persisted: %+v", got)
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

func TestValidateEgressNormalizesEmptyListsAndAssignsIDs(t *testing.T) {
	cfg := model.EgressConfig{
		Proxies: []model.EgressProxy{{Name: "jp", Host: "127.0.0.1", Port: 1080}},
		Rules:   []model.EgressRule{{Name: "direct", Action: model.EgressDirect}},
	}
	if err := validateEgress(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Proxies[0].ID == "" || cfg.Rules[0].ID == "" {
		t.Fatalf("IDs were not assigned: %+v", cfg)
	}
	if cfg.Proxies[0].Protocol != model.EgressProtocolSocks5 {
		t.Fatalf("unexpected proxy protocol %q", cfg.Proxies[0].Protocol)
	}

	empty := model.EgressConfig{}
	if err := validateEgress(&empty); err != nil {
		t.Fatal(err)
	}
	if empty.Proxies == nil || empty.Rules == nil {
		t.Fatalf("empty egress lists were not normalized: %+v", empty)
	}
}

func TestEgressDTOMasksSecretsWithoutMutatingStoreConfig(t *testing.T) {
	stored := &model.EgressConfig{Proxies: []model.EgressProxy{{ID: "p1", Name: "landing", Password: "secret"}}}
	got := egressDTO(stored)
	if got.Proxies[0].Password == "secret" || got.Proxies[0].Password == "" {
		t.Fatalf("egress API DTO should mask, not expose or omit, stored password: %+v", got.Proxies[0])
	}
	if stored.Proxies[0].Password != "secret" {
		t.Fatal("creating a DTO must not mutate the persisted config")
	}
}

func TestHasRetryablePortFailure(t *testing.T) {
	if !hasRetryablePortFailure(model.TaskResult{PortResults: []model.PortInstanceResult{{OK: false, Error: "mita start failed"}}}) {
		t.Fatal("expected failed instance to be retryable")
	}
	if hasRetryablePortFailure(model.TaskResult{PortResults: []model.PortInstanceResult{{OK: true, Running: false}}}) {
		t.Fatal("successful stopped/removed instance should not be retried")
	}
}
