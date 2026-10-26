# 多协议代理支持实现计划

## 概述
为 MieruPilot 添加 NaïveProxy、AnyTLS 和 ShadowTLS v3 代理节点部署支持。

## 实现策略
根据用户选择:
1. **每个端口独立选择代理类型** - 同一节点可混合部署不同代理
2. **TLS证书双模式** - 支持自动ACME和手动上传
3. **完全向后兼容** - 现有mieru节点无需迁移

## 已完成的工作

### 1. 数据模型扩展 ✅
- `internal/model/model.go`:
  - 添加代理类型常量 (`ProxyTypeMieru`, `ProxyTypeNaive`, `ProxyTypeAnyTLS`, `ProxyTypeShadowTLS`)
  - Port结构体新增字段:
    - `ProxyType` - 代理类型
    - `TLSCert`, `TLSKey` - TLS证书(NaïveProxy)
    - `Domain` - 域名(用于ACME或SNI)
    - `HandshakeAddr` - 握手服务器(ShadowTLS)
    - `AutoACME` - 自动证书申请开关

### 2. 代理实现包 ✅
创建了三个新代理的实现:

#### NaïveProxy (`internal/proxy/naive/naive.go`)
- 基于Caddy + forwardproxy
- 支持ACME自动证书
- 支持手动证书上传
- 生成Caddy JSON配置
- 生成分享链接: `naive+https://user:pass@domain:port`

#### AnyTLS (`internal/proxy/anytls/anytls.go`)  
- 命令行参数配置
- 支持TCP/UDP
- 生成分享链接: `anytls://password@host:port`

#### ShadowTLS v3 (`internal/proxy/shadowtls/shadowtls.go`)
- JSON配置文件
- 需要握手服务器地址
- 生成分享链接: `shadowtls://password@host:port?handshake=example.com:443`

### 3. 统一代理管理器 ✅
`internal/proxy/manager.go`:
- `GetProxyType()` - 获取代理类型(默认mieru向后兼容)
- `GenerateShareLink()` - 统一生成分享链接
- `ValidatePort()` - 验证端口配置
- `SupportedProxyTypes()` - 列出支持的代理类型

### 4. Agent安装逻辑 ✅
为每种代理创建了安装模块:
- `internal/agent/install_naive.go` - NaïveProxy安装
- `internal/agent/install_anytls.go` - AnyTLS安装  
- `internal/agent/install_shadowtls.go` - ShadowTLS安装

### 5. Agent实例管理 ✅
`internal/agent/instances_proxy.go`:
- `startProxyInstance()` - 启动代理实例
- `stopProxyInstance()` - 停止代理实例
- 为每个代理类型创建systemd服务
- 配置文件管理
- 证书文件管理(NaïveProxy)

## 待完成的工作

### 1. Agent集成 🔧
修改 `internal/agent/agent.go`:
```go
// 在 ensureMita() 后添加
func (a *Agent) ensureProxies() {
    // 检测需要的代理类型
    // 自动安装缺失的代理
}

// 扩展 applyConfig() 
// 支持不同代理类型的实例管理
```

### 2. Manager API扩展 🔧
修改 `internal/manager/server.go`:
- 端口创建API支持proxyType参数
- TLS证书上传API (POST `/api/nodes/:id/ports/:portId/cert`)
- 获取支持的代理类型列表API

### 3. 分享链接生成 🔧
修改 `internal/manager/web_api.go`:
```go
// 修改分享链接生成逻辑
func (s *Server) handleShareLink(w http.ResponseWriter, r *http.Request) {
    port := getPortFromRequest()
    
    switch proxy.GetProxyType(port) {
    case model.ProxyTypeMieru:
        // 现有的mieru逻辑
    case model.ProxyTypeNaive:
        link = naive.SimpleLink(node, port)
    // ... 其他类型
    }
}
```

### 4. Clash订阅支持 🔧
扩展 `internal/mieru/mieru.go` 的 `ClashYAML()`:
```yaml
# NaïveProxy
- name: naive-node
  type: http
  server: example.com
  port: 443
  username: user
  password: pass
  tls: true
  sni: example.com

# ShadowTLS
- name: shadowtls-node
  type: shadowtls
  server: host
  port: 443
  password: pass
  handshake: example.com:443
```

