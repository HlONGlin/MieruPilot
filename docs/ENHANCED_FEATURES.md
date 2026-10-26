# MieruPilot 增强功能说明

## 概述

本文档描述了 MieruPilot v1.3.0 中新增和优化的功能：
1. **增强的证书上传功能**
2. **节点测试功能**
3. **详细配置信息查看**

## 1. 增强的证书上传功能

### 功能特性

#### 1.1 证书验证
上传证书时自动进行以下验证：
- ✅ PEM 格式验证
- ✅ 证书和私钥匹配验证
- ✅ 证书有效期检查（未过期、已生效）
- ✅ 支持多种私钥格式：RSA、EC、PKCS8

#### 1.2 证书信息展示
成功上传后显示：
- 📋 域名列表（CN + SANs）
- 📅 有效期（开始时间 → 结束时间）
- 🏢 颁发者信息
- 🔑 密钥类型和大小
- ⏰ 剩余有效天数

#### 1.3 安全增强
- 私钥文件权限自动设置为 `0600`
- 证书文件权限自动设置为 `0644`
- 上传前后端双重验证

### API 端点

#### POST `/api/nodes/:nodeID/ports/:portNumber/cert`

**请求体：**
```json
{
  "certificate": "-----BEGIN CERTIFICATE-----\n...",
  "privateKey": "-----BEGIN PRIVATE KEY-----\n..."
}
```

**成功响应 (200):**
```json
{
  "success": true,
  "message": "证书上传成功",
  "certInfo": {
    "domains": ["example.com", "*.example.com"],
    "validFrom": "2024-01-01T00:00:00Z",
    "validUntil": "2025-01-01T00:00:00Z",
    "issuer": "Let's Encrypt",
    "keyType": "EC",
    "keySize": 256,
    "daysRemaining": 365
  }
}
```

**错误响应 (400):**
```json
{
  "error": "证书和私钥不匹配: ..."
}
```

#### GET `/api/nodes/:nodeID/ports/:portNumber/cert/info`

查看当前证书信息（不返回私钥内容）

**成功响应 (200):**
```json
{
  "domains": ["example.com"],
  "validFrom": "2024-01-01T00:00:00Z",
  "validUntil": "2025-01-01T00:00:00Z",
  "issuer": "Let's Encrypt",
  "keyType": "RSA",
  "keySize": 2048,
  "daysRemaining": 90
}
```

### 前端集成

详见 `docs/FRONTEND_INTEGRATION.md`

---

## 2. 节点测试功能

### 功能特性

#### 2.1 多层次测试
- **网络连通性**：Ping 测试（ICMP）
- **端口可达性**：TCP 连接测试
- **代理可用性**：实际代理连接测试（对于支持的协议）

#### 2.2 详细测试结果
- ✅ 每个测试项的成功/失败状态
- ⏱️ 响应时间（毫秒）
- 📊 测试摘要和建议
- ⚠️ 失败原因详细说明

### API 端点

#### POST `/api/nodes/:nodeID/test`

**请求体：**
```json
{
  "testType": "full"
}
```

可选的 `testType` 值：
- `quick` - 仅测试网络和端口（快速）
- `full` - 完整测试包括代理连通性（默认）

**成功响应 (200):**
```json
{
  "nodeID": "node-123",
  "nodeIP": "192.168.1.100",
  "testTime": "2024-10-26T10:30:00Z",
  "overallStatus": "success",
  "tests": [
    {
      "name": "网络连通性",
      "status": "success",
      "latency": 15,
      "message": "节点网络连接正常"
    },
    {
      "name": "端口 8443 (mieru)",
      "status": "success",
      "latency": 8,
      "message": "端口可访问"
    },
    {
      "name": "端口 8080 (naive)",
      "status": "warning",
      "latency": 0,
      "message": "端口无法连接，但这可能是正常的（某些协议需要TLS）"
    }
  ],
  "summary": "节点整体状态良好，所有关键服务正常运行",
  "warnings": [
    "端口 8080 (naive) 直接TCP连接失败，这对于NaiveProxy是正常的"
  ]
}
```

**部分失败响应 (200 with warnings):**
```json
{
  "nodeID": "node-123",
  "overallStatus": "warning",
  "tests": [...],
  "summary": "节点部分服务存在问题",
  "warnings": [
    "端口 9090 无法访问"
  ]
}
```

