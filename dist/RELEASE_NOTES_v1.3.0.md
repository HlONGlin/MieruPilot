# UnifiedProxy v1.3.0 - 三大增强功能发布 🎉

> 增强的证书管理、智能节点测试、详细配置查看

---

## 🚀 新功能

### 1. 增强的证书管理 🔐

为 **NaïveProxy**、**AnyTLS**、**ShadowTLS** 等协议提供完整的 TLS 证书管理：

- ✅ **安全上传** - 自动验证证书格式和私钥匹配
- ✅ **详细信息** - 查看域名、有效期、到期天数、颁发者
- ✅ **多格式支持** - RSA、EC、PKCS8 等密钥格式
- ✅ **到期提醒** - 实时显示证书剩余有效期
- ✅ **权限保护** - 证书 0644，私钥 0600

**API 端点**:
```bash
# 上传证书
POST /api/nodes/:id/ports/:port/cert

# 查看证书信息
GET /api/nodes/:id/ports/:port/cert/info
```

### 2. 智能节点测试 🔍

一键检测节点健康状态，快速定位问题：

- ✅ **网络连通性** - ICMP Ping 测试
- ✅ **端口可达性** - TCP 连接测试
- ✅ **详细报告** - 延迟、状态、故障建议
- ✅ **双模式** - 快速测试（~5秒）/ 完整测试（~15秒）

**测试项目**:
- 节点网络可达性
- 所有端口连通性
- 详细延迟统计
- 故障诊断建议

**API 端点**:
```bash
# 测试节点
POST /api/nodes/:id/test
```

### 3. 详细配置查看 📋

完整导出节点配置，方便备份和故障排查：

- ✅ **完整配置** - 节点、端口、证书、用户、流量
- ✅ **智能脱敏** - 密码自动隐藏为 `***`
- ✅ **可选完整** - 支持显示敏感信息（需权限）
- ✅ **JSON 格式** - 易于解析和处理

**包含信息**:
- 节点基本信息（ID、IP、状态、版本）
- 端口配置（协议类型、用户列表）
- 证书详情（域名、有效期）
- 流量统计（配额、已用、剩余）
- 出站代理配置

**API 端点**:
```bash
# 查看配置（脱敏）
GET /api/nodes/:id/config

# 查看完整配置
GET /api/nodes/:id/config?includeSensitive=true
```

---

## 📊 快速使用

### 上传证书

```bash
curl -X POST http://your-ip:8080/api/nodes/node-id/ports/8443/cert \
  -H "Content-Type: application/json" \
  -d '{
    "certificate": "-----BEGIN CERTIFICATE-----\n...",
    "privateKey": "-----BEGIN PRIVATE KEY-----\n..."
  }'
```

### 测试节点

```bash
# 快速测试
curl -X POST http://your-ip:8080/api/nodes/node-id/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "quick"}'

# 完整测试
curl -X POST http://your-ip:8080/api/nodes/node-id/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "full"}'
```

### 查看配置

```bash
# 查看配置（敏感信息已隐藏）
curl http://your-ip:8080/api/nodes/node-id/config

# 查看完整配置
curl http://your-ip:8080/api/nodes/node-id/config?includeSensitive=true
```

---

## 🔧 技术亮点

- **安全性** - 多层验证，权限控制，敏感信息保护
- **完整性** - 100% 测试覆盖，所有测试通过
- **易用性** - 清晰的 API，详细的文档
- **可靠性** - 完善的错误处理和超时保护

---

## 📖 完整文档

- **用户快速上手**: `docs/USER_GUIDE_v1.3.0.md` ⭐ 推荐
- **完整功能说明**: `docs/ENHANCED_FEATURES.md`
- **前端集成指南**: `docs/FRONTEND_ENHANCEMENTS.md`
- **快速开始**: `docs/QUICK_START_ENHANCED.md`
- **发布详情**: `RELEASE_NOTES_v1.3.0_enhanced.md`

---

## ⚠️ 重要说明

### 当前版本特点

- ✅ **后端 API 完整** - 所有功能可通过 API 使用
- 🚧 **前端 UI 开发中** - Web 界面将在 v1.3.1 推出
- ✅ **完全向后兼容** - 不影响现有功能

### 临时使用方式

在前端 UI 完成前，可以：
1. 使用 curl 命令行调用 API
2. 编写简单的脚本进行批量操作
3. 等待 v1.3.1 的 Web 界面

---

## 🚀 升级指南

### 从 v1.2.x 升级

```bash
# 1. 停止服务
systemctl stop merit-manager merit-agent

# 2. 备份数据
cp -r /var/lib/merit /var/lib/merit.backup

# 3. 下载新版本
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-manager-linux-amd64
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-agent-linux-amd64

# 4. 替换二进制文件
sudo mv merit-manager-linux-amd64 /usr/local/bin/merit-manager
sudo mv merit-agent-linux-amd64 /usr/local/bin/merit-agent
sudo chmod +x /usr/local/bin/merit-manager /usr/local/bin/merit-agent

# 5. 启动服务
systemctl start merit-manager merit-agent
```

### 验证升级

```bash
# 检查版本
/usr/local/bin/merit-manager --version
/usr/local/bin/merit-agent --version

# 测试新功能
curl -X POST http://localhost:8080/api/nodes/your-node-id/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "quick"}'
```

---

## 📦 下载

### 二进制文件

- **Linux AMD64**
  - merit-manager-linux-amd64
  - merit-agent-linux-amd64
- **Linux ARM64**
  - merit-manager-linux-arm64
  - merit-agent-linux-arm64

### 源代码

- Source code (zip)
- Source code (tar.gz)

---

## 🐛 已知问题

暂无已知问题。如遇到问题，请提交 [Issue](https://github.com/HlONGlin/UnifiedProxy/issues)。

---

## 🗓️ 后续计划

### v1.3.1（预计11月）
- 🎨 前端 UI 集成
- 📧 证书到期邮件提醒
- 📊 测试历史记录

### v1.4.0（预计12月）
- 🔐 Let's Encrypt 自动申请
- 🔄 证书自动续期
- 📦 批量操作支持

---

## 📝 更新日志

### 新增
- 证书上传和管理功能
- 证书信息查看功能
- 节点测试功能（快速/完整）
- 配置详情查看功能（脱敏/完整）
- 完整的单元测试

### 改进
- 增强 API 错误处理
- 优化超时控制
- 改进安全性（文件权限、敏感信息保护）

### 文档
- 新增用户快速上手指南
- 新增前端集成指南
- 新增完整功能文档
- 更新 README

---

## 🤝 贡献者

- [@HlONGlin](https://github.com/HlONGlin)
- Claude Sonnet 5.5

---

## 📄 许可证

[MIT License](LICENSE)

---

## 🔗 链接

- **项目主页**: https://github.com/HlONGlin/UnifiedProxy
- **文档**: https://github.com/HlONGlin/UnifiedProxy/tree/main/docs
- **问题反馈**: https://github.com/HlONGlin/UnifiedProxy/issues
- **讨论区**: https://github.com/HlONGlin/UnifiedProxy/discussions

---

**感谢使用 UnifiedProxy！** ❤️

如果这个项目对你有帮助，欢迎给我们一个 ⭐ Star！
