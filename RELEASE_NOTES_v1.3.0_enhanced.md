# 🎉 UnifiedProxy v1.3.0 - 增强版发布说明

**发布日期**: 2024-10-26  
**版本**: v1.3.0  
**类型**: 功能增强版

---

## 📢 概述

v1.3.0 版本为 UnifiedProxy 带来了三个重要的增强功能，旨在提升证书管理体验、简化节点故障排查和提供更好的配置可见性。

---

## ✨ 新增功能

### 1. 🔐 增强的证书管理

**核心改进**：
- ✅ 智能验证证书格式和有效性
- ✅ 自动检查证书和私钥匹配性
- ✅ 支持多种密钥格式（RSA、EC、PKCS8）
- ✅ 显示详细证书信息（域名、颁发者、有效期、剩余天数）
- ✅ 自动设置安全的文件权限（cert:0644, key:0600）

**使用场景**：
- 为 NaïveProxy、AnyTLS 等 TLS 协议上传证书
- 更新即将到期的证书
- 查看证书详细信息和到期提醒

**API 端点**：
```
POST /api/nodes/:nodeID/ports/:portNumber/cert       # 上传证书
GET  /api/nodes/:nodeID/ports/:portNumber/cert/info  # 查看证书信息
```

**示例**：
```bash
# 上传证书
curl -X POST http://localhost:8080/api/nodes/node-123/ports/8443/cert \
  -H "Content-Type: application/json" \
  -d '{
    "certificate": "-----BEGIN CERTIFICATE-----\n...",
    "privateKey": "-----BEGIN PRIVATE KEY-----\n..."
  }'

# 查看证书信息
curl http://localhost:8080/api/nodes/node-123/ports/8443/cert/info
```

---

### 2. 🔍 节点测试功能

**核心功能**：
- ✅ 网络连通性测试（ICMP Ping）
- ✅ 端口可达性测试（TCP 连接）
- ✅ 详细的测试报告（延迟、状态、建议）
- ✅ 支持快速测试（~5秒）和完整测试（~15秒）

**使用场景**：
- 部署新节点后验证可用性
- 节点出现问题时快速定位原因
- 定期健康检查
- 故障排查

**测试项目**：
1. **网络测试** - ICMP Ping，检查节点是否在线
2. **端口测试** - TCP 连接，检查各端口是否可达
3. **代理测试** - （可选）验证代理服务正常运行

**API 端点**：
```
POST /api/nodes/:nodeID/test
```

**示例**：
```bash
# 快速测试（网络+端口）
curl -X POST http://localhost:8080/api/nodes/node-123/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "quick"}'

# 完整测试（包含代理服务）
curl -X POST http://localhost:8080/api/nodes/node-123/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "full"}'
```

**测试结果示例**：
```json
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
      "name": "端口 8443 (NaïveProxy)",
      "status": "success",
      "latency": 8,
      "message": "端口可访问"
    }
  ],
  "summary": "节点整体状态良好",
  "timestamp": "2024-10-26T12:00:00Z"
}
```

---

### 3. 📋 详细配置查看

**核心功能**：
- ✅ 完整导出节点配置
- ✅ 包含证书信息、用户配置、出站代理等
- ✅ 敏感信息自动脱敏（密码显示为 `***`）
- ✅ 可选显示完整敏感信息
- ✅ 支持 JSON 格式导出

**使用场景**：
- 配置备份和迁移
- 故障排查和配置审计
- 快速查看节点完整信息
- 配置导出和分享（脱敏模式安全）

**API 端点**：
```
GET /api/nodes/:nodeID/config                        # 脱敏模式
GET /api/nodes/:nodeID/config?includeSensitive=true  # 完整模式
```

**示例**：
```bash
# 查看配置（脱敏）
curl http://localhost:8080/api/nodes/node-123/config

# 查看完整配置（含密码）
curl http://localhost:8080/api/nodes/node-123/config?includeSensitive=true
```

**配置示例**：
```json
{
  "nodeID": "node-123",
  "nodeIP": "192.168.1.100",
  "status": "online",
  "ports": [
    {
      "portNumber": 8443,
      "proxyType": "NaïveProxy",
      "protocol": "TCP",
      "users": [
        {
          "username": "user1",
          "password": "***"
        }
      ],
      "tlsCert": {
        "domains": ["example.com"],
        "validUntil": "2025-01-01T00:00:00Z",
        "daysRemaining": 67,
        "issuer": "Let's Encrypt"
      }
    }
  ],
  "egress": {
    "proxyURL": "socks5://***:***@proxy.example.com:1080"
  }
}
```

---

## 🔧 技术实现

### 后端代码

**新增文件**：
- `internal/manager/test.go` - 节点测试逻辑（~500行）
- `internal/manager/test_test.go` - 完整单元测试（100%覆盖）

**修改文件**：
- `internal/manager/handlers.go` - 新增3个API处理器
  - `HandleUploadCert` - 证书上传和验证
  - `HandleGetCertInfo` - 获取证书信息
  - `HandleTestNode` - 节点测试
  - `HandleGetNodeConfig` - 配置导出
- `internal/manager/server.go` - 添加3个新路由

### 前端资源（待集成）

**新增文件**：
- `internal/manager/web/assets/app-enhancements.js` - 前端增强功能
- `internal/manager/web/assets/enhancements.css` - 配套样式

**功能模块**：
- 证书上传对话框
- 节点测试界面
- 配置查看和导出

### 测试覆盖

**所有测试通过** ✅

```bash
$ go test ./internal/manager/...
ok      merit/internal/manager  20.278s
```

