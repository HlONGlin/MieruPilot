# 生产加固路线图

**版本**: v1.6.0+  
**目标**: 提升多协议部署的安全性、可观测性和可靠性

---

## 🎯 优先级分级

| 级别 | 定义 | 实施窗口 |
|------|------|----------|
| **A** | 安全风险或关键体验问题 | 立即修复 |
| **B** | 生产可靠性和可观测性 | v1.6.x |
| **C** | 防御性保护和维护性 | v1.7.x |
| **D** | 工具和辅助能力 | 按需实施 |

---

## 📋 加固清单

### A 级 - 安全与关键体验

#### A1. 消除 AnyTLS 无身份校验

**当前状态**:
```yaml
问题: anytls-go 启动时生成临时自签名证书,客户端跳过证书验证
风险: 无法验证服务端身份,中间人攻击无法检测
影响: 分享链接和订阅显式设置 insecure=1 & allowInsecure=1
```

**解决方案**:

##### 方案 1: 同域名证书 (推荐)

```
步骤:
1. AnyTLS 复用 NaïveProxy 的证书和私钥
2. 前置条件:
   - 节点已配置域名
   - 已上传或 ACME 签发有效证书
3. 面板 UI 提示: "使用节点证书(需要域名和证书)"
4. 分享链接移除 insecure 参数,使用正常 TLS 验证
```

**实施要点**:
- `internal/proxy/anytls/anytls.go`: 添加 `CertFile` 和 `KeyFile` 参数
- `internal/agent/install_anytls.go`: 从 `/etc/anytls/` 读取证书,如果不存在回退到临时自签
- 前端: 证书上传/ACME 成功后提示 "AnyTLS 现在使用同域名证书"
- 分享链接: 检测到证书文件存在时移除 `insecure` 参数

##### 方案 2: 由 NaïveProxy 承载 (简化版)

```
架构:
Client → NaïveProxy (443, TLS + HTTPS CONNECT) → AnyTLS (本地端口)
                     ↓
               有效证书验证
```

**实施要点**:
- AnyTLS 只监听 `127.0.0.1:随机端口`
- NaïveProxy 配置 upstream 指向本地 AnyTLS
- 客户端只需要 NaïveProxy 配置,透明转发到 AnyTLS
- 代价: 必须同时部署 NaïveProxy (仅 amd64)

**决策矩阵**:

| 场景 | 推荐方案 |
|------|----------|
| 节点已有域名证书 | 方案 1 (同域名证书) |
| 节点没有域名 | 保持现状,面板警告"仅可信节点" |
| 需要最高安全性 | 方案 2 (NaïveProxy 承载) |

**实施计划**:
- [ ] v1.6.1: 实现方案 1 (可选启用)
- [ ] v1.6.2: 前端增加证书状态指示器
- [ ] v1.7.0: 方案 2 作为高级选项

---

#### A2. 节点部署状态可视化

**当前状态**:
```yaml
问题: 证书上传后无动画反馈,不知道是否完成
体验: 用户需要手动刷新或等待不确定的时间
```

**解决方案**:

##### 前端改进

```javascript
// 上传流程状态机
const UPLOAD_STATES = {
  IDLE: '未上传',
  UPLOADING: '上传中...',
  VALIDATING: '验证证书...',
  SYNCING: '同步到 Agent...',
  RESTARTING: '重启服务...',
  SUCCESS: '部署完成 ✅',
  FAILED: '部署失败 ❌'
}

// 实时进度展示
┌─────────────────────────────────────┐
│ 📤 上传证书                          │
│ ▰▰▰▰▰▰▰▰▰▰ 100%                    │
│                                     │
│ ✅ 证书验证通过                      │
│ ⏳ 同步到 Agent...                  │
│ ⏸  等待服务重启                     │
└─────────────────────────────────────┘
```

**实施要点**:
- WebSocket 或 Server-Sent Events 推送部署进度
- 每个阶段显示预计耗时和实际状态
- 失败时显示具体错误和修复建议

##### 后端改进

```go
// internal/manager/handlers.go
type DeploymentProgress struct {
    NodeID    string    `json:"nodeId"`
    Stage     string    `json:"stage"`     // uploading, validating, syncing, restarting
    Progress  int       `json:"progress"`  // 0-100
    Message   string    `json:"message"`
    Error     string    `json:"error,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

