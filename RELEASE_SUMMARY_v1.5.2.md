# UnifiedProxy v1.5.2 发布摘要

## 📅 发布信息

- **版本号:** v1.5.2
- **发布日期:** 2024-10-27
- **版本类型:** 修复版本 (Patch Release)
- **Git Commit:** 752b93e
- **Git Tag:** v1.5.2

## 🎯 发布目标

修复影响协议部署可用性的 5 项关键缺陷，增强功能边界防护，确保所有协议在设计范围内可靠部署。

## 🐛 修复的关键问题

### P1 级别 - 阻断性问题 (3项)

1. **NaïveProxy 认证错误**
   - 问题：认证编码与上游不匹配，所有客户端无法通过认证
   - 影响：NaïveProxy 完全不可用
   - 修复：按上游 `[][]byte` 正确编码，增加模块检查

2. **安装检测路径不一致**
   - 问题：检测用 PATH，启动用固定路径，误报已安装
   - 影响：NaïveProxy、AnyTLS、ShadowTLS、SS 后端
   - 修复：统一使用固定路径检测，验证可执行性

3. **ShadowTLS 后端端口冲突**
   - 问题：新增监听不检查已分配后端端口
   - 影响：端口冲突导致服务无法绑定
   - 修复：新增/修改/恢复统一校验，原子回滚

### P2 级别 - 重要问题 (2项)

4. **Mieru UDP 检测误判**
   - 问题：无条件探测 TCP+UDP，UDP 不验证响应
   - 影响：UDP-only 误报 TCP 不可达
   - 修复：按协议区分检测，UDP 必须收到响应

5. **混合协议同步短路**
   - 问题：Mieru 失败直接 return，其他协议未处理
   - 影响：混合部署时部分协议丢失
   - 修复：收集失败继续处理，汇总错误报告

## 🛡️ 功能边界防护

### 新增 4 层防护（Mieru 独有功能）

1. **API 验证层** - 拒绝非 Mieru 正数流量限额，返回 400
2. **数据处理层** - `updatePortTraffic()` 过滤非 Mieru
3. **前端交互层** - 点击流量管理显示 toast 提示
4. **UI 展示层** - 显示"暂不支持"，隐藏操作选项

**保护功能:**
- 流量统计（仅 Mieru）
- 月度配额（仅 Mieru）
- 出站规则（仅 Mieru）

## 🔧 其他改进

- IPv6 地址 host:port 方括号处理
- AnyTLS 通配监听，ShadowTLS 双栈策略
- UI 提示 ShadowTLS 独立链接不含后端凭据
- ARM 构建文档修正（固定 fork 依赖）

## ✅ 测试验证

### 通过的测试

```bash
✅ go test -count=1 ./...     # 所有单元测试
✅ go vet ./...                # 静态分析
✅ git diff --check            # 格式检查
✅ gofmt 检查                  # 代码格式
✅ node scripts/check-frontend.cjs  # 前端检查
✅ 上游 NaïveProxy 集成测试   # CONNECT 认证
✅ 交叉编译 (amd64/arm64)      # 四项构建
```

### 新增回归测试

- NaïveProxy 上游解码测试
- 安装检测路径回归 (PATH 诱饵、权限、模块)
- 端口冲突原子回滚测试
- UDP 协议检测回归
- 混合协议失败处理

### 待完成验收

❌ **生产环境部署验收:**
- Linux systemd 生命周期
- dpkg/rpm 包安装
- 公网 VPS + 域名 + ACME
- 各协议公网客户端请求
- IPv6 数据链路

**建议:** Debian/Ubuntu + CentOS/Rocky，amd64/arm64 真实部署。

## 📦 交付物

### 二进制文件 (SHA256 已验证)

```
7ac8a083...  merit-agent-linux-amd64    (7.0 MB)
36e35517...  merit-agent-linux-arm64    (6.4 MB)
30c6dbc3...  merit-manager-linux-amd64  (8.2 MB)
76af8ff2...  merit-manager-linux-arm64  (未更新)
8d1b7d86...  merit-manager-windows-amd64.exe  (8.3 MB)
```

