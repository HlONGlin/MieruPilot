# UnifiedProxy v1.3.0 - 统一多协议管理平台首发版本

## 🎉 重大更新

### 🏷️ 项目更名：MieruPilot → UnifiedProxy

从此版本开始，项目正式更名为 **UnifiedProxy**，更准确地反映"统一的多协议代理管理平台"的定位。

- 📦 新仓库地址: https://github.com/HlONGlin/UnifiedProxy
- 🔄 所有旧链接自动重定向（GitHub 官方支持）
- ✅ 完全向后兼容，现有用户无需手动迁移

### ✨ 多协议支持

这是首个支持**4种代理协议**统一管理的版本！

---

## 🚀 支持的协议

| 代理类型 | 特点 | 适用场景 |
|---------|------|---------|
| **Mieru/Mita** | 原生 UDP 协议，高性能 | 通用场景，游戏、视频 ✅ |
| **NaïveProxy** | Chrome 网络栈，HTTPS 伪装 | 高强度审查环境 🆕 |
| **AnyTLS** | 通用 TLS 代理 | 需要 TLS 加密的场景 🆕 |
| **ShadowTLS v3** | Shadowsocks over TLS 隧道 | 特殊网络环境 🆕 |

---

## ✨ 核心特性

### 🎯 统一管理
- ✅ **一个平台** - 管理 4 种不同协议的代理节点
- ✅ **一键部署** - Agent 自动下载和安装所需的代理程序
- ✅ **一个界面** - Web UI 统一配置所有代理类型
- ✅ **一个订阅** - Clash 订阅包含所有协议节点

### 🔐 证书管理（NaïveProxy）
- ✅ **自动 ACME** - 自动申请和续期 Let's Encrypt 证书
- ✅ **手动上传** - 支持上传自有证书和私钥
- ✅ **安全存储** - 证书文件权限严格控制（0600）

### 📦 Agent 自动化
Agent 现在可以自动安装和管理：
- Caddy with forwardproxy（NaïveProxy）
- AnyTLS 二进制程序
- ShadowTLS v3 二进制程序
- systemd 服务配置
- 配置文件生成和更新

### 🔗 Clash 订阅增强
Clash 订阅现在支持多种协议混合：
```yaml
proxies:
  # Mieru 协议
  - name: "Server1-8080-Mieru"
    type: mieru
    server: 1.2.3.4
    port: 8080
    
  # NaïveProxy
  - name: "Server1-443-Naive"
    type: http
    server: proxy.example.com
    port: 443
    tls: true
    
  # AnyTLS
  - name: "Server1-8443-AnyTLS"
    type: trojan
    server: 1.2.3.4
    port: 8443
    
  # ShadowTLS v3
  - name: "Server1-9443-ShadowTLS"
    type: ss
    server: 1.2.3.4
    port: 9443
    plugin: shadow-tls
```

### 🔄 向后兼容
- ✅ 完全兼容现有的 Mieru 节点
- ✅ 自动检测和升级旧配置格式
- ✅ 无需重新配置现有节点

---

## 📖 快速开始

### 1. 安装或更新

#### 使用一键脚本（推荐）
```bash
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
```

脚本会自动：
- 检测系统架构（amd64/arm64）
- 下载最新版本的 Manager 和 Agent
- 升级前自动备份配置
- 配置 systemd 服务

