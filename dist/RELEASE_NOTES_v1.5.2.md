# UnifiedProxy v1.5.2 - 协议部署可靠性修复版本 🔧

> 关键缺陷修复、部署可靠性增强、功能限制防护

**发布日期:** 2024-12-XX  
**修复日期:** 2024-10-27  
**版本类型:** 修复版本（Patch Release）

---

## ⚠️ 重要提示

**本版本修复了影响协议部署可用性的 5 项关键缺陷，强烈建议所有用户升级。**

- ✅ 修复 NaïveProxy 认证错误（P1 - 阻断性缺陷）
- ✅ 修复安装检测路径不一致问题（P1）
- ✅ 修复 ShadowTLS 后端端口冲突（P1）
- ✅ 修复 Mieru UDP 连通性检测误判（P2）
- ✅ 修复混合协议同步短路问题（P2）

**必须同时升级 Manager 和 Agent，并在面板重试同步、刷新客户端订阅。**

---

## 🐛 修复的关键问题

### 1. NaïveProxy 认证配置错误 (P1 - 阻断性)

**问题描述:**  
认证编码与上游 klzgrad/forwardproxy 语义不匹配，导致所有客户端无法通过认证。

**影响范围:**  
- 所有使用 NaïveProxy 的节点完全不可用
- 客户端提交正确账号密码仍被拒绝

**修复内容:**
- 按上游 `[][]byte` 类型正确编码 Basic token
- 由 JSON 再进行一层 Base64 编码
- 增加安装检查：普通 Caddy 不满足要求，必须包含 forward_proxy 模块
- 固定上游版本：Caddy 2.11.7 + klzgrad/forwardproxy v2.11.7-naive2

**验证:**
```bash
# 上游类型解码测试已通过
# 真实 TLS CONNECT 转发测试已通过
# 错误/缺失凭据正确被拒绝
# 旧编码格式被拒绝（兼容性验证）
```

**文件变更:**
- `internal/proxy/naive/naive.go` - 修正认证编码
- `internal/proxy/naive/naive_test.go` - 增加上游解码测试
- `internal/agent/install_naive.go` - 强化安装检查

---

### 2. 安装检测与启动路径不一致 (P1)

**问题描述:**  
安装检测使用 `exec.LookPath()`，接受 PATH 中任意同名程序；但 systemd 启动使用固定路径 `/usr/local/bin/*`。

**影响范围:**
- NaïveProxy、AnyTLS、ShadowTLS、Shadowsocks 后端
- 误报已安装，实际服务无法启动

**修复内容:**
- 统一使用 ExecStart 固定路径进行检测
- NaïveProxy 额外检查 `http.handlers.forward_proxy` 模块
- 验证文件可执行性和必要功能

**回归测试覆盖:**
- PATH 诱饵程序不被误认
- 空文件、目录、无执行权限正确拒绝
- 模块缺失或执行失败正确检测

**文件变更:**
- `internal/agent/install_naive.go` - 固定路径 + 模块检查
- `internal/agent/install_anytls.go` - 固定路径检查
- `internal/agent/install_shadowtls.go` - 固定路径检查
- `internal/proxy/shadowsocks/shadowsocks.go` - 固定路径检查

---

### 3. ShadowTLS 托管后端端口冲突 (P1)

**问题描述:**  
新增监听端口时只检查 `port.Port`，不检查已分配的 `BackendAddr`，导致端口冲突。

**影响范围:**
- 先创建 TCP 443 的 ShadowTLS（后端 127.0.0.1:50000）
- 再创建 TCP 50000 的其他协议
- API 返回 200，但两者无法同时绑定

**修复内容:**
- 新增、修改端口、启用、备份恢复统一校验 TCP 保留端口
- 失败时原子回滚，不持久化
- UDP 可使用相同数字（不冲突）

**API 行为变更:**
- 冲突时返回 400，明确错误原因
- 备份恢复遇冲突拒绝整个恢复操作

**文件变更:**
- `internal/model/model.go` - 增加 `ValidatePortReservations()`
- `internal/manager/handlers.go` - 调用冲突校验
- `internal/manager/backup.go` - 恢复前校验
- `internal/manager/deployment_regression_test.go` - 回归测试

---

### 4. Mieru UDP 连通性检测误判 (P2)

