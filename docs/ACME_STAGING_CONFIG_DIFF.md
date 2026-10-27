# NaïveProxy ACME Staging 配置差异

## 生成日期
2026-10-27

## 测试端口配置
```go
Port{
    ID:       "test-123",
    Protocol: "naive",
    Port:     443,
    Domain:   "example.com",
    Username: "user1",
    Password: "pass1",
    AutoACME: true,
    ACMEStaging: false,  // 切换这个字段
}
```

## Production CA 配置 (ACMEStaging = false)

### TLS Automation 部分
```json
{
  "apps": {
    "tls": {
      "automation": {
        "policies": [
          {
            "subjects": ["example.com"],
            "issuers": [
              {
                "module": "acme"
              }
            ]
          }
        ]
      },
      "certificates": {
        "automate": ["example.com"]
      }
    }
  }
}
```

**关键特征**: `issuers` 中**没有** `ca` 字段，使用 Caddy 默认 CA（Let's Encrypt production）

---

## Staging CA 配置 (ACMEStaging = true)

### TLS Automation 部分
```json
{
  "apps": {
    "tls": {
      "automation": {
        "policies": [
          {
            "subjects": ["example.com"],
            "issuers": [
              {
                "ca": "https://acme-staging-v02.api.letsencrypt.org/directory",
                "module": "acme"
              }
            ]
          }
        ]
      },
      "certificates": {
        "automate": ["example.com"]
      }
    }
  }
}
```

**关键特征**: `issuers` 中**显式指定** `ca` 字段为 Let's Encrypt staging endpoint

---

## 唯一差异点

### Issuer 配置对比

| 字段 | Production | Staging |
|------|-----------|---------|
| `module` | `"acme"` | `"acme"` |
| `ca` | *未设置* | `"https://acme-staging-v02.api.letsencrypt.org/directory"` |

### 完整 Issuer 对象

**Production**:
```json
{
  "module": "acme"
}
```

**Staging**:
```json
{
  "ca": "https://acme-staging-v02.api.letsencrypt.org/directory",
  "module": "acme"
}
```

---

## Staging 证书特征

### 信任链
- **不受浏览器/客户端信任**
- 签发者名称包含 `(STAGING)` 或 `Artificial Apricot` 等测试标识
- 示例: `"issuer": "(STAGING) Artificial Apricot R3"`

### 用途限制
✅ **可以验证**:
- DNS 解析是否正确
- 端口 443 是否可达
- ACME HTTP-01/TLS-ALPN 校验流程
- 自动续期机制
- 证书文件存储路径

❌ **不可以验证**:
- 客户端实际连接（会报证书错误）
- 生产环境负载测试
- 真实用户流量

### 速率限制
Staging CA 的速率限制远高于 Production：
- Production: 50 证书/域名/周
- Staging: 30000 证书/域名/周

---

## 配置切换建议

### 开发/测试阶段
```json
{
  "autoACME": true,
  "acmeStaging": true
}
```

### 生产部署前验证
1. 确认 staging 证书签发成功
2. 确认自动续期触发（可手动删除证书触发重签）
3. 查看 journal 日志无 ACME 错误

### 生产部署
```json
{
  "autoACME": true,
  "acmeStaging": false
}
```

⚠️ **切换到 production 后，需要删除旧的 staging 证书，触发重新签发**：
```bash
# 找到证书路径
journalctl -u merit-naive-<portID>.service | grep "certificate"

# 删除 staging 证书（Caddy 会自动重签）
rm -rf /etc/merit/naive/<portID>_certs/*

# 重启服务触发签发
systemctl restart merit-naive-<portID>.service
```

---

## 代码实现位置

### 配置生成逻辑
`internal/proxy/naive/naive.go:99-106`

```go
// Configure ACME issuer
issuer := map[string]any{"module": "acme"}
// Use Let's Encrypt staging CA when ACMEStaging is true.
// Staging certificates are NOT trusted by browsers or clients - they are only
// for testing the ACME flow (DNS propagation, port reachability, renewal).
if p.ACMEStaging {
    issuer["ca"] = "https://acme-staging-v02.api.letsencrypt.org/directory"
}
```

### 测试覆盖
`internal/proxy/naive/naive_test.go:TestGenerateConfigACMEStaging`
- 断言 staging 配置包含 `"ca"` 字段
- 断言 production 配置不包含 `"ca"` 字段

---

**生成工具**: `go test ./internal/proxy/naive -v -run TestPrintStagingVsProduction`  
**验证状态**: ✅ 测试通过
