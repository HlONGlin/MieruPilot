package manager

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"merit/internal/model"
)

// 生成测试证书
func generateTestCert(daysValid int) (certPEM, keyPEM string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(daysValid) * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		DNSNames:              []string{"test.example.com", "*.test.example.com"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}))

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}))

	return certPEM, keyPEM, nil
}

func TestValidateCertAndKey(t *testing.T) {
	// 生成有效证书
	cert, key, err := generateTestCert(90)
	if err != nil {
		t.Fatalf("生成测试证书失败: %v", err)
	}

	// 测试有效证书
	err = validateCertAndKey(cert, key)
	if err != nil {
		t.Errorf("有效证书验证失败: %v", err)
	}

	// 测试过期证书
	expiredCert, expiredKey, _ := generateTestCert(-10) // 10天前过期
	err = validateCertAndKey(expiredCert, expiredKey)
	if err == nil {
		t.Error("期望过期证书验证失败，但成功了")
	}

	// 测试证书和密钥不匹配
	cert2, _, _ := generateTestCert(90)
	err = validateCertAndKey(cert2, key)
	if err == nil {
		t.Error("期望不匹配的证书和密钥验证失败，但成功了")
	}

	// 测试无效格式
	err = validateCertAndKey("invalid", "invalid")
	if err == nil {
		t.Error("期望无效格式验证失败，但成功了")
	}
}

func TestExtractCertInfo(t *testing.T) {
	cert, _, err := generateTestCert(90)
	if err != nil {
		t.Fatalf("生成测试证书失败: %v", err)
	}

	info := extractCertInfo(cert)
	if info == nil {
		t.Fatal("提取证书信息失败")
	}

	if info.Subject != "test.example.com" {
		t.Errorf("主题不匹配: 期望 test.example.com, 得到 %s", info.Subject)
	}

	if len(info.DNSNames) != 2 {
		t.Errorf("DNS名称数量不匹配: 期望 2, 得到 %d", len(info.DNSNames))
	}

	if info.DaysToExpire < 85 || info.DaysToExpire > 90 {
		t.Errorf("剩余天数不正确: %d", info.DaysToExpire)
	}
}

func TestGetNodeConfig(t *testing.T) {
	s := &Server{}

	node := &model.Node{
		ID:      "test-node",
		Name:    "测试节点",
		Address: "192.168.1.100",
		Domain:  "example.com",
		OS:      "linux",
		Arch:    "amd64",
		Ports: []*model.Port{
			{
				ID:        "port1",
				Port:      8443,
				Protocol:  "tcp",
				ProxyType: model.ProxyTypeNaive,
				Enabled:   true,
				Username:  "user1",
				Domain:    "naive.example.com",
				AutoACME:  true,
			},
			{
				ID:        "port2",
				Port:      9443,
				Protocol:  "udp",
				ProxyType: model.ProxyTypeMieru,
				Enabled:   true,
				Username:  "user2",
			},
		},
	}

	config := s.GetNodeConfig(node)

	// 检查节点信息
	nodeInfo, ok := config["nodeInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("nodeInfo 格式错误")
	}

	if nodeInfo["id"] != "test-node" {
		t.Errorf("节点ID不匹配")
	}

	if nodeInfo["name"] != "测试节点" {
		t.Errorf("节点名称不匹配")
	}

	// 检查端口信息
	ports, ok := config["ports"].([]map[string]interface{})
	if !ok {
		t.Fatal("ports 格式错误")
	}

	if len(ports) != 2 {
		t.Errorf("端口数量不匹配: 期望 2, 得到 %d", len(ports))
	}

	// 检查NaïveProxy配置
	naiveConfig, ok := ports[0]["naive"].(map[string]interface{})
	if !ok {
		t.Fatal("naive 配置格式错误")
	}

	if naiveConfig["domain"] != "naive.example.com" {
		t.Errorf("NaïveProxy域名不匹配")
	}

	if naiveConfig["autoACME"] != true {
		t.Errorf("AutoACME状态不匹配")
	}
}

func TestTestNodeMethods(t *testing.T) {
	s := &Server{}
	ctx := context.Background()

	// 测试ping（会失败因为地址不存在）
	err := s.pingTest(ctx, "192.0.2.1") // TEST-NET-1, 不路由
	if err == nil {
		t.Log("警告: ping测试意外成功（可能有网络连接）")
	}

	// 测试TCP端口（本地端口，会失败）
	result := s.testTCPPort(ctx, "127.0.0.1", 99999)
	if result {
		t.Error("期望TCP测试失败，但成功了")
	}

	// 测试UDP端口
	result = s.testUDPPort(ctx, "127.0.0.1", 99999)
	// UDP测试可能成功（因为UDP是无连接的）
	t.Logf("UDP测试结果: %v", result)
}

func TestTestNode(t *testing.T) {
	s := &Server{}
	ctx := context.Background()

	node := &model.Node{
		ID:      "test-node",
		Name:    "测试节点",
		Address: "192.0.2.1", // TEST-NET-1
		Ports: []*model.Port{
			{
				ID:       "port1",
				Port:     8443,
				Enabled:  true,
				Protocol: "tcp",
			},
		},
	}

	result := s.TestNode(ctx, node)

	if result == nil {
		t.Fatal("TestNode 返回 nil")
	}

	if result.Success {
		t.Log("警告: 节点测试意外成功（可能有网络连接）")
	}

	if len(result.Errors) == 0 {
		t.Log("警告: 没有错误信息（可能有网络连接）")
	}

	t.Logf("测试结果: Success=%v, Errors=%v", result.Success, result.Errors)
}

func TestTestPort(t *testing.T) {
	s := &Server{}
	ctx := context.Background()

	node := &model.Node{
		ID:      "test-node",
		Address: "192.0.2.1",
	}

	port := &model.Port{
		ID:        "port1",
		Port:      8443,
		ProxyType: model.ProxyTypeNaive,
		Domain:    "test.example.com",
		Enabled:   true,
	}

	result := s.TestPort(ctx, node, port)

	if result == nil {
		t.Fatal("TestPort 返回 nil")
	}

	if result.TCPConnectable {
		t.Log("警告: TCP测试意外成功（可能有网络连接）")
	}

	if len(result.Errors) == 0 {
		t.Log("警告: 没有错误信息（可能端口可达）")
	}

	t.Logf("端口测试结果: TCP=%v, Errors=%v", result.TCPConnectable, result.Errors)
}

func BenchmarkValidateCertAndKey(b *testing.B) {
	cert, key, err := generateTestCert(90)
	if err != nil {
		b.Fatalf("生成测试证书失败: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateCertAndKey(cert, key)
	}
}

func BenchmarkExtractCertInfo(b *testing.B) {
	cert, _, err := generateTestCert(90)
	if err != nil {
		b.Fatalf("生成测试证书失败: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractCertInfo(cert)
	}
}
