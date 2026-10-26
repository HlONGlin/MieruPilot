# 多协议支持 - 问题修复

## 修复日期
2026-10-26

## 修复内容

### 1. 配色优化 ✅

**问题**: 界面配色过于亮眼（cyan #22d3ee, orange #f97316）

**修复**: 
- Primary 色由 `#22d3ee` 改为 `#0891b2` (深青色)
- Accent 色由 `#f97316` 改为 `#dc2626` (深红色)
- 其他辅助色也相应降低亮度

**影响文件**:
- `internal/manager/web/assets/style.css`

**效果**: 深色和浅色主题都使用更柔和、专业的配色

---

### 2. AnyTLS 出口连接问题修复 ✅

**问题**: AnyTLS 入口能 ping 通但出口无法建立连接

**原因分析**:
1. 监听地址使用 `:<端口>` 而不是 `0.0.0.0:<端口>`
2. Protocol 字段错误使用（混淆了代理类型和网络协议）
3. 缺少日志输出无法诊断问题

**修复**:
- 监听地址改为 `0.0.0.0:<端口>` 确保所有网卡监听
- 移除错误的 `--protocol` 参数
- 使用正确的 AnyTLS 命令行参数格式：
  ```bash
  anytls server -l 0.0.0.0:<端口> -p <密码>
  ```
- 添加 systemd 日志输出 (`StandardOutput=journal`, `StandardError=journal`)

**影响文件**:
- `internal/proxy/anytls/anytls.go`
- `internal/agent/instances_proxy.go`

**调试方法**:
```bash
# 查看服务日志
journalctl -u anytls-<port-id> -f

# 检查端口监听
ss -tlnp | grep <端口>

# 手动测试
/usr/local/bin/anytls server -l 0.0.0.0:<端口> -p <密码>
```

---

### 3. ShadowTLS v2rayN 兼容性说明 ✅

**问题**: ShadowTLS 生成的链接无法导入 v2rayN

**原因**: v2rayN **不原生支持** ShadowTLS v3 协议

**解决方案**:

#### 方案 1: 使用 Clash.Meta（推荐）
```yaml
proxies:
  - name: "ShadowTLS-Server"
    type: shadowtls
    server: <服务器IP>
    port: <端口>
    password: "<密码>"
    version: 3
    sni: cloudflare.com
```

#### 方案 2: 使用 ShadowTLS 专用客户端
```json
{
  "server": "<服务器IP>",
  "server_port": <端口>,
  "password": "<密码>",
  "sni": "cloudflare.com",
  "version": 3,
  "local_addr": "127.0.0.1:1080"
}
```

#### 方案 3: v2rayN + 插件（需要第三方支持）

**链接格式更新**:
- 原格式: `shadowtls://<密码>@<host>:<端口>?handshake=<握手地址>`
- 新格式: `shadowtls://<host>:<端口>?password=<密码>&handshake=<握手地址>&version=3#ShadowTLS-v3`
- 更明确的参数结构，便于手动配置

**影响文件**:
- `internal/proxy/shadowtls/shadowtls.go`

**支持客户端表**:
| 客户端 | 平台 | ShadowTLS v3 |
|--------|------|--------------|
| Clash.Meta | All | ✅ 原生支持 |
| shadowtls-client | All | ✅ 专用客户端 |
| NekoBox | Android | ✅ 原生支持 |
| v2rayN | Windows | ⚠️ 需要插件 |

---

### 4. 日志增强 ✅

为所有代理服务添加 systemd 日志支持：

```ini
[Service]
StandardOutput=journal
StandardError=journal
```

**影响的代理**:
- NaïveProxy
- AnyTLS
- ShadowTLS

**查看日志**:
```bash
# 单个服务
journalctl -u <service-name> -f

# 所有代理服务
journalctl -u 'naive-proxy-*' -u 'anytls-*' -u 'shadowtls-*' -f
```

---

## 新增文档

### 故障排查指南
新增 `docs/TROUBLESHOOTING.md` 包含：

1. **AnyTLS 调试**
   - 服务状态检查
   - 端口监听验证
   - 网络连通性测试
   - 常见问题解决

2. **ShadowTLS 客户端配置**
   - 各平台支持情况
   - 推荐客户端
   - 手动配置方法

3. **NaïveProxy 证书问题**
   - ACME 自动签发失败排查
   - 手动证书上传指南

4. **通用调试命令**
   - 批量服务管理
   - 网络诊断
   - 性能分析

---

## 测试验证

所有单元测试通过 ✅
```bash
$ go test -v ./internal/proxy/...
PASS: TestGetProxyType
PASS: TestValidatePort
PASS: TestGenerateShareLink (新链接格式)
PASS: TestSupportedProxyTypes
```

---

## 部署注意事项

### 已有 AnyTLS 节点需要重新部署

1. 停止旧服务
   ```bash
   systemctl stop anytls-<port-id>
   ```

2. 更新到新版本 Manager/Agent

3. 重新同步配置
   - 新的命令行参数会自动应用
   - 服务会自动重启

4. 验证服务
   ```bash
   systemctl status anytls-<port-id>
   journalctl -u anytls-<port-id> -n 20
   ```

### ShadowTLS 客户端需要更新配置

如果之前手动配置了 ShadowTLS：
1. 使用新的链接格式
2. 或参考 `TROUBLESHOOTING.md` 使用推荐客户端

---

## 下一步计划

1. **支持更多客户端格式**
   - Clash 订阅格式增强
   - Shadowsocks SIP008 格式
   - Surge 配置生成

2. **增强流量统计**
   - 为非 mieru 代理实现流量监控
   - 统一的流量数据接口

3. **Web UI 集成**
   - 前端代理类型选择器
   - 实时日志查看器
   - 证书管理界面

---

## 文件变更摘要

```
修改:
  internal/manager/web/assets/style.css       (配色优化)
  internal/proxy/anytls/anytls.go             (参数修复)
  internal/proxy/shadowtls/shadowtls.go       (链接格式)
  internal/agent/instances_proxy.go           (日志增强)

新增:
  docs/TROUBLESHOOTING.md                     (故障排查指南)
  docs/PROXY_FIXES.md                         (本文档)
```

---

## 联系与反馈

如遇到问题:
1. 查阅 `docs/TROUBLESHOOTING.md`
2. 使用 `journalctl` 查看详细日志
3. 提交 Issue 时附带日志和配置信息