**完全失败响应 (200 with error status):**
```json
{
  "nodeID": "node-123",
  "overallStatus": "error",
  "tests": [...],
  "summary": "节点不可访问或服务未启动",
  "errors": [
    "节点网络不可达",
    "所有端口无法连接"
  ]
}
```

### 测试逻辑

1. **网络连通性测试**
   - 使用 ICMP Ping 测试
   - 超时时间：3 秒
   - 记录延迟

2. **端口测试**
   - TCP 连接测试
   - 超时时间：5 秒
   - 测试所有配置的端口

3. **代理测试**（可选）
   - 对于 mieru：尝试连接 API
   - 对于 NaiveProxy/AnyTLS/ShadowTLS：HTTP 代理测试
   - 超时时间：10 秒

### 前端集成

在节点详情页面添加"测试节点"按钮：

```javascript
async function testNode(nodeID) {
  showLoading('正在测试节点...');
  
  try {
    const response = await fetch(`/api/nodes/${nodeID}/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ testType: 'full' })
    });
    
    const result = await response.json();
    displayTestResults(result);
  } catch (error) {
    showError('测试失败: ' + error.message);
  }
}
```

---

## 3. 详细配置信息查看

### 功能特性

#### 3.1 完整配置导出
- 节点基本信息
- 所有端口配置（包括 TLS 证书信息）
- 代理特定配置（域名、握手服务器等）
- 出站代理配置
- 网络配置

#### 3.2 敏感信息处理
- 密码和密钥自动脱敏（显示为 `***`）
- 仅显示必要的调试信息
- 支持完整导出（管理员模式）

### API 端点

#### GET `/api/nodes/:nodeID/config`

查看节点完整配置信息

**查询参数：**
- `includeSensitive=true` - 包含敏感信息（需要管理员权限）

**成功响应 (200):**
```json
{
  "nodeID": "node-123",
  "nodeIP": "192.168.1.100",
  "platform": "linux-amd64",
  "version": "1.3.0",
  "status": "running",
  "ports": [
    {
      "portNumber": 8443,
      "proxyType": "mieru",
      "protocol": "TCP",
      "users": [
        {
          "username": "user1",
          "password": "***"
        }
      ]
    },
    {
      "portNumber": 8080,
      "proxyType": "naive",
      "protocol": "TCP",
      "tlsConfig": {
        "serverName": "example.com",
        "certPath": "/opt/mieru/certs/8080.crt",
        "keyPath": "/opt/mieru/certs/8080.key",
        "certInfo": {
          "domains": ["example.com"],
          "validUntil": "2025-01-01T00:00:00Z",
          "daysRemaining": 67
        }
      },
      "users": [
        {
          "username": "admin",
          "password": "***"
        }
      ]
    }
  ],
  "egress": {
    "proxies": [
      {
        "id": "proxy-1",
        "name": "Upstream Proxy",
        "protocol": "socks5",
        "host": "upstream.example.com",
        "port": 1080,
        "enabled": true
      }
    ],
    "rules": [
      {
        "id": "rule-1",
        "name": "国内直连",
        "action": "DIRECT",
        "enabled": true
      }
    ]
  }
}
```

**包含敏感信息的响应 (includeSensitive=true):**
```json
{
  "nodeID": "node-123",
  "ports": [
    {
      "portNumber": 8443,
      "users": [
        {
          "username": "user1",
          "password": "actual-password-here"
        }
      ]
    }
  ],
  "egress": {
    "proxies": [
      {
        "username": "upstream-user",
        "password": "upstream-password"
      }
    ]
  }
}
```

### 前端集成

在节点详情页面添加"查看配置"按钮：

```javascript
async function viewNodeConfig(nodeID) {
  try {
    const response = await fetch(`/api/nodes/${nodeID}/config`);
    const config = await response.json();
    displayConfigModal(config);
  } catch (error) {
    showError('获取配置失败: ' + error.message);
  }
}
```

---

## 使用场景

### 场景 1：部署新节点
1. 添加节点并配置端口
2. 上传 TLS 证书（自动验证）
3. 运行节点测试确认可用
4. 查看配置确认无误

### 场景 2：证书更新
1. 打开节点详情页
2. 查看当前证书信息（到期时间）
3. 上传新证书（自动验证匹配性）
4. 测试节点确认服务正常

### 场景 3：故障排查
1. 节点出现问题时运行测试
2. 查看详细测试结果定位问题
   - 网络不通？
   - 端口被封？
   - 证书过期？
3. 查看完整配置检查设置
4. 修复后重新测试验证

### 场景 4：监控维护
1. 定期查看证书信息（到期提醒）
2. 定期运行节点测试
3. 导出配置用于备份或迁移

---

## 安全考虑

### 证书安全
- ✅ 私钥文件权限 `0600`（仅 owner 可读写）
- ✅ 证书文件权限 `0644`（所有人可读，owner 可写）
- ✅ 上传时验证证书和私钥匹配
- ✅ API 不返回私钥内容

### 配置信息安全
- ✅ 默认脱敏所有密码和密钥
- ✅ 完整导出需要管理员权限
- ✅ API 访问需要身份验证

### 测试安全
- ✅ 测试不会修改配置
- ✅ 测试超时保护避免长时间阻塞
- ✅ 测试结果不包含敏感信息

---

## 性能考虑

### 证书上传
- 单次上传通常 < 100ms
- 文件大小限制：证书 < 10KB，私钥 < 10KB

### 节点测试
- Quick 模式：~5 秒
- Full 模式：~15 秒
- 建议不要频繁测试（建议间隔 > 1 分钟）

### 配置查看
- 读取配置：< 50ms
- 无性能影响

---

## 错误处理

### 证书上传错误

| 错误信息 | 原因 | 解决方法 |
|---------|------|---------|
| 无法解析证书PEM格式 | 证书格式错误 | 确保是 PEM 格式，以 `-----BEGIN CERTIFICATE-----` 开头 |
| 无法解析私钥PEM格式 | 私钥格式错误 | 确保是 PEM 格式，以 `-----BEGIN PRIVATE KEY-----` 或类似开头 |
| 证书和私钥不匹配 | 上传了不配对的证书和私钥 | 确保证书是用该私钥签发的 |
| 证书已过期 | 证书超过有效期 | 获取新证书 |
| 证书尚未生效 | 证书开始时间在未来 | 检查系统时间或等待证书生效 |

### 节点测试错误

| 状态 | 含义 | 建议 |
|-----|------|------|
| 网络不可达 | 无法 ping 通节点 | 检查网络连接、防火墙、节点 IP |
| 端口无法连接 | TCP 连接失败 | 检查端口是否开放、服务是否运行 |
| 代理测试失败 | 代理服务异常 | 检查代理配置、日志 |

### 配置查看错误

| 错误 | 原因 | 解决方法 |
|-----|------|---------|
| 节点不存在 | 节点 ID 错误 | 检查节点 ID |
| 权限不足 | 没有查看权限 | 联系管理员 |

---

## API 测试示例

### 测试证书上传

```bash
# 生成测试证书
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
  -keyout test-key.pem -out test-cert.pem -days 365 -nodes \
  -subj "/CN=test.example.com"

