package manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"merit/internal/clash"
	"merit/internal/mieru"
	"merit/internal/model"
	"merit/internal/proxy"
	"merit/internal/updater"
	"merit/internal/version"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	page, err := s.webFile("index.html")
	if err != nil {
		http.Error(w, "panel not built", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(strings.ReplaceAll(string(page), "__PANEL_PATH__", s.panelPath)))
}

// --- auth -------------------------------------------------------------------

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": s.checkSession(r),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	if !s.store.VerifyAdmin(req.Username, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "用户名或密码错误"})
		return
	}
	s.setSession(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: s.panelPath, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) setSession(w http.ResponseWriter) {
	token := s.newSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     s.panelPath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// --- nodes ------------------------------------------------------------------

type nodeDTO struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	APIKey      string              `json:"apiKey"`
	Address     string              `json:"address"`
	Domain      string              `json:"domain"`
	Remark      string              `json:"remark"`
	Ports       []*model.Port       `json:"ports"`
	CreatedAt   time.Time           `json:"createdAt"`
	AgentVer    string              `json:"agentVer"`
	MitaVer     string              `json:"mitaVer"`
	OS          string              `json:"os"`
	Arch        string              `json:"arch"`
	MitaRunning bool                `json:"mitaRunning"`
	Registered  bool                `json:"registered"`
	Online      bool                `json:"online"`
	LastSeen    time.Time           `json:"lastSeen"`
	LastError   string              `json:"lastError"`
	SeenIP      string              `json:"seenIP"`
	Deleting    bool                `json:"deleting"`
	Egress      *model.EgressConfig `json:"egress,omitempty"`
	// Version compatibility fields
	VersionCompatible  bool   `json:"versionCompatible"`
	UpgradeRecommended bool   `json:"upgradeRecommended,omitempty"`
	CompatibilityError string `json:"compatibilityError,omitempty"`
}

func (s *Server) toDTO(n *model.Node) nodeDTO {
	online := n.Registered && time.Since(n.LastSeen) < 90*time.Second

	// Check version compatibility
	versionCompatible := true
	upgradeRecommended := false
	compatibilityError := ""

	if n.AgentVer != "" {
		agentVer := version.ExtractVersionFromBuildInfo(n.AgentVer)
		managerVer := version.ExtractVersionFromBuildInfo(s.version)

		if err := version.CheckAgentCompatibility(agentVer, managerVer); err != nil {
			versionCompatible = false
			compatibilityError = err.Error()
		}

		if version.IsUpgradeRecommended(agentVer, managerVer) {
			upgradeRecommended = true
		}
	}

	return nodeDTO{
		ID:                 n.ID,
		Name:               n.Name,
		APIKey:             n.APIKey,
		Address:            n.Address,
		Domain:             n.Domain,
		Remark:             n.Remark,
		Ports:              n.Ports,
		CreatedAt:          n.CreatedAt,
		AgentVer:           n.AgentVer,
		MitaVer:            n.MitaVer,
		OS:                 n.OS,
		Arch:               n.Arch,
		MitaRunning:        n.MitaRunning,
		Registered:         n.Registered,
		Online:             online,
		LastSeen:           n.LastSeen,
		LastError:          n.LastError,
		SeenIP:             n.SeenIP,
		Deleting:           n.Deleting,
		Egress:             egressDTO(n.Egress),
		VersionCompatible:  versionCompatible,
		UpgradeRecommended: upgradeRecommended,
		CompatibilityError: compatibilityError,
	}
}

func (s *Server) handleGetEgress(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	if n.Egress == nil {
		writeJSON(w, http.StatusOK, &model.EgressConfig{})
		return
	}
	writeJSON(w, http.StatusOK, egressDTO(n.Egress))
}

func egressDTO(cfg *model.EgressConfig) *model.EgressConfig {
	if cfg == nil {
		return &model.EgressConfig{Proxies: []model.EgressProxy{}, Rules: []model.EgressRule{}}
	}
	copyCfg := &model.EgressConfig{Rules: append([]model.EgressRule(nil), cfg.Rules...), Proxies: make([]model.EgressProxy, len(cfg.Proxies))}
	copy(copyCfg.Proxies, cfg.Proxies)
	for i := range copyCfg.Proxies {
		if copyCfg.Proxies[i].Password != "" {
			copyCfg.Proxies[i].Password = "••••••••"
		}
	}
	return copyCfg
}

