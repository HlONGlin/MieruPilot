# MieruPilot 增强功能 - 快速开始

## 🎉 新增功能概览

### 1️⃣ 增强的证书上传功能
- ✅ 自动验证证书和私钥匹配
- ✅ 支持多种密钥格式（RSA、EC、PKCS8）
- ✅ 显示详细证书信息（域名、有效期、剩余天数）
- ✅ 安全的文件权限控制

### 2️⃣ 节点测试功能
- ✅ 网络连通性测试（Ping）
- ✅ 端口可达性测试（TCP）
- ✅ 详细测试报告和建议
- ✅ 快速和完整两种测试模式

### 3️⃣ 详细配置查看功能
- ✅ 完整节点配置导出
- ✅ 敏感信息自动脱敏
- ✅ 包含证书、用户、出站代理等信息
- ✅ 支持JSON导出和复制

---

## 📡 API 使用

### 上传证书
```bash
POST /api/nodes/{nodeID}/ports/{portNumber}/cert

# 请求体
{
  "certificate": "-----BEGIN CERTIFICATE-----\n...",
  "privateKey": "-----BEGIN PRIVATE KEY-----\n..."
}

# 响应
{
  "success": true,
  "certInfo": {
    "domains": ["example.com"],
    "validFrom": "2024-01-01T00:00:00Z",
    "validUntil": "2025-01-01T00:00:00Z",
    "issuer": "Let's Encrypt",
    "keyType": "EC",
    "keySize": 256,
    "daysRemaining": 365
  }
}
```

### 查看证书信息
```bash
GET /api/nodes/{nodeID}/ports/{portNumber}/cert/info

# 响应
{
  "domains": ["example.com"],
  "validUntil": "2025-01-01T00:00:00Z",
  "daysRemaining": 365,
  "issuer": "Let's Encrypt"
}
```

### 测试节点
```bash
POST /api/nodes/{nodeID}/test

# 请求体
{
  "testType": "full"  # 或 "quick"
}

# 响应
{
  "nodeID": "node-123",
  "overallStatus": "success",
  "tests": [
    {
      "name": "网络连通性",
      "status": "success",
      "latency": 15,
      "message": "节点网络连接正常"
    },
    {
      "name": "端口 8443",
      "status": "success",
      "latency": 8,
      "message": "端口可访问"
    }
  ],
  "summary": "节点整体状态良好"
}
```

### 查看节点配置
```bash
GET /api/nodes/{nodeID}/config

# 可选参数: ?includeSensitive=true (显示完整密码)

# 响应
{
  "nodeID": "node-123",
  "nodeIP": "192.168.1.100",
  "ports": [...],
  "egress": {...}
}
```

---

## 🧪 测试

所有功能都经过完整测试：

```bash
go test ./internal/manager/...
# 结果: ok  merit/internal/manager  20.278s
```

测试覆盖：
- ✅ 证书格式验证（RSA、EC、PKCS8）
- ✅ 证书和私钥匹配验证
- ✅ 证书有效期检查
- ✅ 证书信息提取
- ✅ 配置导出和脱敏

---

## 🎨 前端集成

### 快速开始

1. **引入样式和脚本**
```html
<link rel="stylesheet" href="/assets/enhancements.css">
<script src="/assets/app-enhancements.js"></script>
```

2. **证书上传**
```javascript
// 显示上传对话框
showCertUploadDialog(nodeID, portNumber);

// 或直接上传
uploadCertificate(nodeID, portNumber, certFile, keyFile);
```

3. **测试节点**
```javascript
// 快速测试
testNode(nodeID, 'quick');

// 完整测试
testNode(nodeID, 'full');
```

4. **查看配置**
```javascript
// 普通模式（脱敏）
viewNodeConfig(nodeID);

// 包含敏感信息
viewNodeConfig(nodeID, true);
```

详细示例见：
- `docs/frontend-integration-example.js` - 完整JavaScript代码
- `docs/frontend-integration-example.css` - 配套样式

---

## 📁 文件结构

```
internal/manager/
├── handlers.go           # 新增3个API处理器
├── server.go            # 新增3个路由
├── test.go              # 节点测试逻辑
└── test_test.go         # 单元测试

internal/manager/web/assets/
├── app-enhancements.js  # 前端增强功能
└── enhancements.css     # 增强功能样式

docs/
├── ENHANCED_FEATURES.md              # 完整功能文档
├── FRONTEND_ENHANCEMENTS.md          # 前端集成详细指南
├── IMPLEMENTATION_SUMMARY_ENHANCED.md # 实施总结
├── frontend-integration-example.js   # 前端代码示例
└── frontend-integration-example.css  # 样式示例
```

