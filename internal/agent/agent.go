// Package agent implements the node side of merit. It installs mita, keeps the
// manager informed about its status and applies configuration tasks.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"merit/internal/mieru"
	"merit/internal/model"
)

// Version is the agent build version.
var Version = "0.1.0"

// Config configures the agent runtime.
type Config struct {
	Manager     string
	APIKey      string
	Interval    time.Duration
	MitaVersion string
	MitaMirror  string
}

// Agent talks to the manager and manages the local mita instance.
type Agent struct {
	cfg    Config
	client *http.Client
	poll   *http.Client
}

// New creates an agent.
func New(cfg Config) (*Agent, error) {
	if cfg.Manager == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("--manager 和 --key 为必填")
	}
	cfg.Manager = strings.TrimRight(cfg.Manager, "/")
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.MitaVersion == "" {
		cfg.MitaVersion = defaultMitaVersion
	}
	return &Agent{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		poll:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

const defaultMitaVersion = "3.38.0"

// Run starts the agent loop and blocks.
func (a *Agent) Run(ctx context.Context) error {
	fmt.Printf("merit-agent %s -> %s\n", Version, a.cfg.Manager)

	installed, err := a.ensureMita()
	if err != nil {
		fmt.Fprintf(os.Stderr, "安装 mita 失败: %v\n", err)
	}

	_ = a.report(a.status(installed, err))
	a.installLoop(ctx)
	a.reportLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tasks, err := a.pollTasks(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "轮询失败: %v\n", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.cfg.Interval):
			}
			_ = a.report(a.status(a.mitaInstalled(), nil))
			continue
		}
		for _, t := range tasks {
			a.apply(t)
		}
	}
}

func (a *Agent) installLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !a.mitaInstalled() {
					_, _ = a.ensureMita()
				}
			}
		}
	}()
}

func (a *Agent) mitaInstalled() bool {
	_, err := exec.LookPath("mita")
	return err == nil
}

func (a *Agent) reportLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = a.report(a.status(a.mitaInstalled(), nil))
			}
		}
	}()
}

// --- manager communication --------------------------------------------------

func (a *Agent) report(st model.AgentStatus) error {
	body, _ := json.Marshal(st)
	req, err := http.NewRequest(http.MethodPost, a.cfg.Manager+"/api/agent/report", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.cfg.APIKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (a *Agent) pollTasks(ctx context.Context) ([]*model.Task, error) {
	url := a.cfg.Manager + "/api/agent/poll?wait=25"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", a.cfg.APIKey)
	resp, err := a.poll.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("poll status %d", resp.StatusCode)
	}
	var tasks []*model.Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (a *Agent) reportResult(res model.TaskResult) {
	body, _ := json.Marshal(res)
	req, err := http.NewRequest(http.MethodPost, a.cfg.Manager+"/api/agent/result", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.cfg.APIKey)
	resp, err := a.client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// --- task execution ---------------------------------------------------------

func (a *Agent) apply(t *model.Task) {
	switch t.Kind {
	case model.TaskSync:
		msg, err := a.applyConfig(t.Config)
		res := model.TaskResult{TaskID: t.ID, OK: err == nil, Message: msg}
		if err != nil {
			res.Message = err.Error()
		}
		st := a.status(a.mitaInstalled(), err)
		res.Status = &st
		a.reportResult(res)
	default:
		a.reportResult(model.TaskResult{TaskID: t.ID, OK: false, Message: "unknown task"})
	}
}

func (a *Agent) applyConfig(cfg *model.DesiredConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("缺少配置")
	}
	if !a.mitaInstalled() {
		if _, err := a.ensureMita(); err != nil {
			return "", fmt.Errorf("mita 未安装且自动安装失败: %w", err)
		}
	}
	if err := a.applyPortInstances(cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("已应用 %d 个独立 mita 实例", len(cfg.PortBindings)), nil
}

// applyPortInstances runs one mita process per enabled port. This is required
// for per-port egress because mita's native egress rules do not match inbound
// ports. Each instance gets its own JSON config, systemd unit and UDS socket.
func (a *Agent) applyPortInstances(cfg *model.DesiredConfig) error {
	const unitPrefix = "merit-mita-"
	const configDir = "/etc/merit-mita"
	// Disable the package's legacy single-instance service so it cannot race
	// with the per-port services after a reboot.
	_, _ = a.run("systemctl", "disable", "--now", "mita.service")
	_, _ = a.run("mita", "stop")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return err
	}
	// mita runs as the dedicated mita user and must be able to read its
	// per-port JSON files. The agent itself normally runs as root.
	if out, err := a.run("chown", "-R", "mita:mita", configDir); err != nil {
		return fmt.Errorf("设置 mita 配置目录权限失败: %v %s", err, out)
	}

	desired := map[int]bool{}
	for i, binding := range cfg.PortBindings {
		if i >= len(cfg.Users) {
			return fmt.Errorf("端口 %d 缺少账号配置", binding.Port)
		}
		desired[binding.Port] = true
		portCfg := *cfg
		portCfg.Enable = true
		portCfg.PortBindings = []model.PortBinding{binding}
		portCfg.Users = []model.UserCred{cfg.Users[i]}
		portCfg.Egress = egressForPort(cfg.Egress, binding.Port)
		raw, err := mieru.ServerConfigJSON(&portCfg)
		if err != nil {
			return fmt.Errorf("生成端口 %d 配置失败: %w", binding.Port, err)
		}
		configPath := filepath.Join(configDir, fmt.Sprintf("%d.json", binding.Port))
		if err := os.WriteFile(configPath, raw, 0o640); err != nil {
			return err
		}
		if out, err := a.run("chown", "mita:mita", configPath); err != nil {
			return fmt.Errorf("设置端口 %d 配置权限失败: %v %s", binding.Port, err, out)
		}
		unit := instanceUnit(binding.Port, configPath)
		unitPath := filepath.Join("/etc/systemd/system", unit+".service")
		if err := os.WriteFile(unitPath, []byte(instanceService(unit, configPath)), 0o644); err != nil {
			return err
		}
	}

	if _, err := a.run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd 重载失败: %w", err)
	}
	for port := range desired {
		unit := instanceUnit(port, "")
		if out, err := a.run("systemctl", "enable", "--now", unit+".service"); err != nil {
			return fmt.Errorf("启动端口 %d 的 mita 实例失败: %v %s", port, err, out)
		}
	}
	return a.removeStaleInstances(desired)
}

func egressForPort(cfg *model.EgressConfig, port int) *model.EgressConfig {
	if cfg == nil {
		return nil
	}
	proxies := make(map[string]model.EgressProxy, len(cfg.Proxies))
	for _, p := range cfg.Proxies {
		proxies[p.Name] = p
	}
	rules := append([]model.EgressRule(nil), cfg.Rules...)
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Order < rules[j].Order })
	for _, r := range rules {
		if len(r.Ports) > 0 && !containsPort(r.Ports, port) {
			continue
		}
		out := &model.EgressConfig{Proxies: []model.EgressProxy{}, Rules: []model.EgressRule{{Action: r.Action, Domains: r.Domains, IPRanges: r.IPRanges, ProxyNames: r.ProxyNames, Enabled: true}}}
		for _, name := range r.ProxyNames {
			if p, ok := proxies[name]; ok && p.Enabled {
				out.Proxies = append(out.Proxies, p)
			}
		}
		return out
	}
	return nil
}