**问题描述:**  
- 无条件探测所有 Mieru 端口的 TCP 和 UDP，不按 `port.Protocol` 区分
- UDP 检测只验证发送成功，不读取响应
- 混淆端口连通性与代理业务可用性

**影响范围:**
- UDP-only 端口误报 TCP 不可达
- TCP-only 端口浪费探测 UDP
- UDP 写成功不能证明服务存在

**修复内容:**
- 按配置协议选择传输层检测（TCP/UDP/Both）
- UDP 检测必须收到响应才标记为 reachable
- 区分三种状态：
  - 端口连通性（transport layer）
  - 代理认证（application layer）
  - Agent 服务状态

**UI 展示变更:**
- UDP-only：仅显示 UDP 状态
- TCP-only：仅显示 TCP 状态  
- Mieru UDP：标记为 "待协议客户端验证"

**文件变更:**
- `internal/manager/test.go` - 按协议区分检测
- `internal/manager/test_test.go` - UDP 回归测试
- README.md - 更新检测说明

---

### 5. 混合协议同步短路问题 (P2)

**问题描述:**  
Mieru 安装或应用失败时直接 `return`，后续其他协议循环不再执行。

**影响范围:**
- 同时提交 Mieru 和其他协议配置
- Mieru 失败导致其他协议未分发、无端口结果

**修复内容:**
- 按协议/端口收集失败，继续处理独立实例
- 汇总错误，每个未成功端口提供明确结果
- 独立协议状态不依赖 mita 安装成功

**错误报告改进:**
- 聚合多个端口错误
- 明确标识哪些成功、哪些失败
- 失败原因具体到端口级别

**文件变更:**
- `internal/agent/agent.go` - 修改同步控制流
- `internal/agent/deployment_test.go` - 混合失败回归
- `internal/agent/deployment_regression_test.go` - 完整回归套件

---

## 🛡️ 功能限制防护增强

### 流量统计限制（仅 Mieru）

**新增防护层:**

1. **API 验证层** ✅
   - 拒绝为非 Mieru 设置正数 `trafficLimit`
   - 返回 400: "当前仅 Mieru 支持流量统计和月度限额"

2. **数据处理层** ✅  
   - `updatePortTraffic()` 过滤非 Mieru 流量
   - 不参与累计、不触发超额停用

3. **前端交互层** ✅
   - 非 Mieru 点击流量管理显示 toast 提示
   - 不弹出设置窗口

4. **UI 展示层** ✅
   - 非 Mieru 显示 "暂不支持流量统计"
   - 操作菜单隐藏 "流量设置" 选项

**能力边界明确:**
- NaïveProxy、AnyTLS、ShadowTLS 不提供流量统计
- 出站规则仅适用于 Mieru
- README 和审计报告已明确标注

**文件变更:**
- `internal/manager/handlers.go:622-624` - API 拒绝
- `internal/manager/agent_api.go:282-284` - 数据过滤
- `internal/manager/web/assets/app.js:823-826` - 前端拦截
- `internal/manager/web/assets/app.js:876-879` - UI 差异化

---

## 🔧 其他改进

### IPv6 支持改进

- Manager 正确构造 IPv6 `host:port`（方括号处理）
- AnyTLS 使用通配监听 `0.0.0.0`（待 IPv6 实测）
- ShadowTLS 双栈监听策略：
  - IPv6 可用且 `net.ipv6.bindv6only=0`：使用 `::`
  - 其他情况：保留 IPv4，避免客户端回退失败

**注意:** 实际 Linux IPv6 数据链路仍需现场验收。

### UI 提示优化

- 面板提示：独立 ShadowTLS URI 不含后端 SS 凭据
- 建议优先使用 mihomo 订阅（包含完整后端信息）
- README 明确 tar/xz 依赖和架构限制

### ARM 构建文档修正

- NaïveProxy ARM 手动构建说明已纠正
- 固定 fork 依赖 `CADDY_VERSION` 和 Caddy 补丁
- 不能套用通用 `xcaddy` 命令

---

## ✅ 验证报告

### 测试覆盖

**单元测试:**
```bash
go test -count=1 ./...
# 所有测试通过
# 新增回归测试覆盖所有修复点
```