func (s *Server) handlePutEgress(w http.ResponseWriter, r *http.Request) {
	var cfg model.EgressConfig
	if err := readJSON(r, &cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	if err := validateEgress(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		for _, rule := range cfg.Rules {
			for _, portNumber := range rule.Ports {
				for _, port := range n.Ports {
					if rule.Enabled && port.Port == portNumber && model.NormalizeProxyType(port.ProxyType) != model.ProxyTypeMieru {
						return &portValidationError{fmt.Errorf("出站规则仅支持 Mieru 端口，%d 使用 %s", portNumber, port.ProxyType)}
					}
				}
			}
		}
		if len(cfg.Proxies) == 0 && len(cfg.Rules) == 0 {
			n.Egress = nil
		} else {
			oldPasswords := make(map[string]string)
			if n.Egress != nil {
				for _, old := range n.Egress.Proxies {
					oldPasswords[old.ID] = old.Password
				}
			}
			for i := range cfg.Proxies {
				if cfg.Proxies[i].Password == "••••••••" || cfg.Proxies[i].Password == "" {
					if password := oldPasswords[cfg.Proxies[i].ID]; password != "" {
						cfg.Proxies[i].Password = password
					}
				}
			}
			n.Egress = &cfg
		}
		return nil
	})
	if err != nil {
		var invalid *portValidationError
		if errors.As(err, &invalid) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueueSync(n)
	writeJSON(w, http.StatusOK, s.toDTO(n))
}

func (s *Server) handleTestEgress(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil || n.Egress == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "出站不存在"})
		return
	}
	var proxy *model.EgressProxy
	for i := range n.Egress.Proxies {
		if n.Egress.Proxies[i].ID == r.PathValue("proxyId") {
			proxy = &n.Egress.Proxies[i]
			break
		}
	}
	if proxy == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "出站不存在"})
		return
	}
	if !n.Registered || time.Since(n.LastSeen) > 90*time.Second {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "节点离线，请等待 Agent 上线后再测试"})
		return
	}
	rt := s.nodeRuntime(n.ID)
	rt.mu.Lock()
	if previous := rt.tests[proxy.ID]; previous != nil && previous.State == "pending" && time.Since(previous.CreatedAt) < time.Minute {
		copyResult := *previous
		rt.mu.Unlock()
		writeJSON(w, http.StatusAccepted, copyResult)
		return
	}
	for id, task := range rt.pending {
		if task.Kind == model.TaskTestEgress && task.TestProxy != nil && task.TestProxy.ID == proxy.ID {
			delete(rt.pending, id)
		}
	}
	taskID := "test-egress-" + mieru.RandomID()
	result := &model.EgressTestResult{TaskID: taskID, ProxyID: proxy.ID, State: "pending", Source: "agent", CreatedAt: time.Now(), Message: "等待节点执行 SOCKS5 测试"}
	proxyCopy := *proxy
	rt.tests[proxy.ID] = result
	rt.pending[taskID] = &model.Task{ID: taskID, Kind: model.TaskTestEgress, TestProxy: &proxyCopy, CreatedAt: time.Now()}
	copyResult := *result
	rt.mu.Unlock()
	select {
	case rt.notify <- struct{}{}:
	default:
	}
	writeJSON(w, http.StatusAccepted, copyResult)
}

func (s *Server) handleGetEgressTest(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	rt := s.nodeRuntime(n.ID)
	rt.mu.Lock()
	result := rt.tests[r.PathValue("proxyId")]
	if result == nil {
		rt.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "尚未测试或 Manager 已重启"})
		return
	}
	if result.State == "pending" && time.Since(result.CreatedAt) >= time.Minute {
		result.State, result.Message = "timeout", "节点未在一分钟内返回结果，请检查 Agent 版本和日志"
		delete(rt.pending, result.TaskID)
	}
	copyResult := *result
	rt.mu.Unlock()
	writeJSON(w, http.StatusOK, copyResult)
}

func validateEgress(cfg *model.EgressConfig) error {
	if cfg.Proxies == nil {
		cfg.Proxies = []model.EgressProxy{}
	}
	if cfg.Rules == nil {
		cfg.Rules = []model.EgressRule{}
	}
	seen := map[string]bool{}
	for i := range cfg.Proxies {
		p := &cfg.Proxies[i]
		if strings.TrimSpace(p.ID) == "" {
			p.ID = mieru.RandomID()
		}
		p.Name = strings.TrimSpace(p.Name)
		p.Host = strings.TrimSpace(p.Host)
		p.Protocol = model.EgressProtocolSocks5
		if p.Name == "" || p.Host == "" || p.Port < 1 || p.Port > 65535 {
			return fmt.Errorf("出站代理名称、地址或端口无效")
		}
		if seen[p.Name] {
			return fmt.Errorf("出站代理名称重复: %s", p.Name)
		}
		seen[p.Name] = true
	}
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if strings.TrimSpace(rule.ID) == "" {
			rule.ID = mieru.RandomID()
		}
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			rule.Name = fmt.Sprintf("规则 %d", i+1)
		}
		switch rule.Action {
		case model.EgressDirect, model.EgressReject:
		case model.EgressProxyAction:
			if len(rule.ProxyNames) == 0 {
				return fmt.Errorf("规则 %s 使用 PROXY 时必须选择出站代理", rule.Name)
			}
			for _, name := range rule.ProxyNames {
				if !seen[name] {
					return fmt.Errorf("规则 %s 引用了不存在的出站代理: %s", rule.Name, name)
				}
			}
		default:
			return fmt.Errorf("规则 %s 的动作无效", rule.Name)
		}
		for _, port := range rule.Ports {
			if port < 1 || port > 65535 {
				return fmt.Errorf("规则 %s 的端口无效: %d", rule.Name, port)
			}
		}
		rule.Order = i
	}
	return nil
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.store.Nodes()
	out := make([]nodeDTO, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, s.toDTO(n))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Remark string `json:"remark"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "节点名称不能为空"})
		return
	}
	n := &model.Node{
		ID:        mieru.RandomID(),
		Name:      req.Name,
		APIKey:    mieru.RandomID() + mieru.RandomID(),
		Remark:    req.Remark,
		Ports:     []*model.Port{},
		CreatedAt: time.Now(),
	}
	if err := s.store.AddNode(n); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.toDTO(n))
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	writeJSON(w, http.StatusOK, s.toDTO(n))
}

func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    *string `json:"name"`
		Address *string `json:"address"`
		Domain  *string `json:"domain"`
		Remark  *string `json:"remark"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		if req.Name != nil {
			n.Name = strings.TrimSpace(*req.Name)
		}
		if req.Address != nil {
			n.Address = strings.TrimSpace(*req.Address)
		}
		if req.Domain != nil {
			n.Domain = strings.TrimSpace(*req.Domain)
		}
		if req.Remark != nil {
			n.Remark = *req.Remark
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.toDTO(n))
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n := s.store.Node(id)
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "node not found"})
		return
	}
	// force only removes the panel record. It is the fallback for a host that
	// will never come back; the remote cleanup cannot run without an agent.
	if r.URL.Query().Get("force") == "1" {
		s.deleteNodeRecord(w, id)
		return
	}
	// Nothing was deployed, so there is nothing to tear down remotely.
	if !n.Deleting && !n.Registered && n.AgentID == "" {
		s.deleteNodeRecord(w, id)
		return
	}
	if n.Deleting {
		// Retry idempotently; re-queue the purge if it was lost.
		s.ensurePurge(n)
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "deleting": true})
		return
	}
	updated, err := s.store.Update(id, func(n *model.Node) error {
		n.Deleting = true
		n.LastError = "正在清理远程 Agent、mita 实例与账号…"
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueuePurge(updated)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "deleting": true})
}

