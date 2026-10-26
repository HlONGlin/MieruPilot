# 前端增强功能集成指南

本文档说明如何将增强的证书管理、节点测试和配置查看功能集成到现有前端。

## 新增功能

### 1. 增强的证书上传
- ✅ 支持查看当前证书信息（主题、颁发者、有效期、剩余天数）
- ✅ 支持自动ACME和手动上传两种模式
- ✅ 上传时自动验证证书格式和有效性
- ✅ 显示证书健康状态（有效/即将过期/已过期）

### 2. 节点测试
- ✅ Ping延迟测试
- ✅ TCP端口连通性测试
- ✅ 针对不同代理类型的特定测试
  - NaïveProxy: HTTPS和TLS证书验证
  - Mieru: UDP端口测试
  - ShadowTLS/AnyTLS: TCP连通性

### 3. 端口测试
- ✅ 单独测试每个端口
- ✅ 显示TLS证书状态
- ✅ 显示错误详情

### 4. 配置查看
- ✅ 节点完整信息（ID、地址、域名、系统等）
- ✅ 所有端口的详细配置
- ✅ 流量使用情况
- ✅ 代理特定配置（ACME、证书状态、握手地址等）
- ✅ 系统信息（Agent版本、Mita状态等）
- ✅ 出站配置

## API端点

### 证书管理

#### 上传/更新证书
```
POST /api/nodes/{id}/ports/{pid}/cert
```

请求体:
```json
{
  "autoACME": false,
  "cert": "-----BEGIN CERTIFICATE-----\n...",
  "key": "-----BEGIN PRIVATE KEY-----\n..."
}
```

响应:
```json
{
  "ok": true,
  "message": "证书配置已更新",
  "autoACME": false,
  "certInfo": {
    "subject": "example.com",
    "issuer": "Let's Encrypt",
    "notBefore": "2024-01-01T00:00:00Z",
    "notAfter": "2024-04-01T00:00:00Z",
    "daysToExpire": 45,
    "dnsNames": ["example.com", "*.example.com"]
  }
}
```

#### 获取证书信息
```
GET /api/nodes/{id}/ports/{pid}/cert
```

响应:
```json
{
  "autoACME": false,
  "hasCert": true,
  "domain": "example.com",
  "certInfo": { /* 同上 */ },
  "certStatus": "valid",
  "certStatusText": "有效（45天后过期）"
}
```

证书状态:
- `valid`: 证书有效
- `expiring`: 7天内过期
- `expired`: 已过期

### 节点测试

#### 测试节点
```
POST /api/nodes/{id}/test
```

响应:
```json
{
  "success": true,
  "timestamp": "2024-10-26T10:00:00Z",
  "pingLatency": 45,
  "tcpConnectable": true,
  "httpsConnectable": true,
  "errors": [],
  "details": {
    "ping": "45ms"
  }
}
```

#### 测试端口
```
POST /api/nodes/{id}/ports/{pid}/test
```

响应:
```json
{
  "portId": "port-123",
  "port": 8443,
  "proxyType": "naive",
  "tcpConnectable": true,
  "tlsValid": {
    "valid": true,
    "issuer": "Let's Encrypt",
    "subject": "example.com",
    "notBefore": "2024-01-01T00:00:00Z",
    "notAfter": "2024-04-01T00:00:00Z",
    "daysToExpire": 45
  },
  "serviceRunning": true,
  "errors": []
}
```

### 配置查看

#### 获取节点配置
```
GET /api/nodes/{id}/config
```

响应:
```json
{
  "nodeInfo": {
    "id": "node-123",
    "name": "节点1",
    "address": "192.168.1.100",
    "domain": "example.com",
    "os": "linux",
    "arch": "amd64",
    "online": true,
    "lastSeen": "2024-10-26T10:00:00Z"
  },
  "ports": [
    {
      "id": "port-123",
      "port": 8443,
      "protocol": "tcp",
      "proxyType": "naive",
      "enabled": true,
      "username": "user123",
      "traffic": {
        "monthlyLimit": 107374182400,
        "used": 1073741824,
        "resetAt": "2024-11-01T00:00:00Z"
      },
      "naive": {
        "domain": "example.com",
        "autoACME": true,
        "hasCert": false
      }
    }
  ],
  "system": {
    "agentVersion": "1.3.0",
    "mitaVersion": "3.7.1",
    "mitaRunning": true
  },
  "egress": {
    "enabled": true,
    "type": "socks5",
    "server": "proxy.example.com",
    "port": 1080,
    "username": "outbound"
  }
}
```

## 前端集成步骤

### 1. 引入新文件

在 `internal/manager/web/index.html` 中添加:

```html
<!-- 在 </head> 前 -->
<link rel="stylesheet" href="/assets/enhancements.css">

<!-- 在 app.js 后 -->
<script src="/assets/app-enhancements.js"></script>
```

### 2. 修改现有 uploadCert 函数

在 `app.js` 中，将现有的 `uploadCert` 函数替换为:

```javascript
async function uploadCert(id) {
  return uploadCertEnhanced(state.node.id, id);
}
```

### 3. 添加测试按钮

在节点详情页添加测试按钮。修改 `detailView` 函数，在节点操作区域添加:

