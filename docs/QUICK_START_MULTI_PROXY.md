# MieruPilot 多协议代理 - 快速开始指南

本指南将帮助你快速使用 MieruPilot 的多协议支持功能。

## 支持的代理类型

| 代理类型 | 特点 | 适用场景 |
|---------|------|---------|
| **Mieru** | 原生支持，UDP协议 | 通用场景，游戏、视频 |
| **NaïveProxy** | Chrome网络栈，伪装HTTPS | 需要高度伪装的环境 |
| **AnyTLS** | 通用TLS代理 | 需要TLS加密的场景 |
| **ShadowTLS** | Shadowsocks + TLS隧道 | 特殊网络环境 |

## 快速部署

### 1. 部署 Manager

```bash
# 启动 Manager
./manager -listen :3000 -data ./data

# Manager 会自动启动在 http://localhost:3000
```

### 2. 部署 Agent

```bash
# 在远程服务器上运行
./agent \
  -manager http://YOUR_MANAGER_IP:3000 \
  -key YOUR_API_KEY \
  -interval 5s
```

Agent 会自动：
- 注册到 Manager
- 下载并安装所需的代理程序
- 根据配置启动代理实例

### 3. 添加代理端口

通过 Manager Web UI 或 API 添加不同类型的代理端口。

## 使用示例

### 示例 1: 添加 Mieru 端口（默认类型）

```bash
curl -X POST http://localhost:3000/api/nodes/YOUR_NODE_ID/ports \
  -H "Content-Type: application/json" \
  -d '{
    "port": 8080,
    "protocol": "tcp",
    "username": "user123",
    "password": "securepass"
  }'
```

**获取分享链接：**
```
mieru://user123:securepass@YOUR_SERVER_IP:8080
```

### 示例 2: 添加 NaïveProxy 端口

```bash
curl -X POST http://localhost:3000/api/nodes/YOUR_NODE_ID/ports \
  -H "Content-Type: application/json" \
  -d '{
    "port": 443,
    "protocol": "tcp",
    "username": "user456",
    "password": "securepass2",
    "proxyType": "naive",
    "domain": "example.com",
    "autoACME": true
  }'
```

**分享链接格式：**
```
naive+https://user456:securepass2@example.com:443
```

**客户端配置（NaïveProxy）：**
```json
{
  "listen": "socks://127.0.0.1:1080",
  "proxy": "https://user456:securepass2@example.com:443"
}
```

### 示例 3: 添加 AnyTLS 端口

```bash
curl -X POST http://localhost:3000/api/nodes/YOUR_NODE_ID/ports \
  -H "Content-Type: application/json" \
  -d '{
    "port": 8443,
    "protocol": "tcp",
    "username": "trojan",
    "password": "mypassword",
    "proxyType": "anytls"
  }'
```

**分享链接格式：**
```
anytls://mypassword@YOUR_SERVER_IP:8443
```

**客户端配置（V2Ray/Xray）：**
```json
{
  "protocol": "trojan",
  "settings": {
    "servers": [{
      "address": "YOUR_SERVER_IP",
      "port": 8443,
      "password": "mypassword"
    }]
  }
}
```

### 示例 4: 添加 ShadowTLS 端口

```bash
curl -X POST http://localhost:3000/api/nodes/YOUR_NODE_ID/ports \
  -H "Content-Type: application/json" \
  -d '{
    "port": 9443,
    "protocol": "tcp",
    "username": "shadowsocks",
    "password": "strongpass",
    "proxyType": "shadowtls",
    "handshakeAddr": "cloudflare.com:443",
    "sni": "cloudflare.com"
  }'
```

**分享链接格式：**
```
shadowtls://strongpass@YOUR_SERVER_IP:9443?handshake=cloudflare.com:443&sni=cloudflare.com
```

## 获取 Clash 订阅

### 所有节点的订阅

```bash
curl http://localhost:3000/api/subscription/clash
```

### 单个节点的订阅

```bash
curl http://localhost:3000/api/nodes/YOUR_NODE_ID/clash
```

### Clash 配置示例

生成的 Clash 配置会包含所有类型的代理：

```yaml
proxies:
  - name: "Node1-8080-Mieru"
    type: mieru
    server: 1.2.3.4
    port: 8080
    username: user123
    password: securepass

  - name: "Node1-443-Naive"
    type: http
    server: example.com
    port: 443
    username: user456
    password: securepass2
    tls: true
    sni: example.com

  - name: "Node1-8443-AnyTLS"
    type: trojan
    server: 1.2.3.4
    port: 8443
    password: mypassword

  - name: "Node1-9443-ShadowTLS"
    type: ss
    server: 1.2.3.4
    port: 9443
    cipher: chacha20-ietf-poly1305
    password: strongpass
    plugin: shadow-tls
    plugin-opts:
      handshake: cloudflare.com:443
      sni: cloudflare.com

proxy-groups:
  - name: Proxy
    type: select
    proxies:
      - Node1-8080-Mieru
      - Node1-443-Naive
      - Node1-8443-AnyTLS
      - Node1-9443-ShadowTLS

rules:
  - MATCH,Proxy
```

## 证书管理

### 上传 TLS 证书（用于 NaïveProxy）

```bash
curl -X POST http://localhost:3000/api/nodes/YOUR_NODE_ID/ports/PORT_ID/upload-cert \
  -F "cert=@/path/to/cert.pem" \
  -F "key=@/path/to/key.pem"
```

### 使用自动 ACME（Let's Encrypt）

在添加 NaïveProxy 端口时设置 `autoACME: true`：

```json
{
  "port": 443,
  "proxyType": "naive",
  "domain": "example.com",
  "autoACME": true
}
```