# 上传证书
curl -X POST http://localhost:8080/api/nodes/node-123/ports/8443/cert \
  -H "Content-Type: application/json" \
  -d "{
    \"certificate\": \"$(cat test-cert.pem | sed ':a;N;$!ba;s/\n/\\n/g')\",
    \"privateKey\": \"$(cat test-key.pem | sed ':a;N;$!ba;s/\n/\\n/g')\"
  }"
```

### 测试节点

```bash
# 快速测试
curl -X POST http://localhost:8080/api/nodes/node-123/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "quick"}'

# 完整测试
curl -X POST http://localhost:8080/api/nodes/node-123/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "full"}'
```

### 查看配置

```bash
# 普通模式（脱敏）
curl http://localhost:8080/api/nodes/node-123/config

# 包含敏感信息
curl http://localhost:8080/api/nodes/node-123/config?includeSensitive=true
```

---

## 后续计划

### v1.4.0
- [ ] 自动证书续期提醒
- [ ] 批量节点测试
- [ ] 测试历史记录
- [ ] 性能监控集成

### v1.5.0
- [ ] Let's Encrypt 自动证书申请
- [ ] 证书健康度评分
- [ ] 自动化故障恢复
- [ ] 配置版本管理

---

## 相关文档

- [前端集成指南](FRONTEND_INTEGRATION.md)
- [API 参考](API_REFERENCE.md)
- [多代理支持](MULTI_PROXY_SUPPORT.md)
- [开发者指南](DEVELOPER_GUIDE.md)

---

**版本**: v1.3.0  
**更新日期**: 2024-10-26  
**维护者**: MieruPilot Team