// 部署检测链
func (s *Server) checkDeploymentComplete(nodeID string) DeploymentStatus {
    // 1. 证书文件存在且有效
    // 2. Agent 已同步配置
    // 3. 相关服务已启动
    // 4. 健康检查通过
    return DeploymentStatus{...}
}
```

**实施计划**:
- [ ] v1.6.1: 添加 SSE 端点 `/api/nodes/:id/deploy/progress`
- [ ] v1.6.1: 前端进度条和状态动画
- [ ] v1.6.2: 完整的部署验收检查链

---

### B 级 - 可观测性

#### B1. 证书续期可观测

**当前状态**:
```yaml
功能: ACME 自动续期存在,但无监控
风险: 续期失败导致服务中断,无提前告警
```

**解决方案**:

##### 监控维度

```yaml
证书到期告警:
  - 30 天前: INFO 级别通知
  - 7 天前: WARNING 级别,面板首页横幅
  - 3 天前: ERROR 级别,WebDAV 备份描述添加告警

续期事件记录:
  - 续期尝试时间
  - 续期结果 (成功/失败)
  - 失败原因 (DNS 解析/端口占用/ACME 限流)
  - 下次续期时间

面板展示:
┌─────────────────────────────────────┐
│ 🔐 证书状态                          │
│                                     │
│ example.com                         │
│ 签发: Let's Encrypt                 │
│ 到期: 2026-12-25 (58 天)           │
│ 上次续期: 2026-10-15 ✅            │
│ 下次续期: 2026-11-25 (预计)        │
│                                     │
│ [查看续期日志] [手动续期]           │
└─────────────────────────────────────┘
```

##### 续期演练

**实施步骤**:
```bash
# 1. 缩短证书有效期触发续期
# (测试环境使用 Let's Encrypt Staging)

# 2. 模拟续期失败场景
- DNS 解析失败
- 443 端口被占用
- ACME 限流
- 磁盘空间不足

# 3. 验证告警触发
- 面板首页横幅
- WebDAV 备份描述
- (可选) Webhook 通知

# 4. 恢复并验证自动重试
```

**实施计划**:
- [ ] v1.6.1: 证书到期时间监控和告警
- [ ] v1.6.2: 续期事件日志和面板展示
- [ ] v1.6.3: 续期演练脚本和文档

---

### C 级 - 防御性保护

#### C1. 防止 Agent 版本再次漂移

**当前状态**:
```yaml
问题: Manager v1.6 + Agent v1.4 = 功能不一致
案例: 旧版 Agent 不会启动 ShadowTLS 后端,导致转发失败
```

**解决方案**:

##### 版本检查机制

```go
// internal/manager/agent_api.go
type AgentInfo struct {
    Version   string `json:"version"`   // Agent 版本
    GitCommit string `json:"gitCommit"` // Git commit hash
    BuildTime string `json:"buildTime"`
}

func (s *Server) checkAgentCompatibility(agentVersion, managerVersion string) error {
    if !isCompatible(agentVersion, managerVersion) {
        return fmt.Errorf("Agent 版本 %s 与 Manager 版本 %s 不兼容,请升级", 
            agentVersion, managerVersion)
    }
    return nil
}

// 兼容性规则
func isCompatible(agent, manager string) bool {
    // 主版本号必须一致
    // 次版本号差距不超过 1
    // 例如: 1.6.x 可以管理 1.6.x 和 1.5.x
}
```

##### 面板展示

```
节点列表:
┌──────────────────────────────────────────┐
│ 节点名称  | 地址            | 版本      │
│──────────┼─────────────────┼───────────│
│ 🟢 节点1  | 203.0.113.100   | v1.6.0 ✅ │
│ 🔴 节点2  | 203.0.113.101   | v1.4.2 ⚠️ │
│          │                 | [一键升级] │
└──────────────────────────────────────────┘

