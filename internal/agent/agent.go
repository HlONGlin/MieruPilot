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
	"errors"
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
	"sync/atomic"
	"time"

	"merit/internal/buildinfo"
	"merit/internal/mieru"
	"merit/internal/model"
	"merit/internal/netcheck"
	"merit/internal/proxy/anytls"
	"merit/internal/proxy/naive"
	"merit/internal/proxy/shadowsocks"
	"merit/internal/proxy/shadowtls"
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
	runCommand func(string, ...string) (string, error)
	unitDir    string
	mitaDir    string
	// Test hooks for the managed ShadowTLS backend; nil uses the real host.
	ensureBackendBinary func() error
	backendPortFree     func(addr string) bool
	// purging disables the maintenance loops so they cannot reinstall mita
	// while the node is being torn down.
	purging atomic.Bool
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
			if a.purging.Load() {
				// The manager already removed the node; reporting would only
				// log a 401 while the cleanup unit is being scheduled.
				continue
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
				if a.purging.Load() {
					return
				}
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
				if a.purging.Load() {
					return
				}
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
	case model.TaskPurge:
		a.purging.Store(true)
		msg, err := a.purgeAll()
		res := model.TaskResult{TaskID: t.ID, OK: err == nil, Message: msg}
		if err != nil {
			res.Message = err.Error()
		}
		if reportErr := a.reportResult(res); reportErr != nil {
			return reportErr
		}
		if err == nil {
			// Report first so the manager can remove the node record, then
			// detach a short-lived unit that deletes the agent itself.
			if removeErr := a.scheduleSelfRemoval(); removeErr != nil {
				fmt.Fprintf(os.Stderr, "安排 Agent 自移除失败: %v\n", removeErr)
			}
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

	// 分离不同代理类型的端口
	mieruBindings := []model.PortBinding{}
	mieruUsers := []model.UserCred{}
	mieruPortIDs := []string{}
	otherPorts := []*model.Port{}

	// 使用完整Port信息来判断代理类型
	for _, port := range cfg.Ports {
		if port == nil {
			return "", nil, fmt.Errorf("端口配置为空")
		}
		proxyType := port.ProxyType
		if proxyType == "" {
			proxyType = model.ProxyTypeMieru
		}

		if proxyType == model.ProxyTypeMieru {
			// Mieru端口使用原有逻辑
			mieruBindings = append(mieruBindings, model.PortBinding{Port: port.Port, Protocol: port.Protocol})
			mieruUsers = append(mieruUsers, model.UserCred{Name: port.Username, Password: port.Password})
			mieruPortIDs = append(mieruPortIDs, port.ID)
		} else {
			// 其他代理类型
			otherPorts = append(otherPorts, port)
		}
	}
	// Older managers send only the parallel mita arrays.
	if len(cfg.Ports) == 0 && len(cfg.PortBindings) > 0 {
		mieruBindings, mieruUsers, mieruPortIDs = cfg.PortBindings, cfg.Users, cfg.PortIDs
	}

	var allResults []model.PortInstanceResult
	var syncErrors []error
	if len(mieruBindings) == 0 && !cfg.Partial {
		stopped, err := a.removeStaleInstances(map[int]bool{})
		if err != nil {
			syncErrors = append(syncErrors, err)
		}
		for i := range stopped {
			for _, target := range cfg.DisabledPorts {
				if target.Port == stopped[i].Port {
					stopped[i].PortID = target.ID
				}
			}
		}
		allResults = append(allResults, stopped...)
	}

	// 处理mieru端口（现有逻辑）
	if len(mieruBindings) > 0 {
		var mitaErr error
		if !a.mitaInstalled() {
			if _, instErr := a.ensureMita(); instErr != nil {
				mitaErr = fmt.Errorf("mita 未安装且自动安装失败: %w", instErr)
			}
		}

		mieruCfg := *cfg
		mieruCfg.PortBindings = mieruBindings
		mieruCfg.Users = mieruUsers
		mieruCfg.PortIDs = mieruPortIDs

		var results []model.PortInstanceResult
		if mitaErr == nil {
			results, mitaErr = a.applyPortInstances(&mieruCfg)
		}
		if mitaErr != nil {
			syncErrors = append(syncErrors, mitaErr)
			reported := map[int]bool{}
			for i := range results {
				result := &results[i]
				if !result.OK && result.Error == "" {
					result.Error = mitaErr.Error()
				}
				reported[result.Port] = true
			}
			for i, binding := range mieruBindings {
				if reported[binding.Port] {
					continue
				}
				result := model.PortInstanceResult{Port: binding.Port, Error: mitaErr.Error(), CheckedAt: time.Now()}
				if i < len(mieruPortIDs) {
					result.PortID = mieruPortIDs[i]
				}
				results = append(results, result)
			}
		}
		allResults = append(allResults, results...)
	}

	// 处理其他代理类型
	for _, port := range otherPorts {
		result := model.PortInstanceResult{
			PortID:    port.ID,
			Port:      port.Port,
			CheckedAt: time.Now(),
		}

		// 确保代理已安装
		if err := a.ensureProxyInstalled(port); err != nil {
			result.Error = fmt.Sprintf("安装代理失败: %v", err)
			result.OK = false
			allResults = append(allResults, result)
			syncErrors = append(syncErrors, fmt.Errorf("端口 %d: %s", port.Port, result.Error))
			continue
		}

		// 启动代理实例
		if err := a.startProxyInstance(port); err != nil {
			result.Error = fmt.Sprintf("启动代理失败: %v", err)
			result.OK = false
			allResults = append(allResults, result)
			syncErrors = append(syncErrors, fmt.Errorf("端口 %d: %s", port.Port, result.Error))
			continue
		}

		// 检查运行状态
		running := a.isProxyInstanceRunning(port)
		result.Running = running
		result.OK = running
		if !running {
			result.Error = "代理启动后未处于运行状态"
			syncErrors = append(syncErrors, fmt.Errorf("端口 %d: %s", port.Port, result.Error))
		}

		allResults = append(allResults, result)
	}

	// Stop proxy instances (naive/anytls/shadowtls) that are no longer part of the
	// desired configuration. They live in their own systemd units, so they would
	// otherwise keep running with stale credentials after a port is removed.
	if !cfg.Partial {
		stoppedProxies, err := a.removeStaleProxyInstances(cfg)
		if err != nil {
			syncErrors = append(syncErrors, err)
		}
		allResults = append(allResults, stoppedProxies...)
	}

	msg := fmt.Sprintf("已应用 %d 个代理实例（%d mieru，%d 其他）",
		len(cfg.Ports), len(mieruBindings), len(otherPorts))

	return msg, allResults, errors.Join(syncErrors...)
}

// ensureProxyInstalled 确保指定代理类型已安装
func (a *Agent) ensureProxyInstalled(port *model.Port) error {
	proxyType := port.ProxyType
	if proxyType == "" {
		proxyType = model.ProxyTypeMieru
	}

	switch proxyType {
	case model.ProxyTypeMieru:
		_, err := a.ensureMita()
		return err
	case model.ProxyTypeNaive:
		_, err := a.ensureNaive()
		return err
	case model.ProxyTypeAnyTLS:
		_, err := a.ensureAnyTLS()
		return err
	case model.ProxyTypeShadowTLS:
		_, err := a.ensureShadowTLS()
		return err
	default:
		return fmt.Errorf("不支持的代理类型: %s", proxyType)
	}
}

// applyPortInstances runs one mita process per enabled port. This is required
// for per-port egress because mita's native egress rules do not match inbound
// ports. Each instance gets its own JSON config, systemd unit and UDS socket.
func (a *Agent) applyPortInstances(cfg *model.DesiredConfig) ([]model.PortInstanceResult, error) {
	configDir := a.mitaConfigDir()
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
		unitPath := filepath.Join(a.systemdDir(), unit+".service")
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
	path := filepath.Join(a.mitaConfigDir(), fmt.Sprintf("%d.json", port))
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
	return writeFileAtomic(path, raw, 0o640)
}

func writeFileAtomic(path string, raw []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".merit-config-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
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
	entries, err := os.ReadDir(a.systemdDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
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
		_ = os.Remove(filepath.Join(a.systemdDir(), name))
		_ = os.Remove(filepath.Join(a.mitaConfigDir(), fmt.Sprintf("%d.json", port)))
		_ = os.Remove(filepath.Join(a.mitaConfigDir(), fmt.Sprintf("%d.json.applied", port)))
		_ = os.Remove(filepath.Join(a.mitaConfigDir(), fmt.Sprintf("%d.json.previous", port)))
	}
	// Remove orphaned configs too (for example after an interrupted Agent
	// reinstall where the unit file was already removed).
	configs, err := os.ReadDir(a.mitaConfigDir())
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
		_ = os.Remove(filepath.Join(a.mitaConfigDir(), entry.Name()))
	}
	if len(stopped) == 0 {
		return stopped, nil
	}
	if _, err := a.run("systemctl", "daemon-reload"); err != nil {
		return stopped, fmt.Errorf("清理实例后 systemd 重载失败: %w", err)
	}
	return stopped, nil
}