**静态分析:**
```bash
go vet ./...           # 通过
git diff --check       # 通过
gofmt 检查             # 通过
```

**前端检查:**
```bash
node scripts/check-frontend.cjs  # 通过
# 验证非 Mieru 流量展示及操作限制
```

**上游集成测试:**
```bash
# NaïveProxy 上游 CONNECT 认证测试
NAIVE_TEST_CADDY=/path/to/caddy go test -count=1 \
  -run TestUpstreamCONNECTAuthentication -v ./internal/proxy/naive
# TLS HTTP/1.1 CONNECT 转发通过
# 无凭据、错误凭据、旧编码均被拒绝
```

**交叉编译:**
```bash
# Linux amd64/arm64 Manager 和 Agent 四项交叉编译通过
# 产物验证：ELF 格式正确，架构匹配
```

### 未完成的验收

**生产环境验收待完成:**
- ❌ Linux systemd 真实部署和重启
- ❌ dpkg/rpm 包安装验证
- ❌ 公网 VPS + 域名 + ACME 签发/续期
- ❌ Mieru TCP/UDP 公网客户端请求
- ❌ NaïveProxy 完整 HTTPS 伪装
- ❌ ShadowTLS + SS 端到端链路
- ❌ IPv6 公网数据传输

**建议:** 在 Debian/Ubuntu 与 CentOS/Rocky、amd64/arm64 上完成真实部署验收。

---

## 📦 升级指南

### ⚠️ 升级要求

**必须同时升级 Manager 和 Agent:**
```bash
# 1. 停止服务
systemctl stop merit-manager merit-agent

# 2. 备份数据
cp /var/lib/merit/merit.json /var/lib/merit/merit.json.backup-$(date +%Y%m%d)

# 3. 下载 v1.5.2
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.5.2/merit-manager-linux-amd64
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.5.2/merit-agent-linux-amd64

# 4. 替换二进制
chmod +x merit-*-linux-amd64
mv merit-manager-linux-amd64 /opt/merit/merit-manager
mv merit-agent-linux-amd64 /opt/merit/merit-agent

# 5. 启动服务
systemctl start merit-manager merit-agent

# 6. 验证版本
/opt/merit/merit-manager --version
/opt/merit/merit-agent --version
```

### 升级后必做操作

1. **重试同步所有节点:**
   - 进入每个节点详情页
   - 点击所有端口的 "重试同步"

2. **刷新客户端订阅:**
   - Clash/mihomo 客户端更新订阅
   - NaïveProxy 需重新验证认证

3. **验证 NaïveProxy 节点:**
   ```bash
   # 使用客户端测试 CONNECT 请求
   # 确认认证通过且能正常转发
   ```

### 一键脚本升级

```bash
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)
# 选择 "1. 安装或更新 Manager"
```

---

## 📋 已知限制

### 功能限制（设计决策）

- **流量统计:** 仅 Mieru（其他协议无可靠指标接口）
- **月度配额:** 仅 Mieru
- **出站规则:** 仅 Mieru
- **速率限制:** mita 不支持可靠的服务端限速（配置保留但不生效）

### 架构限制

- **NaïveProxy 自动安装:** 仅 amd64（上游限制）
- **Agent 发布架构:** amd64 / arm64（无 ARMv7）
- **Manager 平台:** Linux / Windows / macOS
- **Agent 平台:** Linux with systemd

### 依赖要求

- **Mieru:** dpkg (Debian/Ubuntu) 或 rpm (CentOS/RedHat/Rocky)
- **NaïveProxy / SS:** tar + xz-utils
- **所有协议:** root 权限

---

## 🔗 相关文档

- **审计报告:** `docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md`
- **能力边界:** README.md 第 48-66 行
- **多协议指南:** `docs/QUICK_START_MULTI_PROXY.md`
- **增强功能:** `docs/ENHANCED_FEATURES.md`

---

## 🤝 贡献者

- [@HlONGlin](https://github.com/HlONGlin)
- Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>

---

## 📄 许可证

[MIT License](LICENSE)

---

## 🙏 致谢

感谢社区反馈和测试支持，帮助我们发现并修复这些关键问题。

**如果本版本解决了您的问题，欢迎给项目一个 ⭐ Star！**

---

**发布团队**  
UnifiedProxy Project  
2024-12-XX

