# 仓库迁移指南：MieruPilot → UnifiedProxy

## 📢 重要通知

自 **2026年10月26日** 起，本项目已从 `MieruPilot` 更名为 **UnifiedProxy**，以更准确地反映项目的多协议代理支持特性。

---

## 🔄 自动重定向

GitHub 会自动将所有对旧仓库名的访问重定向到新名称，包括：

✅ **Git 操作** - `git clone`、`git push`、`git pull` 继续正常工作  
✅ **网页访问** - 所有旧链接自动跳转  
✅ **Release 下载** - 旧的下载链接仍然有效  
✅ **Issues/PR** - 现有 Issues 和 Pull Requests 继续可用  

---

## 📝 更新建议

虽然旧链接会自动重定向，但建议您更新以下内容：

### 1. **本地 Git 远程地址**（可选但推荐）

```bash
cd /path/to/your/repo
git remote set-url origin https://github.com/HlONGlin/UnifiedProxy.git
```

验证更新：
```bash
git remote -v
# 应该显示：
# origin  https://github.com/HlONGlin/UnifiedProxy.git (fetch)
# origin  https://github.com/HlONGlin/UnifiedProxy.git (push)
```

### 2. **书签和收藏**

更新您的浏览器书签：
- **旧地址**: https://github.com/HlONGlin/MieruPilot
- **新地址**: https://github.com/HlONGlin/UnifiedProxy

### 3. **自动化脚本**

如果您的脚本中硬编码了仓库路径，建议更新：

```bash
# 旧脚本
curl -fsSL https://github.com/HlONGlin/MieruPilot/raw/main/one-click.sh

# 新脚本（推荐）
curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh
```

### 4. **一键脚本自动更新**

从 **v1.3.0** 版本开始，`one-click.sh` 和自动更新功能已经使用新的仓库地址。

如果您使用旧版本的一键脚本，请重新下载：
```bash
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
```

---

## 🎯 更名原因

| 原因 | 说明 |
|------|------|
| **功能扩展** | 项目已从单一的 Mieru 管理扩展到支持 4 种代理协议 |
| **名称准确性** | "UnifiedProxy" 更准确地描述"统一代理管理平台"的定位 |
| **未来规划** | 为持续添加更多代理协议（Hysteria2、TUIC、Reality 等）做准备 |

---

## ✅ 已更新的内容

项目维护者已完成以下更新（v1.3.0+）：

- ✅ `internal/updater/updater.go` - GitHub 更新检查
- ✅ `one-click.sh` - 一键安装脚本
- ✅ 所有文档（README、FAQ、开发指南等）
- ✅ Release 说明和下载链接
- ✅ GitHub Actions 工作流

**您无需手动更新这些文件，只需拉取最新代码。**

---

## 🚀 版本兼容性

### v1.3.0 及以上
- ✅ 完全使用新仓库地址
- ✅ 自动更新功能使用 `UnifiedProxy`

### v1.2.x 及以下
- ⚠️ 仍使用旧仓库地址
- ✅ 由于 GitHub 重定向，功能仍然正常
- 💡 建议升级到 v1.3.0

---

## 📖 更新后的链接

| 资源 | 新地址 |
|------|--------|
| **仓库主页** | https://github.com/HlONGlin/UnifiedProxy |
| **Releases** | https://github.com/HlONGlin/UnifiedProxy/releases |
| **Issues** | https://github.com/HlONGlin/UnifiedProxy/issues |
| **Discussions** | https://github.com/HlONGlin/UnifiedProxy/discussions |
| **文档** | https://github.com/HlONGlin/UnifiedProxy/tree/main/docs |
| **一键脚本** | https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh |

---

## 🛠️ 故障排查

### Q: 我的 `git push` 还能工作吗？
**A**: 是的！GitHub 会自动重定向，但建议更新远程 URL（见上文）。

### Q: 已经安装的 Manager 会自动更新吗？
**A**: 是的。自动更新功能会从新仓库检查更新（如果您升级到 v1.3.0+）。

### Q: 旧的 Release 下载链接还能用吗？
**A**: 能用。GitHub 会重定向旧链接到新地址。

### Q: 我需要重新 clone 仓库吗？
**A**: 不需要。只需更新远程 URL：
```bash
git remote set-url origin https://github.com/HlONGlin/UnifiedProxy.git
```

### Q: 我的 Star 和 Watch 还在吗？
**A**: 是的！所有 Stars、Watchers、Forks 都会自动迁移。

---

## 📞 需要帮助？

如果您在迁移过程中遇到任何问题：

- 🐛 [报告问题](https://github.com/HlONGlin/UnifiedProxy/issues)
- 💬 [讨论区提问](https://github.com/HlONGlin/UnifiedProxy/discussions)
- 📖 [查看文档](https://github.com/HlONGlin/UnifiedProxy/tree/main/docs)

---

## 📅 时间线

| 日期 | 事件 |
|------|------|
| **2026-10-26** | 仓库更名为 UnifiedProxy |
| **2026-10-26** | 发布 v1.3.0（首个使用新名称的版本） |
| **永久** | GitHub 自动重定向旧链接 |

---

🎉 **感谢您对 UnifiedProxy 的支持！**

---

## 🏷️ 标签

`仓库迁移` `更名` `UnifiedProxy` `MieruPilot` `多协议代理`