⚠️ 检测到 1 个节点版本过旧,可能导致功能不可用
```

##### 升级前快照

```bash
# 自动备份机制
upgrade_agent() {
    local node_id=$1
    local new_version=$2
    
    # 1. 创建快照
    backup_name="pre-upgrade-${node_id}-$(date +%Y%m%d-%H%M%S)"
    create_local_backup "$backup_name"
    
    # 2. 下载新版本
    download_agent "$new_version"
    
    # 3. 停止 Agent
    systemctl stop merit-agent
    
    # 4. 替换二进制
    cp /path/to/new/merit-agent /usr/local/bin/
    
    # 5. 启动并验证
    systemctl start merit-agent
    if ! check_agent_health; then
        # 回滚
        restore_from_backup "$backup_name"
        return 1
    fi
    
    return 0
}
```

**实施计划**:
- [ ] v1.6.1: Agent 版本检查和面板告警
- [ ] v1.6.2: 升级前自动快照
- [ ] v1.7.0: 一键升级 Agent 功能

---

#### C2. 锁定 Manager/Agent 同版本

**实施方案**:

##### 编译时版本嵌入

```go
// internal/buildinfo/buildinfo.go
var (
    Version   = "1.6.0"           // 从 git tag 获取
    Commit    = ""                // git rev-parse HEAD
    BuildTime = ""                // date -u
    MinAgentVersion = "1.6.0"     // Manager 要求的最低 Agent 版本
)
```

##### 握手协议

```go
// Agent 首次连接时携带版本信息
type HandshakeRequest struct {
    NodeID    string `json:"nodeId"`
    Version   string `json:"version"`
    GitCommit string `json:"gitCommit"`
}

// Manager 验证并返回兼容性
type HandshakeResponse struct {
    Accepted       bool   `json:"accepted"`
    ManagerVersion string `json:"managerVersion"`
    MinAgentVersion string `json:"minAgentVersion"`
    UpgradeURL     string `json:"upgradeUrl,omitempty"` // Agent 下载地址
}
```

**实施计划**:
- [ ] v1.6.1: 版本握手协议
- [ ] v1.6.2: 拒绝不兼容的 Agent 连接

---

### D 级 - 工具和辅助

#### D1. 升级后一键回归测试

**目标**: 把本轮手动测试固化为可重复执行的验收脚本

##### 回归测试脚本

```bash
#!/usr/bin/env bash
# scripts/acceptance/regression_test.sh

set -euo pipefail

# 测试配置
TEST_NODE_IP="203.0.113.100"
TEST_DOMAIN="example.com"

# 7 端口探测 (来自你的实测)
declare -A EXPECTED_PORTS=(
    ["37065"]="Mieru TCP"
    ["48902"]="Mieru TCP"
    ["50900"]="Mieru TCP"
    ["40590"]="Mieru UDP"
    ["23112"]="AnyTLS"
    ["40900"]="ShadowTLS v3"
    ["443"]="NaïveProxy"
)

echo "🔍 开始回归测试..."
echo "目标节点: $TEST_NODE_IP"
echo ""

# 1. 端口连通性测试
echo "1️⃣ 端口连通性测试"
for port in "${!EXPECTED_PORTS[@]}"; do
    protocol="${EXPECTED_PORTS[$port]}"
    
    if [[ $port == "40590" ]]; then
        # UDP 测试
        if nc -u -z -w3 "$TEST_NODE_IP" "$port" 2>/dev/null; then
            echo "  ✅ $port (UDP) - $protocol"
        else
            echo "  ❌ $port (UDP) - $protocol 无响应"
        fi
    else
        # TCP 测试
        if nc -z -w3 "$TEST_NODE_IP" "$port" 2>/dev/null; then
            echo "  ✅ $port (TCP) - $protocol"
        else
            echo "  ❌ $port (TCP) - $protocol 无响应"
        fi
    fi
done

echo ""

# 2. Mieru TCP 转发测试
echo "2️⃣ Mieru TCP HTTPS 转发测试"
for port in 37065 48902 50900; do
    # 使用 mieru 客户端发起 HTTPS 请求
    if timeout 10 mieru-test-client --port "$port" --target https://www.google.com > /dev/null 2>&1; then
        echo "  ✅ Mieru TCP ($port) - HTTPS 转发通过"
    else
        echo "  ❌ Mieru TCP ($port) - HTTPS 转发失败"
    fi
done

