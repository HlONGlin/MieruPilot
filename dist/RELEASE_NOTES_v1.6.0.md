# UnifiedProxy v1.6.0 - 部署可靠性与可观测性增强版本 🔍

> 依赖预检、证书生命周期可见、生产验收脚本

**发布日期:** 2026-10-27  
**版本类型:** 功能版本（Feature Release）

---

## 🎯 核心改进

**本版本新增三项关键改进，提升部署可靠性和可观测性:**

- ✅ 统一依赖预检（协议/端口作用域，提前发现问题）
- ✅ 证书生命周期可见（三级预警、ACME 状态、staging 开关）
- ✅ 生产验收脚本（三态结论、结构化测试、JSON 输出）
- ✅ 端口占用判定优化（基于 systemd MainPID 身份验证）

---

## ✨ 新增功能

### 1. 统一依赖预检 (Unified Preflight Checks)

**功能描述:**  
在协议部署前自动检查系统依赖，按协议和端口作用域精准校验。

**检查项目:**
- ✅ Root 权限验证
- ✅ 架构支持（amd64/arm64）
- ✅ 包管理器（Mieru 需要 dpkg 或 rpm）
- ✅ 解压工具（NaïveProxy/SS 需要 tar + xz）
- ✅ 端口可用性（按 systemd MainPID 判定身份）

**提前报错:**
```
缺少依赖: xz 解压能力
安装方法:
  Debian/Ubuntu: apt-get install xz-utils
  RHEL/CentOS/Rocky: yum install xz
  Alpine: apk add xz
```

**优势:**
- 失败提前至同步前，不产生半成品配置
- 按协议差异化检查，避免误报
- 三大发行版系（Debian/RHEL/Alpine）安装命令开箱可用

**文件变更:**
- `internal/agent/preflight.go` - 预检框架
- `internal/agent/preflight_test.go` - 单元测试
- `internal/agent/agent.go` - 集成到同步流程

---

### 2. 证书生命周期可见性 (Certificate Lifecycle Visibility)

**功能描述:**  
NaïveProxy 证书状态全生命周期可观测。

**证书过期预警（三级）:**
| 剩余天数 | 警告级别 | 展示样式 |
|---------|---------|---------|
| > 30 天 | valid | 正常 |
| 7-30 天 | warning | 黄色警告 |
| 0-7 天 | critical | 红色严重 |
| < 0 天 | expired | 红色过期 |

**ACME 状态追踪:**
- 最后成功签发时间
- 续期计数（日志窗口内）
- 失败原因（error、rateLimited、pending 等）
- staging 模式开关状态

**Per-Port Staging 开关:**
```
POST /api/ports/:id/acme-staging
{
  "enabled": true  // 切换为 Let's Encrypt staging 环境
}
```

**适用场景:**
- 测试 ACME 流程避免触发 rate limit
- 验证域名配置和 DNS 解析
- 生产签发前的预演

**文件变更:**
- `internal/manager/handlers.go` - 过期分级逻辑、ACME 状态 API
- `internal/manager/web/assets/app.js` - 前端证书状态展示

---

### 3. 生产验收脚本 (Production Acceptance Tests)

**功能描述:**  
结构化测试套件，验证 UnifiedProxy 在 Linux + systemd 环境的部署完整性。

**测试架构:**
```
scripts/acceptance/
├── run-acceptance.sh          # 主脚本
├── checks/
│   ├── 01-systemd-units.sh    # systemd 单元文件
│   ├── 02-running-status.sh   # 运行状态
│   ├── 03-port-binding.sh     # 端口绑定
│   ├── 04-identity.sh         # 身份一致性
│   ├── 05-config-files.sh     # 配置文件
│   ├── 06-restart.sh          # 重启测试（需开关）
│   └── 07-acme.sh             # ACME 签发（需开关）
└── lib/common.sh              # 共享库
```

**使用方法:**
```bash
# 默认只读检查（生产安全）
./scripts/acceptance/run-acceptance.sh

# 包含主动测试
./scripts/acceptance/run-acceptance.sh --with-restart --with-acme-issuance

# 指定输出文件
./scripts/acceptance/run-acceptance.sh --output report.json --log test.log
```

**三态结论:**
| 退出码 | 状态 | 含义 |
|-------|------|------|
| 0 | PASS | 全部检查通过 |
| 1 | FAIL | 至少一项失败 |
| 2 | INCONCLUSIVE | 无检查被执行（未部署/权限不足） |

**JSON 输出:**
```json
{
  "overall": "pass",
  "summary": {"pass": 12, "fail": 0, "skip": 2},
  "checks": [
    {
      "name": "systemd_manager_exists",
      "result": "PASS",
      "message": "Unit file exists: merit-manager.service"
    }
  ],
  "timestamp": "2026-10-27T10:30:00+00:00"
}
```

**文件变更:**
- `scripts/acceptance/run-acceptance.sh` - 主控脚本
- `scripts/acceptance/checks/*.sh` - 7 项检查
- `scripts/acceptance/lib/common.sh` - 工具库
- `README.md` - 使用说明

