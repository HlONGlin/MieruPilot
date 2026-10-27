# v1.6.1 变更日志

**发布日期**: 2024-10-27  
**类型**: 功能增强  
**重点**: Agent版本兼容性检查、证书到期监控

---

## 🎉 新功能

### C1. Agent 版本兼容性检查

**为什么需要?**
- v1.6.0 部署测试中发现:如果 Manager 升级到 v1.6 但 Agent 还是 v1.4,ShadowTLS 后端不会启动
- 版本漂移导致的功能异常难以排查

**实施内容**:
- ✅ 语义化版本解析和兼容性检查 (`internal/version`)
- ✅ Agent 报告时自动检查版本,不兼容时记录错误
- ✅ 前端展示版本兼容性状态和升级建议
- ✅ 完整的测试覆盖

**兼容性规则**:
- 主版本号必须相同 (v1.x.x ✅, v2.x.x ❌)
- 次版本号差距≤1 (v1.6 管理 v1.5/v1.6/v1.7 ✅, v1.4 ❌)

**API 变更**:
```json
// /api/agent/report 响应增加
{
  "ok": true,
  "managerVersion": "1.6.1",
  "upgradeRecommended": false
}

// 节点 DTO 增加
{
  "versionCompatible": true,
  "upgradeRecommended": false,
  "compatibilityError": ""
}
```

---

### B1. 证书到期监控

**为什么需要?**
- NaïveProxy 手动上传的证书没有自动续期
- 证书过期导致服务中断,但没有提前告警

**实施内容**:
- ✅ 后台监控服务,每 6 小时扫描所有证书
- ✅ 分级告警 (warning: 30天内, critical: 7天内, expired: 已过期)
- ✅ 跳过 ACME 自动续期的证书
- ✅ API 端点获取需要关注的证书列表

**使用方式**:
```bash
# 查看证书监控日志
journalctl -u merit-manager | grep "证书"

# API 获取过期证书列表
curl http://localhost:8080/api/certificates/expiring
```

**示例输出**:
```
ℹ️  证书需要续期 [节点: 节点1, 端口: 443, 域名: example.com] - 即将过期（25 天后）
⚠️  证书即将过期 [节点: 节点2, 端口: 443, 域名: test.com] - 即将过期（5 天后）
```

---

## 🔧 改进

### 版本信息增强
- Agent 报告时返回 Manager 版本和升级建议
- 节点列表显示版本兼容性状态

### 代码质量
- 新增 `internal/version` 包,统一版本管理逻辑
- 完整的单元测试覆盖

---

## 🐛 修复

### 无重大bug修复
- 本版本主要是新功能添加

---

## 📊 测试验证

### 单元测试
```bash
# 版本兼容性测试
cd internal/version && go test -v
# PASS: 全部8个测试用例通过

# Manager 测试
go test -count=1 ./internal/manager/...
# PASS: 21.603s
```

### 集成测试

**版本检查场景**:
- ✅ Manager v1.6.1 + Agent v1.6.0: 兼容
- ✅ Manager v1.6.1 + Agent v1.5.0: 兼容,建议升级
- ❌ Manager v1.6.1 + Agent v1.4.0: 不兼容,显示错误

**证书监控场景**:
- ✅ 证书30天后过期: warning 级别
- ✅ 证书5天后过期: critical 级别  
- ✅ 证书已过期: expired 级别
- ✅ ACME 自动续期: 跳过监控

---

## 📦 二进制校验

### SHA256 Checksums

```
0ea766013c154ef44bcec8bd4fdc11be9e1de27e88d1705f082b782e14a14c12 *merit-agent-linux-amd64
4855559fa6a5e946ece63f9c602e5096ab1a86667db3824cd2dfb020950f4d68 *merit-agent-linux-arm64
fd08e800b0d00b2f914a022024b1e88743470fce51c10d26c3c8b9ca6ea9240b *merit-manager-linux-amd64
76af8ff2b7449b320a3eefd9272f0cdd5568ae8250baf7622028279b37ed95cb *merit-manager-linux-arm64
f9986e790b0b9cbdbf1acfae296771f28c7a31a4afb7a7227bf052f20f06b5f2 *merit-manager-windows-amd64.exe
```

**Verification**:
```bash
# Linux/macOS
cd dist && sha256sum -c SHA256SUMS_v1.6.1

# Windows PowerShell
Get-FileHash merit-manager-windows-amd64.exe -Algorithm SHA256
```

---

## 🔄 升级指南

### 从 v1.6.0 升级

**Manager 升级**:
```bash
# 下载新版
wget https://github.com/HlONGlin/MieruPilot/releases/download/v1.6.1/merit-manager-linux-amd64

# 停止服务
systemctl stop merit-manager

# 替换二进制
mv merit-manager-linux-amd64 /usr/local/bin/merit-manager
chmod +x /usr/local/bin/merit-manager

# 启动服务
systemctl start merit-manager
```

**Agent 升级** (建议但非必须):
```bash
# 使用面板的一键升级命令或:
curl -fsSL 'http://manager-url/install.sh?key=your-key' | sudo bash
```

### 兼容性说明

- ✅ **向后兼容**: v1.6.1 Manager 可以管理 v1.5.x 和 v1.6.x Agent
- ✅ **数据库兼容**: 无需迁移
- ✅ **配置兼容**: 无需修改
- ⚠️ **Agent v1.4.x**: 不兼容,必须升级

---

## 🚀 下一步计划 (v1.6.2)

1. **A2. 证书上传进度可视化**
   - SSE 实时推送部署进度
   - 前端进度条和状态动画

2. **B1 增强. 续期事件日志**
   - 记录 ACME 续期历史
   - 面板展示续期时间线

3. **C1 增强. 升级前自动快照**
   - Agent 升级前自动备份
   - 失败时自动回滚

---

## 📚 相关文档

- [v1.6.1 实施总结](docs/V1.6.1_IMPLEMENTATION.md)
- [生产加固路线图](docs/PRODUCTION_HARDENING_ROADMAP.md)
- [验收测试指南](scripts/acceptance/README.md)

---

**发布人员**: holi  
**变更审核**: -  
**最后更新**: 2024-10-27