### 文档

- `dist/RELEASE_NOTES_v1.5.2.md` - 完整发布说明
- `CHANGELOG_v1.5.2.md` - 变更日志
- `docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md` - 详细审计报告
- `README.md` - 更新能力边界说明
- `dist/SHA256SUMS` - 校验和文件

### 代码统计

```
69 files changed, 7953 insertions(+), 1048 deletions(-)

新增文件: 35
修改文件: 34
新增测试: 15 个测试文件
```

## ⚠️ 升级要求

### 必须操作

1. **同时升级 Manager 和 Agent** （必须）
2. **重试同步所有节点** （必须）
3. **刷新客户端订阅** （必须，尤其 NaïveProxy）

### 升级步骤

```bash
# 1. 停止服务
systemctl stop merit-manager merit-agent

# 2. 备份数据
cp /var/lib/merit/merit.json /var/lib/merit/merit.json.backup-$(date +%Y%m%d)

# 3. 下载并替换
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.5.2/merit-*
chmod +x merit-*-linux-amd64
mv merit-manager-linux-amd64 /opt/merit/merit-manager
mv merit-agent-linux-amd64 /opt/merit/merit-agent

# 4. 启动服务
systemctl start merit-manager merit-agent

# 5. 验证版本
/opt/merit/merit-manager --version
# 输出: merit-manager 1.5.2 (...)
```

## 📋 已知限制

### 功能限制（设计决策）

- 流量统计: 仅 Mieru（其他协议无可靠指标）
- 月度配额: 仅 Mieru
- 出站规则: 仅 Mieru
- 速率限制: mita 不支持（配置保留但不生效）

### 架构限制

- NaïveProxy 自动安装: 仅 amd64（上游限制）
- Agent 发布: amd64/arm64（无 ARMv7）
- 解压依赖: tar + xz-utils

### 依赖要求

- Mieru: dpkg 或 rpm
- NaïveProxy/SS: tar + xz
- 所有协议: root 权限 + Linux systemd

## 📊 影响评估

### 严重性

- **高** - NaïveProxy 用户无法使用（完全阻断）
- **中** - 安装检测误报（导致服务启动失败）
- **中** - 端口冲突（导致绑定失败）
- **低** - UDP 检测误判（显示问题，不影响功能）
- **低** - 混合同步短路（部分场景）

### 影响范围

- NaïveProxy: 100% 用户受影响
- 其他协议: 部分场景受影响
- 混合部署: 受影响

### 紧急程度

**强烈建议所有用户立即升级**，特别是：
- 使用 NaïveProxy 的用户（必须升级）
- 混合部署多种协议的用户
- 遇到安装/端口问题的用户

## 🔗 相关链接

- **GitHub Release:** https://github.com/HlONGlin/UnifiedProxy/releases/tag/v1.5.2
- **完整发布说明:** [dist/RELEASE_NOTES_v1.5.2.md](dist/RELEASE_NOTES_v1.5.2.md)
- **审计报告:** [docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md](docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md)
- **项目主页:** https://github.com/HlONGlin/UnifiedProxy

## 🤝 贡献者

- [@HlONGlin](https://github.com/HlONGlin)
- Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>

## 📝 后续计划

### v1.5.3 (如需)
- 生产环境验收反馈修复
- IPv6 实际部署验证
- 文档补充完善

### v1.6.0 (计划)
- 性能优化
- 监控增强
- 用户体验改进

---

**发布完成时间:** 2024-10-27 14:30 CST  
**发布状态:** ✅ 已完成  
**Git 提交:** ce569b4 (release) + 752b93e (docs)  
**Git 标签:** v1.5.2

---

**感谢使用 UnifiedProxy！如果本版本解决了您的问题，欢迎 ⭐ Star 支持！**
