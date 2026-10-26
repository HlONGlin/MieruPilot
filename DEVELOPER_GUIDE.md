# 开发者指南 - 多协议代理支持

## 快速开始

### 环境要求

- Go 1.24+
- Node.js (用于前端检查)
- Linux开发环境（或WSL）

### 构建项目

```bash
# 克隆项目
git clone https://github.com/HlONGlin/MieruPilot.git
cd MieruPilot

# 构建
./build.sh

# 运行测试
go test ./internal/proxy/... -v
```

### 项目结构

```
MieruPilot/
├── cmd/
│   ├── manager/      # Manager主程序
│   └── agent/        # Agent主程序
├── internal/
│   ├── model/        # 数据模型
│   ├── mieru/        # Mieru特定实现
│   ├── proxy/        # 🆕 多代理支持
│   ├── agent/        # Agent逻辑
│   ├── manager/      # Manager逻辑
│   └── store/        # 数据持久化
└── docs/            # 文档
```

## 当前状态

### ✅ 已完成

1. **数据模型** - Port结构支持多代理类型
2. **代理实现** - 三种新代理的配置生成
3. **Agent安装** - 自动下载和安装代理
4. **实例管理** - systemd服务管理
5. **单元测试** - 核心逻辑测试覆盖

### 🔧 进行中

- Manager API集成
- Agent主循环集成
- 前端UI开发

### ⏳ 待开始

- Clash订阅支持
- 流量统计适配
- 端到端测试

## 继续开发指南

### 第一步：Agent集成

**文件：** `internal/agent/agent.go`

需要修改的地方：

#### 1. 扩展 `ensureMita()` 为通用的 `ensureProxies()`

```go
// ensureProxies 确保所有需要的代理都已安装
func (a *Agent) ensureProxies(ports []*model.Port) error {
    needed := make(map[string]bool)
    
    for _, p := range ports {
        proxyType := proxy.GetProxyType(p)
        needed[proxyType] = true
    }
    
    var errs []error
    
    if needed[model.ProxyTypeMieru] {
        if _, err := a.ensureMita(); err != nil {
            errs = append(errs, fmt.Errorf("mita: %w", err))
        }
    }
    
    if needed[model.ProxyTypeNaive] {
        if _, err := a.ensureNaive(); err != nil {
            errs = append(errs, fmt.Errorf("naive: %w", err))
        }
    }
    
    if needed[model.ProxyTypeAnyTLS] {
        if _, err := a.ensureAnyTLS(); err != nil {
            errs = append(errs, fmt.Errorf("anytls: %w", err))
        }
    }
    
    if needed[model.ProxyTypeShadowTLS] {
        if _, err := a.ensureShadowTLS(); err != nil {
            errs = append(errs, fmt.Errorf("shadowtls: %w", err))
        }
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("proxy installation errors: %v", errs)
    }
    
    return nil
}
```

#### 2. 修改 `applyConfig()` 支持多代理

```go
func (a *Agent) applyConfig(cfg *model.DesiredConfig) (string, []model.PortInstanceResult, error) {
    if cfg == nil {
        return "", nil, fmt.Errorf("缺少配置")
    }
    
    // 按代理类型分组
    mieruPorts := []model.Port{}
    otherPorts := []*model.Port{}
    
    for i, binding := range cfg.PortBindings {
        port := &model.Port{
            ID:       cfg.PortIDs[i],
            Port:     binding.Port,
            Protocol: binding.Protocol,
            // ... 其他字段需要从配置中获取
        }
        
        proxyType := proxy.GetProxyType(port)
        if proxyType == model.ProxyTypeMieru {
            mieruPorts = append(mieruPorts, *port)
        } else {
            otherPorts = append(otherPorts, port)
        }
    }
    
    // 处理mieru端口（现有逻辑）
    if len(mieruPorts) > 0 {
        // 现有的mita配置逻辑
    }
    
    // 处理其他代理端口
    for _, port := range otherPorts {
        if err := a.startProxyInstance(port); err != nil {
            return "", nil, err
        }
    }
    
    return "配置应用成功", nil, nil
}
```

### 第二步：Manager API扩展

**文件：** `internal/manager/web_api.go` (或类似的API处理文件)

#### 1. 添加端口创建支持proxyType

```go
// POST /api/nodes/:nodeId/ports
func (s *Server) handleCreatePort(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Port          int    `json:"port"`
        Protocol      string `json:"protocol"`
        ProxyType     string `json:"proxyType"`     // 新增
        Label         string `json:"label"`
        Domain        string `json:"domain"`        // 新增
        AutoACME      bool   `json:"autoACME"`      // 新增
        TLSCert       string `json:"tlsCert"`       // 新增
        TLSKey        string `json:"tlsKey"`        // 新增
        HandshakeAddr string `json:"handshakeAddr"` // 新增
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 验证端口配置
    port := &model.Port{
        ProxyType:     req.ProxyType,
        Domain:        req.Domain,
        AutoACME:      req.AutoACME,
        TLSCert:       req.TLSCert,
        TLSKey:        req.TLSKey,
        HandshakeAddr: req.HandshakeAddr,
    }
    
    if err := proxy.ValidatePort(port); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 创建端口...
}
```

