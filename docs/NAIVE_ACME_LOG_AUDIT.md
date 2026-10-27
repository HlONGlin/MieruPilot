# NaïveProxy ACME 日志来源审计

## 审计日期
2026-10-27

## 审计目标
确定 NaïveProxy (Caddy) 的 ACME 事件日志的来源和获取方式。

## Unit 名称构造

### 代码位置
`internal/agent/instances_proxy.go:50-61`

### 函数实现
```go
// proxyServiceName returns the systemd unit name (without ".service") of a proxy
// instance by protocol, using the same generation logic that createNaiveInstance
// and other installers use.
func proxyServiceName(protocol string, portID string) string {
	switch protocol {
	case "naive":
		return "merit-naive-" + safePortID(portID)
	case "anytls":
		return "merit-anytls-" + safePortID(portID)
	case "shadowtls":
		return "merit-shadowtls-" + safePortID(portID)
	default:
		return ""
	}
}
```

### 历史兼容性
根据代码注释和实际部署，存在两种前缀：
1. **新前缀** (当前生成): `merit-naive-<portID>`
2. **旧前缀** (存量单元): `naive-proxy-<portID>`

**ACME 监控必须兼容两种前缀**，定位 unit 时需要尝试：
1. 先尝试 `merit-naive-<portID>.service`
2. 如不存在，回退到 `naive-proxy-<portID>.service`

## Systemd Unit 配置

### Unit 文件位置
`/etc/systemd/system/merit-naive-<portID>.service`

### 创建代码位置
`internal/agent/instances_proxy.go:263-312`

### 日志相关配置

从 `writeProxyUnit` 函数生成的 unit 配置中（第 291-292 行）：

```ini
[Service]
Type=simple
ExecStart=/usr/local/bin/caddy-naive run --config /etc/merit/naive/<portID>.json
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
```

**关键发现**:
- `StandardOutput=journal` - 标准输出重定向到 journal
- `StandardError=journal` - 标准错误重定向到 journal
- **没有** `SyslogIdentifier` - 使用 systemd 默认标识符（服务名）

## Caddy 日志配置

### 配置生成代码位置
`internal/proxy/naive/naive.go:69-137`

### Caddy JSON 配置结构

```json
{
  "admin": {
    "disabled": true
  },
  "apps": {
    "http": { ... },
    "tls": {
      "automation": {
        "policies": [
          {
            "subjects": ["example.com"],
            "issuers": [
              {
                "module": "acme",
                "ca": "https://acme-staging-v02.api.letsencrypt.org/directory"  // 仅 staging 时
              }
            ]
          }
        ]
      },
      "certificates": { ... }
    }
  }
}
```

**关键发现**:
- **没有** `logging` 配置块
- Caddy 使用默认日志行为：输出到 stdout/stderr
- ACME 事件由 `tls.issuance.acme` logger 输出

## Caddy 默认日志行为

根据 Caddy 官方文档和测试，无 `logging` 配置时：
- 日志格式: **JSON 结构化日志**
- 输出目标: stdout/stderr
- 日志级别: INFO 及以上

ACME 相关 logger：
- `tls.issuance.acme` - ACME 流程（签发、续期）
- `tls.obtain` - 证书获取（包括失败）
- `tls.renew` - 证书续期事件

## 日志读取方式

### 命令行方式
```bash
# 查询特定 unit 的最近日志
journalctl -u merit-naive-<portID>.service -n 1000 --no-pager

# 带时间窗口
journalctl -u merit-naive-<portID>.service --since "1 day ago" --no-pager

# 仅 ACME 相关日志（通过 grep）
journalctl -u merit-naive-<portID>.service -n 1000 --no-pager | grep -E 'tls\.(issuance|obtain|renew)'
```

### Go 代码方式
```go
// 读取最近 1000 行日志
cmd := exec.Command("journalctl", "-u", unitName+".service", "-n", "1000", "--no-pager", "-o", "cat")
output, err := cmd.Output()
if err != nil {
    return nil, fmt.Errorf("读取 journal 失败: %w", err)
}

lines := strings.Split(string(output), "\n")
```

## ACME 事件日志示例

### 成功签发/续期
```json
{"level":"info","ts":1730000000.123,"logger":"tls.issuance.acme","msg":"certificate obtained successfully","domain":"example.com"}
```

### 失败事件
```json
{"level":"error","ts":1730000000.456,"logger":"tls.obtain","msg":"could not get certificate from issuer","error":"acme: authorization error for example.com: 403 urn:ietf:params:acme:error:unauthorized: Invalid response from http://example.com/.well-known/acme-challenge/..."}
```

### 续期触发
```json
{"level":"info","ts":1730000100.789,"logger":"tls.renew","msg":"attempting certificate renewal","domain":"example.com"}
```

## 日志解析策略

### 解析逻辑
1. 逐行读取 journal 输出（最近 N 行或时间窗口）
2. 尝试 JSON 解析每一行
3. 忽略非 JSON 行和无关日志行
4. 匹配以下条件：
   - `logger` 包含 `tls.issuance.acme` 或 `tls.obtain` 或 `tls.renew`
   - `msg` 包含关键词：
     - 成功: `"certificate obtained successfully"`, `"renewed certificate"`
     - 失败: `"could not get certificate"`, `"authorization error"`, `"challenge failed"`

### 容错处理
- 非 JSON 行: 跳过
- 无 `logger` 字段: 跳过
- 无 `ts` 字段: 使用当前时间
- 无 `error` 字段: 从 `msg` 提取错误信息

## 结论

### 日志来源
**systemd journal**, 通过 `StandardOutput=journal` 和 `StandardError=journal` 配置。

### 读取方式
`journalctl -u <unitName>.service`

### Unit 名称
- **新部署**: `merit-naive-<portID>.service`
- **存量部署**: `naive-proxy-<portID>.service`（需兼容）

### 日志格式
**JSON 结构化日志**（Caddy 默认）

### 解析范围限制
- **行数限制**: 最近 1000 行（避免性能问题）
- **时间窗口**: 最近 7 天（可配置）
- **解析容错**: 必须容忍非 JSON 行和无关日志

### 安全注意事项
- journal 需要 root 权限或将 agent 用户加入 `systemd-journal` 组
- 不解析敏感字段（不记录证书私钥等）

---

**审计人员**: Claude Sonnet 5.5  
**审计依据**: 代码审查（instances_proxy.go, naive.go）+ Caddy 官方文档  
**审计结果**: **已确认 - 日志走 journal，格式为 JSON**
