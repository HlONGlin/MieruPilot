# 验收测试脚本

UnifiedProxy 生产部署验收测试工具集,用于快速验证所有协议的部署状态。

## 📋 目录

- [快速开始](#快速开始)
- [测试脚本说明](#测试脚本说明)
- [配置文件](#配置文件)
- [测试覆盖范围](#测试覆盖范围)
- [故障排查](#故障排查)

---

## 🚀 快速开始

### 1. 准备配置文件

```bash
cd scripts/acceptance

# 复制配置模板
cp test_config.env.example test_config.env

# 编辑配置文件,填写实际的节点信息
nano test_config.env
```

### 2. 运行快速验收

```bash
# 基础连通性测试 (约 30 秒)
./quick_check.sh

# 预期输出:
# =========================================
#   UnifiedProxy 快速验收
# =========================================
# 
# 目标节点: 203.0.113.100
# 测试时间: 2026-10-27 20:30:00
# 
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 🧪 测试: Mieru TCP 端口连通性
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ✅ Mieru TCP:37065 端口开放
# ✅ Mieru TCP:48902 端口开放
# ✅ Mieru TCP:50900 端口开放
# ...
# =========================================
#   测试结果汇总
# =========================================
# 
# ✅ 通过: 12
# ❌ 失败: 0
# ⏭  跳过: 2
# 
# 🎉 所有测试通过!
```

### 3. 解读结果

| 结果 | 说明 | 操作 |
|------|------|------|
| ✅ 通过 | 测试成功 | 无需操作 |
| ❌ 失败 | 测试失败 | 查看[故障排查](#故障排查) |
| ⏭ 跳过 | 配置未设置或工具不可用 | 可选:补充配置或安装依赖 |

---

## 📝 测试脚本说明

### `quick_check.sh` - 快速验收

**用途**: 日常验收、小版本升级后的快速检查

**测试项**:
- ✅ Mieru TCP/UDP 端口连通性
- ✅ AnyTLS 端口连通性
- ✅ ShadowTLS 端口连通性
- ✅ NaïveProxy 端口连通性和 TLS 握手
- ✅ Manager API 健康检查
- ✅ Clash 订阅生成

**执行时间**: ~30 秒

**依赖** (可选):
- `nc` (netcat) - UDP 端口测试
- `openssl` - TLS 证书验证
- `curl` - HTTP/HTTPS 请求

### `regression_test.sh` - 完整回归测试

**用途**: 首次部署、重大版本升级、修复验证

**测试项**:
- ✅ 所有 `quick_check.sh` 的测试项
- ✅ Mieru TCP/UDP 实际转发测试 (需要客户端)
- ✅ AnyTLS 认证测试 (正确/错误密码)
- ✅ ShadowTLS + SS 后端完整链路
- ✅ NaïveProxy 认证测试 (正确/错误凭据,恢复测试)
- ✅ 订阅链接生成和协议覆盖

**执行时间**: ~5 分钟

**依赖**:
- Mieru 客户端
- NaïveProxy 客户端
- Shadowsocks 客户端 (或 mihomo)

---

## ⚙️ 配置文件

### `test_config.env` 示例

```bash
# 节点信息
NODE_IP="203.0.113.100"
NODE_NAME="生产节点 1"

# Mieru 配置
MIERU_TCP_PORTS="37065,48902,50900"
MIERU_UDP_PORTS="40590"

# AnyTLS 配置
ANYTLS_PORT="23112"
ANYTLS_PASSWORD="strongPassword123"

# ShadowTLS 配置
SHADOWTLS_PORT="40900"
SHADOWTLS_PASSWORD="anotherPassword456"

# NaïveProxy 配置
NAIVE_PORT="443"
NAIVE_DOMAIN="proxy.example.com"
NAIVE_USER="admin"
NAIVE_PASS="securePass789"

# Manager API
MANAGER_URL="http://192.168.1.100:8080"
SUBSCRIPTION_TOKEN="your_subscription_token"
```

### 配置说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `NODE_IP` | ✅ | 节点公网 IP 地址 |
| `MIERU_TCP_PORTS` | ⬜ | Mieru TCP 端口列表,逗号分隔 |
| `MIERU_UDP_PORTS` | ⬜ | Mieru UDP 端口列表,逗号分隔 |
| `ANYTLS_PORT` | ⬜ | AnyTLS 监听端口 |
| `ANYTLS_PASSWORD` | ⬜ | AnyTLS 密码 (用于认证测试) |
| `SHADOWTLS_PORT` | ⬜ | ShadowTLS 监听端口 |
| `NAIVE_PORT` | ⬜ | NaïveProxy 端口 (默认 443) |
| `NAIVE_DOMAIN` | ⬜ | NaïveProxy 域名 |
| `MANAGER_URL` | ⬜ | Manager 面板地址 |
| `SUBSCRIPTION_TOKEN` | ⬜ | Clash 订阅令牌 |

> **注意**: 未配置的协议会自动跳过相关测试

---

## 🔍 测试覆盖范围

### 当前支持

| 协议 | 端口连通 | TLS 握手 | 认证测试 | 转发测试 |
|------|----------|----------|----------|----------|
| **Mieru TCP** | ✅ | N/A | ⬜ | ⬜ |
| **Mieru UDP** | ✅ | N/A | ⬜ | ⬜ |
| **AnyTLS** | ✅ | ⬜ | ⬜ | ⬜ |
| **ShadowTLS** | ✅ | ⬜ | ⬜ | ⬜ |
| **NaïveProxy** | ✅ | ✅ | ⬜ | ⬜ |

> ✅ 已支持 | ⬜ 计划支持 | N/A 不适用

### 路线图

**v1.6.1** (quick_check.sh)
- [x] 基础端口连通性测试
- [x] NaïveProxy TLS 握手测试
- [x] Manager API 健康检查
- [x] Clash 订阅生成验证

**v1.6.2** (regression_test.sh)
- [ ] Mieru 客户端转发测试
- [ ] AnyTLS 认证测试 (正确/错误密码)
- [ ] ShadowTLS + SS 完整链路测试
- [ ] NaïveProxy 认证测试

**v1.6.3**
- [ ] 性能基准测试 (吞吐量、延迟)
- [ ] 并发连接测试
- [ ] 长时间稳定性测试

---

## 🔧 故障排查

### 常见问题

#### 1. 端口不可达

**症状**:
```
❌ Mieru TCP:37065 端口不可达
```

**排查步骤**:
```bash
# 1. 检查服务状态
systemctl status merit-agent

# 2. 检查端口监听
ss -tlnp | grep 37065

# 3. 检查防火墙
# UFW
ufw status | grep 37065

# Firewalld
firewall-cmd --list-ports | grep 37065

# 4. 查看日志
journalctl -u merit-agent -n 50 --no-pager
```

**解决方案**:
- 服务未启动: `systemctl start merit-agent`
- 端口未监听: 检查面板配置,重试同步
- 防火墙拦截: `ufw allow 37065/tcp` 或 `firewall-cmd --add-port=37065/tcp --permanent`

---

#### 2. NaïveProxy TLS 握手失败

**症状**:
```
✅ NaïveProxy:443 端口开放
❌ NaïveProxy TLS 握手失败
```

**排查步骤**:
```bash
# 1. 测试 TLS 连接
echo | openssl s_client -connect $NODE_IP:443 -servername $NAIVE_DOMAIN

# 2. 检查证书
echo | openssl s_client -connect $NODE_IP:443 -servername $NAIVE_DOMAIN 2>/dev/null | openssl x509 -noout -text

# 3. 检查 Caddy 日志
journalctl -u caddy -n 50 --no-pager
```

**常见原因**:
- 证书未上传或已过期
- 域名未正确解析到节点 IP
- SNI 不匹配
- Caddy 未正确启动

---

#### 3. Manager API 不可达

**症状**:
```
❌ Manager API 不可达
```

**排查步骤**:
```bash
# 1. 检查 Manager 服务
systemctl status merit-manager

# 2. 检查端口监听
ss -tlnp | grep 8080

# 3. 测试本地访问
curl http://localhost:8080/api/health

# 4. 查看日志
journalctl -u merit-manager -n 50 --no-pager
```

---

#### 4. 订阅生成失败

**症状**:
```
❌ Clash 订阅生成失败
```

**排查步骤**:
```bash
# 1. 检查订阅 URL
curl -v "http://manager-ip:8080/api/clash/subscribe?token=your_token"

# 2. 验证令牌
# 在面板中查看或重新生成订阅令牌

# 3. 检查节点和端口配置
# 确保至少有一个已启用的端口
```

---

## 📊 测试报告示例

### 成功案例

```
=========================================
  UnifiedProxy 快速验收
=========================================

目标节点: 203.0.113.100
测试时间: 2026-10-27 20:30:00

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧪 测试: Mieru TCP 端口连通性
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Mieru TCP:37065 端口开放
✅ Mieru TCP:48902 端口开放
✅ Mieru TCP:50900 端口开放

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧪 测试: Mieru UDP 端口连通性
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Mieru UDP:40590 端口开放

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧪 测试: AnyTLS 连通性和认证
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ AnyTLS:23112 端口开放
  ℹ️  认证测试需要 AnyTLS 客户端,已跳过

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧪 测试: ShadowTLS v3 连通性
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ ShadowTLS:40900 端口开放
  ℹ️  完整链路测试需要 Shadowsocks 客户端,已跳过

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧪 测试: NaïveProxy 连通性和认证
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ NaïveProxy:443 端口开放
✅ NaïveProxy TLS 握手成功
  ℹ️  证书到期时间: Dec 25 00:00:00 2026 GMT
  ℹ️  认证测试需要 NaïveProxy 客户端,已跳过

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🧪 测试: Manager API 健康检查
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Manager API 可达
✅ Clash 订阅生成成功
  ℹ️  订阅包含 7 个代理节点

=========================================
  测试结果汇总
=========================================

✅ 通过: 12
❌ 失败: 0
⏭  跳过: 4

🎉 所有测试通过!
```

---

## 🔗 相关文档

- [生产加固路线图](../../docs/PRODUCTION_HARDENING_ROADMAP.md)
- [协议部署审计](../../docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md)
- [订阅链接修复](../../docs/SUBSCRIPTION_LINK_FIX.md)
- [多协议快速开始](../../docs/QUICK_START_MULTI_PROXY.md)

---

## 🤝 贡献

欢迎提交新的测试用例和改进建议!

**添加新测试**:
1. Fork 项目
2. 在 `quick_check.sh` 中添加测试函数
3. 更新本文档的"测试覆盖范围"
4. 提交 PR

---

**最后更新**: 2026-10-27  
**维护者**: holi
