# 协议部署可行性检查 - 执行摘要

**检查日期**: 2026-10-27  
**检查对象**: UnifiedProxy v1.5.2 工作区代码  
**检查人**: holi  

---

## 📊 总体评估

### 当前状态

| 维度 | 状态 | 说明 |
|------|------|------|
| **工作区源码** | ✅ 可行 | 5 项关键缺陷已修复,测试通过 |
| **dist 二进制** | ⚠️ 过时 | 与源码不一致,存在已知错误 |
| **生产部署** | ⚠️ 待验收 | 需要 Linux + systemd + 公网环境验证 |

### 协议可行性判定

| 协议 | 工作区 | 生产就绪度 | 关键限制 |
|------|--------|------------|----------|
| **Mieru** | ✅ 可行 | ⭐⭐⭐⭐ | Linux + systemd + dpkg/rpm |
| **NaïveProxy** | ✅ 已修复 | ⭐⭐⭐ | 仅 amd64 自动安装,需域名证书 |
| **AnyTLS** | ✅ 可行 | ⭐⭐⭐ | 临时自签证书,无身份验证 |
| **ShadowTLS** | ✅ 可行 | ⭐⭐⭐ | 需握手站点和 SS 后端 |

---

## 🔴 关键发现

### 1. 已修复的缺陷 (v1.5.2)

**P1 级别** (阻塞性):
1. ✅ **NaïveProxy 认证错误** - 上游类型不匹配,正常客户端无法通过认证
2. ✅ **安装检测路径不一致** - PATH 中的同名程序误报已安装
3. ✅ **ShadowTLS 后端端口冲突** - 新增监听可占用已有后端端口

**P2 级别** (影响体验):
4. ✅ **Mieru UDP 连通性检测误判** - TCP/UDP 混合检测不按协议区分
5. ✅ **混合同步短路问题** - Mieru 安装失败导致其他协议不被处理

**修复验证**:
```bash
✅ go test -count=1 ./...     # 全量通过
✅ go vet ./...                # 通过  
✅ node scripts/check-frontend.cjs  # 通过
✅ 上游 NaïveProxy 集成测试  # 通过
✅ Linux amd64/arm64 交叉编译  # 通过
```

### 2. 未完成的验收

❌ **生产环境验收** (必需):
- Linux + systemd 真实部署
- dpkg/rpm 安装验证
- 公网 VPS + 域名 + ACME
- Mieru TCP/UDP 公网客户端请求
- NaïveProxy ACME 签发/续期
- ShadowTLS + SS 完整公网链路
- IPv6 数据链路验证

### 3. 能力边界

**流量统计与限额**: 仅 Mieru

**IPv6 支持**:
- AnyTLS: 固定 0.0.0.0, 不监听 IPv6
- ShadowTLS: 只在 IPv6 可用且 bindv6only=0 时双栈
- Manager: IPv6 地址拼接需修复

**证书与依赖**:
- NaïveProxy: 需域名、443 端口或手动证书
- AnyTLS: 临时自签证书,客户端跳过验证 ⚠️
- ShadowTLS: 需握手站点和后端服务

---

## 🎯 立即行动项

### 升级部署 (优先级: 🔴 高)

```bash
# 1. 重新构建
./build.sh

# 2. 同时升级 Manager 和 Agent
systemctl stop merit-manager
cp dist/merit-manager-linux-amd64 /usr/local/bin/merit-manager
systemctl start merit-manager

# 在每个节点:
systemctl stop merit-agent
cp dist/merit-agent-linux-amd64 /usr/local/bin/merit-agent
systemctl start merit-agent

# 3. 面板重试同步
# 访问面板,对每个端口点击"重试同步"

# 4. 刷新客户端订阅
# 重新导入 Clash 订阅链接
```

### 生产验收 (优先级: 🔴 高)

```bash
# 使用自动化验收脚本
cd scripts/acceptance
cp test_config.env.example test_config.env
nano test_config.env  # 填写实际节点信息
./quick_check.sh
```

**预期结果**:
- ✅ 所有端口连通性测试通过
- ✅ NaïveProxy TLS 握手成功
- ✅ Clash 订阅生成包含所有代理

---

## 🛡️ 生产加固计划

### v1.6.1 (2 周内)

**A1. 消除 AnyTLS 无身份校验**
- 方案: 复用 NaïveProxy 的同域名证书
- 效果: 客户端可验证服务端身份,防止中间人攻击

**A2. 证书上传进度可视化**
- 显示上传、验证、同步、重启各阶段进度
- 失败时提供明确的错误信息和修复建议

**B1. 证书续期可观测**
- 到期前 30/7/3 天告警
- 续期事件日志和面板展示
- 续期演练脚本

**C1. Agent 版本检查**
- 握手时验证版本兼容性
- 面板显示版本不匹配告警
- 一键升级 Agent 功能

**D1. 回归测试脚本**
- 固化当前手动测试为可重复脚本
- 升级后一键验收

详见: [生产加固路线图](./PRODUCTION_HARDENING_ROADMAP.md)

---

## 📚 相关文档

### 审计与修复
- [协议部署审计 2026-10-27](./PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md) - 缺陷详情和修复验证

### 实施方案
- [生产加固路线图](./PRODUCTION_HARDENING_ROADMAP.md) - 安全性和可观测性改进计划
- [证书上传进度实施方案](./CERT_UPLOAD_PROGRESS.md) - UI/UX 增强设计

### 用户指南
- [验收测试 README](../scripts/acceptance/README.md) - 自动化测试使用指南
- [多协议快速开始](./QUICK_START_MULTI_PROXY.md) - 部署教程
- [WebDAV 云备份](./WEBDAV_BACKUP.md) - 配置备份功能

---

## ✅ 结论

### 可行性判定

**工作区代码**: ✅ **具备可行性**

所有协议的主体方案在源码层面可行,5 项关键缺陷已修复并通过测试。但需要注意:

1. **dist 二进制过时** - 必须重新构建和部署
2. **生产验收未完成** - Linux + systemd + 公网环境需要现场验证
3. **能力边界明确** - 流量统计仅 Mieru,IPv6 支持有限,AnyTLS 无身份验证

### 下一步

**立即执行**:
1. ✅ 重新构建 dist 二进制
2. ✅ 升级 Manager 和 Agent (必须同时升级)
3. ✅ 运行验收测试脚本
4. ✅ 生产环境真实验证

**短期改进** (v1.6.x):
1. AnyTLS 身份校验增强
2. 证书上传进度可视化
3. 证书到期监控
4. Agent 版本检查

**持续改进**:
- 参考生产加固路线图逐步实施
- 收集生产环境反馈
- 完善文档和最佳实践

---

**检查完成时间**: 2026-10-27 22:30  
**下次复核**: v1.6.0 发布前  
**负责人**: holi  

---

<div align="center">

**项目状态: 可行,待生产验收** ✅

</div>