echo ""

# 3. Mieru UDP 转发测试
echo "3️⃣ Mieru UDP 传输测试"
if timeout 10 mieru-test-client --port 40590 --protocol udp --target https://www.google.com > /dev/null 2>&1; then
    echo "  ✅ Mieru UDP (40590) - UDP 传输承载 HTTPS 请求通过"
else
    echo "  ❌ Mieru UDP (40590) - UDP 传输失败"
fi

echo ""

# 4. AnyTLS 认证测试
echo "4️⃣ AnyTLS 认证测试"
# 正确密码
if test_anytls_proxy "$TEST_NODE_IP" 23112 "$CORRECT_PASSWORD"; then
    echo "  ✅ AnyTLS (23112) - 正确密码转发通过"
else
    echo "  ❌ AnyTLS (23112) - 正确密码转发失败"
fi

# 错误密码
if ! test_anytls_proxy "$TEST_NODE_IP" 23112 "wrong_password"; then
    echo "  ✅ AnyTLS (23112) - 错误密码被正确拒绝"
else
    echo "  ❌ AnyTLS (23112) - 错误密码未被拒绝"
fi

echo ""

# 5. ShadowTLS 完整链路测试
echo "5️⃣ ShadowTLS v3 完整链路测试"
if test_shadowtls_with_ss_backend "$TEST_NODE_IP" 40900; then
    echo "  ✅ ShadowTLS (40900) - 完整订阅及托管 SS 后端转发通过"
else
    echo "  ❌ ShadowTLS (40900) - 转发失败"
fi

echo ""

# 6. NaïveProxy 认证测试
echo "6️⃣ NaïveProxy 认证测试"
# 正确凭据
if test_naive_proxy "$TEST_NODE_IP" 443 "$CORRECT_USER" "$CORRECT_PASS"; then
    echo "  ✅ NaïveProxy (443) - 正确凭据转发通过"
else
    echo "  ❌ NaïveProxy (443) - 正确凭据转发失败"
fi

# 错误密码
if ! test_naive_proxy "$TEST_NODE_IP" 443 "$CORRECT_USER" "wrong_password"; then
    echo "  ✅ NaïveProxy (443) - 错误密码被正确拒绝"
else
    echo "  ❌ NaïveProxy (443) - 错误密码未被拒绝"
fi

# 错误凭据后恢复
if test_naive_proxy "$TEST_NODE_IP" 443 "$CORRECT_USER" "$CORRECT_PASS"; then
    echo "  ✅ NaïveProxy (443) - 错误密码失败后恢复正确凭据成功"
else
    echo "  ❌ NaïveProxy (443) - 恢复失败"
fi

echo ""

# 7. 订阅链接生成测试
echo "7️⃣ 订阅链接生成测试"
SUBSCRIPTION_URL="http://manager-ip:8080/api/clash/subscribe?token=xxx"

if curl -sS "$SUBSCRIPTION_URL" | grep -q "proxies:"; then
    echo "  ✅ Clash 订阅生成成功"
    
    # 检查各协议是否包含在订阅中
    subscription_content=$(curl -sS "$SUBSCRIPTION_URL")
    
    if echo "$subscription_content" | grep -q "type: mieru"; then
        echo "    ✅ 包含 Mieru 配置"
    fi
    
    if echo "$subscription_content" | grep -q "type: shadowtls"; then
        echo "    ✅ 包含 ShadowTLS 配置"
    fi
    
    # NaïveProxy 不应该在 Clash 订阅中
    if ! echo "$subscription_content" | grep -q "type: naive"; then
        echo "    ✅ 正确排除 NaïveProxy"
    fi
else
    echo "  ❌ Clash 订阅生成失败"
fi

echo ""
echo "✅ 回归测试完成!"
```

##### 保险箱脚本 (可随时复跑)

```bash
# scripts/acceptance/quick_check.sh
# 快速验收 - 5 分钟内完成

#!/usr/bin/env bash

# 从配置文件读取测试节点
source ./test_config.env

# 快速检查
check_port_open() { nc -z -w2 "$1" "$2" 2>/dev/null; }
check_https_proxy() { 
    curl -x "$1" --connect-timeout 5 --max-time 10 https://www.google.com > /dev/null 2>&1
}

