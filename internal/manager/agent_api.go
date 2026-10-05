package manager

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"merit/internal/model"
)

func agentKey(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	if a := r.Header.Get("Authorization"); len(a) > 7 && a[:7] == "Bearer " {
		return a[7:]
	}
	return r.URL.Query().Get("key")
}

func remoteIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) handleAgentReport(w http.ResponseWriter, r *http.Request) {
	key := agentKey(r)
	node := s.store.NodeByAPIKey(key)
	if node == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid api key"})
		return
	}
	var st model.AgentStatus
	if err := readJSON(r, &st); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	firstRegister := !node.Registered
	ip := st.PublicIP
	if ip == "" {
		ip = remoteIP(r)
	}
	n, err := s.store.Update(node.ID, func(n *model.Node) error {
		n.Registered = true
		n.LastSeen = time.Now()
		n.AgentVer = st.AgentVersion
		n.MitaVer = st.MitaVersion
		n.OS = st.OS
		n.Arch = st.Arch
		n.MitaRunning = st.MitaRunning
		n.LastError = st.Error
		n.SeenIP = ip
		if n.Address == "" {
			n.Address = ip
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if firstRegister {
		s.enqueueSync(n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAgentPoll(w http.ResponseWriter, r *http.Request) {
	node := s.store.NodeByAPIKey(agentKey(r))
	if node == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid api key"})
		return
	}
	wait := 25 * time.Second
	if v := r.URL.Query().Get("wait"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 && secs <= 55 {
			wait = time.Duration(secs) * time.Second
		}
	}

	_, _ = s.store.Update(node.ID, func(n *model.Node) error {
		n.LastSeen = time.Now()
		n.SeenIP = remoteIP(r)
		return nil
	})

	tasks := s.pollTasks(node.ID, wait)
	if tasks == nil {
		tasks = []*model.Task{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleAgentResult(w http.ResponseWriter, r *http.Request) {
	node := s.store.NodeByAPIKey(agentKey(r))
	if node == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid api key"})
		return
	}
	var res model.TaskResult
	if err := readJSON(r, &res); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	s.clearTask(node.ID, res.TaskID)
	_, _ = s.store.Update(node.ID, func(n *model.Node) error {
		n.LastSeen = time.Now()
		if !res.OK {
			n.LastError = res.Message
		} else if res.Status != nil {
			n.MitaRunning = res.Status.MitaRunning
			n.MitaVer = res.Status.MitaVersion
			n.LastError = ""
		} else {
			n.LastError = ""
		}
		return nil
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
