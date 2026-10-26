<div align="center">

# 🚀 MieruPilot

**一键生成并管理 mieru 节点的集中控制平台**

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](go.mod)

[特性](#-特性) • [快速开始](#-快速开始) • [架构](#-架构) • [使用指南](#-使用指南) • [开发](#-开发)

---

</div>

## 📋 目录

- [概述](#-概述)
- [特性](#-特性)
- [架构](#-架构)
- [快速开始](#-快速开始)
- [使用指南](#-使用指南)
- [编译构建](#-编译构建)
- [配置说明](#-配置说明)
- [客户端导入](#-客户端导入)
- [流量管理](#-流量管理)
- [自动更新](#-自动更新)
- [常见问题](#-常见问题)
- [与 Zelay 的区别](#-与-zelay-的区别)
- [开发](#-开发)
- [许可证](#-许可证)

---

## 🎯 概述

**MieruPilot** 是一个集中式的 mieru 节点管理平台，采用 **Manager + Agent** 架构设计。一台控制机统一管理所有节点服务器，让你专注于使用，而不是繁琐的配置。

参考 [Zelay](https://github.com/enp6/Zelay) 的设计思路，但目标不是转发，而是**一键生成和管理 mieru 节点**。

### 核心理念

- **一个端口号** → 自动生成用户名、密码、分享链接、Clash 订阅
- **一条命令** → Agent 自动安装、配置、启停 `mita` 服务
- **一个面板** → 集中控制所有主机，告别逐台 SSH

---

## ✨ 特性

### 🎨 傻瓜化操作
- 添加节点后复制一条命令到目标服务器执行
- Agent 自动下载并安装 `mita`
- 无需手动配置，开箱即用

### ⚡ 只填端口
- 面板仅需填写端口号和协议
- 账号密码与分享链接自动生成
- 配置实时下发到节点

### 📊 流量管理
- **流量统计**：实时监控每个端口的流量使用
- **流量限制**：设置每个端口的月度流量配额
- **带宽控制**：限制端口的速度上限
- **自动重置**：流量按月自动归零

### 🔄 自动更新
- 自动检测 GitHub Releases 新版本
- 一键更新 Manager 和 Agent
- 支持版本回滚

### 🎛️ 统一管理
- 单个 Web 面板管理所有主机
- 实时状态监控
- 批量操作支持

### 📦 单文件部署
- Manager 和 Agent 均为 Go 单二进制
- 无需 Docker、无需外部数据库
- 数据持久化到 JSON 文件

### 🔗 多种导入方式
- `mierus://` 简单分享链接
- Clash / mihomo 订阅支持
- 单节点 YAML 导出

### 🤖 自动探测
- 自动获取公网 IP
- 系统和架构自动识别
- `mita` 版本与运行状态实时上报

---

## 🏗️ 架构

```
                    ┌─────────────────────────────┐
   浏览器 ──────────▶│  Manager（控制机，Web 面板） │
                    └──────────────┬──────────────┘
                          HTTP 长轮询 │ 下发配置 / 上报状态
             ┌───────────────────────┼───────────────────────┐
             ▼                       ▼                       ▼
      ┌────────────┐          ┌────────────┐          ┌────────────┐
      │  Agent A   │          │  Agent B   │          │  Agent C   │
      │  mita      │          │  mita      │          │  mita      │
      └────────────┘          └────────────┘          └────────────┘
```

### 组件说明

| 组件 | 职责 | 位置 |
|------|------|------|
| **Manager** | Web 控制台、配置管理、任务下发 | 控制机 |
| **Agent** | 自动安装 mita、执行同步任务、上报状态 | 节点服务器 |
| **mita** | mieru 服务端，实际处理代理流量 | 节点服务器 |

---

## 🚀 快速开始

### 一键安装脚本

使用项目提供的一键脚本完成 Manager 的安装与管理：

```bash
bash <(curl -fsSL https://github.com/HlONGlin/MieruPilot/raw/main/one-click.sh)
```

### 脚本菜单

```
1. 安装或更新 Manager
2. 启动 Manager
3. 停止 Manager
4. 彻底卸载并清除全部数据
5. 重置管理员账号（保留节点数据）
```

### 首次安装

1. 运行一键脚本，选择 `1. 安装或更新 Manager`
2. 脚本会自动：
   - 根据服务器架构下载 Manager 和 Agent
   - 安装到 `/opt/merit`
   - 创建 `systemd` 服务
   - 提示设置管理员账号和密码
3. 安装完成后，脚本会输出带随机安全后缀的管理地址：
   ```
   http://服务器IP:3000/panel/随机字符串
   ```
4. 打开管理地址，使用刚设置的账号登录

### 系统要求

- **控制机**：Linux/Windows/macOS，支持 amd64/arm64
- **节点服务器**：
  - Linux with systemd
  - dpkg (Debian/Ubuntu) 或 rpm (CentOS/RedHat/Rocky)
  - amd64 或 arm64 架构

### 依赖安装

如果提示没有 `curl` 或 `wget`：

```bash
# Ubuntu / Debian
apt-get install -y curl wget

# Alpine
apk add curl wget

# Fedora
dnf install -y curl wget

# CentOS / Rocky / AlmaLinux / Oracle Linux / Amazon Linux
yum install -y curl wget
```

---

## 📖 使用指南

### 1. 登录面板

打开安装完成后输出的管理地址，使用管理员账号登录。

### 2. 添加节点

1. 点击「添加节点」
2. 填写节点名称（如 `hk-1`）
3. 复制页面上的部署命令

### 3. 部署 Agent

在目标服务器以 root 权限执行部署命令：

```bash
curl -fsSL http://你的IP:3000/install.sh | sudo bash -s -- --key <API_KEY>
```

Agent 会自动：
- 下载并安装到 `/opt/merit`
- 创建 systemd 服务并开机自启
- 连接 Manager 并上报状态

### 4. 配置端口

1. 节点上线后，点击节点进入详情页
2. 填写端口号（1025-65535）
3. 选择协议（TCP/UDP）
4. 点击「一键生成」

Manager 会自动：
- 生成随机用户名和密码
- 下发配置到 Agent
- Agent 配置并启动 mita 服务
- 生成分享链接和 Clash 订阅

### 5. 流量管理

在端口设置中可以配置：

- **月度流量限制**：设置每月流量配额（如 100GB）
- **带宽限制**：限制端口速度（如 10MB/s）
- **流量统计**：查看实时使用情况和剩余流量
- **手动重置**：提前重置流量计数

---

## 🛠️ 编译构建

### 目录结构

```
cmd/manager             Manager 入口（Web 面板 + Agent API）
cmd/agent               Agent 入口（部署在节点主机）
internal/manager        HTTP 服务、鉴权、节点/端口 API、任务队列
internal/agent          自动安装 mita、轮询任务、上报状态
internal/mieru          mita 服务端配置、分享链接、Clash 订阅生成
internal/store          JSON 文件持久化
internal/updater        自动更新检查和下载
internal/traffic        流量统计和控制
internal/manager/web    内嵌 Web 面板
```

### 构建要求

- Go 1.24+
- 前端资源已内嵌在 Go binary 中

### 构建命令

```bash
# Linux / macOS
./build.sh

# Windows PowerShell
./build.ps1

# 或使用 Makefile
make
```

### 构建产物

产物位于 `dist/` 目录：

```
dist/merit-manager-linux-amd64
dist/merit-manager-windows-amd64.exe
dist/merit-agent-linux-amd64
dist/merit-agent-linux-arm64
```

> ⚠️ Manager 会通过 `/download/agent` 分发 Agent 二进制给节点，请保持 `dist/` 与 Manager 同目录（或用 `--agent-dir` 指定）。

### 前端开发

前端资源位于：
- `internal/manager/web/index.html`
- `internal/manager/web/assets/style.css`
- `internal/manager/web/assets/app.js`

修改后需要重新构建 Manager。

检查前端代码：

```bash
node scripts/check-frontend.cjs
```

---

## ⚙️ 配置说明

### Manager 启动参数

```bash
./dist/merit-manager-linux-amd64 \
  --addr :3000 \
  --agent-dir dist \
  --data data/merit.json
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--addr` | `:3000` | 面板与 Agent 通信监听地址 |
| `--data` | `data/merit.json` | 数据文件路径，备份它即可备份全部配置 |
| `--agent-dir` | `dist` | 存放 agent 二进制的目录 |
| `--public-url` | 空 | 对外访问地址，如 `http://1.2.3.4:3000`；留空自动用请求 Host |
| `--panel-path` | 自动生成 | 管理面板随机访问路径，例如 `/panel/abc123`；重启后保持不变 |
| `--reset-admin` | false | 交互式重置管理员账号 |
| `--version` | false | 显示版本信息 |

### Agent 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--manager` | 必填 | Manager 地址 |
| `--key` | 必填 | 节点 API Key |
| `--mita-version` | `3.38.0` | 自动安装的 mita 版本 |
| `--mita-mirror` | 空 | GitHub 加速前缀，如 `https://ghproxy.net` |
| `--interval` | `5s` | 出错后的重试间隔 |

Agent 需要 **root** 权限（用于安装软件包和管理 mita 服务）。

### 重置管理员账号

如果忘记账号或密码：

```bash
# 方法 1：使用一键脚本
# 选择「5. 重置管理员账号」

# 方法 2：手动执行
systemctl stop merit-manager
/opt/merit/merit-manager --data /var/lib/merit/merit.json --reset-admin
systemctl start merit-manager
```

重置时节点数据和随机管理地址不会丢失。

### 完全卸载

在一键管理工具中选择 `4`，并输入 `DELETE` 确认。该操作会删除：
- Manager 程序
- 管理员账号
- 节点数据
- 随机管理地址
- 所有配置文件

清除后重新选择 `1` 即可重新安装。

---

## 📱 客户端导入

### 简单分享链接

每个端口都有独立的 `mierus://` 链接：

```
mierus://用户名:密码@服务器?port=端口&protocol=TCP
```

复制链接后在 mieru 客户端中导入。

### Clash 订阅

在「系统设置」页面获取公开订阅地址：

```
http://你的服务器:3000/panel/随机字符/sub?token=订阅令牌
```

将该地址填入 Clash Verge / mihomo 等客户端。

### 单节点 Clash

在节点详情页可以下载该节点的 `clash.yaml` 配置文件。

---

## 📊 流量管理

### 设置流量限制

1. 进入节点详情页
2. 点击端口的「设置」按钮
3. 配置流量参数：
   - **月度流量限制**：单位 GB，0 表示不限制
   - **带宽限制**：单位 MB/s，0 表示不限制

### 查看流量统计

端口卡片会实时显示：
- 已用流量 / 总限额
- 使用百分比
- 剩余流量
- 距离重置时间

### 流量重置规则

- 自动按月重置（每月 1 号）
- 可手动提前重置
- 超出限制后端口自动停用

### Agent 流量上报

Agent 定期上报每个端口的流量使用情况，Manager 汇总并持久化。

---

## 🔄 自动更新

### 检查更新

在「系统设置」页面点击「检查更新」按钮，系统会自动：
1. 从 GitHub Releases 获取最新版本
2. 对比当前版本号
3. 显示更新日志和下载链接

### 更新 Manager

```bash
# 使用一键脚本
bash <(curl -fsSL https://github.com/HlONGlin/MieruPilot/raw/main/one-click.sh)
# 选择「1. 安装或更新 Manager」
```

脚本会在替换前自动备份数据到 `/var/lib/merit/backups/`。

### 更新 Agent

重新执行节点的部署命令，Agent 会自动更新并重启服务。

---

## 🔐 安全建议

### 网络安全

- Manager 面板建议放在 HTTPS 反向代理之后
- 或仅在内网/白名单环境访问
- 使用强管理员密码

### 数据安全

- 数据文件包含节点密钥与账号
- 文件权限已设为 `0600`
- 定期备份 `merit.json`

### 系统安全

- Agent 以 root 运行，请确保 Manager 地址可信
- 定期更新系统和依赖
- 启用防火墙规则

### 备份与恢复

系统设置提供「导出备份」和「选择备份文件」恢复功能：
- 备份限制 16MB
- 包含节点账号及出站凭据
- 恢复前会保存当前配置为 `merit.json.before-restore-*.json`（权限 600）
- 恢复保留当前管理员和管理路径，替换节点与订阅配置

---

## ❓ 常见问题

### Q: 节点显示「未部署」？

**A:** 部署命令未执行成功，或 Agent 连不上 Manager。

解决方案：
1. 检查安全组/防火墙是否放通 Manager 端口
2. 确认部署命令中的 Manager 地址正确
3. 在节点服务器上检查 Agent 日志：
   ```bash
   journalctl -u merit-agent -f
   ```

### Q: 节点在线但 mita 未启动？

**A:** 该节点还没有启用端口。

解决方案：
1. 进入节点详情页
2. 添加端口并点击「一键生成」
3. 等待 Agent 同步并启动 mita

### Q: 自动安装 mita 失败？

**A:** 多为 GitHub 访问受限导致。

解决方案：
1. 在 Agent 部署命令中追加镜像参数：
   ```bash
   --mita-mirror https://ghproxy.net
   ```
2. 或编辑 systemd 服务的 `ExecStart` 添加该参数
3. 重启 Agent 服务

### Q: 支持哪些系统？

**A:** 节点端要求：
- Linux with systemd
- dpkg (Debian/Ubuntu) 或 rpm (CentOS/RedHat/Rocky)
- amd64 或 arm64 架构

Manager 支持 Linux/Windows/macOS。

### Q: 流量统计不准确？

**A:** 可能的原因：
1. Agent 版本过旧，需要更新
2. mita 服务重启导致计数器重置
3. 时间同步问题

建议定期更新 Agent 到最新版本。

### Q: 如何迁移到新服务器？

**A:** 
1. 在旧服务器备份 `merit.json`
2. 在新服务器安装 Manager
3. 停止 Manager 服务
4. 替换 `merit.json` 文件
5. 启动 Manager 服务
6. 确保 Agent 能访问新的 Manager 地址

---

## 🆚 与 Zelay 的区别

| 特性 | Zelay | MieruPilot |
|------|-------|------------|
| **目标** | Realm 转发 | 生成/管理 mieru 节点 |
| **操作** | 配置监听/远程地址 | 只填端口，其余自动生成 |
| **内核** | Realm | mita（見える/mieru） |
| **流量管理** | 基础统计 | 流量限制、带宽控制、月度重置 |
| **自动更新** | ❌ | ✅ |
| **多协议** | TCP/UDP 转发 | TCP/UDP mieru 节点 |

---

## 💻 开发

### 工作原理

1. Manager 为每个节点保存期望配置（启用的端口 + 自动生成的用户）
2. 端口变化时，Manager 把最新完整配置放入该节点的任务队列
3. Agent 长轮询拉取任务，写临时 JSON 后执行 `mita replace config`
4. Agent 执行 `mita stop && mita start` 生效
5. Agent 定期上报状态（在线、mita 是否运行、版本、公网 IP、流量统计）

> ⚠️ Manager 是该节点 mita 配置的唯一来源。请勿手动在同一 mita 上添加端口/用户，否则会被下一次同步覆盖。

### 配置同步机制

- 待执行同步任务保存于 `merit.json` 的 `tasks` 字段
- Manager 重启时恢复，成功确认后从磁盘队列删除
- 一次性测试任务不跨重启恢复
- 每端口实例只在配置或服务文件变化、实例停止时重新启动
- 确认启动成功后记录 `.applied` 配置摘要
- 新配置启动失败时尝试恢复 `.previous` 配置

### 出站检测

- 由所选节点的 Agent 发起
- 测试路径：节点 → SOCKS5 落地机 → 出口 IP 检测服务
- 验证 SOCKS5 握手/认证和代理 CONNECT
- 页面显示测试进度、耗时和出口 IP
- 需要更新 Agent 才能执行该测试
- 结果暂存在 Manager 内存中，重启后可重新测试

### 版本信息

管理程序和 Agent 支持 `--version`：
- 输出版本号、源码提交和提交时间
- 未提交源码构建会标记 `+modified`
- 运行日志和面板也会显示该版本信息

### 升级备份

升级脚本在替换 Manager 前，将数据备份到 `/var/lib/merit/backups/`：
- 目录权限 `700`
- 备份文件权限 `600`
- Agent 部署命令会明确重启服务，确保加载新二进制

---

## 📄 许可证

[MIT License](LICENSE)

---

<div align="center">

**[⬆ 回到顶部](#-mierupilot)**

Made with ❤️ by [HlONGlin](https://github.com/HlONGlin)

</div>