---

## 🔧 优化改进

### 端口占用判定优化

**变更内容:**  
改用 systemd 单元 MainPID 判定端口占用者身份，替代原有的进程名匹配。

**判定逻辑:**
```go
func checkPortAvailability(port int, protocol string, unitName string) error {
    pid := getPIDOccupyingPort(port, protocol)
    if pid == 0 {
        return nil // 端口未被占用
    }
    
    unitPID := getUnitMainPID(unitName)
    if pid == unitPID {
        return nil // 自己的进程占用,允许
    }
    
    return fmt.Errorf("端口被其他进程占用: PID %d", pid)
}
```

**优势:**
- 避免进程名伪装误判
- 支持多实例精准识别
- systemd 重启后 PID 变化自动适配

**文件变更:**
- `internal/agent/preflight.go:197-254` - MainPID 身份验证
- `internal/agent/preflight_test.go` - 回归测试

---

## ✅ 验证报告

### 测试覆盖

**单元测试:**
```bash
go test ./...
# 全部通过
# 新增 preflight、acceptance 测试覆盖
```

**静态分析:**
```bash
go vet ./...                           # 通过
shellcheck scripts/acceptance/*.sh     # 通过
node scripts/check-frontend.cjs        # 通过
```

**验收脚本本地测试:**
```bash
./scripts/acceptance/run-acceptance.sh
# 退出码 2 (INCONCLUSIVE)
# JSON 输出合法,包含 overall 字段
# 未部署环境正确识别
```

### 生产环境验收

**验收脚本已就绪,待用户在真实 Linux/VPS 环境运行反馈。**

以下四项原始验收仍为待完成:
- ❌ Linux systemd 真实部署和重启
- ❌ 公网 VPS + 域名 + ACME 签发/长期续期
- ❌ IPv6 公网数据传输链路
- ❌ MainPID 身份一致性在真机验证（由验收脚本 `04-identity.sh` 检查项覆盖）

**建议:** 在 Debian/Ubuntu 和 CentOS/Rocky、amd64/arm64 组合上运行验收脚本并反馈结果。

---

## 📦 升级指南

### 升级要求

**必须同时升级 Manager 和 Agent:**
```bash
# 1. 停止服务
systemctl stop merit-manager merit-agent

# 2. 备份数据
cp /var/lib/merit/merit.json /var/lib/merit/merit.json.backup-$(date +%Y%m%d)

# 3. 下载 v1.6.0
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.6.0/merit-manager-linux-amd64
wget https://github.com/HlONGlin/UnifiedProxy/releases/download/v1.6.0/merit-agent-linux-amd64

# 4. 替换二进制
chmod +x merit-*-linux-amd64
mv merit-manager-linux-amd64 /opt/merit/merit-manager
mv merit-agent-linux-amd64 /opt/merit/merit-agent

# 5. 启动服务
systemctl start merit-manager merit-agent

# 6. 验证版本
/opt/merit/merit-manager --version  # 应显示 v1.6.0
/opt/merit/merit-agent --version    # 应显示 v1.6.0
```

### 升级后操作

1. **运行验收脚本（可选但推荐）:**
   ```bash
   curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/v1.6.0/scripts/acceptance/run-acceptance.sh | bash
   ```

2. **验证证书状态:**
   - 进入 Manager Web UI
   - 查看 NaïveProxy 端口证书剩余天数
   - 确认 ACME 状态无异常

3. **测试部署（可选）:**
   ```bash
   # 带重启和 ACME 测试（谨慎,会中断服务）
   ./scripts/acceptance/run-acceptance.sh --with-restart --with-acme-issuance
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

- **NaïveProxy 自动安装:** 仅 amd64（上游限制,arm64 需手动构建）
- **Agent 发布架构:** amd64 / arm64（无 ARMv7）
- **Manager 平台:** Linux / Windows / macOS
- **Agent 平台:** Linux with systemd

### 依赖要求

- **Mieru:** dpkg (Debian/Ubuntu) 或 rpm (RHEL/CentOS/Rocky)
- **NaïveProxy / Shadowsocks:** tar + xz-utils / xz
- **所有协议:** root 权限

### ACME 限制

- **renewalCount 字段:** 基于日志查询窗口的计数,非终身累计（取决于日志保留和轮转策略）
- **Staging 切换:** 切换模式会触发重新签发,旧证书立即失效

---

## 🔗 相关文档

- **v1.5.2 审计报告:** `docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md`
- **能力边界:** README.md 第 48-66 行
- **验收脚本文档:** `scripts/acceptance/README.md`（如存在）
- **CHANGELOG:** `CHANGELOG.md`

---

## 🤝 贡献者

- [@HlONGlin](https://github.com/HlONGlin)
- Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>

---

## 📄 许可证

[MIT License](LICENSE)

---

## 🙏 致谢

感谢社区在生产环境中的持续反馈,帮助我们不断提升部署可靠性和可观测性。

**如果本版本对您有帮助,欢迎给项目一个 ⭐ Star！**

---

**发布团队**  
UnifiedProxy Project  
2026-10-27
