// Package agent implements the node side of merit. It installs mita, keeps the
// manager informed about its status and applies configuration tasks.
package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"merit/internal/buildinfo"
	"merit/internal/mieru"
	"merit/internal/model"
	"merit/internal/netcheck"
)

// Version is the agent build version.
var Version = buildinfo.String()

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
	cfg        Config
	client     *http.Client
	poll       *http.Client
	instanceID string
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
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("生成 Agent 实例 ID 失败: %w", err)
	}
	return &Agent{
		cfg:        cfg,
		client:     &http.Client{Timeout: 30 * time.Second},
		poll:       &http.Client{Timeout: 60 * time.Second},
		instanceID: hex.EncodeToString(idBytes),
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
			if err := a.apply(t); err != nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(a.cfg.Interval):
				}
			}
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("状态上报失败: HTTP %d", resp.StatusCode)
	}
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

func (a *Agent) reportResult(res model.TaskResult) error {
	body, _ := json.Marshal(res)
	req, err := http.NewRequest(http.MethodPost, a.cfg.Manager+"/api/agent/result", bytes.NewReader(body))
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("任务结果上报失败: HTTP %d", resp.StatusCode)
	}
	return nil
}

// --- task execution ---------------------------------------------------------

func (a *Agent) apply(t *model.Task) error {
	switch t.Kind {
	case model.TaskTestEgress:
		result := &model.EgressTestResult{TaskID: t.ID, State: "completed", Source: "agent"}
		if t.TestProxy == nil {
			result.Message = "任务缺少落地机配置"
		} else {
			result.ProxyID = t.TestProxy.ID
			ip, latency, err := netcheck.CheckSOCKS5Egress(*t.TestProxy)
			result.OK, result.ExitIP, result.LatencyMs = err == nil, ip, latency
			result.Message = "SOCKS5 握手与代理出口检测成功"
			if err != nil {
				result.Message = err.Error()
			}
		}
		result.CheckedAt = time.Now()
		return a.reportResult(model.TaskResult{TaskID: t.ID, OK: result.OK, Message: result.Message, EgressTest: result})
	case model.TaskSync:
		msg, portResults, err := a.applyConfig(t.Config)
		if err != nil && len(portResults) == 0 && t.Config != nil {
			for i, binding := range t.Config.PortBindings {
				result := model.PortInstanceResult{Port: binding.Port, Error: err.Error(), CheckedAt: time.Now()}
				if i < len(t.Config.PortIDs) {
					result.PortID = t.Config.PortIDs[i]
				}
				portResults = append(portResults, result)
			}
		}
		res := model.TaskResult{TaskID: t.ID, OK: err == nil, Message: msg, PortResults: portResults}
		if err != nil {
			res.Message = err.Error()
		}
		st := a.status(a.mitaInstalled(), err)
		res.Status = &st
		if reportErr := a.reportResult(res); reportErr != nil {
			fmt.Fprintf(os.Stderr, "任务结果回报失败: %v\n", reportErr)
			return reportErr
		}
		return err
	default:
		a.reportResult(model.TaskResult{TaskID: t.ID, OK: false, Message: "unknown task"})
		return fmt.Errorf("unknown task kind: %s", t.Kind)
	}
}

func (a *Agent) applyConfig(cfg *model.DesiredConfig) (string, []model.PortInstanceResult, error) {
	if cfg == nil {
		return "", nil, fmt.Errorf("缺少配置")
	}
	if !a.mitaInstalled() {
		if _, err := a.ensureMita(); err != nil {
			return "", nil, fmt.Errorf("mita 未安装且自动安装失败: %w", err)
		}
	}
	results, err := a.applyPortInstances(cfg)
	if err != nil {
		return "", results, err
	}
	return fmt.Sprintf("已应用 %d 个独立 mita 实例", len(cfg.PortBindings)), results, nil
}

