# 🎉 v1.6.1 发布总结

**发布日期**: 2024-10-27  
**类型**: 功能增强 + 生产可靠性提升  
**状态**: ✅ 已完成,可以部署

---

## ✨ 亮点

### C1. Agent 版本兼容性检查 🔍
**防止版本漂移导致的隐藏故障**

- 自动检测 Manager 与 Agent 版本兼容性
- 面板实时显示版本状态和升级建议
- 不兼容时明确提示错误原因

**真实案例**: v1.6.0 部署时发现 Manager 升级但 Agent 未升级,导致 ShadowTLS 后端不启动

### B1. 证书到期监控 📅
**主动告警,避免服务中断**

- 后台每 6 小时扫描所有手动上传的证书
- 分级告警: 30天/7天/已过期
- 跳过 ACME 自动续期的证书

**价值**: 证书过期是生产环境最常见的服务中断原因之一

---

## 📦 交付物

### 二进制文件

- ✅ merit-manager-linux-amd64 (8.2MB)
- ✅ merit-manager-linux-arm64 (7.6MB)
- ✅ merit-manager-windows-amd64.exe (8.4MB)
- ✅ merit-agent-linux-amd64 (7.0MB)
- ✅ merit-agent-linux-arm64 (6.5MB)

### 文档

- ✅ [CHANGELOG_v1.6.1.md](CHANGELOG_v1.6.1.md) - 完整变更日志
- ✅ [V1.6.1_IMPLEMENTATION.md](docs/V1.6.1_IMPLEMENTATION.md) - 实施总结
- ✅ [V1.6.1_DEPLOYMENT_CHECKLIST.md](docs/V1.6.1_DEPLOYMENT_CHECKLIST.md) - 部署检查清单
- ✅ [RELEASE_NOTES_v1.6.1.md](dist/RELEASE_NOTES_v1.6.1.md) - 发布说明

### 测试覆盖

- ✅ 版本兼容性: 8个单元测试全部通过
- ✅ Manager 测试: 20.8秒,全部通过
- ✅ 证书监控: 功能验证完成

---

## 🚀 部署指南

### 快速升级 (5分钟)

```bash
# 1. 下载新版
wget https://github.com/HlONGlin/MieruPilot/releases/download/v1.6.1/merit-manager-linux-amd64

# 2. 停止服务
systemctl stop merit-manager

# 3. 替换二进制
mv merit-manager-linux-amd64 /usr/local/bin/merit-manager
chmod +x /usr/local/bin/merit-manager

# 4. 启动服务
systemctl start merit-manager

# 5. 验证
systemctl status merit-manager
```

### 完整部署

参考: [V1.6.1_DEPLOYMENT_CHECKLIST.md](docs/V1.6.1_DEPLOYMENT_CHECKLIST.md)

---

## ✅ 质量保证

### 测试结果

```
✅ 构建: 成功 (Linux amd64/arm64, Windows amd64)
✅ 单元测试: 全部通过
✅ Manager 测试: 20.842s - PASS
✅ 版本检查: 8/8 测试通过
✅ 证书监控: 功能验证完成
✅ 编译检查: 无警告
```

### 兼容性

- ✅ 向后兼容: v1.6.1 Manager 可管理 v1.5.x/v1.6.x Agent
- ✅ 数据库兼容: 无需迁移
- ✅ 配置兼容: 无需修改
- ⚠️ Agent v1.4.x: 不兼容,需升级

---

## 📊 API 变更

### 新增端点

```bash
# 获取即将过期的证书列表
GET /api/certificates/expiring

# 响应示例
[
  {
    "nodeId": "abc123",
    "nodeName": "节点1",
    "port": 443,
    "domain": "example.com",
    "expiresAt": "2024-12-25T00:00:00Z",
    "daysUntilExpiry": 7,
    "level": "critical",
    "message": "即将过期（7 天后）"
  }
]
```

### 现有端点增强

```bash
# Agent 报告响应增加版本信息
POST /api/agent/report

# 响应
{
  "ok": true,
  "managerVersion": "1.6.1",
  "upgradeRecommended": false,
  "message": "建议升级 Agent 到最新版本"
}

# 节点 DTO 增加版本兼容性字段
GET /api/nodes

# 响应 (每个节点)
{
  "versionCompatible": true,
  "upgradeRecommended": false,
  "compatibilityError": ""
}
```

---

## 🎯 影响分析

### 性能影响

| 指标 | 变化 | 说明 |
|------|------|------|
| CPU | +<1% | 版本检查和证书扫描 |
| 内存 | +<10MB | DTO字段和监控服务 |
| 磁盘 | 仅日志 | 无额外存储 |
| 网络 | +50 bytes/报告 | Agent响应增加版本信息 |

### 功能影响

- ✅ **新增**: 版本兼容性检查
- ✅ **新增**: 证书到期监控
- ✅ **增强**: 节点 DTO 包含版本信息
- ⭕ **无变更**: 现有功能保持不变

---

## 🐛 已知问题

### 无重大问题

---

## 📝 后续计划 (v1.6.2)

### 优先级 A - 关键体验

- [ ] **A2. 证书上传进度可视化**
  - SSE 实时推送部署状态
  - 前端进度条和状态动画
  - 每阶段预计耗时和实际状态

### 优先级 B - 可观测性

- [ ] **B1 增强. 续期事件日志**
  - 记录 ACME 续期历史
  - 面板展示续期时间线
  - 失败原因分析

### 优先级 C - 防御性保护

- [ ] **C1 增强. 升级前自动快照**
  - Agent 升级前自动备份
  - 升级失败时自动回滚
  - 快照保留策略

---

## 🙏 致谢

感谢以下贡献者:

- **holi** - 功能实现、测试和文档

---

## 📚 相关资源

- [GitHub Release](https://github.com/HlONGlin/MieruPilot/releases/tag/v1.6.1)
- [完整变更日志](CHANGELOG_v1.6.1.md)
- [实施文档](docs/V1.6.1_IMPLEMENTATION.md)
- [部署检查清单](docs/V1.6.1_DEPLOYMENT_CHECKLIST.md)
- [生产加固路线图](docs/PRODUCTION_HARDENING_ROADMAP.md)

---

**发布人**: holi  
**发布时间**: 2024-10-27  
**下一版本**: v1.6.2 (预计 2 周内)