func (s *Server) deleteNodeRecord(w http.ResponseWriter, id string) {
	if err := s.store.Delete(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.mu.Lock()
	delete(s.rt, id)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAddPort(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port            int    `json:"port"`
		Protocol        string `json:"protocol"`
		ProxyType       string `json:"proxyType"`
		TLSFingerprint  string `json:"tlsFingerprint"`
		NaiveTransport  string `json:"naiveTransport"`
		Label           string `json:"label"`
		Domain          string `json:"domain"`
		AutoACME        bool   `json:"autoACME"`
		TLSCert         string `json:"tlsCert"`
		TLSKey          string `json:"tlsKey"`
		HandshakeAddr   string `json:"handshakeAddr"`
		BackendAddr     string `json:"backendAddr"`
		BackendPassword string `json:"backendPassword"`
		BackendCipher   string `json:"backendCipher"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	proto := strings.ToUpper(strings.TrimSpace(req.Protocol))
	if proto != model.ProtocolUDP {
		proto = model.ProtocolTCP
	}

	// 默认代理类型为 mieru
	proxyType := strings.TrimSpace(req.ProxyType)
	if proxyType == "" {
		proxyType = model.ProxyTypeMieru
	}

	// 验证代理类型
	validTypes := map[string]bool{
		model.ProxyTypeMieru:     true,
		model.ProxyTypeNaive:     true,
		model.ProxyTypeAnyTLS:    true,
		model.ProxyTypeShadowTLS: true,
	}
	if !validTypes[proxyType] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "不支持的代理类型"})
		return
	}
	if minPort := model.MinListenPort(proxyType); req.Port < minPort || req.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("端口需在 %d-65535 之间", minPort)})
		return
	}
	if proxyType != model.ProxyTypeMieru && proto != model.ProtocolTCP {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "该代理类型仅支持 TCP 监听"})
		return
	}

	var added *model.Port
	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		for _, p := range n.Ports {
			if p.Port == req.Port {
				return errConflict
			}
		}

		p := mieru.NewPortWithType(req.Port, proto, strings.TrimSpace(req.Label), proxyType)

		p.TLSFingerprint = strings.ToLower(strings.TrimSpace(req.TLSFingerprint))
		p.NaiveTransport = strings.ToLower(strings.TrimSpace(req.NaiveTransport))
		// 设置代理特定配置
		switch proxyType {
		case model.ProxyTypeAnyTLS:
			p.Domain = strings.TrimSpace(req.Domain)
		case model.ProxyTypeNaive:
			p.Domain = strings.TrimSpace(req.Domain)
			p.AutoACME = req.AutoACME
			if !req.AutoACME {
				p.TLSCert = strings.TrimSpace(req.TLSCert)
				p.TLSKey = strings.TrimSpace(req.TLSKey)
			}
			if req.AutoACME {
				p.TLSCert, p.TLSKey = "", ""
			}
		case model.ProxyTypeShadowTLS:
			p.HandshakeAddr = strings.TrimSpace(req.HandshakeAddr)
			if p.HandshakeAddr == "" {
				p.HandshakeAddr = "cloudflare.com:443"
			}
			p.BackendAddr = strings.TrimSpace(req.BackendAddr)
			p.BackendPassword = strings.TrimSpace(req.BackendPassword)
			p.BackendCipher = strings.TrimSpace(req.BackendCipher)
			// 后端地址留空时由 Manager 分配本机后端并生成凭据
			if err := finalizeShadowTLSBackend(n, p, false, "", true, p.BackendPassword != ""); err != nil {
				return &portValidationError{err}
			}
		}

		// 部署前先校验，避免把必然起不来的实例下发到节点
		if err := validateDeployablePort(p); err != nil {
			return &portValidationError{err}
		}

		n.Ports = append(n.Ports, p)
		if err := model.ValidatePortReservations(n); err != nil {
			return &portValidationError{err}
		}
		added = p
		return nil
	})
	if err != nil {
		if err == errConflict {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "该端口已存在"})
			return
		}
		var invalid *portValidationError
		if errors.As(err, &invalid) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": invalid.Error()})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueueSync(n)
	writeJSON(w, http.StatusOK, added)
}

var errConflict = &simpleError{"conflict"}

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

// portValidationError marks a port configuration that cannot be deployed, so the
// handler can answer 400 instead of 404.
type portValidationError struct{ err error }

func (e *portValidationError) Error() string { return e.err.Error() }
func (e *portValidationError) Unwrap() error { return e.err }

func (s *Server) handleUpdatePort(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port           *int    `json:"port"`
		Protocol       *string `json:"protocol"`
		Username       *string `json:"username"`
		Password       *string `json:"password"`
		Enabled        *bool   `json:"enabled"`
		Label          *string `json:"label"`
		Regenerate     bool    `json:"regenerate"`
		TrafficLimit   *int64  `json:"trafficLimit"`
		BandwidthLimit *int64  `json:"bandwidthLimit"`
		ResetTraffic   bool    `json:"resetTraffic"`
		// 新增字段用于更新代理配置
		TLSFingerprint  *string `json:"tlsFingerprint"`
		NaiveTransport  *string `json:"naiveTransport"`
		Domain          *string `json:"domain"`
		TLSCert         *string `json:"tlsCert"`
		TLSKey          *string `json:"tlsKey"`
		AutoACME        *bool   `json:"autoACME"`
		HandshakeAddr   *string `json:"handshakeAddr"`
		BackendAddr     *string `json:"backendAddr"`
		BackendPassword *string `json:"backendPassword"`
		BackendCipher   *string `json:"backendCipher"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		p := n.FindPort(r.PathValue("pid"))
		if p == nil {
			return errNodeNotFound
		}
		wasManaged, prevAddr := p.BackendManaged, p.BackendAddr
		if req.Port != nil {
			if *req.Port < model.MinListenPort(p.ProxyType) || *req.Port > 65535 {
				return &portValidationError{fmt.Errorf("监听端口超出范围")}
			}
			for _, other := range n.Ports {
				if other.ID != p.ID && other.Port == *req.Port {
					return &portValidationError{fmt.Errorf("监听端口已被其他入口占用")}
				}
			}
			p.Port = *req.Port
		}
		if req.Protocol != nil {
			proto := strings.ToUpper(strings.TrimSpace(*req.Protocol))
			if proto != model.ProtocolTCP && proto != model.ProtocolUDP {
				return &portValidationError{fmt.Errorf("协议必须是 TCP 或 UDP")}
			}
			if model.NormalizeProxyType(p.ProxyType) != model.ProxyTypeMieru && proto != model.ProtocolTCP {
				return &portValidationError{fmt.Errorf("该代理类型仅支持 TCP")}
			}
			p.Protocol = proto
		}
		if req.Username != nil {
			if strings.TrimSpace(*req.Username) == "" {
				return &portValidationError{fmt.Errorf("用户名不能为空")}
			}
			p.Username = strings.TrimSpace(*req.Username)
		}
		if req.Password != nil {
			if *req.Password == "" {
				return &portValidationError{fmt.Errorf("密码不能为空")}
			}
			p.Password = *req.Password
		}
		if model.NormalizeProxyType(p.ProxyType) != model.ProxyTypeMieru && req.TrafficLimit != nil && *req.TrafficLimit > 0 {
			return &portValidationError{fmt.Errorf("当前仅 Mieru 支持流量统计和月度限额，%s 尚不支持", p.ProxyType)}
		}
		if req.Enabled != nil {
			p.Enabled = *req.Enabled
		}
		if req.Label != nil {
			p.Label = strings.TrimSpace(*req.Label)
		}
		if req.Regenerate {
			p.Username = mieru.GenerateUsername()
			p.Password = mieru.GeneratePassword()
		}
		if req.TrafficLimit != nil {
			p.TrafficLimit = maxInt64(0, *req.TrafficLimit)
		}
		if req.BandwidthLimit != nil {
			p.BandwidthLimit = *req.BandwidthLimit
		}
		if req.ResetTraffic {
			p.TrafficUsed = 0
			p.TrafficRaw = 0
			p.TrafficResetAt = time.Now()
			if p.AutoDisabled {
				p.AutoDisabled = false
				p.Enabled = true
			}
		}
		if p.AutoDisabled && (p.TrafficLimit == 0 || p.TrafficUsed < p.TrafficLimit) {
			p.AutoDisabled = false
			p.Enabled = true
		}
		if model.NormalizeProxyType(p.ProxyType) == model.ProxyTypeMieru && p.TrafficLimit > 0 && p.TrafficUsed >= p.TrafficLimit && p.Enabled {
			p.Enabled = false
			p.AutoDisabled = true
		}

		if req.TLSFingerprint != nil {
			p.TLSFingerprint = strings.ToLower(strings.TrimSpace(*req.TLSFingerprint))
		}
		if req.NaiveTransport != nil {
			p.NaiveTransport = strings.ToLower(strings.TrimSpace(*req.NaiveTransport))
		}
		// 更新代理特定配置
		if req.Domain != nil {
			p.Domain = strings.TrimSpace(*req.Domain)
		}
		if req.TLSCert != nil {
			p.TLSCert = strings.TrimSpace(*req.TLSCert)
		}
		if req.TLSKey != nil {
			p.TLSKey = strings.TrimSpace(*req.TLSKey)
		}
		if req.AutoACME != nil {
			p.AutoACME = *req.AutoACME
		}
		if req.HandshakeAddr != nil {
			p.HandshakeAddr = strings.TrimSpace(*req.HandshakeAddr)
		}
		if req.BackendAddr != nil {
			p.BackendAddr = strings.TrimSpace(*req.BackendAddr)
		}
		if req.BackendPassword != nil {
			p.BackendPassword = strings.TrimSpace(*req.BackendPassword)
		}
		if req.BackendCipher != nil {
			p.BackendCipher = strings.TrimSpace(*req.BackendCipher)
		}
		if p.AutoACME {
			// 自动证书模式下不保留手动证书，避免节点上的旧文件继续被引用
			p.TLSCert, p.TLSKey = "", ""
		}
		credsGiven := req.BackendPassword != nil && strings.TrimSpace(*req.BackendPassword) != ""
		if err := finalizeShadowTLSBackend(n, p, wasManaged, prevAddr, req.BackendAddr != nil, credsGiven); err != nil {
			return &portValidationError{err}
		}
		if err := model.ValidatePortReservations(n); err != nil {
			return &portValidationError{err}
		}
		if err := validateDeployablePort(p); err != nil {
			return &portValidationError{err}
		}

		return nil
	})
	if err != nil {
		var invalid *portValidationError
		if errors.As(err, &invalid) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": invalid.Error()})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueueSync(n)
	writeJSON(w, http.StatusOK, s.toDTO(n))
}

// handleUploadCert 处理TLS证书上传（用于NaïveProxy）
func (s *Server) handleUploadCert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cert     string `json:"cert"`
		Key      string `json:"key"`
		AutoACME bool   `json:"autoACME"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}

	req.Cert = strings.TrimSpace(req.Cert)
	req.Key = strings.TrimSpace(req.Key)

	// 如果启用ACME，清空手动证书
	if req.AutoACME {
		req.Cert = ""
		req.Key = ""
	} else {
		// 手动上传证书，需要验证
		if req.Cert == "" || req.Key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "证书和密钥不能为空"})
			return
		}

		// 验证证书格式
		if !strings.Contains(req.Cert, "BEGIN CERTIFICATE") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "证书格式无效，必须包含 BEGIN CERTIFICATE"})
			return
		}
		if !strings.Contains(req.Key, "BEGIN") || !strings.Contains(req.Key, "PRIVATE KEY") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "私钥格式无效，必须包含 PRIVATE KEY"})
			return
		}

		// 尝试解析证书以验证有效性
		if err := validateCertAndKey(req.Cert, req.Key); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("证书验证失败: %v", err)})
			return
		}
	}

	var certInfo *CertInfo
	if req.Cert != "" {
		certInfo = extractCertInfo(req.Cert)
	}

	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		p := n.FindPort(r.PathValue("pid"))
		if p == nil {
			return errNodeNotFound
		}
		if p.ProxyType != model.ProxyTypeNaive {
			return fmt.Errorf("只有 NaïveProxy 类型的端口才能配置证书")
		}

		p.TLSCert = req.Cert
		p.TLSKey = req.Key
		p.AutoACME = req.AutoACME
		if err := validateDeployablePort(p); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	s.enqueueSync(n)

	response := map[string]any{
		"ok":       true,
		"message":  "证书配置已更新",
		"autoACME": req.AutoACME,
	}
	if certInfo != nil {
		response["certInfo"] = certInfo
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetProxyTypes 返回支持的代理类型列表
func (s *Server) handleGetProxyTypes(w http.ResponseWriter, r *http.Request) {
	types := []map[string]any{
		{
			"type":        model.ProxyTypeMieru,
			"name":        "Mieru (默认)",
			"description": "mieru 自定义协议，支持TCP/UDP",
			"icon":        "🚀",
		},
		{
			"type":        model.ProxyTypeNaive,
			"name":        "NaïveProxy",
			"description": "基于Chrome网络栈的TLS代理，需要域名和证书",
			"icon":        "🌐",
		},
		{
			"type":        model.ProxyTypeAnyTLS,
			"name":        "AnyTLS（自签名证书）",
			"description": "anytls-go 使用临时自签名证书，导出配置跳过证书校验",
			"icon":        "🔐",
		},
		{
			"type":        model.ProxyTypeShadowTLS,
			"name":        "ShadowTLS v3",
			"description": "TCP 隧道，需要握手服务器和独立部署的后端数据服务",
			"icon":        "🔒",
		},
	}
	writeJSON(w, http.StatusOK, types)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

var errNodeNotFound = &simpleError{"端口不存在"}

func (s *Server) handleDeletePort(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("pid")
	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		out := n.Ports[:0]
		found := false
		for _, p := range n.Ports {
			if p.ID == pid {
				found = true
				continue
			}
			out = append(out, p)
		}
		if !found {
			return errNodeNotFound
		}
		n.Ports = out
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueueSync(n)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRetryPortSync(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	if err := s.enqueuePortSync(n, r.PathValue("pid")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "message": "已加入同步队列"})
}

// --- links / install --------------------------------------------------------

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	base := s.baseURL(r)
	installURL := base + "/install.sh?key=" + n.APIKey
	cmd := "curl -fsSL '" + installURL + "' | sudo bash -s -- --key " + n.APIKey
	writeJSON(w, http.StatusOK, map[string]any{
		"command":      cmd,
		"installPage":  installURL,
		"apiKey":       n.APIKey,
		"downloadHost": base,
	})
}

func (s *Server) handleLinks(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	type link struct {
		ID        string `json:"id"`
		Label     string `json:"label"`
		Port      int    `json:"port"`
		Protocol  string `json:"protocol"`
		ProxyType string `json:"proxyType"`
		Link      string `json:"link"`
	}
	out := []link{}
	for _, p := range n.Ports {
		if !p.Enabled {
			continue
		}

		proxyType := p.ProxyType
		if proxyType == "" {
			proxyType = model.ProxyTypeMieru
		}

		var shareLink string
		if proxyType == model.ProxyTypeMieru {
			shareLink = mieru.SimpleLink(n, p)
		} else {
			// 使用proxy包生成其他类型的分享链接
			shareLink = s.generateProxyLink(n, p)
		}

		// 只有在能够生成有效链接时才添加到列表
		// 每个协议自行判断地址是否可用
		if shareLink != "" {
			out = append(out, link{
				ID:        p.ID,
				Label:     p.Label,
				Port:      p.Port,
				Protocol:  p.Protocol,
				ProxyType: proxyType,
				Link:      shareLink,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// generateProxyLink 为不同代理类型生成分享链接。非 mieru 协议统一交给 proxy 包，
// 避免 Manager 与 Agent 两侧各写一份拼接逻辑（转义不一致会导致链接不可用）。
func (s *Server) generateProxyLink(n *model.Node, p *model.Port) string {
	if model.NormalizeProxyType(p.ProxyType) == model.ProxyTypeMieru {
		return mieru.SimpleLink(n, p)
	}
	return proxy.GenerateShareLink(n, p)
}

func (s *Server) handleNodeClash(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	yaml := clash.GenerateYAMLWithNotes(clash.BuildEntries(n), clash.UnsupportedPorts(n))
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	_, _ = w.Write([]byte(yaml))
}

// --- settings / subscription ------------------------------------------------

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	token := s.store.SubToken()
	writeJSON(w, http.StatusOK, map[string]any{
		"subToken":        token,
		"subscriptionURL": s.baseURL(r) + "/sub?token=" + token,
		"version":         s.version,
	})
}

func (s *Server) handleRotateSubToken(w http.ResponseWriter, r *http.Request) {
	token, err := s.store.RotateSubToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subToken": token})
}

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("token") != s.store.SubToken() {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}
	var entries []clash.Entry
	var notes []string
	for _, n := range s.store.Nodes() {
		entries = append(entries, clash.BuildEntries(n)...)
		notes = append(notes, clash.UnsupportedPorts(n)...)
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Subscription-Userinfo", "upload=0; download=0; total=0; expire=0")
	_, _ = w.Write([]byte(clash.GenerateYAMLWithNotes(entries, notes)))
}

func (s *Server) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	node := s.store.NodeByAPIKey(key)
	if node == nil {
		http.Error(w, "invalid key", http.StatusForbidden)
		return
	}
	base := s.baseURL(r)
	script := installScript(base, key)
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	_, _ = w.Write([]byte(script))
}

func installScript(base, key string) string {
	return `#!/bin/sh
set -eu

	MANAGER="` + base + `"
	KEY="` + key + `"
	RELEASE_BASE="https://github.com/HlONGlin/MieruPilot/releases/latest/download"

if [ "$(id -u)" -ne 0 ]; then
	echo "请以 root 运行：curl -fsSL ` + base + `/install.sh | sudo bash -s -- --key ` + key + `" >&2
	exit 1
fi

case "$(uname -m)" in
	x86_64|amd64) ARCH=amd64 ;;
	aarch64|arm64) ARCH=arm64 ;;
	*) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
