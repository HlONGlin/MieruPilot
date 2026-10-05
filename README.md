# merit · 一键生成 mieru 节点

参考 [Zelay](https://github.com/enp6/Zelay) 的 **Manager + Agent** 思路，但目标不是转发，而是**一台机器集中控制所有主机、一键生成 mieru 节点**。

你只需要填一个端口号，用户名、密码、分享链接、Clash 订阅全部自动生成；节点服务器上的 `mita` 由 Agent 自动安装、自动配置、自动启停。

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

## 特性

- **傻瓜化**：添加节点后复制一条命令到目标服务器执行，Agent 自动安装 `mita`。
- **只填端口**：在面板填写端口和协议，账号密码与分享链接自动生成并实时下发。
- **统一管理**：一个面板管理所有主机，无需逐个 SSH。
- **单文件**：Manager 与 Agent 都是 Go 单二进制，无 Docker、无外部数据库。
- **多种导入方式**：`mierus://` 简单分享链接、Clash / mihomo 订阅。
- **自动探测**：自动读取公网 IP、系统、架构、`mita` 版本与运行状态。

## 目录结构

```
cmd/manager        Manager 入口（Web 面板 + Agent API）
cmd/agent          Agent 入口（部署在节点主机）
internal/manager   HTTP 服务、鉴权、节点/端口 API、任务队列
internal/agent     自动安装 mita、轮询任务、上报状态
internal/mieru     mita 服务端配置、分享链接、Clash 订阅生成
internal/store     JSON 文件持久化
internal/manager/web  内嵌 Web 面板
```

## 编译

需要 Go 1.24+。

```sh
# Linux / macOS
./build.sh

# Windows PowerShell
./build.ps1

# 或使用 Makefile
make
```

产物位于 `dist/`：

```
dist/merit-manager-linux-amd64
dist/merit-manager-windows-amd64.exe
dist/merit-agent-linux-amd64
dist/merit-agent-linux-arm64
```

> Manager 会把 `dist/merit-agent-linux-<arch>` 通过 `/download/agent` 分发给节点，所以编译后请保持 `dist/` 与 Manager 同目录（或用 `--agent-dir` 指定）。

## 启动 Manager

```sh
./dist/merit-manager-linux-amd64 --addr :3000 --agent-dir dist --data data/merit.json
```

参数：

| 参数 | 默认 | 说明 |
|------|------|------|
| `--addr` | `:3000` | 面板与 Agent 通信监听地址 |
| `--data` | `data/merit.json` | 数据文件，备份它即可备份全部配置 |
| `--agent-dir` | `dist` | 存放 agent 二进制的目录 |
| `--public-url` | 空 | 对外访问地址，如 `http://1.2.3.4:3000`；留空自动用请求 Host |

打开 `http://你的IP:3000`：

1. 首次访问创建管理员账号。
2. 「添加节点」→ 填写名称（如 `hk-1`）。
3. 复制页面上的部署命令，到目标服务器以 root 执行。
4. 节点上线后，在详情页填写端口，点击「一键生成」。

部署命令形如：

```sh
curl -fsSL http://你的IP:3000/install.sh | sudo bash -s -- --key <API_KEY>
```

## Agent 说明

安装脚本会写入 systemd 服务 `/etc/systemd/system/merit-agent.service` 并开机自启。Agent 手动运行参数：

| 参数 | 默认 | 说明 |
|------|------|------|
| `--manager` | 必填 | Manager 地址 |
| `--key` | 必填 | 节点 API Key |
| `--mita-version` | `3.38.0` | 自动安装的 mita 版本 |
| `--mita-mirror` | 空 | GitHub 加速前缀，如 `https://ghproxy.net` |
| `--interval` | `5s` | 出错后的重试间隔 |

Agent 需要 **root** 权限（安装软件包、管理 mita 服务）。

## 客户端导入

- **简单分享链接**：详情页每个端口可复制 `mierus://用户名:密码@服务器?port=端口&protocol=TCP`。
- **Clash 订阅**：设置页提供公开订阅地址 `/sub?token=...`，可直接填入 Clash Verge / mihomo 等。
- **单节点 Clash**：节点详情页可下载该节点的 `clash.yaml`。

## 工作原理

1. Manager 为每个节点保存期望配置（启用的端口 + 自动生成的用户）。
2. 端口变化时，Manager 把最新完整配置放入该节点的任务队列。
3. Agent 长轮询拉取任务，写临时 JSON 后执行 `mita replace config`，再 `mita stop && mita start` 生效。
4. Agent 定期上报状态（在线、mita 是否运行、版本、公网 IP）。

> Manager 是该节点 mita 配置的唯一来源。请勿在同一 mita 上再手动添加端口/用户，否则会被下一次同步覆盖。

## 安全建议

- Manager 面板建议放在 HTTPS 反向代理之后，或仅在内网/白名单访问。
- 数据文件包含节点密钥与账号，权限已设为 `0600`，请妥善备份。
- Agent 以 root 运行，请确保 Manager 地址可信。

## 常见问题

**Q: 节点显示「未部署」？**
A: 部署命令未执行成功，或 Agent 连不上 Manager。检查安全组/防火墙是否放通 Manager 端口。

**Q: 节点在线但 mita 未启动？**
A: 该节点还没有启用端口。先在详情页添加端口并生成。

**Q: 自动安装 mita 失败？**
A: 多为 GitHub 访问受限。可在 Agent 部署命令中追加 `--mita-mirror https://ghproxy.net`（编辑 systemd 服务的 `ExecStart`）。

**Q: 支持哪些系统？**
A: 节点端需要 systemd + dpkg（Debian/Ubuntu）或 rpm（CentOS/RedHat/Rocky）的 Linux，支持 amd64 与 arm64。

## 与 Zelay 的区别

| | Zelay | merit |
|--|-------|-------|
| 目标 | Realm 转发 | 生成/管理 mieru 节点 |
| 操作 | 配置监听/远程地址 | 只填端口，其余自动生成 |
| 内核 | Realm | mita（见える/mieru） |

## License

MIT
