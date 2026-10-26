package manager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"merit/internal/model"
)

// TestResult 节点测试结果
type TestResult struct {
	Success       bool              `json:"success"`
	Timestamp     time.Time         `json:"timestamp"`
	PingLatency   int64             `json:"pingLatency"`   // 毫秒
	TCPConnectable bool             `json:"tcpConnectable"` // TCP 端口连通性
	HTTPSConnectable bool           `json:"httpsConnectable"` // HTTPS 可访问性（针对NaïveProxy）
	ProxyTest     *ProxyTestResult  `json:"proxyTest,omitempty"` // 代理功能测试
	Errors        []string          `json:"errors,omitempty"`
	Details       map[string]string `json:"details,omitempty"`
}

// ProxyTestResult 代理功能测试结果
type ProxyTestResult struct {
	Reachable bool   `json:"reachable"` // 是否可达
	ExitIP    string `json:"exitIP,omitempty"` // 出口IP
	Latency   int64  `json:"latency"` // 延迟ms
	Error     string `json:"error,omitempty"`
}

// PortTestResult 端口详细测试结果
type PortTestResult struct {
	PortID        string            `json:"portId"`
	Port          int               `json:"port"`
	ProxyType     string            `json:"proxyType"`
	TCPConnectable bool             `json:"tcpConnectable"`
	UDPReachable   bool             `json:"udpReachable,omitempty"`
	TLSValid       *TLSTestResult   `json:"tlsValid,omitempty"` // TLS证书验证（NaïveProxy）
	ServiceRunning bool             `json:"serviceRunning"` // 服务是否运行
	Errors        []string          `json:"errors,omitempty"`
}

// TLSTestResult TLS证书测试
type TLSTestResult struct {
	Valid        bool      `json:"valid"`
	Issuer       string    `json:"issuer,omitempty"`
	Subject      string    `json:"subject,omitempty"`
	NotBefore    time.Time `json:"notBefore,omitempty"`
	NotAfter     time.Time `json:"notAfter,omitempty"`
	DaysToExpire int       `json:"daysToExpire"`
	Error        string    `json:"error,omitempty"`
}

// TestNode 测试节点连通性和可用性
func (s *Server) TestNode(ctx context.Context, node *model.Node) *TestResult {
	result := &TestResult{
		Timestamp: time.Now(),
		Details:   make(map[string]string),
		Errors:    []string{},
	}

	if node.Address == "" {
		result.Errors = append(result.Errors, "节点地址为空")
		return result
	}

	// 1. Ping测试（ICMP或TCP SYN）
	pingStart := time.Now()
	if err := s.pingTest(ctx, node.Address); err == nil {
		result.PingLatency = time.Since(pingStart).Milliseconds()
		result.Details["ping"] = fmt.Sprintf("%dms", result.PingLatency)
	} else {
		result.Errors = append(result.Errors, fmt.Sprintf("Ping失败: %v", err))
	}

	// 2. 测试启用的端口
	enabledPorts := 0
	for _, p := range node.Ports {
		if p.Enabled {
			enabledPorts++
			// 测试TCP连通性
			if s.testTCPPort(ctx, node.Address, p.Port) {
				result.TCPConnectable = true
			}
		}
	}

	if enabledPorts == 0 {
		result.Errors = append(result.Errors, "无启用的端口")
	}

	result.Success = len(result.Errors) == 0 && result.TCPConnectable
	return result
}

// TestPort 测试特定端口的可用性
func (s *Server) TestPort(ctx context.Context, node *model.Node, port *model.Port) *PortTestResult {
	result := &PortTestResult{
		PortID:    port.ID,
		Port:      port.Port,
		ProxyType: port.ProxyType,
		Errors:    []string{},
	}

	if node.Address == "" {
		result.Errors = append(result.Errors, "节点地址为空")
		return result
	}

	// 1. TCP连通性测试
	result.TCPConnectable = s.testTCPPort(ctx, node.Address, port.Port)
	if !result.TCPConnectable {
		result.Errors = append(result.Errors, fmt.Sprintf("TCP端口 %d 不可达", port.Port))
	}

	// 2. 根据代理类型进行特定测试
	switch port.ProxyType {
	case "naive":
		// NaïveProxy HTTPS测试
		if port.Domain != "" {
			tlsResult := s.testTLS(ctx, port.Domain, port.Port)
			result.TLSValid = tlsResult
			if !tlsResult.Valid {
				result.Errors = append(result.Errors, fmt.Sprintf("TLS证书无效: %s", tlsResult.Error))
			}
		} else {
			result.Errors = append(result.Errors, "未配置域名")
		}

	case "shadowtls":
		// ShadowTLS 连通性测试
		if result.TCPConnectable {
			result.ServiceRunning = true
		}

	case "anytls":
		// AnyTLS 连通性测试
		if result.TCPConnectable {
			result.ServiceRunning = true
		}

	case "mieru", "":
		// Mieru UDP测试
		result.UDPReachable = s.testUDPPort(ctx, node.Address, port.Port)
		if !result.UDPReachable {
			result.Errors = append(result.Errors, fmt.Sprintf("UDP端口 %d 不可达", port.Port))
		}
	}

	return result
}