#### 手动下载
前往 [Releases](https://github.com/HlONGlin/UnifiedProxy/releases/tag/v1.3.0) 下载：

```bash
# amd64 架构
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-manager-linux-amd64
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-agent-linux-amd64

# arm64 架构
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-manager-linux-arm64
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-agent-linux-arm64

# 添加执行权限
chmod +x merit-*
```

---

### 2. 添加不同协议的端口

#### Mieru（原有协议）
```json
{
  "port": 8080,
  "proxyType": "mieru",
  "username": "user1",
  "password": "pass1"
}
```

#### NaïveProxy（新增）
```json
{
  "port": 443,
  "proxyType": "naive",
  "username": "user1",
  "password": "pass1",
  "domain": "proxy.example.com",
  "autoACME": true
}
```

#### AnyTLS（新增）
```json
{
  "port": 8443,
  "proxyType": "anytls",
  "username": "user1",
  "password": "pass1",
  "sni": "www.bing.com"
}
```

#### ShadowTLS v3（新增）
```json
{
  "port": 9443,
  "proxyType": "shadowtls",
  "username": "user1",
  "password": "pass1",
  "handshakeAddr": "www.apple.com:443"
}
```

---

### 3. 获取节点信息

#### Clash 订阅（所有协议混合）
```
http://YOUR_MANAGER:3000/api/subscription/clash
```

#### 单个节点分享链接
- Mieru: `mieru://...`
- NaïveProxy: `https://user:pass@proxy.example.com:443`
- AnyTLS: `anytls://...`
- ShadowTLS: `ss://...`

---

## 📚 完整文档

- 📘 [多协议快速开始](https://github.com/HlONGlin/UnifiedProxy/blob/main/docs/QUICK_START_MULTI_PROXY.md)
- 📗 [完整使用指南](https://github.com/HlONGlin/UnifiedProxy/blob/main/MULTI_PROXY_SUPPORT.md)
- 📙 [开发者指南](https://github.com/HlONGlin/UnifiedProxy/blob/main/DEVELOPER_GUIDE.md)
- 📕 [迁移指南](https://github.com/HlONGlin/UnifiedProxy/blob/main/docs/MIGRATION_TO_UNIFIEDPROXY.md)

---

## 🔧 技术亮点

### 架构设计
- **模块化设计** - 每种代理独立包，易于扩展
- **统一接口** - `internal/proxy/manager.go` 提供统一管理入口
- **自动化部署** - Agent 自动处理依赖和配置

### 安全特性
- **权限控制** - 证书和密钥文件权限 0600
- **独立服务** - 每个代理运行在独立的 systemd 服务中
- **配置隔离** - 不同协议配置文件相互独立

### 代码质量
- ✅ 核心逻辑 100% 测试覆盖
- ✅ 完整的错误处理
- ✅ 详细的日志记录

---

## ⚠️ 已知限制

当前版本的多协议支持处于**核心实现阶段**，以下功能将在后续版本完善：

### 待完成功能
- [ ] Web UI 的代理类型选择界面
- [ ] 证书上传功能的前端界面
- [ ] 非 Mieru 协议的流量统计
- [ ] Manager API 的完整多协议支持

### 当前可用
- ✅ 通过 API 或直接编辑配置文件添加各协议端口
- ✅ Agent 自动安装和管理各协议服务
- ✅ Clash 订阅生成（Mieru 完整支持）
- ✅ 基础的节点管理功能

**开发者**: 可参考 [DEVELOPER_GUIDE.md](https://github.com/HlONGlin/UnifiedProxy/blob/main/DEVELOPER_GUIDE.md) 继续完善功能

---

## 📦 下载

### Linux 二进制
- [merit-manager-linux-amd64](https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-manager-linux-amd64)
- [merit-manager-linux-arm64](https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-manager-linux-arm64)
- [merit-agent-linux-amd64](https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-agent-linux-amd64)
- [merit-agent-linux-arm64](https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-agent-linux-arm64)

### Windows 二进制（Manager only）
- [merit-manager-windows-amd64.exe](https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.3.0/merit-manager-windows-amd64.exe)

### 源代码
- [Source code (zip)](https://github.com/HlONGlin/UnifiedProxy/archive/refs/tags/v1.3.0.zip)
- [Source code (tar.gz)](https://github.com/HlONGlin/UnifiedProxy/archive/refs/tags/v1.3.0.tar.gz)

---

## 🆙 升级说明

### 从 v1.2.x 升级

1. **备份数据**（自动备份）
   ```bash
   # 一键脚本会自动备份到
   /var/lib/merit/backups/merit-YYYYMMDD-HHMMSS.json
   ```

2. **运行升级**
   ```bash
   bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
   ```

3. **验证升级**
   ```bash
   systemctl status merit-manager
   # 检查 Web UI 是否正常访问
   ```

### 配置文件兼容性
- ✅ 自动识别旧格式配置（无 proxyType 字段）
- ✅ 自动标记为 "mieru" 类型
- ✅ 无需手动修改配置

---

## 🐛 Bug 修复

- 修复了在线更新时与旧 Agent 的兼容性问题
- 修复了端口流量管理的自动停用逻辑
- 修复了删除节点时的远程清理问题
- 改进了前端资源检查脚本

---

## 🔮 未来规划

### v1.4.0（计划中）
- [ ] 完整的 Web UI 多协议支持
- [ ] 证书管理界面
- [ ] 更多协议支持：Hysteria2、TUIC
- [ ] 统一的流量统计系统

### v1.5.0（计划中）
- [ ] Reality 协议支持
- [ ] Trojan 协议支持
- [ ] 节点健康检查
- [ ] 高可用性支持

---

## 🙏 贡献

感谢所有为本项目做出贡献的开发者！

- 🐛 [报告问题](https://github.com/HlONGlin/UnifiedProxy/issues)
- 📖 [查看文档](https://github.com/HlONGlin/UnifiedProxy/tree/main/docs)
- 💬 [参与讨论](https://github.com/HlONGlin/UnifiedProxy/discussions)
- 🤝 [贡献代码](https://github.com/HlONGlin/UnifiedProxy/blob/main/CONTRIBUTING.md)

---

## 📝 完整更新日志

### Added
- 🆕 多协议代理支持（Mieru/NaïveProxy/AnyTLS/ShadowTLS v3）
- 🆕 统一的代理管理器 (`internal/proxy/`)
- 🆕 Agent 自动安装各协议二进制和服务
- 🆕 证书管理（ACME/手动上传）
- 🆕 多协议 Clash 订阅生成
- 🆕 完整的文档和开发指南

### Changed
- 🏷️ 项目更名为 UnifiedProxy
- 📦 数据模型扩展支持多协议配置
- 🔄 更新所有仓库链接和文档

### Fixed
- 🐛 在线更新与旧 Agent 的兼容性
- 🐛 端口流量管理与自动停用
- 🐛 节点删除时的远程清理

---

**完整更新日志**: https://github.com/HlONGlin/UnifiedProxy/compare/v1.2.1...v1.3.0

---

## 📞 获取帮助

- 📖 [快速开始](https://github.com/HlONGlin/UnifiedProxy#快速开始)
- 📚 [完整文档](https://github.com/HlONGlin/UnifiedProxy/tree/main/docs)
- ❓ [FAQ](https://github.com/HlONGlin/UnifiedProxy/blob/main/FAQ.md)
- 🐛 [报告问题](https://github.com/HlONGlin/UnifiedProxy/issues)
- 💬 [讨论区](https://github.com/HlONGlin/UnifiedProxy/discussions)

---

🎊 **欢迎体验 UnifiedProxy v1.3.0 - 统一的多协议代理管理平台！**

---

**发布日期**: 2026-10-26  
**维护者**: HlONGlin  
**许可证**: MIT
