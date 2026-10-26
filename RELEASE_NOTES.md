# Release v1.0.0

这是 MieruPilot 的第一个正式版本！

## ✨ 主要特性

### 核心功能
- **集中管理**：通过 Web 面板统一管理所有 mieru 节点
- **一键部署**：Agent 自动安装和配置 mita 服务
- **自动生成**：端口配置自动生成用户名、密码和分享链接
- **实时监控**：节点状态、mita 运行状态实时上报

### 🆕 流量管理
- ✅ 实时流量统计
- ✅ 月度流量限制
- ✅ 带宽速度控制  
- ✅ 自动/手动流量重置
- ✅ 流量使用可视化

### 🔄 自动更新
- ✅ 检查 GitHub Releases 新版本
- ✅ 版本对比和更新提示
- ✅ 更新日志展示
- ✅ 一键跳转下载页面

### 📱 客户端支持
- `mierus://` 简单分享链接
- Clash / mihomo 订阅
- 单节点 YAML 导出

### 🔐 安全特性
- 随机管理面板路径
- Session 认证机制
- 订阅地址 Token 保护
- 数据文件权限保护

### 🛠️ 运维功能
- 配置备份与恢复
- 出站 SOCKS5 代理
- 路由规则管理
- 出站连通性测试

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
feat: 添加流量统计和管理功能
feat: 添加自动更新检查
feat: 美化 README 文档
feat: 添加 GitHub Actions 自动发布
feat: 完善版本信息系统
docs: 添加详细的使用文档和常见问题
```
