// Package manager implements the merit control panel and agent API.
package manager

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"merit/internal/buildinfo"
	"merit/internal/model"
	"merit/internal/store"
)

//go:embed web
var webFS embed.FS

// Version is the manager build version.
var Version = buildinfo.String()

// Config configures the manager server.
type Config struct {
	Addr      string
	DataPath  string
	AgentDir  string
	PublicURL string
	PanelPath string
}

// Server is the manager HTTP server.
type Server struct {
	cfg       Config
	store     *store.Store
	mux       *http.ServeMux
	version   string
	panelPath string
	requestMu sync.RWMutex
	cloudMu   sync.Mutex

	mu       sync.Mutex
	sessions map[string]time.Time
	rt       map[string]*nodeRuntime
	taskSeq  uint64
}

type nodeRuntime struct {
	mu          sync.Mutex
	pending     map[string]*model.Task
	notify      chan struct{}
	initialized bool
	tests       map[string]*model.EgressTestResult
}

// New creates a manager server, opening the data store.
func New(cfg Config) (*Server, error) {
	st, err := store.Open(cfg.DataPath)
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:      cfg,
		store:    st,
		mux:      http.NewServeMux(),
		version:  Version,
		sessions: map[string]time.Time{},
		rt:       map[string]*nodeRuntime{},
	}
	panelPath, err := st.EnsurePanelPath(cfg.PanelPath)
	if err != nil {
		return nil, err
	}
	s.panelPath = panelPath
	for id, tasks := range st.PendingTasks() {
		runtime := s.nodeRuntime(id)
		runtime.pending = tasks
		for taskID, task := range tasks {
			if task.Kind == model.TaskTestEgress {
				delete(runtime.pending, taskID)
			}
		}
		_ = st.SetTasks(id, runtime.pending)
	}
	s.routes()

	// Start certificate expiry monitoring (check every 6 hours)
	go s.MonitorCertificates(6 * time.Hour)

	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && (r.URL.Path == s.panelPath+"/api/backup/restore" || r.URL.Path == s.panelPath+"/api/backup/webdav/restore") {
			s.requestMu.Lock()
			defer s.requestMu.Unlock()
		} else {
			s.requestMu.RLock()
			defer s.requestMu.RUnlock()
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	p := s.panelPath
	assets, _ := fs.Sub(webFS, "web/assets")
	assetHandler := http.StripPrefix(p+"/assets/", http.FileServer(http.FS(assets)))
	s.mux.Handle("GET "+p+"/assets/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		assetHandler.ServeHTTP(w, r)
	}))
	s.mux.HandleFunc("GET "+p, s.handleIndex)
	s.mux.HandleFunc("GET "+p+"/", s.handleIndex)

	s.mux.HandleFunc("GET "+p+"/api/me", s.handleMe)
	s.mux.HandleFunc("POST "+p+"/api/login", s.handleLogin)
	s.mux.HandleFunc("POST "+p+"/api/logout", s.handleLogout)

	s.mux.Handle("GET "+p+"/api/nodes", s.auth(s.handleListNodes))
	s.mux.Handle("POST "+p+"/api/nodes", s.auth(s.auditMutation(s.handleCreateNode)))
	s.mux.Handle("GET "+p+"/api/nodes/{id}", s.auth(s.handleGetNode))
	s.mux.Handle("PATCH "+p+"/api/nodes/{id}", s.auth(s.auditMutation(s.handleUpdateNode)))
	s.mux.Handle("DELETE "+p+"/api/nodes/{id}", s.auth(s.auditMutation(s.handleDeleteNode)))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/ports", s.auth(s.auditMutation(s.handleAddPort)))
	s.mux.Handle("PATCH "+p+"/api/nodes/{id}/ports/{pid}", s.auth(s.auditMutation(s.handleUpdatePort)))
	s.mux.Handle("DELETE "+p+"/api/nodes/{id}/ports/{pid}", s.auth(s.auditMutation(s.handleDeletePort)))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/ports/{pid}/retry", s.auth(s.auditMutation(s.handleRetryPortSync)))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/ports/{pid}/cert", s.auth(s.auditMutation(s.handleUploadCert)))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/ports/{pid}/cert", s.auth(s.handleGetCertInfo))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/ports/{pid}/client.json", s.auth(s.handlePortClientConfig))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/test", s.auth(s.handleTestNode))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/ports/{pid}/test", s.auth(s.handleTestPort))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/config", s.auth(s.handleGetNodeConfig))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/egress", s.auth(s.handleGetEgress))
	s.mux.Handle("PUT "+p+"/api/nodes/{id}/egress", s.auth(s.auditMutation(s.handlePutEgress)))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/egress/{proxyId}/test", s.auth(s.handleTestEgress))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/egress/{proxyId}/test", s.auth(s.handleGetEgressTest))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/install", s.auth(s.handleInstall))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/links", s.auth(s.handleLinks))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/clash.yaml", s.auth(s.handleNodeClash))

	s.mux.Handle("GET "+p+"/api/proxy-types", s.auth(s.handleGetProxyTypes))
	s.mux.Handle("GET "+p+"/api/settings", s.auth(s.handleSettings))
	s.mux.Handle("GET "+p+"/api/certificates/expiring", s.auth(s.handleGetExpiringCertificates))
	s.mux.Handle("GET "+p+"/api/audit", s.auth(s.handleAudit))
	s.mux.Handle("GET "+p+"/api/backup", s.auth(s.handleBackup))
	s.mux.Handle("POST "+p+"/api/backup/restore", s.auth(s.auditMutation(s.handleRestore)))
	s.mux.Handle("GET "+p+"/api/backup/webdav", s.auth(s.handleCloudBackupSettings))
	s.mux.Handle("PUT "+p+"/api/backup/webdav", s.auth(s.auditMutation(s.handleSaveCloudBackupSettings)))
	s.mux.Handle("POST "+p+"/api/backup/webdav/test", s.auth(s.handleTestCloudBackup))
	s.mux.Handle("GET "+p+"/api/backup/webdav/files", s.auth(s.handleListCloudBackups))
	s.mux.Handle("POST "+p+"/api/backup/webdav/upload", s.auth(s.auditMutation(s.handleUploadCloudBackup)))
	s.mux.Handle("POST "+p+"/api/backup/webdav/restore", s.auth(s.auditMutation(s.handleRestoreCloudBackup)))
	s.mux.Handle("POST "+p+"/api/settings/sub-token/rotate", s.auth(s.auditMutation(s.handleRotateSubToken)))
	s.mux.Handle("GET "+p+"/api/update/check", s.auth(s.handleCheckUpdate))
	s.mux.Handle("POST "+p+"/api/update/apply", s.auth(s.auditMutation(s.handleApplyUpdate)))

	s.mux.HandleFunc("GET "+p+"/sub", s.handleSubscription)
	s.mux.HandleFunc("GET "+p+"/install.sh", s.handleInstallScript)
	s.mux.HandleFunc("GET "+p+"/download/agent", s.handleAgentBinary)

	s.mux.HandleFunc("POST "+p+"/api/agent/report", s.handleAgentReport)
	s.mux.HandleFunc("GET "+p+"/api/agent/poll", s.handleAgentPoll)
	s.mux.HandleFunc("POST "+p+"/api/agent/result", s.handleAgentResult)
}

