# NaïveProxy 部署完整指南

## 📖 目录

- [什么是 NaïveProxy](#什么是-naïveproxy)
- [前置准备](#前置准备)
- [部署流程](#部署流程)
- [证书配置](#证书配置)
- [客户端配置](#客户端配置)
- [故障排查](#故障排查)
- [高级配置](#高级配置)

---

## 什么是 NaïveProxy

**NaïveProxy** 是基于 Chromium 网络栈的高伪装代理协议，具有以下特点：

### 优势
- ✅ **强伪装性** - 流量完全伪装为标准 HTTPS
- ✅ **抗检测** - 使用真实的 Chrome 网络栈，难以被指纹识别
- ✅ **高性能** - 基于 Caddy 服务器，性能出色
- ✅ **TLS 1.3** - 支持最新的 TLS 协议

### 适用场景
- 需要强伪装性的网络环境
- 对流量特征检测敏感的地区
- 需要与正常网站混淆的场景

### 与其他协议对比

| 特性 | NaïveProxy | Mieru | ShadowTLS |
|------|-----------|-------|-----------|
| 伪装性 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 性能 | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| 配置难度 | ⭐⭐⭐ | ⭐ | ⭐⭐ |
| 需要域名 | 是 | 否 | 否 |
| 需要证书 | 是 | 否 | 否 |

---

## 前置准备

### 1. 域名准备

NaïveProxy **必须使用域名**，不能只用 IP。

#### 选项 A: 购买域名（推荐）
- 推荐平台：Cloudflare Registrar、NameSilo、Namesilo
- 费用：约 $1-10/年（.com/.net/.org）
- 优势：稳定、可靠、支持自动续费

#### 选项 B: 免费域名
- 平台：Freenom（.tk/.ml/.ga/.cf）
- 注意：可能被回收、稳定性差
- 仅建议测试使用

### 2. DNS 解析设置

将域名解析到你的服务器：

**Cloudflare DNS 设置示例**：
```
类型: A
名称: proxy（或 @）
内容: 你的服务器公网IP
代理状态: 仅DNS（关闭橙色云朵）
TTL: 自动
```

**验证 DNS 解析**：
```bash
# 在本地电脑测试
nslookup proxy.yourdomain.com

# 或使用 dig
dig proxy.yourdomain.com +short
```

确保返回你的服务器 IP。

### 3. 端口准备

NaïveProxy 使用 **443 端口**（HTTPS）：

```bash
# 检查端口是否被占用
ss -tlnp | grep :443

# 如果被占用，停止占用的服务
# 例如停止 nginx
systemctl stop nginx
```

### 4. 防火墙配置

开放必要端口：

```bash
# 443 端口（NaïveProxy HTTPS）
ufw allow 443/tcp
# 或 iptables
iptables -I INPUT -p tcp --dport 443 -j ACCEPT

# 如果使用 ACME HTTP-01，还需要 80 端口
ufw allow 80/tcp
```

**云服务商安全组**：
- 阿里云/腾讯云/AWS 等：在控制台添加入站规则
- 开放 TCP 443（必须）
- 开放 TCP 80（ACME 自动签发证书时需要）

---

## 部署流程

### 第 1 步：部署 Manager 和 Agent

如果还没有安装 UnifiedProxy，先完成基础部署：

#### 安装 Manager（控制机）

```bash
# 使用一键脚本
bash <(curl -fsSL https://github.com/HlONGlin/UnifiedProxy/raw/main/one-click.sh)

# 选择 1. 安装或更新 Manager
# 按提示设置管理员账号和密码
```

#### 部署 Agent（节点服务器）

1. 在 Manager 面板点击「添加节点」
2. 填写节点名称（如 `hk-naive-01`）
3. 复制生成的部署命令
4. 在节点服务器以 root 权限执行：

```bash
curl -fsSL http://你的Manager地址:3000/install.sh | sudo bash -s -- --key <API_KEY>
```

等待 Agent 安装完成并上线。

### 第 2 步：添加 NaïveProxy 端口

1. 在 Manager 面板进入节点详情页
2. 点击「添加端口」或「配置端口」
3. 填写以下信息：

```
端口号: 443
代理类型: NaïveProxy  ← 选择这个！
域名: proxy.yourdomain.com
协议: TCP
```

4. 选择证书配置方式（二选一）

---

## 证书配置

NaïveProxy 需要 TLS 证书。有两种配置方式：

### 方式 1: 自动 ACME 签发（推荐）

**优势**：
- ✅ 全自动，无需手动操作
- ✅ 自动续期，永不过期
- ✅ 使用 Let's Encrypt 权威 CA

**要求**：
- 域名必须正确解析到服务器
- 80 端口可访问（用于 ACME HTTP-01 验证）
- 或 443 端口可访问（用于 ACME TLS-ALPN-01 验证）

**操作步骤**：

1. 在端口配置界面勾选 **「启用 ACME 自动签发」**
2. 确认域名已正确填写
3. 点击「保存」

Manager 会指示 Agent：
- 使用 Caddy 的 ACME 功能
- 自动从 Let's Encrypt 申请证书
- 自动处理续期（证书到期前自动续签）

**首次签发时间**：约 10-30 秒

**验证签发成功**：
```bash
# 在节点服务器查看服务日志
journalctl -u naive-proxy-<port-id> -f

# 应该看到类似输出
# "certificate obtained successfully"
# "serving HTTPS on :443"
```

**常见问题**：
- ❌ 域名解析未生效 → 等待 DNS 传播（最多 24 小时）
- ❌ 80 端口被占用 → 停止占用服务或使用 TLS-ALPN-01
- ❌ 防火墙未开放 → 检查安全组和 iptables

---

### 方式 2: 手动上传证书

**适用场景**：
- 无法开放 80 端口
- 使用泛域名证书
- 使用自签名证书（仅测试）
- 已有现成证书

**操作步骤**：

#### 2.1 准备证书文件

**选项 A: 使用已有证书**

如果你已经有证书（如 Cloudflare SSL、其他 CA 签发）：
- 准备 `fullchain.pem`（完整证书链）
- 准备 `privkey.pem`（私钥）

**选项 B: 生成自签名证书（仅测试）**

```bash
# 生成自签名证书（仅用于测试，客户端需要忽略证书验证）
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout privkey.pem \
  -out fullchain.pem \
  -days 365 \
  -subj "/CN=proxy.yourdomain.com"
```

#### 2.2 上传到 Manager

1. 在端口配置界面**不要勾选** ACME 自动签发
2. 点击「上传证书」
3. 复制 `fullchain.pem` 内容粘贴到「证书内容」
4. 复制 `privkey.pem` 内容粘贴到「私钥内容」
5. 点击「保存」

**证书格式示例**：

```
证书内容（fullchain.pem）：
-----BEGIN CERTIFICATE-----
MIIFXzCCBEegAwIBAgISA...
（完整的证书内容）
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIFFjCCAv6gAwIBAgIRAJ...
（中间证书）
-----END CERTIFICATE-----

私钥内容（privkey.pem）：
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0B...
（完整的私钥内容）
-----END PRIVATE KEY-----
```

**注意事项**：
- ⚠️ 手动证书不会自动续期，需要在过期前重新上传
- ⚠️ 自签名证书客户端需要特殊配置
- ⚠️ 私钥文件非常敏感，请勿泄露

---

### 第 3 步：启动服务

1. 配置完成后，Manager 会自动下发任务到 Agent
2. Agent 自动：
   - 下载 Caddy（带 forwardproxy 插件）
   - 生成配置文件
   - 创建 systemd 服务
   - 启动 NaïveProxy

**查看启动状态**：

```bash
# 在节点服务器执行
systemctl status naive-proxy-<port-id>

# 查看实时日志
journalctl -u naive-proxy-<port-id> -f
```

**成功标志**：
```
● naive-proxy-abc123.service - NaiveProxy Instance abc123
   Active: active (running)
   
日志输出：
[INFO] Caddy serving HTTPS on :443
[INFO] Certificate obtained successfully
```

### 第 4 步：获取分享链接

1. 在 Manager 面板的节点详情页
2. 找到刚配置的 NaïveProxy 端口
3. 点击「复制链接」

**链接格式**：
```
naive+https://用户名:密码@proxy.yourdomain.com:443
```

---

## 客户端配置

### Windows 客户端

#### 推荐：Clash Verge Rev

1. 下载 [Clash Verge Rev](https://github.com/clash-verge-rev/clash-verge-rev/releases)
2. 在 Manager 获取 Clash 订阅地址
3. 在 Clash Verge 添加订阅
4. 启用 NaïveProxy 节点

**手动配置**：
```yaml
proxies:
  - name: "NaiveProxy-HK"
    type: naive
    server: proxy.yourdomain.com
    port: 443
    username: "自动生成的用户名"
    password: "自动生成的密码"
    tls: true
```

#### 备选：NaïveProxy 官方客户端

1. 下载 [naiveproxy-windows](https://github.com/klzgrad/naiveproxy/releases)
2. 创建配置文件 `config.json`：

```json
{
  "listen": "socks://127.0.0.1:1080",
  "proxy": "https://用户名:密码@proxy.yourdomain.com:443"
}
```

3. 运行：
```cmd
naive.exe config.json
```

---

### macOS 客户端

#### 推荐：Clash Verge Rev

与 Windows 相同，使用 Clash 订阅或手动配置。

#### 备选：NaïveProxy CLI

```bash
# 安装 Homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 下载 naiveproxy
curl -LO https://github.com/klzgrad/naiveproxy/releases/download/v123.0.6312.58-1/naiveproxy-v123.0.6312.58-1-mac-x64.tar.xz
tar xf naiveproxy-*.tar.xz
cd naiveproxy-*

# 创建配置
cat > config.json <<EOF
{
  "listen": "socks://127.0.0.1:1080",
  "proxy": "https://用户名:密码@proxy.yourdomain.com:443"
}
EOF

# 运行
./naive config.json
```

---

### Linux 客户端

```bash
# 下载对应架构的 naiveproxy
wget https://github.com/klzgrad/naiveproxy/releases/download/v123.0.6312.58-1/naiveproxy-v123.0.6312.58-1-linux-x64.tar.xz

tar xf naiveproxy-*.tar.xz
cd naiveproxy-*

# 创建配置
cat > config.json <<EOF
{
  "listen": "socks://127.0.0.1:1080",
  "proxy": "https://用户名:密码@proxy.yourdomain.com:443"
}
EOF

# 运行
./naive config.json
```

---

### Android 客户端

#### 推荐：NekoBox

1. 安装 [NekoBox](https://github.com/MatsuriDayo/NekoBoxForAndroid/releases)
2. 添加配置：
   - 类型：NaïveProxy
   - 服务器：proxy.yourdomain.com
   - 端口：443
   - 用户名/密码：从 Manager 获取

#### 备选：v2rayNG（需插件）

需要额外安装 NaïveProxy 插件。

---

### iOS 客户端

#### Shadowrocket

1. 打开 Shadowrocket
2. 添加服务器
3. 类型：HTTP/HTTPS
4. 服务器：proxy.yourdomain.com
5. 端口：443
6. 用户名/密码：从 Manager 获取
7. 启用 TLS

**注意**：iOS 客户端支持有限，建议优先使用其他协议。

---

## 故障排查

### 问题 1: 服务启动失败

**症状**：
```bash
systemctl status naive-proxy-xxx
● naive-proxy-xxx.service - failed
```

**排查步骤**：

1. **查看详细日志**
```bash
journalctl -u naive-proxy-xxx -n 50
```

2. **检查端口占用**
```bash
ss -tlnp | grep :443
```
如果被占用，停止占用服务：
```bash
systemctl stop nginx  # 或其他服务
```

3. **检查配置文件**
```bash
cat /etc/merit/naive/<port-id>.json
```

4. **手动测试**
```bash
/usr/local/bin/caddy-naive run --config /etc/merit/naive/<port-id>.json
```

---

### 问题 2: ACME 证书签发失败

**症状**：日志显示 `failed to obtain certificate`

**原因 1：DNS 未生效**
```bash
# 验证 DNS
dig proxy.yourdomain.com +short

# 应该返回你的服务器 IP
```

**解决方法**：等待 DNS 传播完成（可能需要数小时）

**原因 2：80 端口不可访问**
```bash
# 测试 80 端口
curl -I http://proxy.yourdomain.com

# 检查防火墙
ufw status
iptables -L -n | grep 80
```

**解决方法**：开放 80 端口或改用手动证书

**原因 3：频率限制**

Let's Encrypt 有速率限制：
- 每个域名每周 50 次
- 失败尝试也计数

**解决方法**：等待一段时间后重试，或使用手动证书

---

### 问题 3: 客户端连接超时

**排查步骤**：

1. **测试服务器连通性**
```bash
# 本地测试
curl -I https://proxy.yourdomain.com
```

2. **检查服务状态**
```bash
systemctl status naive-proxy-xxx
```

3. **检查防火墙**
```bash
# 云服务商安全组是否开放 443
# 本地防火墙是否开放 443
ufw status
```

4. **验证证书**
```bash
openssl s_client -connect proxy.yourdomain.com:443 -servername proxy.yourdomain.com
```

5. **检查用户名密码**

确保客户端使用的用户名密码与 Manager 显示的一致。

---

### 问题 4: 证书过期

**手动证书过期**：

Let's Encrypt 证书有效期 90 天。使用 ACME 自动签发会自动续期，手动证书需要：

1. 重新生成/获取证书
2. 在 Manager 重新上传
3. Agent 会自动应用新证书

**ACME 续期失败**：

查看日志：
```bash
journalctl -u naive-proxy-xxx | grep -i renew
```

通常是因为服务重启导致，重启服务即可触发续期。

---

### 问题 5: 性能问题

**症状**：速度慢、延迟高

**优化建议**：

1. **启用 BBR 拥塞控制**
```bash
echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
sysctl -p
```

2. **增加系统限制**
```bash
cat >> /etc/security/limits.conf <<EOF
* soft nofile 51200
* hard nofile 51200
EOF
```

3. **检查带宽**
```bash
# 安装 iftop 监控实时流量
apt install iftop
iftop -i eth0
```

---

## 高级配置

### 1. 多域名配置

一个服务器可以部署多个 NaïveProxy 端口，使用不同域名：

```
端口 443  → proxy1.domain.com
端口 8443 → proxy2.domain.com
端口 9443 → proxy3.domain.com
```

每个端口独立配置证书和账号。

### 2. 伪装网站

NaïveProxy 可以配置回落（fallback），当访问不匹配时显示正常网站。

**未来支持**：Manager 会添加配置界面，当前需要手动编辑配置文件。

### 3. 与 CDN 结合

**Cloudflare CDN**：

不推荐开启橙色云朵（CDN），因为：
- ❌ Cloudflare 会限制长连接
- ❌ 增加延迟
- ❌ 可能被识别为代理流量

**正确使用**：
- DNS 解析设置为「仅 DNS」（灰色云朵）
- 使用 Cloudflare 的 DNS 解析即可
- 不启用 CDN 加速

### 4. 端口复用

如果 443 被其他服务占用，可以：

**方案 A：使用其他端口**
```
端口: 8443
访问: https://proxy.yourdomain.com:8443
```

**方案 B：使用 Nginx SNI 分流**

由 Nginx 监听 443，根据 SNI（域名）分流：
- proxy.domain.com → NaïveProxy (127.0.0.1:10443)
- www.domain.com → 正常网站 (127.0.0.1:8080)

配置示例（未来版本支持）。

---

## 最佳实践

### 1. 域名选择

✅ **推荐**：
- 使用独立域名（非主站域名）
- 选择常见后缀（.com/.net/.org）
- 域名看起来正常（如 cdn-assets.com）

❌ **避免**：
- 使用主站域名（避免关联）
- 明显的代理相关词汇（proxy/vpn/tunnel）
- 过于随机的域名

### 2. 证书管理

✅ **推荐**：
- 优先使用 ACME 自动签发
- 证书到期前 30 天会自动续期
- 定期检查证书状态

❌ **避免**：
- 使用自签名证书（除非测试）
- 手动证书忘记续期

### 3. 安全加固

```bash
# 1. 禁用不必要的服务
systemctl disable apache2
systemctl disable postfix

# 2. 配置防火墙只允许必要端口
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   # SSH
ufw allow 443/tcp  # NaïveProxy
ufw enable

# 3. 启用自动安全更新
apt install unattended-upgrades
dpkg-reconfigure -plow unattended-upgrades
```

### 4. 监控与日志

```bash
# 查看服务状态
systemctl status naive-proxy-*

# 实时日志
journalctl -u naive-proxy-* -f

# 查看连接数
ss -tn | grep :443 | wc -l

# 查看流量
iftop -i eth0 -f "port 443"
```

---

## 快速参考

### 常用命令

```bash
# 服务管理
systemctl start naive-proxy-<id>     # 启动
systemctl stop naive-proxy-<id>      # 停止
systemctl restart naive-proxy-<id>   # 重启
systemctl status naive-proxy-<id>    # 状态

# 日志查看
journalctl -u naive-proxy-<id> -f    # 实时日志
journalctl -u naive-proxy-<id> -n 50 # 最近50行

# 配置文件
cat /etc/merit/naive/<id>.json       # 查看配置
ls -lh /etc/merit/naive/certs/       # 查看证书

# 网络诊断
ss -tlnp | grep :443                 # 端口监听
curl -I https://your-domain.com      # 测试访问
openssl s_client -connect your-domain.com:443  # 测试证书
```

### 配置检查清单

部署前确认：
- [ ] 域名已购买并解析到服务器
- [ ] DNS 解析已生效（nslookup 验证）
- [ ] 443 端口未被占用
- [ ] 防火墙已开放 443（如需 ACME 还要 80）
- [ ] 云服务商安全组已配置
- [ ] Manager 和 Agent 已正常运行

部署后验证：
- [ ] 服务状态为 active (running)
- [ ] 日志无错误信息
- [ ] 证书签发/上传成功
- [ ] 客户端能成功连接
- [ ] 可以正常访问外网

---

## 相关文档

- [多协议支持说明](QUICK_START_MULTI_PROXY.md)
- [故障排查指南](TROUBLESHOOTING.md)
- [开发者指南](DEVELOPER_GUIDE.md)
- [主 README](../README.md)

---

## 获取帮助

遇到问题？

1. 查看 [故障排查指南](TROUBLESHOOTING.md)
2. 查看服务日志：`journalctl -u naive-proxy-<id> -f`
3. 在 [GitHub Issues](https://github.com/HlONGlin/UnifiedProxy/issues) 提问
4. 提问时请附上：
   - 系统版本（`cat /etc/os-release`）
   - 服务日志（隐藏敏感信息）
   - 配置内容（隐藏密码）

---

**祝使用愉快！** 🎉