// removeStaleProxyInstances stops the naive/anytls/shadowtls instances whose port
// is no longer enabled. The units are named after the port ID, so a scan of the
// systemd directory finds every orphan, including ones left by an interrupted
// earlier sync.
func (a *Agent) removeStaleProxyInstances(cfg *model.DesiredConfig) ([]model.PortInstanceResult, error) {
	desired := map[string]bool{}
	for _, port := range cfg.Ports {
		if port == nil || !port.Enabled {
			continue
		}
		if model.NormalizeProxyType(port.ProxyType) != model.ProxyTypeMieru {
			desired[safePortID(port.ID)] = true
		}
	}
	disabled := map[string]model.PortInstanceTarget{}
	for _, target := range cfg.DisabledPorts {
		disabled[safePortID(target.ID)] = target
	}

	entries, err := os.ReadDir(a.systemdDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []model.PortInstanceResult
	removed := false
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".service") {
			continue
		}
		serviceName := strings.TrimSuffix(name, ".service")
		// A managed backend whose ShadowTLS port is gone would otherwise keep
		// running forever, because it has no instance of its own to report.
		if backendID, isBackend := parseBackendUnitName(serviceName); isBackend {
			if !desired[backendID] {
				if err := a.removeProxyInstance(serviceName, "", backendID); err != nil {
					return results, fmt.Errorf("清理孤立的 ShadowTLS 后端 %s 失败: %w", serviceName, err)
				}
				removed = true
			}
			continue
		}
		proxyType, portID, ok := parseProxyUnitName(serviceName)
		if !ok || desired[portID] {
			continue
		}
		port := a.proxyUnitPort(serviceName)
		if err := a.removeProxyInstance(serviceName, proxyType, portID); err != nil {
			return results, fmt.Errorf("清理已停用端口 %d 的 %s 实例失败: %w", port, proxyType, err)
		}
		removed = true
		target := disabled[portID]
		results = append(results, model.PortInstanceResult{
			PortID:    target.ID,
			Port:      firstNonZero(port, target.Port),
			Running:   false,
			OK:        true,
			CheckedAt: time.Now(),
		})
	}
	if !removed {
		return results, nil
	}
	if out, err := a.run("systemctl", "daemon-reload"); err != nil {
		return results, fmt.Errorf("清理代理实例后 systemd 重载失败: %v %s", err, strings.TrimSpace(out))
	}
	return results, nil
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