#### 2. 添加证书上传API

```go
// POST /api/nodes/:nodeId/ports/:portId/cert
func (s *Server) handleUploadCert(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Cert string `json:"cert"`
        Key  string `json:"key"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 验证证书格式
    // 保存到Port
    // 触发配置同步
    
    w.WriteHeader(http.StatusOK)
}
```

#### 3. 添加获取支持的代理类型API

```go
// GET /api/proxy-types
func (s *Server) handleGetProxyTypes(w http.ResponseWriter, r *http.Request) {
    types := proxy.SupportedProxyTypes()
    
    response := make([]map[string]string, 0)
    for _, t := range types {
        response = append(response, map[string]string{
            "type": t,
            "name": getProxyTypeName(t),
        })
    }
    
    json.NewEncoder(w).Encode(response)
}

func getProxyTypeName(t string) string {
    names := map[string]string{
        model.ProxyTypeMieru:     "Mieru (默认)",
        model.ProxyTypeNaive:     "NaïveProxy",
        model.ProxyTypeAnyTLS:    "AnyTLS",
        model.ProxyTypeShadowTLS: "ShadowTLS v3",
    }
    return names[t]
}
```

### 第三步：前端UI更新

**文件：** `internal/manager/web/assets/app.js`

#### 1. 添加代理类型选择

```javascript
// 在添加端口的表单中
function renderPortForm() {
    return `
        <form id="addPortForm">
            <label>端口号</label>
            <input type="number" name="port" required />
            
            <label>协议</label>
            <select name="protocol">
                <option value="TCP">TCP</option>
                <option value="UDP">UDP</option>
            </select>
            
            <label>代理类型</label>
            <select name="proxyType" id="proxyTypeSelect" onchange="updateProxyFields()">
                <option value="mieru">Mieru (默认)</option>
                <option value="naive">NaïveProxy</option>
                <option value="anytls">AnyTLS</option>
                <option value="shadowtls">ShadowTLS v3</option>
            </select>
            
            <div id="proxySpecificFields"></div>
            
            <label>标签</label>
            <input type="text" name="label" />
            
            <button type="submit">创建端口</button>
        </form>
    `;
}
```

#### 2. 动态显示代理特定配置

```javascript
function updateProxyFields() {
    const proxyType = document.getElementById('proxyTypeSelect').value;
    const container = document.getElementById('proxySpecificFields');
    
    let html = '';
    
    if (proxyType === 'naive') {
        html = `
            <label>域名 *</label>
            <input type="text" name="domain" required />
            
            <label>证书模式</label>
            <select name="certMode" onchange="updateCertFields()">
                <option value="acme">自动ACME</option>
                <option value="manual">手动上传</option>
            </select>
            
            <div id="certFields"></div>
        `;
    } else if (proxyType === 'shadowtls') {
        html = `
            <label>握手服务器 *</label>
            <input type="text" name="handshakeAddr" 
                   placeholder="cloudflare.com:443" required />
            <small>推荐: cloudflare.com:443, www.google.com:443</small>
        `;
    }
    
    container.innerHTML = html;
}

function updateCertFields() {
    const certMode = document.querySelector('[name="certMode"]').value;
    const container = document.getElementById('certFields');
    
    if (certMode === 'manual') {
        container.innerHTML = `
            <label>证书文件</label>
            <textarea name="tlsCert" rows="10" placeholder="-----BEGIN CERTIFICATE-----"></textarea>
            
            <label>私钥文件</label>
            <textarea name="tlsKey" rows="10" placeholder="-----BEGIN PRIVATE KEY-----"></textarea>
        `;
    } else {
        container.innerHTML = '';
    }
}
```

#### 3. 显示代理类型图标

```javascript
function getProxyTypeIcon(proxyType) {
    const icons = {
        'mieru': '🚀',
        'naive': '🌐',
        'anytls': '🔐',
        'shadowtls': '🔒'
    };
    return icons[proxyType] || '❓';
}