PASS=0
FAIL=0

# Mieru TCP
for port in 37065 48902 50900; do
    if check_port_open "$NODE_IP" "$port"; then
        echo "✅ Mieru TCP:$port"
        ((PASS++))
    else
        echo "❌ Mieru TCP:$port"
        ((FAIL++))
    fi
done

# AnyTLS
if check_https_proxy "anytls://$ANYTLS_PASSWORD@$NODE_IP:23112"; then
    echo "✅ AnyTLS:23112"
    ((PASS++))
else
    echo "❌ AnyTLS:23112"
    ((FAIL++))
fi

# ShadowTLS
if check_https_proxy "shadowtls://...@$NODE_IP:40900"; then
    echo "✅ ShadowTLS:40900"
    ((PASS++))
else
    echo "❌ ShadowTLS:40900"
    ((FAIL++))
fi

# NaïveProxy
if check_https_proxy "https://$NAIVE_USER:$NAIVE_PASS@$NODE_IP:443"; then
    echo "✅ NaïveProxy:443"
    ((PASS++))
else
    echo "❌ NaïveProxy:443"
    ((FAIL++))
fi

echo ""
echo "通过: $PASS, 失败: $FAIL"

if [ $FAIL -eq 0 ]; then
    echo "🎉 全部测试通过!"
    exit 0
else
    echo "⚠️  存在失败项,请检查"
    exit 1
fi
```

**使用方式**:

```bash
# 完整回归测试 (首次验收或重大升级)
./scripts/acceptance/regression_test.sh

# 快速检查 (日常或小版本升级)
./scripts/acceptance/quick_check.sh

# 配置文件示例 (test_config.env)
NODE_IP="203.0.113.100"
ANYTLS_PASSWORD="your_password"
NAIVE_USER="your_user"
NAIVE_PASS="your_pass"
```

**实施计划**:
- [ ] v1.6.1: 创建基础回归测试脚本
- [ ] v1.6.2: 完善所有协议的测试覆盖
- [ ] v1.7.0: 集成到 CI/CD 流程

---

## 📊 实施时间线

```
v1.6.1 (2 周)
├─ A1. AnyTLS 同域名证书支持
├─ A2. 部署状态可视化 (SSE + 进度条)
├─ B1. 证书到期监控
├─ C1. Agent 版本检查
└─ D1. 基础回归测试脚本

v1.6.2 (2 周)
├─ A2. 完整部署验收检查链
├─ B1. 续期事件日志和面板展示
├─ C1. 升级前自动快照
└─ D1. 完善协议测试覆盖

v1.6.3 (1 周)
├─ B1. 续期演练文档
└─ 发布稳定版

v1.7.0 (长期)
├─ A1. NaïveProxy 承载 AnyTLS (高级选项)
├─ C1. 一键升级 Agent
└─ C2. 强制版本锁定
```

---

## 🔧 开发指南

### 本地开发环境

```bash
# 1. 安装依赖
go install github.com/cosmtrek/air@latest  # 热重载
npm install -g nodemon                     # 前端热重载

# 2. 启动开发模式
./scripts/dev.sh

# 3. 运行测试
go test -count=1 ./...
node scripts/check-frontend.cjs
```

### 测试清单

每个 PR 必须通过:
- [ ] `go test -count=1 ./...`
- [ ] `go vet ./...`
- [ ] `node scripts/check-frontend.cjs`
- [ ] 相关的回归测试脚本
- [ ] 手动验收 (如果涉及协议部署)

---

## 📚 相关文档

- [协议部署复核](./PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md)
- [订阅链接修复](./SUBSCRIPTION_LINK_FIX.md)
- [WebDAV 备份](./WEBDAV_BACKUP.md)
- [多协议快速开始](./QUICK_START_MULTI_PROXY.md)
- [增强功能](./ENHANCED_FEATURES.md)

---

## 🤝 贡献指南

欢迎提交:
- 🐛 Bug 修复
- ✨ 新功能提案
- 📝 文档改进
- 🧪 测试覆盖

请参考本文档的优先级分级,优先处理 A/B 级项目。

---

**最后更新**: 2026-10-27  
**维护者**: holi