// purgeAll stops and removes every trace installed by the agent: the per-port
// mita instances, their config files (which hold the account credentials), the
// legacy mita service and the mita package. The agent itself is removed by
// scheduleSelfRemoval after the result has been reported.
func (a *Agent) purgeAll() (string, error) {
	var problems []string
	var warnings []string

	// Stop the package's legacy single-instance service so it cannot restart
	// while the per-port instances are being removed.
	_, _ = a.run("systemctl", "disable", "--now", "mita.service")
	_, _ = a.run("mita", "stop")

	entries, err := os.ReadDir("/etc/systemd/system")
	if err != nil {
		problems = append(problems, fmt.Sprintf("读取 systemd 目录失败: %v", err))
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "merit-mita-") || !strings.HasSuffix(name, ".service") {
			continue
		}
		unit := strings.TrimSuffix(name, ".service")
		if out, err := a.run("systemctl", "disable", "--now", unit+".service"); err != nil {
			// A vanished unit is fine; a still-running one must be retried.
			if active, _ := a.instanceRunning(parseInstancePort(unit)); active {
				problems = append(problems, fmt.Sprintf("停止 %s 失败: %v %s", unit, err, strings.TrimSpace(out)))
				continue
			}
		}
		if err := os.Remove(filepath.Join("/etc/systemd/system", name)); err != nil && !os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("删除 %s 失败: %v", name, err))
		}
	}
	_, _ = a.run("systemctl", "daemon-reload")

	// Remove the proxy instances of the non-mieru protocols together with the
	// binaries and configs the agent installed for them.
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".service") {
			continue
		}
		serviceName := strings.TrimSuffix(name, ".service")
		if backendID, isBackend := parseBackendUnitName(serviceName); isBackend {
			if err := a.removeProxyInstance(serviceName, "", backendID); err != nil {
				problems = append(problems, err.Error())
			}
			continue
		}
		proxyType, portID, ok := parseProxyUnitName(serviceName)
		if !ok {
			continue
		}
		if err := a.removeProxyInstance(serviceName, proxyType, portID); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if err := os.RemoveAll("/etc/merit-mita"); err != nil {
		problems = append(problems, fmt.Sprintf("删除端口配置与账号失败: %v", err))
	}
	// Remove the legacy single-instance config as well; it embeds an account
	// created before the per-port layout.
	_ = os.RemoveAll("/etc/mita")
	_ = os.RemoveAll("/var/lib/mita")
	_ = os.RemoveAll("/var/log/mita")
	_ = os.RemoveAll("/var/run/mita")
	// NaiveProxy configs and certificates, plus the binaries the agent installed
	// for the extra protocols.
	_ = os.RemoveAll("/etc/merit")
	for _, binary := range []string{naive.BinaryPath, anytls.ServerBinaryPath, shadowtls.BinaryPath, shadowsocks.BinaryPath} {
		if err := os.Remove(binary); err != nil && !os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("删除 %s 失败: %v", binary, err))
		}
	}
	_, _ = a.run("systemctl", "daemon-reload")

	// Remove the mita package. This is best effort: all instances are already
	// stopped and their configs deleted, so a packaging failure should not keep
	// the node in the panel forever.
	if a.mitaInstalled() {
		if kind, err := packageManager(); err == nil {
			var out string
			if kind == "deb" {
				out, err = a.run("dpkg", "-P", "mita")
			} else {
				out, err = a.run("rpm", "-e", "mita")
			}
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("卸载 mita 包失败: %v %s", err, strings.TrimSpace(out)))
			}
		}
	}

	if len(problems) > 0 {
		return "", fmt.Errorf("清理未完成: %s", strings.Join(problems, "；"))
	}
	msg := "已停止并删除所有 mita 实例、端口账号与 mita 包，Agent 即将自移除"
	if len(warnings) > 0 {
		msg += "（" + strings.Join(warnings, "；") + "）"
	}
	return msg, nil
}

