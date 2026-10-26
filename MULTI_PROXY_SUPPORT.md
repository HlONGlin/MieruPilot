# 多协议代理支持文档

## 概述

MieruPilot 现已支持多种代理协议，每个端口可以独立选择使用的代理类型：

- **Mieru** - 原生支持的见える代理（默认）
- **NaïveProxy** - 基于Chrome网络栈的TLS代理
- **AnyTLS** - 轻量级TLS混淆代理
- **ShadowTLS v3** - TLS流量伪装代理

## 特性

### 灵活部署
- ✅ 每个端口独立选择代理类型
- ✅ 同一节点可混合部署不同类型代理
- ✅ 完全向后兼容现有Mieru节点

### 证书管理(NaïveProxy)
- ✅ 自动ACME证书申请和续期
- ✅ 手动上传自有证书
- ✅ 灵活切换证书模式

### 统一管理
- ✅ 统一的Web管理界面
- ✅ 自动生成各类型分享链接
- ✅ Clash/Mihomo订阅支持

## 代理类型对比

| 特性 | Mieru | NaïveProxy | AnyTLS | ShadowTLS v3 |
|------|-------|------------|--------|--------------|
| **传输协议** | 自定义 | HTTPS | TLS | TLS伪装 |
| **需要域名** | ❌ | ✅ | ❌ | ❌ |
| **需要证书** | ❌ | ✅ | ❌ | ❌ |
| **UDP支持** | ✅ | ❌ | ✅ | ❌ |
| **抗封锁能力** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **性能** | 高 | 中 | 高 | 高 |
| **使用场景** | 通用 | 严格审查地区 | 轻量部署 | TLS伪装 |

## 使用指南

### 1. Mieru (默认)

#### 特点
- 自定义协议，经过充分测试
- 支持TCP/UDP
- 无需额外配置

#### 配置示例
```json
{
  "port": 8080,
  "protocol": "TCP",
  "proxyType": "mieru"
}
```

#### 分享链接
```
mierus://username:password@host:8080?protocol=TCP&...
```

### 2. NaïveProxy

#### 特点
- 使用Chrome网络栈，流量特征接近真实浏览器
- **需要TLS证书和域名**
- 最强抗检测能力
- 仅支持TCP

#### 配置要求
- 域名（用于TLS证书）
- 证书（自动ACME或手动上传）
- 端口443或其他HTTPS端口

#### 配置示例

**自动ACME证书：**
```json
{
  "port": 443,
  "protocol": "TCP",
  "proxyType": "naive",
  "domain": "example.com",
  "autoACME": true
}
```

**手动证书：**
```json
{
  "port": 443,
  "protocol": "TCP",
  "proxyType": "naive",
  "domain": "example.com",
  "autoACME": false,
  "tlsCert": "-----BEGIN CERTIFICATE-----\n...",
  "tlsKey": "-----BEGIN PRIVATE KEY-----\n..."
}
```

#### 注意事项
- **ACME模式需要80端口**用于HTTP-01验证（或使用DNS-01）
- 域名必须解析到服务器IP
- 证书自动续期，无需手动维护

#### 分享链接
```
naive+https://username:password@example.com:443
```

#### Clash配置
```yaml
- name: naive-node
  type: http
  server: example.com
  port: 443
  username: user
  password: pass
  tls: true
  sni: example.com
```

### 3. AnyTLS

#### 特点
- 轻量级TLS混淆
- 支持TCP/UDP
- 无需域名和证书
- 配置简单

#### 配置示例
```json
{
  "port": 8443,
  "protocol": "TCP",
  "proxyType": "anytls"
}
```

#### 分享链接
```
anytls://password@host:8443
```

### 4. ShadowTLS v3

#### 特点
- TLS流量伪装
- 需要指定握手服务器（模仿目标）
- 高性能
- 仅支持TCP

#### 配置示例
```json
{
  "port": 443,
  "protocol": "TCP",
  "proxyType": "shadowtls",
  "handshakeAddr": "cloudflare.com:443"
}
```

#### 握手服务器选择
推荐使用高可用的HTTPS网站：
- `cloudflare.com:443`
- `www.google.com:443`
- `www.microsoft.com:443`

#### 分享链接
```
shadowtls://password@host:443?handshake=cloudflare.com:443
```

## API参考

### 创建端口(支持指定代理类型)

**POST** `/api/nodes/:nodeId/ports`

```json
{
  "port": 443,
  "protocol": "TCP",
  "proxyType": "naive",
  "label": "HK Node",
  "domain": "example.com",
  "autoACME": true
}
```

### 上传TLS证书(NaïveProxy)

**POST** `/api/nodes/:nodeId/ports/:portId/cert`

```json
{
  "cert": "-----BEGIN CERTIFICATE-----\n...",
  "key": "-----BEGIN PRIVATE KEY-----\n..."
}
```

### 获取分享链接

**GET** `/api/nodes/:nodeId/ports/:portId/share`

响应：
```json
{
  "link": "naive+https://user:pass@example.com:443",
  "proxyType": "naive"
}
```

## 前端UI指南

### 添加端口时选择代理类型

1. 进入节点详情页
2. 点击「添加端口」
3. 选择代理类型：
   - Mieru (默认)
   - NaïveProxy
   - AnyTLS
   - ShadowTLS v3
