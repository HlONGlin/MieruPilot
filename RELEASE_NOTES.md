# Release v1.2.0

本版本补齐端口流量管理能力：面板可以查看 mita 实时统计、设置月度限额、手动或自动重置流量，并在达到限额后自动停用端口。带宽速率限制因 mita 当前不提供可靠的服务端能力，暂不生效。

## 新特性

- 通过每个端口的 mita metrics 采集上下行流量。
- Manager 持久化累计用量，兼容 mita 重启导致的原始计数下降。
- 支持月度自动重置、面板手动重置及超额自动停用/恢复。
- Web 面板增加流量列、限额设置和重置操作。
- 一键部署默认从最新 GitHub Release 下载版本。

## 说明

- 带宽限制字段继续保留 API 兼容性，但当前不会改变实际速率。
- Agent 必须在线并能通过各端口 UDS 执行 `mita get metrics`，Manager 才能获得新的流量样本。

本次更新聚焦节点生命周期管理：删除节点时不再只移除面板记录，而是自动清理目标机上的 Agent、mita 实例、端口账号与 mita 包。

## ✨ 主要特性

### 🧹 节点远程清理
- 删除节点时自动下发清理任务
- 停止并删除所有 `mita` 端口实例与 systemd 单元
- 删除 `/etc/merit-mita` 下的端口配置与账号凭据
- 卸载 `mita` 包，并在上报成功后自移除 Agent
- 清理完成后节点自动从面板移除

### ⏳ 清理状态与可靠性
- 节点进入「清理中」状态，面板自动刷新进度
- 清理任务持久化，Manager 重启后仍会继续
- 离线节点保留任务，重连后自动补执行
- 清理期间阻止旧同步任务覆盖清理指令
- 清理失败保留任务并显示错误，支持自动重试

### 🛡️ 强制移除
- 目标机永久失联时，可使用「强制移除记录」
- 仅删除面板记录，界面会明确提示远程环境不会被清理

### 🗑️ 端口删除
- 删除端口会立即停止对应实例并删除账号
- 操作确认与提示文案已明确说明清理行为

### 📚 文档与工程
- 优化 README 结构，突出一键安装脚本
- 添加常见问题文档与开发文档
- 完善 GitHub Issue / PR 模板

## 📦 支持平台

| 平台 | 架构 | Manager | Agent |
|------|------|---------|-------|
| Linux | x86_64 | ✅ | ✅ |
| Linux | ARM64 | ✅ | ✅ |
| Windows | x86_64 | ✅ | - |
| macOS | Intel | ✅ | - |
| macOS | Apple Silicon | ✅ | - |

## 🚀 快速开始

### 一键安装

```bash
bash <(curl -fsSL https://github.com/HlONGlin/MieruPilot/raw/main/one-click.sh)
```

### 手动安装

1. 下载对应平台的二进制文件
2. 赋予执行权限：`chmod +x merit-manager-linux-amd64`
3. 启动 Manager：
   ```bash
   ./merit-manager-linux-amd64 --addr :3000 --data merit.json
   ```
4. 访问管理面板并按提示操作

## 📖 文档

完整文档请查看 [README.md](https://github.com/HlONGlin/MieruPilot#readme)

## ⚠️ 注意事项

- Agent 需要 root 权限运行
- 删除节点需要 Agent 在线才能完成远程清理；目标机失联时可强制移除记录
- 强制移除只删除面板记录，远程 Agent 与端口不会被清理
- 建议将 Manager 放在 HTTPS 反向代理后
- 定期备份 `merit.json` 数据文件
- 确保防火墙放通 Manager 端口

## 🐛 已知问题

- 流量统计依赖 Agent 定期上报，可能有轻微延迟
- 大量并发连接时流量计数可能有小幅偏差

## 🔜 未来计划

- [ ] 流量图表可视化
- [ ] WebSocket 实时推送
- [ ] 多用户管理
- [ ] API 接口文档
- [ ] Docker 镜像

## 💬 反馈与支持

遇到问题或有建议？欢迎：
- 提交 [Issue](https://github.com/HlONGlin/MieruPilot/issues)
- 发起 [Discussion](https://github.com/HlONGlin/MieruPilot/discussions)

---

**完整更新日志**

```
feat: 删除节点时自动清理远程 Agent 与 mita 实例
docs: 优化 README 结构，突出一键安装脚本
chore: 添加 GitHub 模板
docs: 添加常见问题文档
docs: 添加开发文档和构建工具
```
