package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"merit/internal/clash"
	"merit/internal/model"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var proxyProbeSlots = make(chan struct{}, 4)

// probeProxy authenticates through the exported client configuration, then
// requests a fixed HTTPS endpoint. No caller-controlled URL or command is run.
func (s *Server) probeProxy(ctx context.Context, n *model.Node, p *model.Port) *ProxyTestResult {
	result := &ProxyTestResult{}
	select {
	case proxyProbeSlots <- struct{}{}:
		defer func() { <-proxyProbeSlots }()
	case <-ctx.Done():
		result.Error = "测试繁忙，请稍后重试"
		return result
	}
	typ := model.NormalizeProxyType(p.ProxyType)
	binary := "mihomo"
	if typ == model.ProxyTypeNaive {
		binary = "naive"
	}
	executable := filepath.Join(s.cfg.AgentDir, "probe-tools", binary)
	if info, err := os.Stat(executable); err != nil || info.IsDir() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "merit-probe-")
	if err != nil {
		result.Error = "创建测试目录失败"
		return result
	}
	defer os.RemoveAll(dir)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		result.Error = err.Error()
		return result
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	configPath := filepath.Join(dir, "config.json")
	args := []string{}
	if typ == model.ProxyTypeNaive {
		cfg, e := clientConfig(n, p)
		if e != nil {
			result.Error = e.Error()
			return result
		}
		cfg["listen"] = fmt.Sprintf("socks://127.0.0.1:%d", port)
		raw, e := json.Marshal(cfg)
		if e != nil {
			result.Error = e.Error()
			return result
		}
		err = os.WriteFile(configPath, raw, 0600)
		args = []string{configPath}
	} else {
		single := *n
		single.Ports = []*model.Port{p}
		entries := clash.BuildEntries(&single)
		if len(entries) != 1 {
			result.Error = "该端口缺少完整客户端参数"
			return result
		}
		yaml := strings.Replace(clash.GenerateYAML(entries), "mixed-port: 7890", fmt.Sprintf("mixed-port: %d", port), 1)
		yaml += "\nexternal-controller: \"\"\ngeodata-mode: false\n"
		err = os.WriteFile(configPath, []byte(yaml), 0600)
		args = []string{"-d", dir, "-f", configPath}
	}
	if err != nil {
		result.Error = "写入测试配置失败"
		return result
	}
	logFile, err := os.OpenFile(filepath.Join(dir, "client.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer logFile.Close()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err = cmd.Start(); err != nil {
		result.Error = "启动协议测试客户端失败"
		return result
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ready := false
	for i := 0; i < 50; i++ {
		c, e := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if e == nil {
			c.Close()
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			result.Error = "测试客户端启动超时"
			return result
		case <-time.After(80 * time.Millisecond):
		}
	}
	if !ready {
		result.Error = "协议客户端未启动，请检查客户端配置或测试工具版本"
		return result
	}
	start := time.Now()
	curl := exec.CommandContext(ctx, "curl", "--silent", "--show-error", "--fail", "--max-time", "10", "--max-filesize", "128", "--proxy", "socks5h://"+address, "https://api.ipify.org")
	raw, e := curl.CombinedOutput()
	result.Latency = time.Since(start).Milliseconds()
	if e != nil {
		result.Error = "代理认证或 HTTPS 转发失败：" + strings.TrimSpace(string(raw))
		if len(result.Error) > 500 {
			result.Error = result.Error[:500]
		}
		return result
	}
	ip := strings.TrimSpace(string(raw))
	if net.ParseIP(ip) == nil {
		result.Error = "代理测试未返回有效出口 IP"
		return result
	}
	result.Reachable = true
	result.ExitIP = ip
	return result
}
