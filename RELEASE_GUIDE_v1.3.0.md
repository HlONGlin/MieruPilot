# 如何发布 v1.3.0 Release

## 📋 准备工作清单

### 1. 确认代码已推送
```bash
git log --oneline -5
# 确认所有更改已推送到 main 分支
```

### 2. 构建二进制文件
```bash
# 在项目根目录运行
make release

# 或者手动构建
GOOS=linux GOARCH=amd64 go build -o dist/merit-manager-linux-amd64 ./cmd/manager
GOOS=linux GOARCH=arm64 go build -o dist/merit-manager-linux-arm64 ./cmd/manager
GOOS=linux GOARCH=amd64 go build -o dist/merit-agent-linux-amd64 ./cmd/agent
GOOS=linux GOARCH=arm64 go build -o dist/merit-agent-linux-arm64 ./cmd/agent
GOOS=windows GOARCH=amd64 go build -o dist/merit-manager-windows-amd64.exe ./cmd/manager
```

### 3. 验证二进制文件
```bash
ls -lh dist/
# 确认所有二进制文件存在且大小正常
```

---

## 🚀 创建 GitHub Release

### 方法 1: 使用 GitHub CLI（推荐）

```bash
# 1. 创建 tag
git tag -a v1.3.0 -m "UnifiedProxy v1.3.0 - 统一多协议管理平台首发版本"
git push origin v1.3.0

# 2. 创建 Release
gh release create v1.3.0 \
  --title "UnifiedProxy v1.3.0 - 统一多协议管理平台" \
  --notes-file dist/RELEASE_NOTES_v1.3.0.md \
  dist/merit-manager-linux-amd64 \
  dist/merit-manager-linux-arm64 \
  dist/merit-manager-windows-amd64.exe \
  dist/merit-agent-linux-amd64 \
  dist/merit-agent-linux-arm64
```

### 方法 2: 使用 GitHub Web 界面

1. **创建 Tag**
   ```bash
   git tag -a v1.3.0 -m "UnifiedProxy v1.3.0"
   git push origin v1.3.0
   ```

2. **访问 GitHub Releases 页面**
   - 访问: https://github.com/HlONGlin/UnifiedProxy/releases/new
   - 或点击仓库页面右侧的 "Releases" → "Draft a new release"

3. **填写 Release 信息**
   - **Choose a tag**: 选择 `v1.3.0`
   - **Release title**: `UnifiedProxy v1.3.0 - 统一多协议管理平台`
   - **Description**: 复制 `dist/RELEASE_NOTES_v1.3.0.md` 的内容

4. **上传二进制文件**
   拖拽以下文件到 "Attach binaries" 区域：
   - `dist/merit-manager-linux-amd64`
   - `dist/merit-manager-linux-arm64`
   - `dist/merit-manager-windows-amd64.exe`
   - `dist/merit-agent-linux-amd64`
   - `dist/merit-agent-linux-arm64`

5. **发布**
   - 勾选 "Set as the latest release"
   - 点击 "Publish release"

---

## 📝 Release Description（复制到 GitHub）

将以下内容复制到 GitHub Release 的描述框：

---

# UnifiedProxy v1.3.0 - 统一多协议管理平台首发版本

## 🎉 重大更新

### 🏷️ 项目更名：MieruPilot → UnifiedProxy

从此版本开始，项目正式更名为 **UnifiedProxy**，更准确地反映"统一的多协议代理管理平台"的定位。

- 📦 新仓库地址: https://github.com/HlONGlin/UnifiedProxy
- 🔄 所有旧链接自动重定向（GitHub 官方支持）
- ✅ 完全向后兼容，现有用户无需手动迁移

### ✨ 多协议支持

这是首个支持**4种代理协议**统一管理的版本！

| 代理类型 | 特点 | 适用场景 |
|---------|------|---------|
| **Mieru/Mita** | 原生 UDP 协议，高性能 | 通用场景，游戏、视频 ✅ |
| **NaïveProxy** | Chrome 网络栈，HTTPS 伪装 | 高强度审查环境 🆕 |
| **AnyTLS** | 通用 TLS 代理 | 需要 TLS 加密的场景 🆕 |
| **ShadowTLS v3** | Shadowsocks over TLS 隧道 | 特殊网络环境 🆕 |

---

## 📖 快速开始

### 一键安装/升级
```bash
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
```

### 手动下载
选择对应平台的二进制文件下载即可。

---

## 📚 完整文档

- 📘 [多协议快速开始](https://github.com/HlONGlin/UnifiedProxy/blob/main/docs/QUICK_START_MULTI_PROXY.md)
- 📗 [完整使用指南](https://github.com/HlONGlin/UnifiedProxy/blob/main/MULTI_PROXY_SUPPORT.md)
- 📙 [开发者指南](https://github.com/HlONGlin/UnifiedProxy/blob/main/DEVELOPER_GUIDE.md)
- 📕 [迁移指南](https://github.com/HlONGlin/UnifiedProxy/blob/main/docs/MIGRATION_TO_UNIFIEDPROXY.md)

---

## ⚠️ 已知限制

当前版本的多协议支持处于**核心实现阶段**，以下功能将在后续版本完善：
- Web UI 的代理类型选择界面
- 证书上传功能的前端界面
- 非 Mieru 协议的流量统计

**当前可用**: 通过 API 或配置文件添加各协议端口，Agent 自动安装和管理。

---

## 🆙 升级说明

从 v1.2.x 升级：
1. 运行一键脚本（自动备份配置）
2. 验证服务状态
3. 配置自动兼容，无需手动修改

---

## 🐛 Bug 修复

- 修复了在线更新时与旧 Agent 的兼容性问题
- 修复了端口流量管理的自动停用逻辑
- 修复了删除节点时的远程清理问题

---

**完整更新日志**: https://github.com/HlONGlin/UnifiedProxy/compare/v1.2.1...v1.3.0

🎊 **欢迎体验 UnifiedProxy v1.3.0！**

---

(完整的 Release Notes 请查看仓库中的 `dist/RELEASE_NOTES_v1.3.0.md`)

---

## ✅ 发布后检查清单

- [ ] Release 页面显示正常
- [ ] 所有二进制文件可下载
- [ ] 一键脚本指向正确的下载链接
- [ ] Release 标记为 "Latest"
- [ ] 在 Discussions 中发布公告
- [ ] （可选）更新社交媒体

---

## 📞 问题反馈

如有任何问题，请访问：
- 🐛 [Issues](https://github.com/HlONGlin/UnifiedProxy/issues)
- 💬 [Discussions](https://github.com/HlONGlin/UnifiedProxy/discussions)