```javascript
// 在节点操作按钮组中添加
<button type="button" class="btn" onclick="testNode('${n.id}')">
  🔍 测试连通性
</button>
<button type="button" class="btn" onclick="viewNodeConfig('${n.id}')">
  📋 查看配置
</button>
```

### 4. 在端口详情添加测试按钮

在端口操作中添加测试选项:

```javascript
// 在端口的更多操作菜单中添加
<option value="test">测试端口</option>
<option value="viewCert">查看证书</option>
```

然后在 `handlePortAction` 中添加处理:

```javascript
switch (action) {
  // ... 其他 case
  case "test":
    testPort(state.node.id, portId);
    break;
  case "viewCert":
    viewCertInfo(state.node.id, portId);
    break;
}
```

### 5. 添加证书查看函数

```javascript
async function viewCertInfo(nodeId, portId) {
  try {
    const info = await api("GET", `/api/nodes/${nodeId}/ports/${portId}/cert`);
    
    if (!info.hasCert) {
      toast("该端口尚未配置证书", "warning");
      return;
    }

    const cert = info.certInfo;
    const statusColor = info.certStatus === "valid" ? "green" : 
                       info.certStatus === "expiring" ? "orange" : "red";
    
    modal(
      "证书信息",
      `<div class="cert-info">
        <h4>TLS 证书详情</h4>
        <p><strong>状态:</strong> <span style="color:${statusColor}">${info.certStatusText}</span></p>
        <p><strong>域名:</strong> ${esc(info.domain)}</p>
        <p><strong>主题:</strong> ${esc(cert.subject)}</p>
        <p><strong>颁发者:</strong> ${esc(cert.issuer)}</p>
        <p><strong>有效期:</strong><br>
           ${new Date(cert.notBefore).toLocaleString()} <br>
           至 ${new Date(cert.notAfter).toLocaleString()}</p>
        <p><strong>剩余天数:</strong> ${cert.daysToExpire} 天</p>
        ${cert.dnsNames?.length ? 
          `<p><strong>支持域名:</strong><br>${cert.dnsNames.join("<br>")}</p>` : ""}
        ${info.autoACME ? 
          '<p class="muted">✨ 使用自动ACME续期</p>' : 
          '<p class="muted">📝 手动上传的证书</p>'}
      </div>`,
      null,
      { showActions: true }
    );
  } catch (e) {
    toast("获取证书信息失败: " + e.message, "error");
  }
}
```

## 样式说明

新增的 CSS 类:

- `.cert-info` - 证书信息卡片
- `.config-table` - 配置信息表格
- `.test-result` - 测试结果容器
- `.modal.wide` - 宽模态框（用于配置查看）
- `small.muted` - 小提示文本
- `.status-icon` - 状态指示图标

Toast 类型:
- `.toast.success` - 成功提示（绿色）
- `.toast.error` - 错误提示（红色）
- `.toast.warning` - 警告提示（黄色）

## 使用示例

### 上传证书
```javascript
// 调用增强的证书上传
uploadCertEnhanced("node-123", "port-456");
```

### 测试节点
```javascript
// 测试整个节点
testNode("node-123");
```

### 测试端口
```javascript
// 测试单个端口
testPort("node-123", "port-456");
```

### 查看配置
```javascript
// 查看完整配置
viewNodeConfig("node-123");
```

### 查看证书信息
```javascript
// 查看证书详情
viewCertInfo("node-123", "port-456");
```

## 注意事项

1. **权限要求**: 所有API端点都需要认证，确保请求包含有效的token
2. **超时设置**: 测试操作可能需要较长时间，前端应显示加载状态
3. **错误处理**: 所有API调用都应有适当的错误处理和用户提示
4. **证书验证**: 证书上传会进行完整验证，包括格式、有效期、密钥匹配
5. **ACME限制**: 自动ACME要求域名已解析且80/443端口可访问

## 调试

开启浏览器控制台可以看到详细的API请求和响应:

```javascript
// 在 app-enhancements.js 开头添加
const DEBUG = true;

// 在API调用前后添加日志
if (DEBUG) console.log("API Request:", method, url, data);
if (DEBUG) console.log("API Response:", result);
```

## 测试清单

- [ ] 证书上传（手动模式）
- [ ] 证书上传（ACME模式）
- [ ] 查看证书信息
- [ ] 证书过期警告
- [ ] 测试节点连通性
- [ ] 测试单个端口
- [ ] NaïveProxy TLS测试
- [ ] 查看节点完整配置
- [ ] 流量信息显示
- [ ] 错误提示正确显示

## 未来扩展

可以考虑添加的功能:

1. **证书自动续期提醒**: 7天内过期时自动提醒
2. **批量测试**: 一次测试所有节点/端口
3. **测试历史**: 保存测试结果历史
4. **性能图表**: 延迟和流量的时序图表
5. **配置导出**: 导出节点配置为JSON/YAML
6. **证书链验证**: 显示完整的证书链
7. **DNS检查**: 验证域名解析是否正确
8. **端口扫描**: 自动检测可用端口

## 相关文件

- `internal/manager/test.go` - 后端测试逻辑
- `internal/manager/handlers.go` - API处理器
- `internal/manager/server.go` - 路由定义
- `internal/manager/web/assets/app-enhancements.js` - 前端增强功能
- `internal/manager/web/assets/enhancements.css` - 增强功能样式
