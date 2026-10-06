package manager

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"merit/internal/mieru"
	"merit/internal/model"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	page, err := s.webFile("index.html")
	if err != nil {
		http.Error(w, "panel not built", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
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
	Egress      *model.EgressConfig `json:"egress,omitempty"`
}

func toDTO(n *model.Node) nodeDTO {
	online := n.Registered && time.Since(n.LastSeen) < 90*time.Second
	return nodeDTO{
		ID:          n.ID,
		Name:        n.Name,
		APIKey:      n.APIKey,
		Address:     n.Address,
		Domain:      n.Domain,
		Remark:      n.Remark,
		Ports:       n.Ports,
		CreatedAt:   n.CreatedAt,
		AgentVer:    n.AgentVer,
		MitaVer:     n.MitaVer,
		OS:          n.OS,
		Arch:        n.Arch,
		MitaRunning: n.MitaRunning,
		Registered:  n.Registered,
		Online:      online,
		LastSeen:    n.LastSeen,
		LastError:   n.LastError,
		SeenIP:      n.SeenIP,
		Egress:      egressDTO(n.Egress),
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
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueueSync(n)
	writeJSON(w, http.StatusOK, toDTO(n))
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
	ip, latency, err := checkSOCKS5Egress(*proxy)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "latencyMs": latency, "exitIP": ip, "message": "SOCKS5 认证及代理出口测试成功"})
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
		out = append(out, toDTO(n))
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
	writeJSON(w, http.StatusOK, toDTO(n))
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	writeJSON(w, http.StatusOK, toDTO(n))
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
	writeJSON(w, http.StatusOK, toDTO(n))
}

func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Label    string `json:"label"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求无效"})
		return
	}
	if req.Port < 1025 || req.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "端口需在 1025-65535 之间"})
		return
	}
	proto := strings.ToUpper(strings.TrimSpace(req.Protocol))
	if proto != model.ProtocolUDP {
		proto = model.ProtocolTCP
	}
	var added *model.Port
	n, err := s.store.Update(r.PathValue("id"), func(n *model.Node) error {
		for _, p := range n.Ports {
			// Each per-port mita service owns a single numeric port and config
			// file. Reusing the number with another transport would collide.
			if p.Port == req.Port {
				return errConflict
			}
		}
		p := mieru.NewPort(req.Port, proto, strings.TrimSpace(req.Label))
		n.Ports = append(n.Ports, p)
		added = p
		return nil
	})
	if err != nil {
		if err == errConflict {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "该端口已存在"})
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

func (s *Server) handleUpdatePort(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled    *bool   `json:"enabled"`
		Label      *string `json:"label"`
		Regenerate bool    `json:"regenerate"`
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
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.enqueueSync(n)
	writeJSON(w, http.StatusOK, toDTO(n))
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
	_, _ = s.store.Update(n.ID, func(n *model.Node) error {
		p := n.FindPort(r.PathValue("pid"))
		if p != nil {
			p.LastSyncAttempt = time.Now()
			p.InstanceError = "正在同步"
		}
		return nil
	})
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
		ID       string `json:"id"`
		Label    string `json:"label"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Link     string `json:"link"`
	}
	out := []link{}
	host := mieru.ClientHost(n)
	for _, p := range n.Ports {
		if !p.Enabled || host == "" {
			continue
		}
		out = append(out, link{
			ID:       p.ID,
			Label:    p.Label,
			Port:     p.Port,
			Protocol: p.Protocol,
			Link:     mieru.SimpleLink(n, p),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleNodeClash(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "节点不存在"})
		return
	}
	yaml := mieru.ClashYAML(mieru.BuildClashEntries(n))
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
	var entries []mieru.ClashEntry
	for _, n := range s.store.Nodes() {
		entries = append(entries, mieru.BuildClashEntries(n)...)
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Subscription-Userinfo", "upload=0; download=0; total=0; expire=0")
	_, _ = w.Write([]byte(mieru.ClashYAML(entries)))
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

if [ "$(id -u)" -ne 0 ]; then
	echo "请以 root 运行：curl -fsSL ` + base + `/install.sh | sudo bash -s -- --key ` + key + `" >&2
	exit 1
fi

case "$(uname -m)" in
	x86_64|amd64) ARCH=amd64 ;;
	aarch64|arm64) ARCH=arm64 ;;
	*) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
esac

echo "==> 下载 merit-agent ($ARCH)"
agent_tmp="$(mktemp /tmp/merit-agent.XXXXXX)"
if ! curl -fsSL "$MANAGER/download/agent?os=linux&arch=$ARCH" -o "$agent_tmp"; then
  echo "下载 merit-agent 失败，请检查 Manager 的 agent 文件是否存在：$MANAGER/download/agent?os=linux&arch=$ARCH" >&2
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
systemctl enable --now merit-agent
echo "==> merit-agent 已启动，请返回面板查看节点状态。"
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