4. 根据所选类型填写相应配置

### NaïveProxy专有配置

**域名配置：**
- 输入已解析的域名
- 域名必须指向服务器IP

**证书模式：**
- **自动ACME**：无需上传，自动申请和续期
- **手动上传**：上传.crt和.key文件

### ShadowTLS专有配置

**握手服务器：**
- 输入目标网站地址(如 `cloudflare.com:443`)
- 建议选择高可用的HTTPS网站

## 部署注意事项

### 端口要求

| 代理类型 | 推荐端口 | 特殊要求 |
|---------|---------|---------|
| Mieru | 任意 | 无 |
| NaïveProxy | 443 | ACME需要80端口 |
| AnyTLS | 任意 | 无 |
| ShadowTLS | 443 | 无 |

### 防火墙配置

**Mieru/AnyTLS/ShadowTLS:**
```bash
ufw allow <port>/tcp
ufw allow <port>/udp  # 如果使用UDP
```

**NaïveProxy with ACME:**
```bash
ufw allow 80/tcp   # HTTP-01验证
ufw allow 443/tcp  # HTTPS服务
```

### 系统要求

所有代理类型：
- Linux with systemd
- root权限
- 网络连通性

NaïveProxy额外要求：
- 域名已解析到服务器
- 80端口可访问(ACME模式)

## 故障排查

### NaïveProxy证书申请失败

**问题：** ACME证书申请失败

**检查：**
1. 域名是否正确解析到服务器IP
   ```bash
   nslookup example.com
   ```
2. 80端口是否被占用
   ```bash
   netstat -tlnp | grep :80
   ```
3. 防火墙是否开放80端口
   ```bash
   ufw status
   ```

**解决：**
- 确保域名解析正确
- 释放80端口或使用手动证书
- 开放防火墙80端口

### ShadowTLS连接失败

**问题：** 客户端无法连接

**检查：**
1. 握手服务器是否可达
   ```bash
   curl -I https://cloudflare.com
   ```
2. 服务状态
   ```bash
   systemctl status shadowtls-<portId>
   ```

**解决：**
- 更换其他可靠的握手服务器
- 检查服务日志排查错误

### 代理安装失败

**问题：** Agent无法自动安装代理

**检查：**
```bash
journalctl -u merit-agent -n 50
```

**解决：**
- 检查网络连接
- 尝试使用镜像加速
- 手动下载并安装二进制文件

## 性能优化

### Mieru
- 使用UDP协议获得更好性能
- 调整MTU值匹配网络环境

### NaïveProxy
- 使用HTTP/2提升并发能力
- 启用QUIC协议(如果支持)

### AnyTLS/ShadowTLS
- 选择地理位置接近的握手服务器
- 调整TCP参数优化吞吐量

## 客户端支持

### Mieru
- [官方客户端](https://github.com/enfein/mieru)
- Clash Meta (type: mieru)

### NaïveProxy
- [官方客户端](https://github.com/klzgrad/naiveproxy)
- Clash Meta (type: http + tls)
- SagerNet/NekoBox

### AnyTLS
- [官方客户端](https://github.com/anytls/anytls)

### ShadowTLS
- 需配合Shadowsocks使用
- [Shadow-TLS客户端](https://github.com/ihciah/shadow-tls)

## 常见问题

### Q: 可以同时运行多种代理类型吗？
A: 可以！每个端口独立选择代理类型，同一节点可以混合部署。

### Q: NaïveProxy必须使用443端口吗？
A: 不必须，但建议使用443以提高兼容性和隐蔽性。

### Q: 旧的Mieru节点会受影响吗？
A: 不会，完全向后兼容。旧节点自动识别为Mieru类型。

### Q: ACME证书多久续期一次？
A: Let's Encrypt证书有效期90天，Caddy会自动在到期前30天续期。

### Q: 可以为多个端口共享一个证书吗？
A: NaïveProxy的证书是按域名管理的，同一域名的多个端口可以共享证书。

### Q: 哪种代理类型最安全？
A: NaïveProxy使用真实浏览器栈，抗检测能力最强。但所有类型在正确配置下都很安全。

### Q: 流量统计支持所有代理类型吗？
A: 当前主要支持Mieru的流量统计。其他代理类型的流量统计功能正在开发中。

## 进阶配置

### 多域名NaïveProxy

为不同用户使用不同域名：
```json
{
  "ports": [
    {"port": 443, "domain": "a.example.com", "proxyType": "naive"},
    {"port": 444, "domain": "b.example.com", "proxyType": "naive"}
  ]
}
```

### 混合部署策略

推荐配置：
- **主力端口**: Mieru (通用，性能好)
- **备用端口**: NaïveProxy (高隐蔽，应对封锁)
- **轻量端口**: AnyTLS (简单快速)

### 自定义握手服务器列表

为ShadowTLS准备多个握手服务器：
```
cloudflare.com:443
www.apple.com:443
www.microsoft.com:443
azure.microsoft.com:443
```

## 贡献

欢迎提交Issue和Pull Request！

特别需要：
- 更多代理类型支持
- 客户端配置生成器
- 流量统计改进
- 性能优化建议

## 许可证

MIT License - 参见 [LICENSE](LICENSE)

---

**注意：** 本功能仍在积极开发中，部分特性可能还未完全实现。请关注项目更新。
