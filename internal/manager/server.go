// Package manager implements the merit control panel and agent API.
package manager

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"merit/internal/model"
	"merit/internal/store"
)

//go:embed web
var webFS embed.FS

// Version is the manager build version.
var Version = "0.1.0"

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

	mu       sync.Mutex
	sessions map[string]time.Time
	rt       map[string]*nodeRuntime
}

type nodeRuntime struct {
	mu      sync.Mutex
	pending map[string]*model.Task
	notify  chan struct{}
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
	s.routes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	p := s.panelPath
	s.mux.HandleFunc("GET "+p, s.handleIndex)
	s.mux.HandleFunc("GET "+p+"/", s.handleIndex)

	s.mux.HandleFunc("GET "+p+"/api/me", s.handleMe)
	s.mux.HandleFunc("POST "+p+"/api/setup", s.handleSetup)
	s.mux.HandleFunc("POST "+p+"/api/login", s.handleLogin)
	s.mux.HandleFunc("POST "+p+"/api/logout", s.handleLogout)

	s.mux.Handle("GET "+p+"/api/nodes", s.auth(s.handleListNodes))
	s.mux.Handle("POST "+p+"/api/nodes", s.auth(s.handleCreateNode))
	s.mux.Handle("GET "+p+"/api/nodes/{id}", s.auth(s.handleGetNode))
	s.mux.Handle("PATCH "+p+"/api/nodes/{id}", s.auth(s.handleUpdateNode))
	s.mux.Handle("DELETE "+p+"/api/nodes/{id}", s.auth(s.handleDeleteNode))
	s.mux.Handle("POST "+p+"/api/nodes/{id}/ports", s.auth(s.handleAddPort))
	s.mux.Handle("PATCH "+p+"/api/nodes/{id}/ports/{pid}", s.auth(s.handleUpdatePort))
	s.mux.Handle("DELETE "+p+"/api/nodes/{id}/ports/{pid}", s.auth(s.handleDeletePort))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/install", s.auth(s.handleInstall))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/links", s.auth(s.handleLinks))
	s.mux.Handle("GET "+p+"/api/nodes/{id}/clash.yaml", s.auth(s.handleNodeClash))

	s.mux.Handle("GET "+p+"/api/settings", s.auth(s.handleSettings))
	s.mux.Handle("POST "+p+"/api/settings/sub-token/rotate", s.auth(s.handleRotateSubToken))

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
		r = &nodeRuntime{pending: map[string]*model.Task{}, notify: make(chan struct{}, 1)}
		s.rt[id] = r
	}
	return r
}

func (s *Server) enqueueSync(n *model.Node) {
	r := s.nodeRuntime(n.ID)
	r.mu.Lock()
	r.pending[model.TaskSync] = &model.Task{
		ID:        model.TaskSync,
		Kind:      model.TaskSync,
		Config:    n.BuildDesired(),
		CreatedAt: time.Now(),
	}
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
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
	delete(r.pending, taskID)
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
	return http.ListenAndServe(s.cfg.Addr, s.mux)
}

func (s *Server) webFile(name string) ([]byte, error) {
	return fs.ReadFile(webFS, "web/"+name)
}