func parseInstancePort(unit string) int {
	var port int
	_, _ = fmt.Sscanf(strings.TrimPrefix(unit, "merit-mita-"), "%d", &port)
	return port
}

// scheduleSelfRemoval starts a transient systemd unit that deletes the agent
// service and binary after the purge result has been delivered. The transient
// unit lives outside the agent's cgroup, so stopping merit-agent cannot kill
// the cleanup itself.
func (a *Agent) scheduleSelfRemoval() error {
	script := "sleep 3; systemctl disable --now merit-agent.service >/dev/null 2>&1 || true; " +
		"rm -f /etc/systemd/system/merit-agent.service /usr/local/bin/merit-agent; " +
		"systemctl daemon-reload >/dev/null 2>&1 || true"
	unit := fmt.Sprintf("merit-agent-cleanup-%d", time.Now().UnixNano())
	if out, err := a.run("systemd-run", "--unit="+unit, "--collect", "/bin/sh", "-c", script); err == nil {
		return nil
	} else if strings.TrimSpace(out) != "" {
		fmt.Fprintf(os.Stderr, "systemd-run 自移除失败: %v %s\n", err, strings.TrimSpace(out))
	}
	// Non-systemd fallback: detach a best-effort shell.
	cmd := exec.Command("/bin/sh", "-c", "( "+script+" ) >/dev/null 2>&1 &")
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
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
	}
	// Other protocols must remain observable even if mita installation failed.
	st.PortInstances = a.instanceStatuses()
	if installed {
		for _, instance := range st.PortInstances {
			if instance.Running {
				if active, _ := a.instanceRunning(instance.Port); active {
					st.MitaRunning = true
					break
				}
			}
		}
	}
	return st
}

