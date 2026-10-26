# 多协议代理故障排查指南

## AnyTLS 入口能 ping 通但出口不通

### 可能原因

1. **端口未正确监听**
   ```bash
   # 检查端口是否监听
   ss -tlnp | grep <端口号>
   # 或
   netstat -tlnp | grep <端口号>
   ```

2. **服务未正常运行**
   ```bash
   # 检查服务状态
   systemctl status anytls-<port-id>
   
   # 查看服务日志
   journalctl -u anytls-<port-id> -f
   ```

3. **命令行参数错误**
   ```bash
   # 手动测试 AnyTLS
   /usr/local/bin/anytls server -l 0.0.0.0:<端口> -p <密码>
   ```

4. **防火墙/安全组规则**
   ```bash
   # 检查 iptables
   iptables -L -n | grep <端口>
   
   # 检查 firewalld
   firewall-cmd --list-all
   ```

5. **客户端配置错误**
   - 确认客户端使用正确的协议类型
   - 确认密码无特殊字符转义问题
   - 确认服务器地址和端口正确

### 调试步骤

#### 1. 检查服务状态
```bash
systemctl status anytls-<port-id>
```

如果显示 `failed` 或 `inactive`，查看详细日志：
```bash
journalctl -u anytls-<port-id> -n 50
```

#### 2. 测试本地连接
```bash
# 在服务器上测试
curl -v --socks5 127.0.0.1:<端口> http://example.com
```

#### 3. 检查网络连通性
```bash
# 从客户端测试 TCP 连接
telnet <服务器IP> <端口>
# 或
nc -zv <服务器IP> <端口>
```

#### 4. 查看进程详情
```bash
# 查找 AnyTLS 进程
ps aux | grep anytls

# 查看进程打开的文件和端口
lsof -p <PID>
```

#### 5. 抓包分析
```bash
# 在服务器上抓包
tcpdump -i any -n port <端口> -A
```

### 常见问题

#### Q: 服务启动失败
**A**: 检查配置文件语法和权限
```bash
# 检查配置文件
cat /etc/merit/anytls/<port-id>.json

# 手动运行查看错误
/usr/local/bin/anytls server -l 0.0.0.0:<端口> -p <密码>
```

#### Q: 端口被占用
**A**: 检查并释放端口
```bash
# 查看占用端口的进程
lsof -i :<端口>

# 停止冲突服务
systemctl stop <conflicting-service>
```

#### Q: 客户端连接超时
**A**: 检查防火墙和安全组
```bash
# 临时开放端口（测试用）
iptables -I INPUT -p tcp --dport <端口> -j ACCEPT

# 永久添加规则
firewall-cmd --permanent --add-port=<端口>/tcp
firewall-cmd --reload
```

---

## ShadowTLS 链接导入 v2rayN 失败

### 问题说明

v2rayN 目前**不原生支持** ShadowTLS v3 协议。需要手动配置或使用支持的客户端。

### 解决方案

#### 方案 1: 使用 ShadowTLS 专用客户端

**Windows**:
- 下载 [shadowtls-windows-client](https://github.com/ihciah/shadow-tls/releases)
- 配置本地 SOCKS5 代理
- v2rayN 连接到本地代理

**配置示例** (`client-config.json`):
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

启动客户端:
```bash
shadowtls-client.exe --config client-config.json
```

#### 方案 2: 使用 Clash.Meta

Clash.Meta 支持 ShadowTLS v3:

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

#### 方案 3: 手动配置 v2rayN (需要插件)

1. 安装 ShadowTLS 插件到 v2rayN
2. 使用自定义配置:

```json
{
  "protocol": "shadowtls",
  "settings": {
    "servers": [{
      "address": "<服务器IP>",
      "port": <端口>,
      "password": "<密码>",
      "version": 3,
      "sni": "cloudflare.com"
    }]
  }
}
```

### 推荐客户端

| 客户端 | 平台 | ShadowTLS v3 支持 |
|--------|------|-------------------|
| Clash.Meta | All | ✅ 原生支持 |
| shadowtls-client | All | ✅ 专用客户端 |
| v2rayN | Windows | ⚠️ 需要插件 |
| NekoBox | Android | ✅ 原生支持 |
| Shadowrocket | iOS | ⚠️ 部分支持 |

---

## NaïveProxy 证书问题

### ACME 自动签发失败

1. **确认域名解析**
   ```bash
   dig <你的域名> +short
   nslookup <你的域名>
   ```

2. **确认 80/443 端口开放**
   ```bash
   # ACME HTTP-01 需要 80 端口
   # ACME TLS-ALPN-01 需要 443 端口
   ss -tlnp | grep -E ':(80|443)'
   ```

3. **查看 Caddy 日志**
   ```bash
   journalctl -u naive-proxy-<port-id> -f
   ```

### 手动证书上传

如果 ACME 失败，可以使用自己的证书:

1. **生成自签名证书**（测试用）:
   ```bash
   openssl req -x509 -newkey rsa:4096 -nodes \
     -keyout key.pem -out cert.pem -days 365 \
     -subj "/CN=<你的域名>"
   ```

2. **上传证书**:
   - 在管理界面选择"手动上传证书"
   - 粘贴 `cert.pem` 和 `key.pem` 内容

3. **验证证书**:
   ```bash
   openssl x509 -in /etc/merit/naive/certs/<port-id>.crt -noout -text
   ```

---

## 通用调试命令

### 查看所有代理服务状态
```bash
systemctl list-units | grep -E '(naive|anytls|shadowtls)'
```

### 批量查看日志
```bash
journalctl -u 'naive-proxy-*' -u 'anytls-*' -u 'shadowtls-*' -f
```

### 测试端口连通性
```bash
# TCP
nc -zv <服务器IP> <端口>

# UDP
nc -zuv <服务器IP> <端口>
```

### 检查配置文件
```bash
# NaïveProxy
ls -lh /etc/merit/naive/
cat /etc/merit/naive/<port-id>.json

# AnyTLS
ls -lh /etc/merit/anytls/
cat /etc/merit/anytls/<port-id>.json

# ShadowTLS
ls -lh /etc/merit/shadowtls/
cat /etc/merit/shadowtls/<port-id>.json
```

### 性能诊断
```bash
# 查看 CPU 使用率
top -p $(pgrep -d',' -f 'anytls|naive|shadowtls')

# 查看网络连接
ss -tn | grep <端口>

# 查看带宽使用
iftop -i <网卡>
```

---

## 联系支持

如果以上方法都无法解决问题，请收集以下信息:

1. **系统信息**
   ```bash
   uname -a
   cat /etc/os-release
   ```

2. **服务日志**
   ```bash
   journalctl -u <service-name> -n 100 > service.log
   ```

3. **配置文件**（隐藏敏感信息）
   ```bash
   cat /etc/merit/<proxy-type>/<port-id>.json | sed 's/password.*/password: ***/'
   ```

4. **网络状态**
   ```bash
   ss -tlnp > network.txt
   iptables -L -n > firewall.txt
   ```

提交 Issue 时附上这些文件。
