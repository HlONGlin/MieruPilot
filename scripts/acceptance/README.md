# MeritPilot Production Acceptance Test Suite

生产环境验收测试脚本，验证部署在生产 VPS 上的 MeritPilot 服务是否正常运行。

## 🛡️ 安全特性

**默认模式：完全只读，零副作用**

- ✅ 不会重启任何服务
- ✅ 不会触发证书签发
- ✅ 不会修改任何配置
- ✅ 不会占用端口或资源

可以放心在生产环境运行默认命令。

## 使用方法

### 基础用法（只读模式）

```bash
cd scripts/acceptance
bash run-acceptance.sh
```

生成两个文件：
- `acceptance_YYYYMMDD_HHMMSS.log` - 详细执行日志
- `acceptance_YYYYMMDD_HHMMSS.json` - 结构化测试结果

### 主动测试（需显式启用）

⚠️ **警告：以下选项有副作用，仅在测试环境或明确需要时使用**

#### 测试服务重启
```bash
bash run-acceptance.sh --with-restart
```

**风险**：
- 每个被测服务将短暂重启（~5 秒）
- 正在连接的客户端会断开
- 不会丢失配置或数据

**适用场景**：
- 验证 systemd 自动重启配置
- 测试服务恢复速度
- 非高峰时段的主动测试

#### 测试 ACME 证书签发
```bash
bash run-acceptance.sh --with-acme-issuance
```

**风险**：
- 可能触发真实的 Let's Encrypt 证书签发（优先使用 staging）
- 频繁签发会触发速率限制（50 证书/域名/周）
- staging 证书不受浏览器信任，需手动切换回 production

**适用场景**：
- 首次部署验证 ACME 流程
- 测试域名 DNS 配置
- 测试防火墙 443 端口放行

#### 组合使用
```bash
bash run-acceptance.sh --with-restart --with-acme-issuance
```

### 自定义输出路径
```bash
bash run-acceptance.sh --log /path/to/custom.log --output /path/to/result.json
```

## 检查项目

### 1. Systemd Units (`01_systemd.sh`)
- ✅ 所有 MeritPilot 服务单元已激活（`is-active: active`）
- ✅ 所有服务单元已启用（`is-enabled: enabled`，开机自动启动）
- ✅ 所有服务有有效的 MainPID

### 2. PID 一致性 (`02_pid_consistency.sh`)
- ✅ 实际监听端口的进程 PID == systemd 报告的 MainPID
- 🎯 **任务1 残余风险的真机验证**：确认 preflight 端口占用检查逻辑正确

**原理**：
- 读取 unit 文件中的 `#MeritPort=<port>` 声明
- 通过 `ss -tlnp` 或 `netstat -tlnp` 查找实际监听该端口的进程 PID
- 对比 `systemctl show <unit> -p MainPID`
- **不一致 = FAIL**，记录两个 PID 和端口号

### 3. Mieru (`03_mieru.sh`)
- ✅ TCP 监听在位
- ✅ UDP 监听在位
- ⏸️ 端到端测试：有客户端时执行，无客户端时 SKIP

### 4. NaïveProxy (`04_naiveproxy.sh`)
- ✅ Caddy 进程存在且 PID 正确
- ✅ 证书文件存在
- ✅ 证书剩余天数检查：
  - `< 30 天` → ⚠️ WARNING（不算 FAIL）
  - `< 7 天` → ⚠️ CRITICAL（不算 FAIL）
  - `已过期` → ❌ FAIL
- ✅ Journal 日志中近 7 天无 ACME 失败事件（有失败 → WARNING）

### 5. ShadowTLS (`05_shadowtls.sh`)
- ✅ shadow-tls unit 在位且监听前端端口
- ✅ shadowsocks-backend unit 在位且监听后端端口（127.0.0.1）
- ✅ 两个 unit 都处于 active 状态

### 6. IPv6 (`06_ipv6.sh`)
- ✅ 系统无 IPv6 地址时整项 SKIP
- ✅ 有 IPv6 时检查各服务的 IPv6 监听：
  - **Mieru**: 期望有 IPv6 监听
  - **NaïveProxy**: 期望有 IPv6 监听
  - **AnyTLS**: 当前仅 IPv4（0.0.0.0），不视为 FAIL
  - **ShadowTLS**: 取决于内核 `net.ipv6.bindv6only` 参数