esac

echo "==> 下载最新 merit-agent ($ARCH)"
agent_tmp="$(mktemp /tmp/merit-agent.XXXXXX)"
if ! curl -fsSL "$RELEASE_BASE/merit-agent-linux-$ARCH" -o "$agent_tmp"; then
  echo "下载最新 merit-agent 失败，请检查 GitHub Release 或网络连接。" >&2
  rm -f "$agent_tmp"
  exit 1
fi
mv -f "$agent_tmp" /usr/local/bin/merit-agent
chmod +x /usr/local/bin/merit-agent

echo "==> 写入 systemd 服务"
cat > /etc/systemd/system/merit-agent.service <<UNIT
[Unit]
Description=merit agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/merit-agent --manager $MANAGER --key $KEY
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable merit-agent
systemctl restart merit-agent
systemctl is-active --quiet merit-agent || { echo "Agent 启动失败，请查看 journalctl -u merit-agent" >&2; exit 1; }
/usr/local/bin/merit-agent --version
echo "==> merit-agent 已更新并重新启动，请返回面板查看节点状态。"
`
}

func (s *Server) handleAgentBinary(w http.ResponseWriter, r *http.Request) {
	osName := r.URL.Query().Get("os")
	if osName == "" {
		osName = "linux"
	}
	arch := r.URL.Query().Get("arch")
	if arch == "" {
		arch = "amd64"
	}
	if !validToken(osName) || !validToken(arch) {
		http.Error(w, "invalid arch", http.StatusBadRequest)
		return
	}
	name := "merit-agent-" + osName + "-" + arch
	path := s.cfg.AgentDir + "/" + name
	b, err := os.ReadFile(path)
	if err != nil {
		// The one-click installer stores the selected binary as merit-agent.
		// Keep accepting that name while also supporting the build artifact name.
		path = s.cfg.AgentDir + "/merit-agent"
		b, err = os.ReadFile(path)
		if err != nil {
			http.Error(w, "agent binary not found. 请将 "+name+" 或 merit-agent 放入 "+s.cfg.AgentDir, http.StatusNotFound)
			return
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	_, _ = w.Write(b)
}

func validToken(s string) bool {
	if s == "" || len(s) > 16 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// handleCheckUpdate checks for available updates.
func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	release, err := updater.CheckUpdate(s.version)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "检查更新失败：" + updateErrorHint(err)})
		return
	}
	if release == nil {
		writeJSON(w, http.StatusOK, map[string]any{"upToDate": true})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upToDate":    false,
		"version":     release.TagName,
		"name":        release.Name,
		"body":        release.Body,
		"publishedAt": release.PublishedAt,
	})
}

// handleApplyUpdate downloads and verifies the latest Manager release, then
// schedules a restart after the HTTP response has been delivered.
func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	release, err := updater.CheckUpdate(s.version)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "检查更新失败：" + updateErrorHint(err)})
		return
	}
	if release == nil {
		writeJSON(w, http.StatusOK, map[string]any{"updated": false, "message": "当前已经是最新版本"})
		return
	}
	executable, err := updater.CurrentExecutable()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法定位当前 Manager 程序: " + err.Error()})
		return
	}
	if err := updater.DownloadAndVerify(release, "manager", executable); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "下载或校验更新失败：" + err.Error()})
		return
	}
	if err := updater.ScheduleRestart(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"updated": true, "version": release.TagName, "message": "更新完成，Manager 将自动重启"})
}

// updateErrorHint keeps network failures understandable without leaking an
// upstream HTML error page into the settings panel.
func updateErrorHint(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "Client.Timeout") {
		return "连接 GitHub 超时，请检查服务器网络或代理后重试"
	}
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "i/o timeout") {
		return "无法连接 GitHub，请检查服务器 DNS、网络或代理设置"
	}
	return msg
}

// handleGetCertInfo 获取端口的证书信息
func (s *Server) handleGetCertInfo(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}

	port := n.FindPort(r.PathValue("pid"))
	if port == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "端口不存在"})
		return
	}

	if port.ProxyType != model.ProxyTypeNaive {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "只有 NaïveProxy 端口支持证书"})
		return
	}

	response := s.buildCertStatus(n, port)
	writeJSON(w, http.StatusOK, response)
}

// buildCertStatus constructs the full certificate lifecycle status.
func (s *Server) buildCertStatus(n *model.Node, port *model.Port) CertStatusResponse {
	resp := CertStatusResponse{
		AutoACME:     port.AutoACME,
		ACMEStaging:  port.ACMEStaging,
		HasCert:      port.TLSCert != "",
		Domain:       port.Domain,
		ExpiryLevel:  CertExpiryUnknown,
		ExpiryText:   "未知",
		ACMEDisabled: !port.AutoACME,
	}

	if port.TLSCert != "" {
		certInfo := extractCertInfo(port.TLSCert)
		if certInfo != nil {
			resp.CertInfo = certInfo
			resp.ExpiryLevel, resp.ExpiryText = classifyCertExpiry(certInfo.DaysToExpire)
		} else {
			resp.ExpiryText = "证书解析失败"
		}
	} else if port.AutoACME {
		resp.ExpiryText = "等待 ACME 签发"
	} else {
		resp.ExpiryText = "未配置证书"
	}

	// ACME status from agent reports (if available)
	if port.RuntimeACMEStatus != nil {
		resp.ACMEStatus = port.RuntimeACMEStatus
	}

	return resp
}

// classifyCertExpiry returns the urgency level and display text for a certificate.
func classifyCertExpiry(daysToExpire int) (CertExpiryLevel, string) {
	switch {
	case daysToExpire < 0:
		return CertExpiryExpired, fmt.Sprintf("已过期（%d 天前）", -daysToExpire)
	case daysToExpire <= 7:
		return CertExpiryCritical, fmt.Sprintf("即将过期（%d 天后）", daysToExpire)
	case daysToExpire <= 30:
		return CertExpiryWarning, fmt.Sprintf("即将过期（%d 天后）", daysToExpire)
	default:
		return CertExpiryValid, fmt.Sprintf("有效（%d 天后过期）", daysToExpire)
	}
}

// handleTestNode 测试节点连通性
func (s *Server) handleTestNode(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result := s.TestNode(ctx, n)
	writeJSON(w, http.StatusOK, result)
}

// handleTestPort 测试特定端口
func (s *Server) handleTestPort(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}

	port := n.FindPort(r.PathValue("pid"))
	if port == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "端口不存在"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	result := s.TestPort(ctx, n, port)
	if port.Enabled && len(result.Errors) == 0 {
		result.ProxyTest = s.probeProxy(ctx, n, port)
		if result.ProxyTest != nil {
			result.ProtocolVerified = result.ProxyTest.Reachable
			if result.ProxyTest.Reachable {
				result.Status = "verified"
			} else {
				result.Status = "failed"
				result.Errors = append(result.Errors, result.ProxyTest.Error)
			}
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// handleGetNodeConfig 获取节点详细配置
func (s *Server) handleGetNodeConfig(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}

	config := s.GetNodeConfig(n)
	writeJSON(w, http.StatusOK, config)
}

// handleGetExpiringCertificates returns all certificates that need attention
func (s *Server) handleGetExpiringCertificates(w http.ResponseWriter, r *http.Request) {
	checks := s.GetExpiringCertificates()
	writeJSON(w, http.StatusOK, checks)
}

// --- Certificate helpers ----------------------------------------------------

// CertInfo 证书信息
type CertInfo struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	NotBefore    time.Time `json:"notBefore"`
	NotAfter     time.Time `json:"notAfter"`
	DaysToExpire int       `json:"daysToExpire"`
	DNSNames     []string  `json:"dnsNames,omitempty"`
}

// CertExpiryLevel classifies how urgent a certificate renewal is.
type CertExpiryLevel string

const (
	CertExpiryValid    CertExpiryLevel = "valid"    // > 30 days
	CertExpiryWarning  CertExpiryLevel = "warning"  // 7-30 days
	CertExpiryCritical CertExpiryLevel = "critical" // 0-7 days
	CertExpiryExpired  CertExpiryLevel = "expired"  // < 0 days
	CertExpiryUnknown  CertExpiryLevel = "unknown"  // No cert or parse error
)

// CertStatusResponse is the full certificate lifecycle status returned by the API.
type CertStatusResponse struct {
	AutoACME     bool              `json:"autoACME"`
	ACMEStaging  bool              `json:"acmeStaging,omitempty"`
	HasCert      bool              `json:"hasCert"`
	Domain       string            `json:"domain"`
	CertInfo     *CertInfo         `json:"certInfo,omitempty"`
	ExpiryLevel  CertExpiryLevel   `json:"expiryLevel"`
	ExpiryText   string            `json:"expiryText"`
	ACMEStatus   *model.ACMEStatus `json:"acmeStatus,omitempty"`
	ACMEDisabled bool              `json:"acmeDisabled,omitempty"` // True when AutoACME is off
}

// validateCertAndKey 验证证书和密钥的有效性
// All certificate entry points use the same deployment checks, including PATCH.
func validateDeployablePort(p *model.Port) error {
	if err := proxy.ValidatePort(p); err != nil {
		return err
	}
	if model.NormalizeProxyType(p.ProxyType) != model.ProxyTypeNaive || p.AutoACME {
		return nil
	}
	if err := validateCertAndKey(p.TLSCert, p.TLSKey); err != nil {
		return err
	}
	block, _ := pem.Decode([]byte(p.TLSCert))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if err := cert.VerifyHostname(p.Domain); err != nil {
		return fmt.Errorf("证书与域名不匹配: %w", err)
	}
	return nil
}

func validateCertAndKey(certPEM, keyPEM string) error {
	// 解析证书
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return fmt.Errorf("无法解析证书PEM格式")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("无法解析证书: %v", err)
	}

	// 检查证书有效期
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("证书尚未生效（生效时间: %s）", cert.NotBefore.Format("2006-01-02"))
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("证书已过期（过期时间: %s）", cert.NotAfter.Format("2006-01-02"))
	}

	// 解析私钥
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return fmt.Errorf("无法解析私钥PEM格式")
	}

	// 尝试解析不同类型的私钥（验证格式）
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		_, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "EC PRIVATE KEY":
		_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	default:
		_, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	}
	if err != nil {
		return fmt.Errorf("无法解析私钥: %v", err)
	}

	// 验证证书和私钥匹配
	_, err = tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return fmt.Errorf("证书和私钥不匹配: %v", err)
	}

	return nil
}

// extractCertInfo 提取证书信息
func extractCertInfo(certPEM string) *CertInfo {
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return nil
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil
	}

	info := &CertInfo{
		Subject:      cert.Subject.CommonName,
		Issuer:       cert.Issuer.CommonName,
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		DaysToExpire: int(time.Until(cert.NotAfter).Hours() / 24),
		DNSNames:     cert.DNSNames,
	}

	return info
}
