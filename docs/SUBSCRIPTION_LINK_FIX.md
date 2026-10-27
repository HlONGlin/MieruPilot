# 订阅链接生成修复说明

## 问题描述

**症状**: 在前端展开节点时,除了 Mieru 协议外,其他协议(NaïveProxy、AnyTLS、ShadowTLS)都显示"节点地址未设置,暂时无法生成链接",即使节点已经部署成功且能正常转发流量。

**根本原因**: `internal/manager/handlers.go` 中的 `buildNodeLinks` 函数使用了统一的 `mieru.ClientHost(n)` 来判断是否能生成链接,但不同协议获取地址的优先级逻辑不同:

| 协议 | 地址获取优先级 |
|------|---------------|
| **Mieru** | `n.Domain` → `n.Address` → `n.SeenIP` |
| **NaïveProxy** | `p.Domain` → `n.Domain` → `n.Address` → `n.SeenIP` |
| **AnyTLS** | `n.Address` → `n.Domain` → `n.SeenIP` |
| **ShadowTLS** | `n.Address` → `n.Domain` → `n.SeenIP` |

当节点没有填写 `Domain` 字段时,`mieru.ClientHost(n)` 会回退到 `n.Address` 或 `n.SeenIP`,但这会导致:
- 如果节点只有 `SeenIP` 而没有 `Address/Domain`,统一的检查会拒绝生成所有链接
- NaïveProxy 的 `p.Domain` 字段被忽略

## 修复方案

### 1. 移除统一的地址检查 (`handlers.go:911-914`)

**修改前**:
```go
out := []link{}
host := mieru.ClientHost(n)
for _, p := range n.Ports {
    if !p.Enabled || host == "" {  // ❌ 统一检查导致误判
        continue
    }
    // ...
}
```

**修改后**:
```go
out := []link{}
for _, p := range n.Ports {
    if !p.Enabled {
        continue
    }
    
    // 生成链接
    var shareLink string
    if proxyType == model.ProxyTypeMieru {
        shareLink = mieru.SimpleLink(n, p)
    } else {
        shareLink = s.generateProxyLink(n, p)
    }
    
    // ✅ 只有在能够生成有效链接时才添加到列表
    if shareLink != "" {
        out = append(out, link{...})
    }
}
```

### 2. 让每个 `SimpleLink` 函数在无法确定地址时返回空字符串

#### `internal/mieru/mieru.go:182`
```go
func SimpleLink(n *model.Node, p *model.Port) string {
    host := ClientHost(n)
    
    // 无法确定地址时返回空字符串
    if host == "" {
        return ""
    }
    
    // ... 原有逻辑
}
```

#### `internal/proxy/naive/naive.go:141`
```go
func SimpleLink(n *model.Node, p *model.Port) string {
    host := strings.TrimSpace(p.Domain)
    if host == "" && n != nil {
        host = strings.TrimSpace(n.Domain)
        if host == "" {
            host = strings.TrimSpace(n.Address)
        }
        if host == "" {
            host = strings.TrimSpace(n.SeenIP)
        }
    }
    
    // 无法确定地址时返回空字符串
    if host == "" {
        return ""
    }
    
    // ... 原有逻辑
}
```

#### `internal/proxy/anytls/anytls.go:77`
```go
func SimpleLink(n *model.Node, p *model.Port) string {
    host := hostOf(n)
    
    // 无法确定地址时返回空字符串
    if host == "" {
        return ""
    }
    
    // ... 原有逻辑
}
```

#### `internal/proxy/shadowtls/shadowtls.go:97`
```go
func SimpleLink(n *model.Node, p *model.Port) string {
    host := hostOf(n)
    
    // 无法确定地址时返回空字符串
    if host == "" {
        return ""
    }
    
    // ... 原有逻辑
}
```

## 修复效果

### 修复前
```
节点配置:
  - Name: "测试节点"
  - Address: "" (未填写)
  - Domain: "" (未填写)
  - SeenIP: "203.0.113.100" (自动获取)

前端显示:
  ✅ Mieru TCP (37065): mierus://user:pass@203.0.113.100?port=37065&...
  ❌ NaïveProxy (443): 节点地址未设置,暂时无法生成链接
  ❌ AnyTLS (23112): 节点地址未设置,暂时无法生成链接
  ❌ ShadowTLS (40900): 节点地址未设置,暂时无法生成链接
```

### 修复后
```
节点配置: (同上)

前端显示:
  ✅ Mieru TCP (37065): mierus://user:pass@203.0.113.100?port=37065&...
  ✅ NaïveProxy (443): naive+https://user:pass@203.0.113.100:443#NaïveProxy
  ✅ AnyTLS (23112): anytls://pass@203.0.113.100:23112?insecure=1&...#AnyTLS
  ✅ ShadowTLS (40900): shadowtls://pass@203.0.113.100:40900?handshake=...#ShadowTLS
```

## 测试验证

```bash
# 运行相关单元测试
go test -v -run TestSimpleLink ./internal/proxy/naive ./internal/proxy/anytls ./internal/proxy/shadowtls ./internal/mieru

# 运行完整测试套件
go test -count=1 ./...

# 结果: 全部通过 ✅
```

## 部署步骤

1. **重新构建二进制**:
   ```bash
   ./build.sh
   ```

2. **升级 Manager**:
   ```bash
   systemctl stop merit-manager
   cp dist/merit-manager-linux-amd64 /usr/local/bin/merit-manager
   systemctl start merit-manager
   ```

3. **验证修复**:
   - 打开管理面板
   - 展开任意节点
   - 确认所有已部署的协议都能显示分享链接

## 影响范围

- ✅ 不影响现有协议的部署和转发功能
- ✅ 不影响 Clash 订阅生成 (`clash.BuildEntries` 使用独立的逻辑)
- ✅ 仅影响前端节点详情中的分享链接显示

## 相关文件

- `internal/manager/handlers.go` (修改)
- `internal/mieru/mieru.go` (修改)
- `internal/proxy/naive/naive.go` (修改)
- `internal/proxy/anytls/anytls.go` (修改)
- `internal/proxy/shadowtls/shadowtls.go` (修改)

---

**修复日期**: 2026-10-27  
**版本**: v1.6.0+  
**优先级**: P2 (不影响核心功能,但影响用户体验)
