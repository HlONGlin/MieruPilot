# 常见问题 FAQ

## 安装与部署

### Q: 如何安装 MieruPilot？

**A:** 使用一键安装脚本最简单：

```bash
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
```

或者下载对应平台的二进制文件手动运行。

### Q: Agent 提示权限不足？

**A:** Agent 需要 root 权限来管理 systemd 服务和配置 mieru：

```bash
sudo ./merit-agent-linux-amd64 --manager <URL> --key <密钥>
```

### Q: Manager 启动后无法访问？

**A:** 检查：

1. 防火墙是否放通端口：`sudo ufw allow 3000`
2. 监听地址是否正确：使用 `--addr 0.0.0.0:3000` 而非 `localhost`
3. 是否使用了正确的管理路径（带随机后缀）

### Q: 忘记管理员密码怎么办？

**A:** 停止 Manager，删除 `merit.json` 中的 `admin` 对象，重启后重新初始化。

**注意：** 这会重置管理员账号，但不影响节点配置。

## 节点管理

### Q: 节点状态一直显示"离线"？

**A:** 检查：

1. Agent 是否正常运行：`systemctl status merit-agent`
2. 网络连接是否正常：Agent 能否访问 Manager URL
3. 密钥是否正确
4. 查看 Agent 日志：`journalctl -u merit-agent -f`

### Q: mita 服务启动失败？

**A:** 查看 mita 日志：

```bash
journalctl -u mita -f
```

常见原因：
- 端口被占用
- 配置文件格式错误
- mieru 未正确安装

### Q: 如何手动重启 mita 服务？

**A:** 在节点服务器上执行：

```bash
sudo systemctl restart mita
sudo systemctl status mita
```

### Q: 节点配置修改后多久生效？

**A:** Agent 每分钟自动同步一次配置。如需立即生效：

```bash
sudo systemctl restart merit-agent
```

## 流量管理

### Q: 流量统计不准确？

**A:** 流量统计基于 Agent 定期上报，可能有以下情况：

- **延迟**：Agent 默认每分钟上报一次
- **精度**：大量并发连接时可能有小幅偏差（< 5%）
- **重启后**：Agent 重启会丢失未上报的增量

### Q: 如何设置流量限制？

**A:** 在端口详情页点击"流量管理"，设置月度流量限额。超限后该端口自动停用。

### Q: 流量限制可以针对单个用户吗？

**A:** 当前版本流量限制是按端口粒度。单个端口只有一个用户凭据。

### Q: 如何重置流量统计？

**A:** 在流量管理面板：

- **自动重置**：每月 1 日自动重置
- **手动重置**：点击"重置流量"按钮立即清零

## 出站与路由

### Q: 出站代理如何配置？

**A:** 
1. 在节点详情页找到"出站管理"
2. 添加 SOCKS5 落地机地址
3. 配置路由规则，指定哪些流量走该出站

### Q: 路由规则的匹配顺序？

**A:** **从上到下匹配，第一条命中的规则生效**。可以拖拽调整顺序。

示例：
```
1. [特定域名] → PROXY (日本)
2. [全部端口, 全部目标] → DIRECT
```

### Q: 出站连通性测试失败？

**A:** 检查：

1. SOCKS5 地址和端口是否正确
2. 认证信息（如果需要）
3. Manager 到出站服务器的网络连通性
4. 出站服务器防火墙设置

### Q: 如何实现"特定网站走代理，其他直连"？

**A:**

1. 添加 SOCKS5 出站代理
2. 添加路由规则：
   - 规则 1：域名 `*.google.com` → PROXY
   - 规则 2：全部目标 → DIRECT

## 订阅与客户端

### Q: Clash 订阅无法使用？

**A:** 检查：

1. 订阅 URL 是否完整（包含 token）
2. 节点是否已启用
3. 端口是否已启用
4. 客户端是否支持 mierus:// 协议

### Q: 订阅地址泄露怎么办？

**A:** 在系统设置页面点击"重置令牌"，旧的订阅地址将立即失效。

### Q: 如何在多个客户端使用？

**A:** 订阅地址可以在多个客户端重复使用，每次访问都返回最新配置。

### Q: 支持哪些客户端？

**A:** 

- **Clash / Clash Verge / Clash.Meta / mihomo**：完整支持订阅
- **原生 mieru 客户端**：复制 `mierus://` 链接导入
- **其他客户端**：下载单节点 YAML 配置手动导入

## 备份与恢复

### Q: 如何备份数据？

**A:** 
1. Web 面板：系统设置 → 导出备份
2. 手动：复制 `merit.json` 文件

### Q: 备份包含哪些数据？