// pingTest 简单的TCP连通性测试（替代ICMP ping）
func (s *Server) pingTest(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 尝试连接SSH端口作为ping测试
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(address, "22"))
	if err == nil {
		conn.Close()
		return nil
	}

	// 如果22端口不通，尝试80端口
	conn, err = d.DialContext(ctx, "tcp", net.JoinHostPort(address, "80"))
	if err == nil {
		conn.Close()
		return nil
	}

	return err
}

// testTCPPort 测试TCP端口连通性
func (s *Server) testTCPPort(ctx context.Context, address string, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// testUDPPort 测试UDP端口（简单发送探测包）
func (s *Server) testUDPPort(ctx context.Context, address string, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		return false
	}
	defer conn.Close()

	// 发送探测包
	_, err = conn.Write([]byte("test"))
	return err == nil
}

// testTLS 测试TLS证书
func (s *Server) testTLS(ctx context.Context, domain string, port int) *TLSTestResult {
	result := &TLSTestResult{}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", domain, port), &tls.Config{
		ServerName: domain,
		// 先不验证，获取证书信息
		InsecureSkipVerify: true,
	})
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer conn.Close()

	// 获取证书信息
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		result.Error = "无证书"
		return result
	}

	cert := certs[0]
	result.Subject = cert.Subject.CommonName
	result.Issuer = cert.Issuer.CommonName
	result.NotBefore = cert.NotBefore
	result.NotAfter = cert.NotAfter
	result.DaysToExpire = int(time.Until(cert.NotAfter).Hours() / 24)

	// 验证证书（这次真验证）
	conn2, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", domain, port), &tls.Config{
		ServerName: domain,
	})
	if err != nil {
		result.Error = fmt.Sprintf("证书验证失败: %v", err)
		result.Valid = false
	} else {
		conn2.Close()
		result.Valid = true
	}

	return result
}

// GetNodeConfig 获取节点详细配置信息
func (s *Server) GetNodeConfig(node *model.Node) map[string]interface{} {
	config := map[string]interface{}{
		"nodeInfo": map[string]interface{}{
			"id":       node.ID,
			"name":     node.Name,
			"address":  node.Address,
			"domain":   node.Domain,
			"remark":   node.Remark,
			"os":       node.OS,
			"arch":     node.Arch,
			"online":   node.Registered && time.Since(node.LastSeen) < 90*time.Second,
			"lastSeen": node.LastSeen,
		},
		"ports": []map[string]interface{}{},
		"system": map[string]interface{}{
			"agentVersion": node.AgentVer,
			"mitaVersion":  node.MitaVer,
			"mitaRunning":  node.MitaRunning,
		},
	}

	// 端口配置
	var ports []map[string]interface{}
	for _, p := range node.Ports {
		portConfig := map[string]interface{}{
			"id":           p.ID,
			"port":         p.Port,
			"protocol":     p.Protocol,
			"proxyType":    p.ProxyType,
			"enabled":      p.Enabled,
			"username":     p.Username,
			"label":        p.Label,
			"autoDisabled": p.AutoDisabled,
			"traffic": map[string]interface{}{
				"monthlyLimit": p.TrafficLimit,
				"used":         p.TrafficUsed,
				"resetAt":      p.TrafficResetAt,
			},
		}

		// 添加代理特定配置
		switch p.ProxyType {
		case "naive":
			portConfig["naive"] = map[string]interface{}{
				"domain":   p.Domain,
				"autoACME": p.AutoACME,
				"hasCert":  p.TLSCert != "",
			}
		case "shadowtls":
			portConfig["shadowtls"] = map[string]interface{}{
				"handshakeAddr": p.HandshakeAddr,
			}
		case "anytls":
			portConfig["anytls"] = map[string]interface{}{
				"password": p.Password,
			}
		}

		if p.InstanceError != "" {
			portConfig["error"] = p.InstanceError
		}

		ports = append(ports, portConfig)
	}
	config["ports"] = ports

	// 出站配置
	if node.Egress != nil && (len(node.Egress.Proxies) > 0 || len(node.Egress.Rules) > 0) {
		var proxies []map[string]interface{}
		for _, p := range node.Egress.Proxies {
			proxies = append(proxies, map[string]interface{}{
				"id":       p.ID,
				"name":     p.Name,
				"protocol": p.Protocol,
				"host":     p.Host,
				"port":     p.Port,
				"username": p.Username,
				"enabled":  p.Enabled,
			})
		}

		var rules []map[string]interface{}
		for _, r := range node.Egress.Rules {
			rules = append(rules, map[string]interface{}{
				"id":         r.ID,
				"name":       r.Name,
				"action":     r.Action,
				"proxyNames": r.ProxyNames,
				"enabled":    r.Enabled,
				"order":      r.Order,
			})
		}

		config["egress"] = map[string]interface{}{
			"proxies": proxies,
			"rules":   rules,
		}
	}

	return config
}

// fetchExternalIP 从外部服务获取公网IP
func fetchExternalIP(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.ipify.org?format=json", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return strings.TrimSpace(string(body)), nil
	}

	return result.IP, nil
}