Agent 会自动：
1. 配置 Caddy 的 ACME
2. 申请 Let's Encrypt 证书
3. 自动续期证书

## 客户端配置

### Clash/Clash.Meta

1. 在 Clash 中添加订阅：
   ```
   http://YOUR_MANAGER:3000/api/subscription/clash
   ```

2. 更新订阅并选择代理节点

### NaïveProxy 客户端

创建 `config.json`：
```json
{
  "listen": "socks://127.0.0.1:1080",
  "proxy": "https://user456:securepass2@example.com:443"
}
```

运行：
```bash
naive config.json
```

### V2Ray/Xray (AnyTLS/ShadowTLS)

使用 V2RayN、Qv2ray 等客户端，导入分享链接或手动配置。

## 查看代理状态

### 检查 Agent 状态

```bash
# 在 Agent 服务器上
systemctl status merit-agent

# 查看所有代理服务
systemctl list-units 'merit-*' --all
```

### 查看特定代理日志

```bash
# NaïveProxy
journalctl -u merit-naive-443 -f

# AnyTLS
journalctl -u merit-anytls-8443 -f

# ShadowTLS
journalctl -u merit-shadowtls-9443 -f
```

## 故障排查

### 1. 代理未启动

**检查服务状态：**
```bash
systemctl status merit-naive-443
```

**查看详细日志：**
```bash
journalctl -u merit-naive-443 -n 50
```

**常见问题：**
- 端口被占用
- 证书文件不存在或权限不正确
- 配置文件格式错误

### 2. 证书问题（NaïveProxy）

**检查证书：**
```bash
ls -la /etc/merit-naive/
```

**手动测试 HTTPS：**
```bash
curl -v https://example.com:443
```

### 3. 连接问题

**检查防火墙：**
```bash
# 允许端口
ufw allow 443/tcp
ufw allow 8443/tcp
ufw allow 9443/tcp
```

**测试端口：**
```bash
nc -zv YOUR_SERVER_IP 443
```

### 4. Agent 未同步配置

**手动触发同步：**
```bash
# 重启 Agent
systemctl restart merit-agent

# 查看 Agent 日志
journalctl -u merit-agent -f
```

## 性能优化

### 1. 调整系统参数

编辑 `/etc/sysctl.conf`：
```
# 增加最大文件描述符
fs.file-max = 1000000

# TCP 优化
net.core.rmem_max = 67108864
net.core.wmem_max = 67108864
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
net.ipv4.tcp_congestion_control = bbr
```

应用：
```bash
sysctl -p
```

### 2. 监控资源使用

```bash
# CPU 和内存
htop

# 网络流量
iftop

# 代理进程
ps aux | grep -E 'caddy|anytls|shadow-tls|mita'
```

## 高级配置

### 混合部署策略

在同一节点上部署多种代理类型，应对不同场景：

```
Port 8080  → Mieru        (UDP, 游戏优化)
Port 443   → NaïveProxy   (HTTPS伪装, 高强度审查环境)
Port 8443  → AnyTLS       (通用TLS代理)
Port 9443  → ShadowTLS    (TLS隧道, 特殊场景)
```

### 自动故障转移

Clash 配置中使用 `fallback` 或 `url-test` 组：

```yaml
proxy-groups:
  - name: Auto
    type: url-test
    proxies:
      - Node1-443-Naive
      - Node1-8080-Mieru
      - Node1-8443-AnyTLS
    url: 'http://www.gstatic.com/generate_204'
    interval: 300
```

## API 参考

### 获取支持的代理类型

```bash
GET /api/proxy-types
```

**响应：**
```json
{
  "types": [
    {
      "type": "mieru",
      "name": "Mieru",
      "description": "原生 UDP 代理协议"
    },
    {
      "type": "naive",
      "name": "NaïveProxy",
      "description": "基于 Chrome 网络栈的 HTTPS 代理"
    },
    {
      "type": "anytls",
      "name": "AnyTLS",
      "description": "通用 TLS 代理"
    },
    {
      "type": "shadowtls",
      "name": "ShadowTLS",
      "description": "Shadowsocks over TLS v3"
    }
  ]
}
```

### 添加端口

```bash
POST /api/nodes/:nodeID/ports
Content-Type: application/json

{
  "port": 8080,
  "protocol": "tcp",
  "username": "user",
  "password": "pass",
  "proxyType": "mieru|naive|anytls|shadowtls",
  // NaïveProxy 特定字段
  "domain": "example.com",
  "autoACME": true,
  // ShadowTLS 特定字段
  "handshakeAddr": "cloudflare.com:443",
  "sni": "cloudflare.com"
}
```

### 删除端口

```bash
DELETE /api/nodes/:nodeID/ports/:portID
```

### 获取分享链接

```bash
GET /api/nodes/:nodeID/links
```

## 安全建议

1. **使用强密码**：代理密码至少 16 字符，包含字母、数字和符号
2. **TLS 加密**：生产环境建议使用 NaïveProxy 或 AnyTLS
3. **证书管理**：定期检查证书有效期，配置自动续期
4. **访问控制**：使用防火墙限制 Manager 访问
5. **定期更新**：保持代理程序和 Agent 版本最新

## 下一步

- 查看 [完整文档](./MULTI_PROXY_SUPPORT.md)
- 阅读 [开发者指南](./DEVELOPER_GUIDE.md)
- 查看 [UI 演示](./FRONTEND_UI_DEMO.html)

---

**问题反馈**: 在使用过程中遇到问题？请查看 [故障排查指南](./MULTI_PROXY_SUPPORT.md#故障排查) 或提交 Issue。
