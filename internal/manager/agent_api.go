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
	newAgentInstance := st.AgentID != "" && node.AgentID != st.AgentID
	ip := st.PublicIP
	if ip == "" {
		ip = remoteIP(r)
	}
	n, err := s.store.Update(node.ID, func(n *model.Node) error {
		n.Registered = true
		n.LastSeen = time.Now()
		n.AgentVer = st.AgentVersion
		if st.AgentID != "" {
			n.AgentID = st.AgentID
		}
		n.MitaVer = st.MitaVersion
		n.OS = st.OS
		n.Arch = st.Arch
		n.MitaRunning = st.MitaRunning
		n.LastError = st.Error
		for _, instance := range st.PortInstances {
			for _, p := range n.Ports {
				if p.Port != instance.Port {
					continue
				}
				p.InstanceRunning = instance.Running
				if instance.Error != "" {
					p.InstanceError = instance.Error
				} else if instance.Running && p.InstanceError == "" {
					p.InstanceError = ""
				}
			}
		}
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
	runtime := s.nodeRuntime(n.ID)
	runtime.mu.Lock()
	managerRestarted := !runtime.initialized
	runtime.initialized = true
	runtime.mu.Unlock()
	if firstRegister || managerRestarted || newAgentInstance {
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
	rt := s.nodeRuntime(node.ID)
	rt.mu.Lock()
	if rt.pending[res.TaskID] == nil {
		rt.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ignored": true})
		return
	}
	if task := rt.pending[res.TaskID]; task != nil && task.Kind == model.TaskTestEgress {
		proxyID := task.TestProxy.ID
		if result := rt.tests[proxyID]; result != nil && result.TaskID == res.TaskID {
			result.State, result.OK, result.CheckedAt = "completed", res.OK, time.Now()
			result.Message = res.Message
			if res.EgressTest != nil {
				result.ExitIP, result.LatencyMs = res.EgressTest.ExitIP, res.EgressTest.LatencyMs
			} else {
				result.OK, result.Message = false, "Agent 未返回测试数据，请更新 Agent"
			}
		}
		delete(rt.pending, res.TaskID)
		rt.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	defer rt.mu.Unlock()
	if !res.OK {
		// Keep failed sync work available so the agent can retry after fixing
		// transient issues such as unavailable mita or invalid permissions.
		select {
		case rt.notify <- struct{}{}:
		default:
		}
	}
	_, saveErr := s.store.Update(node.ID, func(n *model.Node) error {
		n.LastSeen = time.Now()
		if res.Status != nil {
			for _, instance := range res.Status.PortInstances {
				for _, p := range n.Ports {
					if p.Port == instance.Port {
						p.InstanceRunning = instance.Running
						if instance.Error != "" {
							p.InstanceError = instance.Error
						} else if instance.Running {
							p.InstanceError = ""
						}
					}
				}
			}
		}
		for _, result := range res.PortResults {
			p := n.FindPort(result.PortID)
			if p == nil {
				continue
			}
			p.LastSyncAttempt = result.CheckedAt
			if result.OK {
				p.InstanceRunning = result.Running
				p.InstanceError = ""
				p.InstanceSyncedAt = result.CheckedAt
			} else {
				p.InstanceRunning = false
				p.InstanceError = result.Error
			}
		}
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
	if saveErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "保存任务状态失败"})
		return
	}
	if res.OK {
		task := rt.pending[res.TaskID]
		delete(rt.pending, res.TaskID)
		if err := s.store.SetTasks(node.ID, rt.pending); err != nil {
			rt.pending[res.TaskID] = task
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "保存任务确认失败"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func hasRetryablePortFailure(res model.TaskResult) bool {
	for _, p := range res.PortResults {
		if !p.OK && p.Error != "端口已停用" {
			return true
		}
	}
	return false
}