// applyPortInstances runs one mita process per enabled port. This is required
// for per-port egress because mita's native egress rules do not match inbound
// ports. Each instance gets its own JSON config, systemd unit and UDS socket.
func (a *Agent) applyPortInstances(cfg *model.DesiredConfig) ([]model.PortInstanceResult, error) {
	const configDir = "/etc/merit-mita"
	seenPorts := make(map[int]bool, len(cfg.PortBindings))
	for i, binding := range cfg.PortBindings {
		if seenPorts[binding.Port] {
			return nil, fmt.Errorf("端口 %d 被重复配置；每个 mita 实例端口号必须唯一", binding.Port)
		}
		seenPorts[binding.Port] = true
		if i >= len(cfg.Users) {
			return nil, fmt.Errorf("端口 %d 缺少账号配置", binding.Port)
		}
	}
	if !cfg.Partial {
		// Disable the package's legacy single-instance service so it cannot race
		// with the per-port services after a reboot. Do not touch it on a
		// single-port retry, which must not affect other instances.
		_, _ = a.run("systemctl", "disable", "--now", "mita.service")
		_, _ = a.run("mita", "stop")
	}
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return nil, err
	}
	// mita runs as the dedicated mita user and must be able to read its
	// per-port JSON files. The agent itself normally runs as root.
	if out, err := a.run("chown", "-R", "mita:mita", configDir); err != nil {
		return nil, fmt.Errorf("设置 mita 配置目录权限失败: %v %s", err, out)
	}

	desired := map[int]bool{}
	results := make([]model.PortInstanceResult, 0, len(cfg.PortBindings))
	changedPorts := map[int]bool{}
	unitChanged := false
	for i, binding := range cfg.PortBindings {
		result := model.PortInstanceResult{Port: binding.Port, OK: false, CheckedAt: time.Now()}
		if i < len(cfg.PortIDs) {
			result.PortID = cfg.PortIDs[i]
		}
		desired[binding.Port] = true
		portCfg := *cfg
		portCfg.Enable = true
		portCfg.PortBindings = []model.PortBinding{binding}
		portCfg.Users = []model.UserCred{cfg.Users[i]}
		portCfg.Egress = egressForPort(cfg.Egress, binding.Port)
		raw, err := mieru.ServerConfigJSON(&portCfg)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			return results, fmt.Errorf("生成端口 %d 配置失败: %w", binding.Port, err)
		}
		configPath := filepath.Join(configDir, fmt.Sprintf("%d.json", binding.Port))
		oldRaw, oldErr := os.ReadFile(configPath)
		changed := oldErr != nil || configDigest(oldRaw) != configDigest(raw)
		appliedDigest, appliedErr := os.ReadFile(configPath + ".applied")
		needsApply := appliedErr != nil || strings.TrimSpace(string(appliedDigest)) != configDigest(raw)
		// Preserve the last confirmed config, not a failed intermediate update.
		if changed && oldErr == nil && appliedErr == nil && strings.TrimSpace(string(appliedDigest)) == configDigest(oldRaw) {
			if err := writeConfigAtomic(configPath+".previous", oldRaw); err != nil {
				return results, fmt.Errorf("备份端口 %d 配置失败: %w", binding.Port, err)
			}
		}
		if changed {
			if err := writeConfigAtomic(configPath, raw); err != nil {
				result.Error = err.Error()
				results = append(results, result)
				return results, err
			}
		}
		if out, err := a.run("chown", "mita:mita", configPath); err != nil {
			result.Error = fmt.Sprintf("%v %s", err, out)
			results = append(results, result)
			return results, fmt.Errorf("设置端口 %d 配置权限失败: %v %s", binding.Port, err, out)
		}
		if out, err := a.run("chmod", "640", configPath); err != nil {
			result.Error = fmt.Sprintf("%v %s", err, out)
			results = append(results, result)
			return results, fmt.Errorf("设置端口 %d 配置模式失败: %v %s", binding.Port, err, out)
		}
		unit := instanceUnit(binding.Port, configPath)
		unitPath := filepath.Join("/etc/systemd/system", unit+".service")
		unitRaw := []byte(instanceService(unit, configPath))
		oldUnit, unitErr := os.ReadFile(unitPath)
		if unitErr != nil || !bytes.Equal(oldUnit, unitRaw) {
			changed = true
			unitChanged = true
		}
		if err := os.WriteFile(unitPath, unitRaw, 0o644); err != nil {
			result.Error = err.Error()
			results = append(results, result)
			return results, err
		}
		results = append(results, result)
		changedPorts[binding.Port] = changed || needsApply
	}
	// Stop per-port units that are no longer enabled. Disabled ports are
	// intentionally omitted from DesiredConfig, so their result is reported by
	// the disable operation rather than as a startup failure.

	if unitChanged {
		if out, err := a.run("systemctl", "daemon-reload"); err != nil {
			return results, fmt.Errorf("systemd 重载失败: %v %s", err, out)
		}
	}
	resultByPort := make(map[int]int, len(results))
	for i := range results {
		resultByPort[results[i].Port] = i
	}
	for port := range desired {
		unit := instanceUnit(port, "")
		if out, err := a.run("systemctl", "enable", unit+".service"); err != nil {
			if i, ok := resultByPort[port]; ok {
				results[i].Error = fmt.Sprintf("启用实例失败: %v %s", err, out)
			}
			return results, fmt.Errorf("启用端口 %d 的 mita 实例失败: %v %s", port, err, out)
		}
		active, _ := a.instanceRunning(port)
		if changedPorts[port] || !active {
			verb := "restart"
			if !active {
				verb = "start"
			}
			if out, err := a.run("systemctl", verb, unit+".service"); err != nil {
				a.restorePreviousConfig(port)
				if i, ok := resultByPort[port]; ok {
					results[i].Error = fmt.Sprintf("%s 实例失败: %v %s", verb, err, out)
				}
				return results, fmt.Errorf("%s 端口 %d 的 mita 实例失败: %v %s", verb, port, err, out)
			}
		}
		if i, ok := resultByPort[port]; ok {
			running, statusErr := a.instanceRunning(port)
			results[i].OK = statusErr == nil && running
			results[i].Running = running
			results[i].CheckedAt = time.Now()
			if statusErr != nil {
				results[i].Error = statusErr.Error()
			} else if !running {
				results[i].Error = "systemd 实例启动后未处于 active 状态"
			}
			if !results[i].OK {
				a.restorePreviousConfig(port)
				return results, fmt.Errorf("端口 %d 应用失败: %s", port, results[i].Error)
			}
			configPath := filepath.Join(configDir, fmt.Sprintf("%d.json", port))
			current, err := os.ReadFile(configPath)
			if err != nil {
				return results, err
			}
			if err := os.WriteFile(configPath+".applied", []byte(configDigest(current)), 0o600); err != nil {
				return results, fmt.Errorf("记录端口 %d 已应用配置失败: %w", port, err)
			}
		}
	}
	if !cfg.Partial {
		stopped, err := a.removeStaleInstances(desired)
		if err != nil {
			return results, err
		}
		for _, stoppedInstance := range stopped {
			for _, disabledPort := range cfg.DisabledPorts {
				if disabledPort.Port == stoppedInstance.Port {
					stoppedInstance.PortID = disabledPort.ID
					break
				}
			}
			results = append(results, stoppedInstance)
		}
		for _, disabledPort := range cfg.DisabledPorts {
			if _, found := findPortResult(results, disabledPort.Port); !found {
				results = append(results, model.PortInstanceResult{PortID: disabledPort.ID, Port: disabledPort.Port, OK: true, Running: false, CheckedAt: time.Now()})
			}
		}
	}
	return results, nil
}

func configDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// restorePreviousConfig attempts to keep the previous working service available.
// The task still fails and remains queued; rollback is not reported as applying
// the newly requested configuration.
func (a *Agent) restorePreviousConfig(port int) {
	path := filepath.Join("/etc/merit-mita", fmt.Sprintf("%d.json", port))
	previous, err := os.ReadFile(path + ".previous")
	if err != nil {
		return
	}
	if err := writeConfigAtomic(path, previous); err != nil {
		return
	}
	if _, err := a.run("chown", "mita:mita", path); err != nil {
		return
	}
	_, _ = a.run("systemctl", "restart", instanceUnit(port, "")+".service")
}

func writeConfigAtomic(path string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".merit-mita-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o640); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func findPortResult(results []model.PortInstanceResult, port int) (model.PortInstanceResult, bool) {
	for _, result := range results {
		if result.Port == port {
			return result, true
		}
	}
	return model.PortInstanceResult{}, false
}

func (a *Agent) instanceRunning(port int) (bool, error) {
	unit := instanceUnit(port, "") + ".service"
	out, err := a.run("systemctl", "is-active", unit)
	if err != nil {
		return false, fmt.Errorf("%s: %s", err, strings.TrimSpace(out))
	}
	return strings.TrimSpace(out) == "active", nil
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
	out := &model.EgressConfig{Proxies: []model.EgressProxy{}, Rules: []model.EgressRule{}}
	usedProxies := map[string]bool{}
	for _, r := range rules {
		if len(r.Ports) > 0 && !containsPort(r.Ports, port) {
			continue
		}
		if !r.Enabled {
			continue
		}
		// Empty conditions mean match all destinations, independently of the
		// selected inbound port instance.
		domains := append([]string(nil), r.Domains...)
		ips := append([]string(nil), r.IPRanges...)
		if len(domains) == 0 && len(ips) == 0 {
			domains = []string{"*"}
			ips = []string{"*"}
		}
		out.Rules = append(out.Rules, model.EgressRule{Action: r.Action, Domains: domains, IPRanges: ips, ProxyNames: append([]string(nil), r.ProxyNames...), Enabled: true, Order: len(out.Rules)})
		for _, name := range r.ProxyNames {
			if p, ok := proxies[name]; ok && p.Enabled && !usedProxies[name] {
				out.Proxies = append(out.Proxies, p)
				usedProxies[name] = true
			}
		}
	}
	if len(out.Rules) == 0 {
		return nil
	}
	return out
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

func (a *Agent) removeStaleInstances(desired map[int]bool) ([]model.PortInstanceResult, error) {
	entries, err := os.ReadDir("/etc/systemd/system")
	if err != nil {
		return nil, err
	}
	var stopped []model.PortInstanceResult
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
		out, stopErr := a.run("systemctl", "disable", "--now", unit+".service")
		if stopErr != nil {
			return stopped, fmt.Errorf("停止已禁用端口 %d 的 mita 实例失败: %v %s", port, stopErr, out)
		}
		stopped = append(stopped, model.PortInstanceResult{Port: port, Running: false, OK: true, CheckedAt: time.Now()})
		_ = os.Remove(filepath.Join("/etc/systemd/system", name))
		_ = os.Remove(filepath.Join("/etc/merit-mita", fmt.Sprintf("%d.json", port)))
		_ = os.Remove(filepath.Join("/etc/merit-mita", fmt.Sprintf("%d.json.applied", port)))
		_ = os.Remove(filepath.Join("/etc/merit-mita", fmt.Sprintf("%d.json.previous", port)))
	}
	// Remove orphaned configs too (for example after an interrupted Agent
	// reinstall where the unit file was already removed).
	configs, err := os.ReadDir("/etc/merit-mita")
	if err != nil && !os.IsNotExist(err) {
		return stopped, err
	}
	for _, entry := range configs {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		port, parseErr := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if parseErr != nil || desired[port] {
			continue
		}
		_ = os.Remove(filepath.Join("/etc/merit-mita", entry.Name()))
	}
	if _, err := a.run("systemctl", "daemon-reload"); err != nil {
		return stopped, fmt.Errorf("清理实例后 systemd 重载失败: %w", err)
	}
	return stopped, nil
}

func (a *Agent) status(installed bool, lastErr error) model.AgentStatus {
	st := model.AgentStatus{
		MitaInstalled: installed,
		OS:            runtimeOS(),
		Arch:          runtime.GOARCH,
		AgentVersion:  Version,
		AgentID:       a.instanceID,
		PublicIP:      publicIP(),
	}
	if lastErr != nil {
		st.Error = lastErr.Error()
	}
	if installed {
		if v, err := a.run("mita", "version"); err == nil {
			st.MitaVersion = strings.TrimSpace(v)
		}
		st.PortInstances = a.instanceStatuses()
		for _, instance := range st.PortInstances {
			if instance.Running {
				st.MitaRunning = true
				break
			}
		}
	}
	return st
}

func (a *Agent) instanceStatuses() []model.PortInstanceStatus {
	entries, err := os.ReadDir("/etc/merit-mita")
	if err != nil {
		return nil
	}
	var out []model.PortInstanceStatus
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		running, statusErr := a.instanceRunning(port)
		result := model.PortInstanceStatus{Port: port, Running: running}
		if statusErr != nil {
			result.Error = statusErr.Error()
		}
		out = append(out, result)
	}
	return out
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