**A:** 
- 所有节点配置
- 端口和用户凭据
- 出站代理设置
- 路由规则
- 订阅令牌

**不包含：**
- 管理员密码
- 流量统计数据

### Q: 如何恢复备份？

**A:**
1. Web 面板：系统设置 → 选择备份文件 → 确认恢复
2. 手动：停止 Manager，替换 `merit.json`，重启

**注意：** 恢复会覆盖当前配置，建议先导出当前备份。

## 更新相关

### Q: 如何更新 MieruPilot？

**A:**

1. 系统设置 → 检查更新 → 查看新版本
2. 下载新版本二进制文件
3. 停止服务，替换文件，重启

**Manager:**
```bash
systemctl stop merit-manager
wget <新版本URL>
chmod +x merit-manager-linux-amd64
mv merit-manager-linux-amd64 /usr/local/bin/merit-manager
systemctl start merit-manager
```

**Agent:** 类似流程

### Q: 更新会丢失数据吗？

**A:** 不会。`merit.json` 数据文件独立于程序，更新前建议备份。

### Q: 可以降级到旧版本吗？

**A:** 可以，但不保证数据格式完全兼容。降级前务必备份数据。

## 安全相关

### Q: MieruPilot 安全吗？

**A:**

- **认证**：Session 机制保护管理面板
- **Token**：订阅地址使用随机 token
- **随机路径**：管理面板使用随机后缀防扫描
- **建议**：
  - 使用强密码
  - 定期更换订阅 token
  - 将 Manager 放在 HTTPS 反向代理后
  - 限制 Manager 访问 IP（如使用防火墙）

### Q: 如何使用 HTTPS？

**A:** 使用 Nginx/Caddy 反向代理：

**Nginx 示例：**
```nginx
server {
    listen 443 ssl;
    server_name yourdomain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**Caddy 示例：**
```
yourdomain.com {
    reverse_proxy localhost:3000
}
```

### Q: 数据存储在哪里？

**A:** 所有数据存储在本地 `merit.json` 文件中，使用 JSON 格式明文存储。

**注意：** 文件包含敏感信息（密码、密钥），确保文件权限为 `600`。

## 故障排查

### Q: 查看日志在哪里？

**A:**

**Manager:**
```bash
# systemd 服务
journalctl -u merit-manager -f

# 直接运行
查看终端输出
```

**Agent:**
```bash
journalctl -u merit-agent -f
```

**mita:**
```bash
journalctl -u mita -f
```

### Q: 端口连接失败？

**A:** 排查步骤：

1. 检查端口是否启用
2. 检查 mita 服务状态：`systemctl status mita`
3. 检查防火墙：`sudo ufw status`
4. 测试端口监听：`ss -tuln | grep <端口>`
5. 查看 mita 日志

### Q: 出现 "context deadline exceeded" 错误？

**A:** 网络超时问题，检查：

1. Agent 到 Manager 的网络连接
2. DNS 解析是否正常
3. 防火墙/安全组设置

### Q: JSON 解析错误？

**A:** `merit.json` 文件损坏：

1. 停止服务
2. 从备份恢复
3. 或手动修复 JSON 格式（使用 JSON 验证工具）

## 性能相关

### Q: Manager 支持多少节点？

**A:** 理论上无限制，实测：
- 10 节点：轻松运行
- 50 节点：正常运行
- 100+ 节点：建议使用更强的硬件

### Q: 单节点支持多少连接？

**A:** 取决于：
- 服务器硬件配置
- 网络带宽
- mieru 本身性能

一般 1核2G 服务器可支持数百并发连接。

### Q: 流量统计影响性能吗？

**A:** 影响极小。统计逻辑在 Agent 端，使用原子操作，性能开销可忽略。

## 其他

### Q: MieruPilot 和 mieru 什么关系？

**A:** 
- **mieru**：底层协议实现（mita 服务端 + mieru 客户端）
- **MieruPilot**：管理工具，简化多节点部署和配置

MieruPilot 依赖 mieru，但不修改其核心功能。

### Q: 可以商用吗？

**A:** 可以。MieruPilot 使用 MIT 开源协议，可自由使用、修改和分发。

### Q: 如何贡献代码？

**A:** 查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解贡献指南。

### Q: 在哪里寻求帮助？

**A:**
- GitHub Issues：报告问题
- GitHub Discussions：讨论和提问
- 查看文档：README.md 和本 FAQ

---

没找到答案？[提交 Issue](https://github.com/HlONGlin/UnifiedProxy/issues/new) 或在 [Discussions](https://github.com/HlONGlin/UnifiedProxy/discussions) 中提问。