func (a *Agent) instanceStatuses() []model.PortInstanceStatus {
	var out []model.PortInstanceStatus
	entries, err := os.ReadDir(a.mitaConfigDir())
	if err == nil {
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
			if total, err := a.trafficForPort(port); err == nil {
				result.TotalTraffic = total
			}
			if statusErr != nil {
				result.Error = statusErr.Error()
			}
			out = append(out, result)
		}
	}
	return append(out, a.proxyInstanceStatuses()...)
}

// proxyInstanceStatuses reports the naive/anytls/shadowtls instances. They are
// tracked through their systemd units rather than config files, and the unit
// records the port it serves.
func (a *Agent) proxyInstanceStatuses() []model.PortInstanceStatus {
	entries, err := os.ReadDir(a.systemdDir())
	if err != nil {
		return nil
	}
	var out []model.PortInstanceStatus
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".service") {
			continue
		}
		serviceName := strings.TrimSuffix(name, ".service")
		if _, _, ok := parseProxyUnitName(serviceName); !ok {
			continue
		}
		active, _ := a.serviceActive(serviceName)
		out = append(out, model.PortInstanceStatus{Port: a.proxyUnitPort(serviceName), Running: active})
	}
	return out
}

type mitaMetrics struct {
	Users map[string]struct {
		DownloadBytes int64 `json:"DownloadBytes"`
		UploadBytes   int64 `json:"UploadBytes"`
	} `json:"users"`
}

// trafficForPort reads the live metrics through the port's private mita UDS.
func (a *Agent) trafficForPort(port int) (int64, error) {
	data, err := os.ReadFile(filepath.Join(a.mitaConfigDir(), strconv.Itoa(port)+".json"))
	if err != nil {
		return 0, err
	}
	var cfg struct {
		Users []struct {
			Name string `json:"name"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || len(cfg.Users) == 0 || cfg.Users[0].Name == "" {
		return 0, fmt.Errorf("无法读取端口 %d 的 mita 用户", port)
	}
	out, err := a.runWithEnv(10*time.Second, map[string]string{"MITA_UDS_PATH": fmt.Sprintf("/var/run/mita/merit-mita-%d.sock", port)}, "mita", "get", "metrics")
	if err != nil {
		return 0, err
	}
	return parseMitaMetrics(out, cfg.Users[0].Name)
}

func parseMitaMetrics(out, username string) (int64, error) {
	start, end := strings.IndexByte(out, '{'), strings.LastIndexByte(out, '}')
	if start < 0 || end < start {
		return 0, fmt.Errorf("mita metrics 返回格式无效")
	}
	var metrics mitaMetrics
	if err := json.Unmarshal([]byte(out[start:end+1]), &metrics); err != nil {
		return 0, err
	}
	m, ok := metrics.Users[username]
	if !ok {
		return 0, fmt.Errorf("mita metrics 中不存在用户 %q", username)
	}
	return m.DownloadBytes + m.UploadBytes, nil
}

// --- process helpers --------------------------------------------------------

func (a *Agent) run(name string, args ...string) (string, error) {
	if a.runCommand != nil {
		return a.runCommand(name, args...)
	}
	return a.runWithEnv(60*time.Second, nil, name, args...)
}

func (a *Agent) systemdDir() string {
	if a.unitDir != "" {
		return a.unitDir
	}
	return proxyUnitDir
}

func (a *Agent) mitaConfigDir() string {
	if a.mitaDir != "" {
		return a.mitaDir
	}
	return "/etc/merit-mita"
}

func (a *Agent) runWithEnv(timeout time.Duration, extra map[string]string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if len(extra) > 0 {
		cmd.Env = os.Environ()
		for k, v := range extra {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
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