### 5. 前端UI更新 🔧
修改 `internal/manager/web/assets/app.js`:

#### 添加端口时选择代理类型
```javascript
// 在添加端口表单中
<select id="proxyType">
  <option value="mieru">Mieru (默认)</option>
  <option value="naive">NaïveProxy</option>
  <option value="anytls">AnyTLS</option>
  <option value="shadowtls">ShadowTLS v3</option>
</select>
```

#### 代理特定配置
```javascript
// 根据选择的代理类型显示不同配置项
if (proxyType === 'naive') {
  // 显示域名输入
  // TLS证书选择: ACME自动 / 手动上传
}
if (proxyType === 'shadowtls') {
  // 显示握手服务器地址输入
}
```

#### 证书上传界面
```javascript
// NaïveProxy的证书上传
<div id="certUpload">
  <input type="file" id="certFile" />
  <input type="file" id="keyFile" />
  <button onclick="uploadCert()">上传证书</button>
</div>
```

### 6. 流量统计适配 🔧
不同代理的流量统计方式:
- Mieru: 现有的mita metrics
- NaïveProxy: Caddy日志解析或metrics API
- AnyTLS: 需要添加流量统计支持
- ShadowTLS: 需要添加流量统计支持

可能需要为非mieru代理实现独立的流量统计逻辑。

### 7. 测试 🔧
- 单元测试各代理配置生成
- 集成测试端到端部署
- 验证分享链接格式
- 验证Clash订阅兼容性

## 安装包URL配置

### NaïveProxy
- 需要Caddy with forwardproxy插件
- 建议: 提供预编译二进制或使用xcaddy构建脚本
- URL: `https://github.com/klzgrad/naiveproxy/releases`

### AnyTLS  
- 官方releases: `https://github.com/anytls/anytls/releases`
- 需要验证架构命名

### ShadowTLS
- 官方releases: `https://github.com/ihciah/shadow-tls/releases`
- 文件名: `shadowtls-{arch}-unknown-linux-musl`

## 向后兼容性

### 数据兼容
- 旧数据中Port没有proxyType字段
- `proxy.GetProxyType()` 默认返回 `model.ProxyTypeMieru`
- 无需数据迁移

### API兼容
- 现有API继续工作,proxyType为可选参数
- 不指定时默认mieru

### 配置兼容
- 现有配置逻辑保持不变
- 新代理类型使用独立配置路径

## 部署注意事项

### 端口要求
- NaïveProxy with ACME: 需要80端口用于HTTP-01验证
- 或使用DNS-01验证(需要DNS API支持)

### 依赖
- NaïveProxy: 无额外依赖
- AnyTLS: 无额外依赖
- ShadowTLS: 无额外依赖

### 权限
- 证书文件权限: 0600
- 配置文件权限: 0600
- 二进制文件: 0755

## 下一步行动

1. **完成Agent集成** - 整合新代理到agent主循环
2. **实现Manager API** - 添加API端点
3. **更新前端UI** - 添加代理类型选择和配置界面
4. **添加Clash订阅** - 支持多种代理类型导出
5. **流量统计** - 为新代理实现流量监控
6. **测试验证** - 完整的端到端测试

## 预估工作量
- Agent集成: 4-6小时
- Manager API: 3-4小时  
- 前端UI: 6-8小时
- 测试调试: 4-6小时
- 总计: 17-24小时

## 风险点
1. **证书管理复杂性** - ACME自动续期需要仔细处理
2. **流量统计不统一** - 不同代理的监控方式不同
3. **Clash兼容性** - 需要验证各代理类型的Clash支持
4. **下载URL可靠性** - 需要确保所有代理的下载源稳定

## 建议
1. 先完成一个代理类型的完整流程作为示例
2. 优先实现NaïveProxy(使用场景最广)
3. 流量统计可以先使用基础实现,后续优化
4. 提供详细的用户文档说明各代理的使用场景