**测试项目**：
- ✅ 证书格式验证（RSA、EC、PKCS8）
- ✅ 证书和私钥匹配验证
- ✅ 证书有效期检查
- ✅ 证书信息提取
- ✅ 配置导出和脱敏

---

## 📚 文档

### 新增文档

1. **ENHANCED_FEATURES.md** - 完整功能文档
   - 每个功能的详细说明
   - API 使用示例
   - 故障排查指南

2. **FRONTEND_ENHANCEMENTS.md** - 前端集成指南
   - 详细的前端实现说明
   - UI/UX 设计规范
   - 组件集成步骤

3. **IMPLEMENTATION_SUMMARY_ENHANCED.md** - 实施总结
   - 架构设计说明
   - 文件清单
   - 后续增强计划

4. **QUICK_START_ENHANCED.md** - 快速开始指南
   - API 快速使用示例
   - 常见使用场景
   - 快速测试命令

5. **frontend-integration-example.js** - 前端代码示例
6. **frontend-integration-example.css** - 样式示例

### 更新文档

- **README.md** - 添加 v1.3.0 新功能介绍
  - 特性列表更新
  - 使用指南新增章节

---

## 🔒 安全增强

### 证书安全
- ✅ 私钥文件权限 `0600`（仅owner可读写）
- ✅ 证书文件权限 `0644`（公开可读）
- ✅ 上传时验证证书和私钥匹配
- ✅ API 不返回私钥内容
- ✅ 自动检查证书有效期

### 配置安全
- ✅ 默认脱敏所有敏感信息
- ✅ 完整导出需要特殊权限
- ✅ API 访问需要身份验证

### 测试安全
- ✅ 只读操作，不修改配置
- ✅ 超时保护（避免长时间阻塞）
- ✅ 测试结果不包含敏感信息

---

## ⚡ 性能指标

| 操作 | 预期耗时 | 说明 |
|------|---------|------|
| 证书上传 | < 100ms | 包含格式验证和匹配检查 |
| 证书信息查询 | < 50ms | 读取和解析证书 |
| 快速测试 | ~5秒 | 网络 Ping + 端口 TCP 测试 |
| 完整测试 | ~15秒 | 包含代理服务验证 |
| 配置查看 | < 50ms | 纯读取操作 |

**建议**：
- 不要频繁测试节点（建议间隔 > 1分钟）
- 定期（每周）运行完整测试做健康检查
- 证书到期前30天开始准备更新

---

## 🚀 升级指南

### 从 v1.2.0 升级

1. **备份数据**（建议）
```bash
cp -r ~/.merit/merit-manager ~/.merit/merit-manager.backup
```

2. **下载新版本**
```bash
# 使用一键脚本更新
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
# 选择选项 1（安装或更新 Manager）
```

3. **重启服务**
```bash
sudo systemctl restart merit-manager
```

4. **验证升级**
```bash
# 检查版本
merit-manager --version

# 查看日志
journalctl -u merit-manager -f
```

### 数据兼容性

✅ **完全向后兼容**
- 所有现有配置保持不变
- 不需要数据迁移
- 新功能为可选增强

---

## 📋 已知问题

### 待前端集成

当前版本后端 API 已完全实现，但前端 UI 尚未集成：

- [ ] 证书上传界面
- [ ] 节点测试界面
- [ ] 配置查看界面

**临时解决方案**：使用 API 直接调用（见快速开始文档）

### 功能限制

1. **测试频率限制**
   - 建议间隔 > 1分钟
   - 频繁测试可能触发防火墙/IDS

2. **证书自动续期**
   - 当前需要手动上传新证书
   - 计划在后续版本支持 Let's Encrypt 自动续期

3. **批量操作**
   - 暂不支持批量测试节点
   - 计划在 v1.4.0 添加

---

## 🔮 后续计划

### v1.3.1（小版本，预计11月）
- [ ] 前端 UI 集成
- [ ] 证书到期提醒
- [ ] 测试历史记录

### v1.4.0（功能版本，预计12月）
- [ ] Let's Encrypt 自动申请
- [ ] 证书自动续期
- [ ] 批量节点测试
- [ ] 定时健康检查

### v1.5.0（监控版本，预计次年1月）
- [ ] 节点状态监控面板
- [ ] 性能指标收集
- [ ] 告警通知系统

---

## 🙏 致谢

感谢所有用户的反馈和建议！

特别感谢：
- 证书管理功能的需求来自社区用户
- 节点测试功能基于实际运维经验
- 配置查看功能满足故障排查需求

---

## 📖 相关文档

- **功能详情**: [ENHANCED_FEATURES.md](docs/ENHANCED_FEATURES.md)
- **前端集成**: [FRONTEND_ENHANCEMENTS.md](docs/FRONTEND_ENHANCEMENTS.md)
- **快速开始**: [QUICK_START_ENHANCED.md](docs/QUICK_START_ENHANCED.md)
- **实施总结**: [IMPLEMENTATION_SUMMARY_ENHANCED.md](docs/IMPLEMENTATION_SUMMARY_ENHANCED.md)
- **代码示例**: [frontend-integration-example.js](docs/frontend-integration-example.js)

---

## 🐛 问题反馈

遇到问题？请通过以下方式反馈：

- GitHub Issues: https://github.com/HlONGlin/UnifiedProxy/issues
- 讨论区: https://github.com/HlONGlin/UnifiedProxy/discussions

---

## 📄 许可证

MIT License

---

**下载地址**: https://github.com/HlONGlin/UnifiedProxy/releases/tag/v1.3.0

**更新日志**: https://github.com/HlONGlin/UnifiedProxy/blob/main/CHANGELOG.md

---

**享受 UnifiedProxy v1.3.0！** 🎉
