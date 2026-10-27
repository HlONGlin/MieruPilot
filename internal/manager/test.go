package manager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"merit/internal/model"
)

// TestResult 节点测试结果
type TestResult struct {
	Status           string            `json:"status"`
	Warnings         []string          `json:"warnings,omitempty"`
	PortResults      []*PortTestResult `json:"portResults,omitempty"`
	ProtocolVerified bool              `json:"protocolVerified"`
	Success          bool              `json:"success"`
	Timestamp        time.Time         `json:"timestamp"`
	PingLatency      int64             `json:"pingLatency"`         // 毫秒
	TCPConnectable   bool              `json:"tcpConnectable"`      // TCP 端口连通性
	HTTPSConnectable bool              `json:"httpsConnectable"`    // HTTPS 可访问性（针对NaïveProxy）
	ProxyTest        *ProxyTestResult  `json:"proxyTest,omitempty"` // 代理功能测试
	Errors           []string          `json:"errors,omitempty"`
	Details          map[string]string `json:"details,omitempty"`
}

// ProxyTestResult 代理功能测试结果
type ProxyTestResult struct {
	Reachable bool   `json:"reachable"`        // 是否可达
	ExitIP    string `json:"exitIP,omitempty"` // 出口IP
	Latency   int64  `json:"latency"`          // 延迟ms
	Error     string `json:"error,omitempty"`
}

// PortTestResult 端口详细测试结果
type PortTestResult struct {
	ProxyTest          *ProxyTestResult `json:"proxyTest,omitempty"`
	Status             string           `json:"status"`
	TCPChecked         bool             `json:"tcpChecked"`
	UDPStatus          string           `json:"udpStatus,omitempty"`
	Warnings           []string         `json:"warnings,omitempty"`
	ProtocolVerified   bool             `json:"protocolVerified"`
	ServiceStatusKnown bool             `json:"serviceStatusKnown"`
	PortID             string           `json:"portId"`
	Port               int              `json:"port"`
	ProxyType          string           `json:"proxyType"`
	TCPConnectable     bool             `json:"tcpConnectable"`
	UDPReachable       bool             `json:"udpReachable,omitempty"`
	TLSValid           *TLSTestResult   `json:"tlsValid,omitempty"` // TLS证书验证（NaïveProxy）
	ServiceRunning     bool             `json:"serviceRunning"`     // 服务是否运行
	Errors             []string         `json:"errors,omitempty"`
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
		Status:    "failed",
		Timestamp: time.Now(),
		Details:   make(map[string]string),
		Errors:    []string{},
	}

	if node.Address == "" {
		result.Errors = append(result.Errors, "节点地址为空")
		return result
	}

	// Test configured listeners, not unrelated SSH/HTTP ports.
	enabledPorts := 0
	unknown := false
	for _, p := range node.Ports {
		if p.Enabled {
			enabledPorts++
			portResult := s.TestPort(ctx, node, p)
			result.PortResults = append(result.PortResults, portResult)
			result.TCPConnectable = result.TCPConnectable || portResult.TCPConnectable
			result.Errors = append(result.Errors, portResult.Errors...)
			result.Warnings = append(result.Warnings, portResult.Warnings...)
			unknown = unknown || portResult.Status == "unknown"
		}
	}

	if enabledPorts == 0 {
		result.Errors = append(result.Errors, "无启用的端口")
	}

	result.Status = "reachable"
	if unknown {
		result.Status = "unknown"
	}
	if len(result.Errors) > 0 {
		result.Status = "failed"
	}
	result.Success = result.Status == "reachable"
	result.Details["scope"] = "仅检测监听端口和证书；未验证代理认证及目标转发"
	return result
}