---

## 🔒 安全特性

### 证书安全
- 私钥文件权限 `0600`（仅owner可读写）
- 证书文件权限 `0644`（所有人可读）
- 上传时验证证书和私钥匹配
- API不返回私钥内容

### 配置安全
- 默认脱敏所有密码和密钥（显示为 `***`）
- 完整导出需要特殊权限
- API访问需要身份验证

### 测试安全
- 只读操作，不修改配置
- 超时保护避免长时间阻塞
- 测试结果不包含敏感信息

---

## 🚀 使用场景

### 场景 1：部署新节点
```
1. 添加节点并配置端口
2. 上传 TLS 证书 → 自动验证 ✅
3. 运行节点测试 → 确认可用 ✅
4. 查看配置 → 确认无误 ✅
```

### 场景 2：证书更新
```
1. 查看当前证书信息 → 发现即将到期 ⚠️
2. 上传新证书 → 自动验证匹配性 ✅
3. 测试节点 → 确认服务正常 ✅
```

### 场景 3：故障排查
```
1. 节点出现问题 → 运行测试 🔍
2. 查看测试结果 → 定位问题
   - 网络不通？
   - 端口被封？
   - 证书过期？
3. 查看完整配置 → 检查设置
4. 修复后重新测试 → 验证 ✅
```

### 场景 4：监控维护
```
1. 定期查看证书信息 → 到期提醒 📅
2. 定期运行节点测试 → 健康检查 🏥
3. 导出配置 → 备份或迁移 💾
```

---

## ⚡ 性能指标

| 操作 | 预期耗时 | 备注 |
|------|---------|------|
| 证书上传 | < 100ms | 包含验证 |
| 快速测试 | ~5秒 | 网络+端口 |
| 完整测试 | ~15秒 | 含代理测试 |
| 配置查看 | < 50ms | 纯读取 |

**建议**：不要频繁测试节点（建议间隔 > 1分钟）

---

## 📊 错误处理

### 常见错误

| 错误 | 原因 | 解决方法 |
|------|------|---------|
| 无法解析证书PEM格式 | 证书格式错误 | 确保是 PEM 格式 |
| 证书和私钥不匹配 | 上传了不配对的文件 | 确保证书是用该私钥签发的 |
| 证书已过期 | 证书超过有效期 | 获取新证书 |
| 网络不可达 | 无法ping通节点 | 检查网络连接、防火墙 |
| 端口无法连接 | TCP连接失败 | 检查端口是否开放 |

---

## 🎯 下一步

### 后端完成 ✅
- ✅ API实现
- ✅ 单元测试
- ✅ 文档完善

### 前端待办 ⏳
- [ ] 证书上传界面
- [ ] 节点测试界面
- [ ] 配置查看界面
- [ ] UI/UX优化

### 未来增强 🚀
- [ ] Let's Encrypt 自动申请
- [ ] 证书自动续期
- [ ] 定时健康检查
- [ ] 测试历史记录
- [ ] 批量操作支持

---

## 📖 完整文档

- **功能详情**: `docs/ENHANCED_FEATURES.md`
- **前端集成**: `docs/FRONTEND_ENHANCEMENTS.md`
- **实施总结**: `docs/IMPLEMENTATION_SUMMARY_ENHANCED.md`
- **代码示例**: `docs/frontend-integration-example.js`
- **样式参考**: `docs/frontend-integration-example.css`

---

## 🤝 贡献

欢迎提交问题和拉取请求！

## 📄 许可证

MIT License

---

**版本**: v1.3.0  
**发布日期**: 2024-10-26  
**维护者**: MieruPilot Team  
**状态**: 后端完成 ✅ | 前端待集成 ⏳

---

## 快速测试命令

```bash
# 生成测试证书
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
  -keyout test-key.pem -out test-cert.pem -days 365 -nodes \
  -subj "/CN=test.example.com"

# 上传证书
curl -X POST http://localhost:8080/api/nodes/node-123/ports/8443/cert \
  -H "Content-Type: application/json" \
  -d @- << EOF
{
  "certificate": "$(cat test-cert.pem | sed ':a;N;$!ba;s/\n/\\n/g')",
  "privateKey": "$(cat test-key.pem | sed ':a;N;$!ba;s/\n/\\n/g')"
}
EOF

# 测试节点
curl -X POST http://localhost:8080/api/nodes/node-123/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "full"}'

# 查看配置
curl http://localhost:8080/api/nodes/node-123/config

# 查看证书信息
curl http://localhost:8080/api/nodes/node-123/ports/8443/cert/info
```

---

**享受新功能！** 🎉