### 7. ACME（默认只读）(`07_acme.sh`)
- ✅ 域名 DNS 解析检查
- ✅ 证书文件存在性检查
- ✅ 证书有效期检查（同 NaïveProxy 的到期分级）
- ✅ Journal 中 ACME 事件历史（成功/失败计数，最近 30 天）
- ⚠️ 不触发新签发（除非 `--with-acme-issuance`）

## 结果判定

### 状态码
- `PASS` - 检查通过
- `FAIL` - 检查失败，会导致整体退出码为 1
- `SKIP` - 检查跳过（前置条件不满足），不视为失败

### 退出码
- `0` - 所有检查通过或跳过
- `1` - 至少一个检查失败

### 示例输出

#### 终端输出
```
=== MeritPilot Production Acceptance Test ===
Start time: 2026-10-27T18:30:00+08:00
Log file: scripts/acceptance/acceptance_20261027_183000.log
Result JSON: scripts/acceptance/acceptance_20261027_183000.json

Test modes:
  ✓  Service restart tests: DISABLED (read-only)
  ✓  ACME issuance tests: DISABLED (read-only)

Running check: 01_systemd
  ✓ PASS: 01_systemd

Running check: 02_pid_consistency
  ✓ PASS: 02_pid_consistency

Running check: 03_mieru
  ✓ PASS: 03_mieru

Running check: 04_naiveproxy
  ✓ PASS: 04_naiveproxy

Running check: 05_shadowtls
  - SKIP: 05_shadowtls

Running check: 06_ipv6
  - SKIP: 06_ipv6

Running check: 07_acme
  ✓ PASS: 07_acme

=== Summary ===
Total checks: 7
Passed: 5
Failed: 0
Skipped: 2

✓ Acceptance test PASSED (or skipped)
```

#### JSON 输出（`acceptance_*.json`）
```json
{
  "timestamp": "2026-10-27T18:30:15+08:00",
  "summary": {
    "total": 7,
    "passed": 5,
    "failed": 0,
    "skipped": 2
  },
  "modes": {
    "with_restart": false,
    "with_acme_issuance": false
  },
  "checks": [
    {
      "name": "01_systemd",
      "status": "PASS",
      "message": "All 8 units are active, enabled, and have valid MainPID",
      "details": "Found 8 units:\n  - merit-mita-abc123\n  - merit-naive-def456\n..."
    },
    {
      "name": "02_pid_consistency",
      "status": "PASS",
      "message": "All units have consistent PIDs",
      "details": "..."
    }
  ]
}
```

JSON 文件可以被其他工具解析，用于自动化验收流程。

## 自动发现机制

### Unit 发现

脚本自动扫描 `/etc/systemd/system/` 下的服务单元：

**已知前缀列表**（必须与 Go 代码同步）：
- `merit-mita-*` - Mieru 协议
- `merit-naive-*` - NaïveProxy（新前缀）
- `naive-proxy-*` - NaïveProxy（旧前缀，兼容存量部署）
- `merit-anytls-*` - AnyTLS 协议
- `merit-shadowtls-*` - ShadowTLS 前端
- `shadowsocks-backend-*` - Shadowsocks 后端（ShadowTLS 使用）

**同步要求**：
此列表必须与 Go 代码保持同步：
- Go 侧定义：`internal/agent/instances_proxy.go:proxyServiceName()`
- Shell 侧使用：各 `checks/*.sh` 脚本

如 Go 侧新增协议或修改前缀，必须同步更新：
1. `run-acceptance.sh` 中的注释
2. `checks/01_systemd.sh` 的 `prefixes` 数组
3. `checks/02_pid_consistency.sh` 的 `prefixes` 数组
4. 其他相关检查脚本
5. 本 README 的前缀列表

### 端口提取

端口号从 unit 文件的注释中提取：
```ini
#MeritPort=443
[Unit]
Description=...
```

格式由 `internal/agent/instances_proxy.go:writeProxyUnit()` 生成，shell 侧通过以下方式读取：
```bash
grep -oP '^#MeritPort=\K\d+' /etc/systemd/system/<unit>.service
```

### 环境变量覆盖

如需手动指定额外的单元或端口（用于测试或特殊场景）：

```bash
# 补充单元列表（不影响自动发现）
export EXTRA_UNITS="custom-proxy-123 custom-proxy-456"

# 补充端口映射（格式：unit:port 空格分隔）
export EXTRA_PORTS="custom-proxy-123:8443 custom-proxy-456:8444"

bash run-acceptance.sh
```