// TestPort 测试特定端口的可用性
func (s *Server) TestPort(ctx context.Context, node *model.Node, port *model.Port) *PortTestResult {
	result := &PortTestResult{
		Status:    "failed",
		PortID:    port.ID,
		Port:      port.Port,
		ProxyType: port.ProxyType,
		Errors:    []string{},
	}

	if node.Address == "" {
		result.Errors = append(result.Errors, "节点地址为空")
		return result
	}

	if !port.Enabled {
		result.Errors = append(result.Errors, "端口已停用")
		return result
	}
	result.Warnings = append(result.Warnings, "端口连通不代表代理认证或目标转发成功，请使用客户端验证实际请求")
	result.ServiceStatusKnown = node.Registered && time.Since(node.LastSeen) < 90*time.Second
	if result.ServiceStatusKnown {
		result.ServiceRunning = port.InstanceRunning
	}
	proxyType := model.NormalizeProxyType(port.ProxyType)
	if proxyType == model.ProxyTypeMieru && strings.EqualFold(port.Protocol, model.ProtocolUDP) {
		// Mieru does not answer arbitrary UDP probes. No response is unknown,
		// not a failed TCP test or a successful UDP business transaction.
		result.UDPStatus = "unknown"
		result.Status = "unknown"
		result.Warnings = append(result.Warnings, "Mieru UDP 需要协议客户端验证；通用 UDP 探测无法确认可达性")
		return result
	}
	result.TCPChecked = true
	result.TCPConnectable = s.testTCPPort(ctx, node.Address, port.Port)
	if !result.TCPConnectable {
		result.Errors = append(result.Errors, fmt.Sprintf("TCP端口 %d 不可达", port.Port))
	}
	if result.TCPConnectable {
		result.Status = "reachable"
	}

	// 2. 根据代理类型进行特定测试
	switch proxyType {
	case "naive":
		// NaïveProxy HTTPS测试
		if port.Domain != "" {
			tlsResult := s.testTLSAt(ctx, node.Address, port.Domain, port.Port)
			result.TLSValid = tlsResult
			if !tlsResult.Valid {
				result.Errors = append(result.Errors, fmt.Sprintf("TLS证书无效: %s", tlsResult.Error))
			}
		} else {
			result.Errors = append(result.Errors, "未配置域名")
		}

	}
	if len(result.Errors) > 0 {
		result.Status = "failed"
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
	conn, err := d.DialContext(ctx, "tcp", probeAddress(address, port))
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
	conn, err := d.DialContext(ctx, "udp", probeAddress(address, port))
	if err != nil {
		return false
	}
	defer conn.Close()

	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if _, err := conn.Write([]byte("test")); err != nil {
		return false
	}
	var response [65535]byte
	n, err := conn.Read(response[:])
	return err == nil && n > 0
}

func probeAddress(host string, port int) string {
	return net.JoinHostPort(strings.Trim(strings.TrimSpace(host), "[]"), strconv.Itoa(port))
}

// testTLS 测试TLS证书
func (s *Server) testTLS(ctx context.Context, domain string, port int) *TLSTestResult {
	return s.testTLSAt(ctx, domain, domain, port)
}

func (s *Server) testTLSAt(ctx context.Context, address, domain string, port int) *TLSTestResult {
	result := &TLSTestResult{}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	firstDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{
		ServerName: domain,
		// 先不验证，获取证书信息
		InsecureSkipVerify: true,
	}}
	connRaw, err := firstDialer.DialContext(ctx, "tcp", probeAddress(address, port))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	conn := connRaw.(*tls.Conn)
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
	verifyDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{
		ServerName: domain,
	}}
	conn2, err := verifyDialer.DialContext(ctx, "tcp", probeAddress(address, port))
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
		switch model.NormalizeProxyType(p.ProxyType) {
		case model.ProxyTypeNaive:
			portConfig["naive"] = map[string]interface{}{
				"domain":   p.Domain,
				"autoACME": p.AutoACME,
				"hasCert":  p.TLSCert != "",
			}
		case model.ProxyTypeShadowTLS:
			portConfig["shadowtls"] = map[string]interface{}{
				"handshakeAddr":  p.HandshakeAddr,
				"backendAddr":    p.BackendAddr,
				"backendManaged": p.BackendManaged,
			}
		case model.ProxyTypeAnyTLS:
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
