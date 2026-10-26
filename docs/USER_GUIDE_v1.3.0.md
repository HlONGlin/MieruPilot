# 🎯 UnifiedProxy v1.3.0 新功能快速上手

> 3个强大的新功能，让节点管理更简单、更安全、更可靠！

---

## 📋 目录

1. [证书管理](#1-证书管理) - 安全上传和管理 TLS 证书
2. [节点测试](#2-节点测试) - 一键检测节点健康状态
3. [配置查看](#3-配置查看) - 完整查看节点配置信息

---

## 1. 证书管理

### 为什么需要？

使用 **NaïveProxy**、**AnyTLS**、**ShadowTLS** 等协议时需要 TLS 证书。现在可以：
- ✅ 安全上传证书和私钥
- ✅ 自动验证证书有效性
- ✅ 查看证书详细信息
- ✅ 到期提醒

### 如何使用？

#### 方法 1：通过 API（临时方案）

```bash
# 上传证书
curl -X POST http://your-ip:8080/api/nodes/your-node-id/ports/8443/cert \
  -H "Content-Type: application/json" \
  -d @- << EOF
{
  "certificate": "$(cat your-cert.pem | sed ':a;N;$!ba;s/\n/\\n/g')",
  "privateKey": "$(cat your-key.pem | sed ':a;N;$!ba;s/\n/\\n/g')"
}
EOF

# 查看证书信息
curl http://your-ip:8080/api/nodes/your-node-id/ports/8443/cert/info
```

#### 方法 2：通过 Web 界面（即将推出）

1. 进入节点详情页
2. 点击端口旁的"上传证书"按钮
3. 选择证书和私钥文件
4. 系统自动验证并显示证书信息

### 测试证书生成

需要快速测试？生成自签名证书：

```bash
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
  -keyout test-key.pem -out test-cert.pem -days 365 -nodes \
  -subj "/CN=your-domain.com"
```

⚠️ **注意**：自签名证书仅用于测试，生产环境请使用正规 CA 签发的证书。

### 证书信息示例

上传后会显示：

```json
{
  "domains": ["example.com", "*.example.com"],
  "validFrom": "2024-01-01T00:00:00Z",
  "validUntil": "2025-01-01T00:00:00Z",
  "issuer": "Let's Encrypt",
  "keyType": "EC",
  "keySize": 256,
  "daysRemaining": 365
}
```

### 常见问题

**Q: 支持什么格式的证书？**
A: PEM 格式，支持 RSA、EC、PKCS8 等密钥类型。

**Q: 证书和私钥可以单独上传吗？**
A: 不可以，必须同时上传并验证匹配性。

**Q: 证书过期了怎么办？**
A: 重新获取新证书后上传即可，系统会自动替换。

---

## 2. 节点测试

### 为什么需要？

- 🔍 部署后验证节点是否正常
- 🐛 出现问题时快速定位原因
- 💚 定期健康检查

### 如何使用？

#### 方法 1：通过 API

```bash
# 快速测试（约5秒）
curl -X POST http://your-ip:8080/api/nodes/your-node-id/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "quick"}'

# 完整测试（约15秒）
curl -X POST http://your-ip:8080/api/nodes/your-node-id/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "full"}'
```

#### 方法 2：通过 Web 界面（即将推出）

1. 进入节点详情页
2. 点击"测试节点"按钮
3. 选择测试类型（快速/完整）
4. 查看测试结果

### 测试项目

| 测试项 | 检查内容 | 快速测试 | 完整测试 |
|--------|---------|---------|---------|
| 网络连通性 | Ping 测试 | ✅ | ✅ |
| 端口可达性 | TCP 连接测试 | ✅ | ✅ |
| 代理服务 | 服务可用性 | ❌ | ✅ |

### 测试结果示例

```json
{
  "nodeID": "hk-node-1",
  "overallStatus": "success",
  "tests": [
    {
      "name": "网络连通性",
      "status": "success",
      "latency": 15,
      "message": "节点网络连接正常，延迟 15ms"
    },
    {
      "name": "端口 8443 (NaïveProxy)",
      "status": "success",
      "latency": 8,
      "message": "端口可访问"
    },
    {
      "name": "端口 8444 (Mieru)",
      "status": "success",
      "latency": 6,
      "message": "端口可访问"
    }
  ],
  "summary": "节点整体状态良好，所有测试通过",
  "timestamp": "2024-10-26T12:00:00Z"
}
```

### 状态说明

- ✅ **success** - 测试通过
- ⚠️ **warning** - 有警告，但基本可用
- ❌ **failed** - 测试失败

### 故障排查

**网络不通？**
- 检查服务器是否在线
- 检查防火墙设置
- 验证 IP 地址是否正确

**端口无法连接？**
- 确认端口已配置
- 检查防火墙是否开放端口
- 验证服务是否运行

**代理服务异常？**
- 查看 Agent 日志
- 检查配置是否正确
- 尝试重启服务

---

## 3. 配置查看

### 为什么需要？

- 📋 快速查看节点完整配置
- 🔒 安全导出（敏感信息自动隐藏）
- 🐛 故障排查时查看详细信息
- 💾 配置备份和迁移

### 如何使用？

#### 方法 1：通过 API

```bash
# 查看配置（密码自动隐藏）
curl http://your-ip:8080/api/nodes/your-node-id/config

# 查看完整配置（包含密码）
curl http://your-ip:8080/api/nodes/your-node-id/config?includeSensitive=true
```

#### 方法 2：通过 Web 界面（即将推出）

1. 进入节点详情页
2. 点击"查看配置"按钮
3. 查看完整配置信息
4. 可选择"显示密码"查看完整信息
5. 点击"导出"或"复制"保存配置

### 配置信息内容

```json
{
  "nodeID": "hk-node-1",
  "nodeIP": "192.168.1.100",
  "status": "online",
  "lastSeen": "2024-10-26T12:00:00Z",
  "version": "v1.3.0",
  
  "ports": [
    {
      "portNumber": 8443,
      "proxyType": "NaïveProxy",
      "protocol": "TCP",
      "users": [
        {
          "username": "user1",
          "password": "***"
        }
      ],
      "tlsCert": {
        "domains": ["example.com"],
        "validUntil": "2025-01-01T00:00:00Z",
        "daysRemaining": 67,
        "issuer": "Let's Encrypt"
      },
      "traffic": {
        "monthlyLimit": 107374182400,
        "used": 12884901888,
        "remaining": 94489280512
      }
    }
  ],
  
  "egress": {
    "enabled": true,
    "proxyURL": "socks5://***:***@proxy.example.com:1080"
  }
}
```

### 包含信息

- ✅ 节点基本信息（ID、IP、状态、版本）
- ✅ 端口配置（协议类型、用户等）
- ✅ 证书信息（域名、有效期、到期提醒）
- ✅ 流量统计（配额、已用、剩余）
- ✅ 出站代理配置

### 安全说明

**默认模式（脱敏）**：
- 密码显示为 `***`
- 代理 URL 中的认证信息隐藏
- 可安全分享和导出

**完整模式**：
- 显示所有敏感信息
- 需要管理员权限
- 仅用于故障排查和备份

### 使用场景

**场景 1：配置备份**
```bash
# 导出所有节点配置
for node in node-1 node-2 node-3; do
  curl http://your-ip:8080/api/nodes/$node/config > $node-config.json
done
```

**场景 2：故障排查**
```bash
# 查看完整配置，包括密码
curl http://your-ip:8080/api/nodes/problem-node/config?includeSensitive=true | jq
```

**场景 3：配置对比**
```bash
# 对比两个节点配置差异
diff <(curl http://your-ip:8080/api/nodes/node-1/config) \
     <(curl http://your-ip:8080/api/nodes/node-2/config)
```

---

## 🚀 完整使用流程

### 场景：部署新的 NaïveProxy 节点

```bash
# 1. 添加节点（通过 Web 界面）
# 2. 部署 Agent
curl -fsSL http://your-ip:8080/install.sh | sudo bash -s -- --key YOUR_KEY

# 3. 上传证书
curl -X POST http://your-ip:8080/api/nodes/new-node/ports/8443/cert \
  -H "Content-Type: application/json" \
  -d '{
    "certificate": "...",
    "privateKey": "..."
  }'

# 4. 查看证书信息
curl http://your-ip:8080/api/nodes/new-node/ports/8443/cert/info

# 5. 测试节点
curl -X POST http://your-ip:8080/api/nodes/new-node/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "full"}'

# 6. 查看完整配置
curl http://your-ip:8080/api/nodes/new-node/config

# 7. 开始使用 🎉
```

---

## 💡 使用技巧

### 1. 证书到期提醒

定期检查证书状态：

```bash
#!/bin/bash
# check-certs.sh - 检查所有证书到期时间

for node in node-1 node-2 node-3; do
  for port in 8443 8444; do
    echo "检查 $node:$port ..."
    curl -s http://your-ip:8080/api/nodes/$node/ports/$port/cert/info | \
      jq -r '"\(.domains[0]) 还有 \(.daysRemaining) 天到期"'
  done
done
```

### 2. 定时健康检查

添加到 crontab：

```bash
# 每小时测试一次关键节点
0 * * * * curl -X POST http://your-ip:8080/api/nodes/critical-node/test \
  -H "Content-Type: application/json" \
  -d '{"testType": "quick"}' | \
  jq -r 'if .overallStatus != "success" then "节点异常!" else empty end' | \
  mail -s "节点告警" admin@example.com
```

### 3. 配置批量导出

```bash
#!/bin/bash
# backup-configs.sh - 备份所有节点配置

BACKUP_DIR="./backups/$(date +%Y%m%d)"
mkdir -p "$BACKUP_DIR"

for node in $(curl -s http://your-ip:8080/api/nodes | jq -r '.[].id'); do
  echo "备份 $node ..."
  curl -s "http://your-ip:8080/api/nodes/$node/config?includeSensitive=true" \
    > "$BACKUP_DIR/$node.json"
done

echo "备份完成: $BACKUP_DIR"
```

---

## 📊 最佳实践

### 证书管理

- ✅ 使用正规 CA 签发的证书（Let's Encrypt、ZeroSSL等）
- ✅ 证书到期前 30 天开始准备更新
- ✅ 保持私钥安全，不要分享
- ✅ 定期检查证书状态

### 节点测试

- ✅ 部署后立即运行完整测试
- ✅ 定期（每天/每周）运行快速测试
- ✅ 出现问题时运行完整测试
- ⚠️ 不要过于频繁测试（建议间隔 > 1分钟）

### 配置管理

- ✅ 定期备份配置（每周/每月）
- ✅ 使用脱敏模式分享配置
- ✅ 完整模式仅用于备份和故障排查
- ✅ 配置变更后及时记录

---

## ❓ 常见问题

### Q1: 前端界面在哪里？

A: 当前版本（v1.3.0）后端 API 已完成，前端界面正在开发中。临时可以使用 API 或编写脚本。预计在 v1.3.1 推出完整的 Web 界面。

### Q2: 如何获取节点 ID？

A: 通过以下方式：
- Web 界面：节点列表中显示
- API：`curl http://your-ip:8080/api/nodes | jq -r '.[].id'`

### Q3: 测试失败怎么办？

A: 按以下步骤排查：
1. 检查测试报告中的具体错误
2. 查看 Agent 日志：`journalctl -u merit-agent -f`
3. 手动 ping 和 telnet 验证
4. 检查防火墙和安全组设置

### Q4: 可以批量测试多个节点吗？

A: 当前版本不支持，但可以编写脚本：

```bash
for node in node-1 node-2 node-3; do
  echo "测试 $node ..."
  curl -X POST http://your-ip:8080/api/nodes/$node/test \
    -H "Content-Type: application/json" \
    -d '{"testType": "quick"}'
  sleep 60  # 避免过于频繁
done
```

### Q5: 配置导出是否包含流量统计？

A: 是的，包含流量配额、已用和剩余量。

---

## 📖 更多文档

- **完整功能说明**: `docs/ENHANCED_FEATURES.md`
- **API 文档**: `docs/FRONTEND_ENHANCEMENTS.md`
- **发布说明**: `RELEASE_NOTES_v1.3.0_enhanced.md`
- **开发总结**: `docs/DEVELOPMENT_COMPLETE_SUMMARY.md`

---

## 🤝 反馈和支持

遇到问题或有建议？

- **GitHub Issues**: https://github.com/HlONGlin/UnifiedProxy/issues
- **讨论区**: https://github.com/HlONGlin/UnifiedProxy/discussions
- **文档**: https://github.com/HlONGlin/UnifiedProxy/tree/main/docs

---

**享受 UnifiedProxy v1.3.0 的新功能！** 🎉

> 如果这个项目对你有帮助，欢迎给我们一个 ⭐ Star！