**注意**：环境变量仅作补充，不会禁用自动发现。

## 开发与维护

### 添加新检查

1. 在 `checks/` 目录创建 `XX_checkname.sh`（编号递增）
2. 实现 `run_check()` 函数，输出格式：
   ```bash
   echo "STATUS: PASS|FAIL|SKIP"
   echo "One-line summary message"
   echo "Detailed output..."
   ```
3. 返回退出码：`0` = PASS, `1` = FAIL, `2` = SKIP
4. 在 `run-acceptance.sh` 的 `CHECK_SCRIPTS` 数组中添加新脚本路径

### 测试 JSON 转义

主脚本包含 `json_escape()` 函数和自测逻辑，用于处理：
- 反斜杠 `\`
- 双引号 `"`
- 换行符 `\n`
- Tab 符 `\t`
- 回车符 `\r`

测试用例：
```bash
# 包含引号、反斜杠、换行的错误信息
Error: "failed" with \backslash and
newline
```

脚本启动时自动运行 `test_json_escape()`，失败则退出。

### Shell 脚本校验

```bash
# 语法检查
bash -n scripts/acceptance/run-acceptance.sh
bash -n scripts/acceptance/checks/*.sh

# ShellCheck（如果安装）
shellcheck scripts/acceptance/run-acceptance.sh
shellcheck scripts/acceptance/checks/*.sh
```

在提交前必须通过语法检查，ShellCheck 告警应尽量清除。

## 常见问题

### Q: 为什么默认不测试服务重启？
A: 生产环境中重启服务有风险（客户端断连），必须由运维人员显式决定。

### Q: 为什么默认不测试 ACME 签发？
A: Let's Encrypt 有速率限制（50 证书/域名/周），频繁签发会被限流。且签发会占用 443 端口，可能影响生产流量。

### Q: SKIP 算不算失败？
A: 不算。SKIP 表示前置条件不满足（如无 IPv6、无客户端、无证书等），不影响退出码。

### Q: 如何在 CI/CD 中使用？
A: 
```bash
# 在部署流水线中执行
bash scripts/acceptance/run-acceptance.sh

# 检查退出码
if [ $? -eq 0 ]; then
  echo "✓ Acceptance passed"
else
  echo "✗ Acceptance failed"
  exit 1
fi

# 上传结果文件作为制品
# acceptance_*.log
# acceptance_*.json
```

### Q: PID 一致性检查失败怎么办？
A: 这是严重问题，说明 systemd 记录的 MainPID 与实际监听端口的进程 PID 不一致。可能原因：
- 服务启动脚本有问题（fork 了子进程）
- ExecStart 配置错误（未使用 `Type=simple`）
- 进程被外部信号杀死后重启，但 systemd 未更新 PID
- **最严重**：另一个进程占用了该端口（preflight 检查失效）

排查步骤：
1. 查看日志中记录的两个 PID 和端口号
2. `ps -p <MainPID>` 和 `ps -p <actual_PID>` 查看进程详情
3. `ss -tlnp | grep <port>` 确认监听进程
4. `systemctl status <unit>` 查看 systemd 状态
5. 如确认是 preflight 逻辑问题，立即上报（这是任务1 的关键验证项）

### Q: 证书 WARNING/CRITICAL 算不算失败？
A: 不算 FAIL，但会在日志和 JSON 中记录为 warning。只有 **已过期** 的证书才算 FAIL。

建议：
- WARNING (<30 天): 关注续期情况
- CRITICAL (<7 天): 尽快排查为何未自动续期
- EXPIRED: 立即修复

## 任务追溯

本验收脚本来自 **任务3：生产验收脚本化**，响应以下需求：

- **任务1 验证**：PID 一致性检查（`02_pid_consistency.sh`）验证 preflight 端口占用逻辑
- **任务2 复用**：证书到期分级口径（<30d warning, <7d critical, 已过期 fail）
- **只读原则**：默认零副作用，主动测试需显式开关
- **自动发现**：扫描 systemd 目录，减少手工配置
- **结果格式**：日志 + JSON，支持人工阅读和自动化解析

---

**维护者**: MeritPilot Team  
**最后更新**: 2026-10-27  
**相关文档**:
- `docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md` - 协议部署审计
- `docs/NAIVE_ACME_LOG_AUDIT.md` - ACME 日志来源审计
- `internal/agent/instances_proxy.go` - Go 侧 unit 名称构造逻辑
