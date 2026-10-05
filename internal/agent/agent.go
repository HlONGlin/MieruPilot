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
	if !cfg.Enable {
		_, _ = a.run("mita", "stop")
		return "已停止 mita", nil
	}

	raw, err := mieru.ServerConfigJSON(cfg)
	if err != nil {
		return "", err
	}
	path := filepath.Join(os.TempDir(), "merit_server_config.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}

	if out, err := a.run("mita", "replace", "config", path); err != nil {
		return "", fmt.Errorf("应用配置失败: %v %s", err, out)
	}
	_, _ = a.run("mita", "stop")
	if out, err := a.run("mita", "start"); err != nil {
		return "", fmt.Errorf("启动 mita 失败: %v %s", err, out)
	}
	return fmt.Sprintf("已应用 %d 个端口", len(cfg.PortBindings)), nil
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