func containsPort(ports []int, port int) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

func instanceUnit(port int, _ string) string { return fmt.Sprintf("%s%d", "merit-mita-", port) }

func instanceService(unit, configPath string) string {
	if configPath == "" {
		return ""
	}
	socket := "/var/run/mita/" + unit + ".sock"
	return fmt.Sprintf(`[Unit]
Description=merit mita instance %s
After=network-online.target
Wants=network-online.target

[Service]
Type=exec
User=mita
Group=mita
Environment="MITA_CONFIG_JSON_FILE=%s"
Environment="MITA_UDS_PATH=%s"
ExecStartPre=+/usr/bin/mkdir -p /var/run/mita
ExecStartPre=+/usr/bin/chown mita:mita /var/run/mita
ExecStartPre=+/usr/bin/chown mita:mita %s
ExecStartPre=+/usr/bin/chmod 640 %s
ExecStart=/usr/bin/mita run
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, unit, configPath, socket, configPath, configPath)
}

func (a *Agent) removeStaleInstances(desired map[int]bool) error {
	entries, err := os.ReadDir("/etc/systemd/system")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "merit-mita-") || !strings.HasSuffix(name, ".service") {
			continue
		}
		var port int
		if _, err := fmt.Sscanf(strings.TrimSuffix(strings.TrimPrefix(name, "merit-mita-"), ".service"), "%d", &port); err != nil || desired[port] {
			continue
		}
		unit := strings.TrimSuffix(name, ".service")
		_, _ = a.run("systemctl", "disable", "--now", unit+".service")
		_ = os.Remove(filepath.Join("/etc/systemd/system", name))
		_ = os.Remove(filepath.Join("/etc/merit-mita", fmt.Sprintf("%d.json", port)))
	}
	_, _ = a.run("systemctl", "daemon-reload")
	return nil
}

func (a *Agent) status(installed bool, lastErr error) model.AgentStatus {
	st := model.AgentStatus{
		MitaInstalled: installed,
		OS:            runtimeOS(),
		Arch:          runtime.GOARCH,
		AgentVersion:  Version,
		PublicIP:      publicIP(),
	}
	if lastErr != nil {
		st.Error = lastErr.Error()
	}
	if installed {
		if v, err := a.run("mita", "version"); err == nil {
			st.MitaVersion = strings.TrimSpace(v)
		}
		if out, err := a.run("mita", "status"); err == nil && strings.Contains(out, "RUNNING") {
			st.MitaRunning = true
		}
	}
	return st
}

// --- process helpers --------------------------------------------------------

func (a *Agent) run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func runtimeOS() string {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
		}
	}
	return runtime.GOOS
}

func publicIP() string {
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://ip.sb",
		"https://api64.ipify.org",
	}
	client := &http.Client{Timeout: 6 * time.Second}
	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
		resp.Body.Close()
		ip := strings.TrimSpace(string(body))
		if ip != "" && !strings.Contains(ip, " ") {
			return ip
		}
	}
	return ""
}