// --- sessions ---------------------------------------------------------------

const sessionCookie = "merit_session"

func (s *Server) newSession() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	token := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(30 * 24 * time.Hour)
	s.mu.Unlock()
	return token
}

func (s *Server) checkSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.sessions, c.Value)
		return false
	}
	return true
}

func (s *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.checkSession(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next(w, r)
	})
}

// --- runtime task queues ----------------------------------------------------

func (s *Server) nodeRuntime(id string) *nodeRuntime {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rt[id]
	if r == nil {
		r = &nodeRuntime{pending: map[string]*model.Task{}, notify: make(chan struct{}, 1), tests: map[string]*model.EgressTestResult{}}
		s.rt[id] = r
	}
	return r
}

func (s *Server) nextTaskID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), atomic.AddUint64(&s.taskSeq, 1))
}

func (s *Server) enqueueSync(n *model.Node) {
	if n.Deleting {
		// The node is being purged; a stale sync must not replace the purge task.
		return
	}
	r := s.nodeRuntime(n.ID)
	r.mu.Lock()
	config := n.BuildDesired()
	encoded, _ := json.Marshal(config)
	digest := sha256.Sum256(encoded)
	taskID := fmt.Sprintf("sync-%x-%s", digest[:8], s.nextTaskID(""))
	// Full desired-state sync supersedes queued port-specific retries and any
	// older full sync. Keep only the newest desired state for this node.
	for id, task := range r.pending {
		if task.Kind == model.TaskSync {
			delete(r.pending, id)
		}
	}
	r.pending[taskID] = &model.Task{
		ID:        taskID,
		Kind:      model.TaskSync,
		Config:    config,
		CreatedAt: time.Now(),
	}
	if err := s.store.SetTasks(n.ID, r.pending); err != nil {
		log.Printf("保存节点 %s 同步任务失败: %v", n.ID, err)
	}
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (s *Server) enqueuePortSync(n *model.Node, portID string) error {
	if n.Deleting {
		return fmt.Errorf("节点正在删除，无法同步端口")
	}
	var target *model.Port
	for _, p := range n.Ports {
		if p.ID == portID {
			target = p
			break
		}
	}
	if target == nil {
		return fmt.Errorf("端口不存在")
	}
	if !target.Enabled {
		return fmt.Errorf("端口已停用，无法同步")
	}
	_, _ = s.store.Update(n.ID, func(current *model.Node) error {
		p := current.FindPort(portID)
		if p != nil {
			p.LastSyncAttempt = time.Now()
			p.InstanceError = "同步中"
		}
		return nil
	})
	cfg := &model.DesiredConfig{
		Enable:       true,
		Partial:      true,
		PortBindings: []model.PortBinding{{Port: target.Port, Protocol: target.Protocol}},
		Users:        []model.UserCred{{Name: target.Username, Password: target.Password}},
		PortIDs:      []string{target.ID},
		LoggingLevel: "INFO",
		Egress:       n.Egress,
	}
	taskID := fmt.Sprintf("sync-port-%s-%s", target.ID, s.nextTaskID(""))
	r := s.nodeRuntime(n.ID)
	r.mu.Lock()
	// Keep retries for independent ports queued together. A pending full sync
	// already covers this port, so do not replace it with a partial task.
	for _, pending := range r.pending {
		if pending.Config != nil && !pending.Config.Partial {
			r.mu.Unlock()
			return nil
		}
	}
	// Replace only an older retry for this same port.
	for id, pending := range r.pending {
		if pending.Config != nil && pending.Config.Partial && len(pending.Config.PortIDs) == 1 && pending.Config.PortIDs[0] == target.ID {
			delete(r.pending, id)
		}
	}
	r.pending[taskID] = &model.Task{ID: taskID, Kind: model.TaskSync, Config: cfg, CreatedAt: time.Now()}
	if err := s.store.SetTasks(n.ID, r.pending); err != nil {
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
	return nil
}

// enqueuePurge supersedes every queued task with a remote teardown request.
// The node record stays in the store until the agent acknowledges success so
// that the agent keeps a valid API key while cleaning up.
func (s *Server) enqueuePurge(n *model.Node) {
	r := s.nodeRuntime(n.ID)
	r.mu.Lock()
	r.pending = map[string]*model.Task{}
	taskID := fmt.Sprintf("purge-%s-%d", n.ID, time.Now().UnixNano())
	r.pending[taskID] = &model.Task{ID: taskID, Kind: model.TaskPurge, CreatedAt: time.Now()}
	if err := s.store.SetTasks(n.ID, r.pending); err != nil {
		log.Printf("保存节点 %s 清理任务失败: %v", n.ID, err)
	}
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

// ensurePurge re-queues the purge task when it was lost (for example a failed
// SetTasks) while the node is still marked as deleting.
func (s *Server) ensurePurge(n *model.Node) {
	r := s.nodeRuntime(n.ID)
	r.mu.Lock()
	for _, task := range r.pending {
		if task.Kind == model.TaskPurge {
			r.mu.Unlock()
			return
		}
	}
	r.mu.Unlock()
	s.enqueuePurge(n)
}

func (s *Server) pollTasks(id string, wait time.Duration) []*model.Task {
	r := s.nodeRuntime(id)
	deadline := time.Now().Add(wait)
	for {
		r.mu.Lock()
		if len(r.pending) > 0 {
			tasks := make([]*model.Task, 0, len(r.pending))
			for _, t := range r.pending {
				tasks = append(tasks, t)
			}
			r.mu.Unlock()
			return tasks
		}
		r.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		select {
		case <-r.notify:
		case <-time.After(remaining):
			return nil
		}
	}
}

func (s *Server) clearTask(id, taskID string) {
	r := s.nodeRuntime(id)
	r.mu.Lock()
	previous := r.pending[taskID]
	delete(r.pending, taskID)
	if err := s.store.SetTasks(id, r.pending); err != nil {
		if previous != nil {
			r.pending[taskID] = previous
		}
		log.Printf("保存节点 %s 任务确认失败: %v", id, err)
	}
	r.mu.Unlock()
}

// --- helpers ----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}

// baseURL returns the externally reachable manager base URL.
func (s *Server) baseURL(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return strings.TrimRight(s.cfg.PublicURL, "/") + s.panelPath
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return scheme + "://" + r.Host + s.panelPath
}

// Serve starts the manager HTTP server.
func (s *Server) Serve() error {
	log.Printf("merit manager %s listening on %s", s.version, s.cfg.Addr)
	return http.ListenAndServe(s.cfg.Addr, s.Handler())
}

func (s *Server) webFile(name string) ([]byte, error) {
	return fs.ReadFile(webFS, "web/"+name)
}