function renderPortCard(port) {
    const icon = getProxyTypeIcon(port.proxyType || 'mieru');
    return `
        <div class="port-card">
            <div class="port-header">
                <span class="proxy-icon">${icon}</span>
                <span class="port-number">${port.port}</span>
                <span class="proxy-type">${port.proxyType || 'mieru'}</span>
            </div>
            <!-- 其他信息 -->
        </div>
    `;
}
```

### 第四步：Clash订阅支持

**文件：** `internal/mieru/mieru.go`

扩展 `ClashYAML()` 函数：

```go
// BuildClashEntriesMultiProxy 构建支持多代理类型的Clash配置
func BuildClashEntriesMultiProxy(nodes []*model.Node) []ClashProxyEntry {
    var entries []ClashProxyEntry
    
    for _, n := range nodes {
        host := ClientHost(n)
        if host == "" {
            continue
        }
        
        for _, p := range n.Ports {
            if !p.Enabled {
                continue
            }
            
            proxyType := mieru.GetProxyType(p)
            
            switch proxyType {
            case model.ProxyTypeMieru:
                entries = append(entries, buildMieruEntry(n, p))
            case model.ProxyTypeNaive:
                entries = append(entries, buildNaiveEntry(n, p))
            case model.ProxyTypeAnyTLS:
                entries = append(entries, buildAnyTLSEntry(n, p))
            case model.ProxyTypeShadowTLS:
                entries = append(entries, buildShadowTLSEntry(n, p))
            }
        }
    }
    
    return entries
}

func buildNaiveEntry(n *model.Node, p *model.Port) ClashProxyEntry {
    name := fmt.Sprintf("%s-%s", n.Name, p.Label)
    return ClashProxyEntry{
        Type:     "http",
        Name:     name,
        Server:   p.Domain,
        Port:     p.Port,
        Username: p.Username,
        Password: p.Password,
        TLS:      true,
        SNI:      p.Domain,
    }
}

// 类似地实现 buildAnyTLSEntry() 和 buildShadowTLSEntry()
```

### 第五步：测试

#### 单元测试

```bash
# 测试代理逻辑
go test ./internal/proxy/... -v

# 测试agent逻辑
go test ./internal/agent/... -v

# 测试manager逻辑
go test ./internal/manager/... -v
```

#### 集成测试

创建 `internal/integration/proxy_test.go`:

```go
func TestNaiveProxyDeployment(t *testing.T) {
    // 启动测试manager
    // 创建naive端口
    // 验证配置下发
    // 检查agent安装naive
    // 验证服务运行
}
```

#### 手动测试清单

- [ ] 创建Mieru端口（验证向后兼容）
- [ ] 创建NaïveProxy端口（ACME模式）
- [ ] 创建NaïveProxy端口（手动证书）
- [ ] 创建AnyTLS端口
- [ ] 创建ShadowTLS端口
- [ ] 同一节点混合部署
- [ ] 分享链接导入客户端
- [ ] Clash订阅导入
- [ ] 证书上传功能
- [ ] 证书自动续期

## 调试技巧

### Agent日志

```bash
# 查看agent日志
journalctl -u merit-agent -f

# 查看特定代理服务日志
journalctl -u naive-proxy-{portId} -f
journalctl -u anytls-{portId} -f
journalctl -u shadowtls-{portId} -f
```

### 配置文件位置

```
/etc/merit/
├── naive/
│   ├── {portId}.json
│   └── certs/
│       └── {portId}/
│           ├── cert.pem
│           └── key.pem
├── anytls/
│   └── {portId}.json
└── shadowtls/
    └── {portId}.json
```

### 常见问题

#### 1. 代理安装失败

**检查：**
```bash
# 检查下载URL
curl -I https://github.com/klzgrad/naiveproxy/releases/latest

# 检查磁盘空间
df -h

# 检查权限
ls -la /usr/local/bin/
```

#### 2. systemd服务启动失败

**检查：**
```bash
# 查看服务状态
systemctl status naive-proxy-{portId}

# 查看详细错误
journalctl -xe

# 检查配置文件语法
cat /etc/merit/naive/{portId}.json | jq .
```

#### 3. ACME证书申请失败

**检查：**
```bash
# 域名解析
nslookup example.com

# 80端口可达性
curl -I http://example.com

# 防火墙规则
ufw status
```

## 贡献代码

### 提交PR前检查

- [ ] 代码通过 `go fmt`
- [ ] 所有测试通过
- [ ] 添加了新功能的测试
- [ ] 更新了相关文档
- [ ] commit消息清晰

### Commit消息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

类型：
- `feat`: 新功能
- `fix`: Bug修复
- `docs`: 文档更新
- `test`: 测试相关
- `refactor`: 重构

示例：
```
feat(proxy): add Hysteria2 support

- Implement Hysteria2 config generation
- Add installation script
- Create systemd service template

Closes #123
```

## 资源链接

### 官方文档

- [Mieru](https://github.com/enfein/mieru)
- [NaïveProxy](https://github.com/klzgrad/naiveproxy)
- [AnyTLS](https://github.com/anytls/anytls)
- [ShadowTLS](https://github.com/ihciah/shadow-tls)

### 相关项目

- [Clash Meta](https://github.com/MetaCubeX/Clash.Meta)
- [Caddy](https://caddyserver.com/)
- [systemd](https://www.freedesktop.org/wiki/Software/systemd/)

## 联系方式

- GitHub Issues: 提交bug和功能请求
- Pull Requests: 贡献代码
- Discussions: 技术讨论

---

**更新时间：** 2024-12-xx
**维护者：** MieruPilot Community
