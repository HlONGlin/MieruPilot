# 协议部署可行性复核（2026-10-27）

## 修复状态

2026-10-27 已针对当前工作区修复本报告复现的五项缺陷，并补充能力限制和回归测试。下方“初始复核”保留修复前证据；其中问题描述不代表修复后的代码状态。线上服务和现有 dist 文件尚未更新，必须重新构建并同时升级 Manager、Agent，再重试同步与刷新订阅。

| 问题 | 已实施的修复 | 验证方式 |
| --- | --- | --- |
| NaïveProxy 认证错误 | 按上游 `[][]byte` 编码 Basic token，由 JSON 再编码；普通 Caddy 不满足安装检查 | 上游类型解码测试；固定上游 fork + Caddy 2.11.7 + 发布所需两个补丁，真实 TLS CONNECT 转发通过，错误/缺失凭据被拒绝，旧编码仍被拒绝 |
| 安装检测与启动路径不一致 | NaïveProxy、AnyTLS、ShadowTLS、SS 后端只接受实际 ExecStart 固定路径；Caddy 必须包含 forward_proxy 模块 | PATH 诱饵、空文件、目录、执行权限和模块缺失/执行失败回归 |
| ShadowTLS 托管后端冲突 | 新增/修改端口、启用与备份恢复统一校验 TCP 保留端口，失败不持久化；UDP 可使用相同数字 | API 原子回滚和恢复回归测试 |
| 连通检测误判 | 按传输检测，UDP 显示 unknown；通用 UDP 必须收到响应；监听连通与认证转发、Agent 服务状态分离 | UDP-only、UDP 无响应/响应、普通 TCP 假代理、无 SSH/80、IPv6 回环测试 |
| 混合同步短路 | Mieru 安装/应用失败后继续处理其他协议；聚合错误，每个未成功端口都提供明确结果；独立协议状态不依赖 mita 安装成功 | 安装失败和 daemon-reload 失败两条回归路径 |

其他处理：

- 非 Mieru 流量值不参与累计或超额停用；API 拒绝正数限额，允许清除旧限额；前端隐藏无效设置并显示能力未支持。启用的显式非 Mieru 出站规则返回 400；新建/编辑路由只列出 Mieru。
- Manager 正确构造 IPv6 host:port；AnyTLS 使用通配监听。ShadowTLS 在 IPv6 可用且 bindv6only=0 时选双栈监听，否则保留 IPv4，避免 IPv4 客户端回退失败。实际 Linux IPv6 数据链路仍待验收。
- 面板提示独立 ShadowTLS URI 不含后端 SS 凭据，优先使用 mihomo 订阅。README 明确 tar/xz、NaïveProxy amd64 自动安装与 Agent amd64/arm64 发布边界。
- ARM 手动构建 NaïveProxy 的错误提示已纠正：固定 fork 依赖其 CADDY_VERSION 和 Caddy 补丁，不能套用通用 xcaddy 命令。临时集成构建按上游流程完成，项目 go.mod/go.sum 未改变。

可选上游集成测试：设置 `NAIVE_TEST_CADDY` 为固定版本且包含发布补丁的可执行文件，然后运行 `go test -count=1 -run TestUpstreamCONNECTAuthentication -v ./internal/proxy/naive`。未设置该变量时显式跳过，不算完成上游实测。本次已显式运行并通过。测试只在临时配置开放回环目标，生产 ACL 未放宽；测试进程会退出清理。

## 修复后验证

- Windows/amd64，Go 1.27.0；`go test -count=1 ./...` 全量通过，`go vet ./...` 通过。
- `node scripts/check-frontend.cjs` 通过；检查 app.js 与 app-enhancements.js 语法，并验证非 Mieru 流量展示及操作限制。
- 上游 NaïveProxy 集成测试显式执行通过：TLS HTTP/1.1 CONNECT 后收到本地目标 HTTP 内容；无凭据、错误凭据及旧编码均拒绝。该测试不覆盖 Chromium 指纹、HTTP/2、HTTP/3 或 ACME。
- Linux amd64、arm64 的 Manager 和 Agent 四项交叉编译均通过；产物位于系统临时目录 `mierupilot-deployment-fixed-linux-*`，未覆盖 dist。
- `git diff --check` 和修改 Go 文件的 gofmt 检查通过；项目依赖文件无变化，集成测试子进程已退出。

## 初始复核：结论与范围

**当前源码不能判定为所有协议均可直接部署并正常使用。** Mieru、AnyTLS、ShadowTLS 的主体方案具备可行性，但有部署前提和缺陷；NaïveProxy 的认证配置存在确定性错误，需要先修复。

对象是当前工作区，包含检查开始前已有的未提交和未跟踪文件，不只是 Git HEAD，也不代表线上发布版或 dist 二进制与源码一致。初始审计阶段只新增此报告；后续修复已修改源码和测试，未替换发布二进制。

| 协议/组件 | 判断 | 条件与边界 |
| --- | --- | --- |
| Mieru / Mita | 有条件可行；未发现确定性的核心配置阻断 | Linux + systemd + root + dpkg/rpm；自动安装 amd64/arm64；面板限制 1025–65535；TCP/UDP 均有实现，但 UDP 健康检查误判 |
| NaïveProxy | **当前不能通过可用性验收** | 认证编码错误；自动安装只支持 amd64；需要带 forwardproxy 的 Caddy、域名和证书；项目限制自动 ACME 为 443，手动证书可用其他端口；不进入 mihomo 订阅 |
| AnyTLS | 有条件可行；官方 Windows 二进制 TCP 链路实测通过 | 自动安装 amd64/arm64；临时自签证书，导出配置跳过验证；监听 TCP，UDP 业务走 UoT；当前只绑定 IPv4 |
| ShadowTLS v3 | 有条件可行；托管后端已经接入源码 | 需要可达的握手站点和 Shadowsocks 后端；自动后端用 shadowsocks-rust；TCP 隧道；存在后端端口冲突校验缺口 |
| Shadowsocks 后端 | 托管部署机制具备可行性 | 不是独立面板协议；仅回环监听；Manager 生成稳定凭据，Agent 先启动后端再启动 ShadowTLS；支持三种经典 AEAD cipher |

## 初始复核：已复现的问题

### 1. P1：NaïveProxy 认证字段与上游语义不匹配

位置：internal/proxy/naive/naive.go:74–80。生成器把一层 Base64 编码的账号密码作为字符串写入 auth_credentials。固定上游 klzgrad/forwardproxy@v2.11.7-naive2 将该字段声明为 [][]byte，JSON 解码会先解掉这一层 Base64；其后 checkCredentials 却与请求中仍为 Base64 文本的 Basic token 比较。

复现：配置经上游类型解码后得到 audit-user:audit-password，请求比较值是 YXVkaXQtdXNlcjphdWRpdC1wYXNzd29yZA==。二者不等，正常客户端提交正确账号密码仍无法认证。现有 TestGenerateConfigUsesAuthCredentials 只检查 JSON 字符串，固定了错误预期。

修复方向：按上游 [][]byte 结构序列化已编码的 Basic token，由 JSON 再进行一层 Base64 编码；补上上游类型解码后的比较测试，并以真实 CONNECT 请求验收。

### 2. P1：安装检测路径与 systemd 启动路径不一致

位置：internal/agent/install_naive.go:56、install_anytls.go:54、install_shadowtls.go:40，以及 internal/proxy/shadowsocks/shadowsocks.go:75。固定路径不存在时，PATH 中任意同名程序仍会使检测返回已安装；ExecStart 却始终引用 /usr/local/bin 下的固定路径。

复现：仅把 caddy 放到临时 PATH，保持 /usr/local/bin/caddy-naive 不存在，naiveInstalled() 仍返回 true，后续服务引用不存在的程序。发行版 Caddy 还可能没有 forwardproxy 模块。其他三项存在相同控制流，未逐个运行真实服务。

修复方向：安装检测和启动统一使用经过验证的实际程序路径，并验证可执行性及必要模块。

### 3. P1：新增监听可占用已有 ShadowTLS 后端端口

位置：internal/manager/handlers.go:504–508；分配器位于 internal/manager/shadowtls_backend.go:71。分配后端会避开已有监听，反方向新增监听校验却只比较 p.Port，不检查已分配的 BackendAddr。

复现：先创建 TCP 443 的托管 ShadowTLS，后端得到 127.0.0.1:50000；再创建 TCP 50000 的 AnyTLS，API 返回 200。AnyTLS 的 0.0.0.0:50000 与回环 TCP 50000 不能同时绑定，哪项失败取决于启动顺序。

修复方向：新增、恢复、启用监听都应检查后端占用，按 TCP/UDP 实际传输区分冲突。

### 4. P2：Mieru TCP/UDP 检测误报，且不能证明业务可用

位置：internal/manager/test.go:103–155、183–210。TestPort 无条件探测 TCP，又对所有 Mieru 端口探测 UDP，没有按 port.Protocol 选择传输。

本机复现：正常 UDP-only 回环监听返回 UDPReachable:true，同时 Errors 包含 TCP 不可达。UDP 检测只验证发送 test 是否成功，没有读取响应；UDP 写成功不能证明远端服务存在。AnyTLS / ShadowTLS 的 ServiceRunning 也只根据 TCP 可连推断，未验证认证、后端和真实请求。

修复方向：按配置选择传输，区分端口连通与代理业务可用，加入协议级请求验收。

### 5. P2：Mieru 失败会短路其他协议的混合同步

位置：internal/agent/agent.go:363–380。先处理 Mieru，安装或实例应用失败时直接 return，后面的其他协议循环不再执行。

复现：清空临时 PATH 令 mita 安装不可用，同时提交 Mieru 和另一个代理分支；结果为空，后者没有分发、没有产生端口结果。此测试验证控制流，不是 Linux 真实混合部署实测。

修复方向：按协议/端口收集失败并继续处理独立实例，再汇总错误；其他协议循环已有类似机制。

## 初始复核：其他边界

- IPv6：AnyTLS、ShadowTLS 的 ListenAddr 固定为 0.0.0.0，不会监听 IPv6。分享链接支持 IPv6 不代表服务端能接收 IPv6。Manager 探测拼接 host:port 时也没有正确处理 IPv6 方括号。
- 出站和限额：egressForPort 只用于 mita，其他协议启动未接入面板出站规则。真实流量采集只读取 mita metrics，其他代理无相应流量值，不能宣称其配额控制已经闭环。
- ShadowTLS 链接：独立 shadowtls URI 仅带传输参数，不带后端 SS 密码和 cipher；用户仍需额外配置数据代理。托管后端 mihomo 订阅生成 SS + shadow-tls 插件参数，是更完整的路径。
- ARMv7：下载函数接受 armv7 资源，但 Agent 安装脚本和发布流水线只提供 amd64/arm64，不能视为整体支持 ARMv7 一键安装。
- 解压依赖：NaïveProxy、shadowsocks-rust 使用 tar.xz，需要 tar 和可用的 xz 解压能力，脚本不会自动安装。
- 公网条件：代码校验域名、证书和私钥，不等于 DNS、云安全组、防火墙、ACME 挑战已验证可达。
- 状态检查：Type=simple 后立即 is-active 只是瞬时检查，不代表持续稳定和业务可用。

## 初始复核：实际验证结果

- 环境：Windows/amd64，Go 1.27.0。
- go test -count=1 ./...：现有全量测试通过，未使用测试结果缓存。
- go vet ./...：通过。
- node scripts/check-frontend.cjs：通过。
- Manager、Agent 分别交叉编译到 Linux amd64、arm64：四个构建通过；产物在系统临时目录，未替换 dist。
- GitHub API 核对固定版本 Mita 3.38.0、forwardproxy v2.11.7-naive2、AnyTLS v0.0.13、ShadowTLS v0.2.25、shadowsocks-rust v1.18.2：对应资产存在，amd64/arm64 文件名匹配当前代码。
- 实际下载检查：NaïveProxy amd64 归档含 caddy，shadowsocks-rust amd64 含 ssserver，AnyTLS Linux amd64 含 anytls-server，ELF 架构均为 x86-64；ShadowTLS amd64 返回有效 ELF 头，Mita amd64 DEB 返回有效 ar 包头。
- 官方 AnyTLS v0.0.13 Windows 二进制使用项目相同服务端参数和分享 URI 格式，完成 SOCKS5 → AnyTLS client → TLS server → 本机 HTTP 的真实 TCP 请求，得到预期响应；进程已停止。
- 临时复现测试确认上述五项缺陷，临时测试文件均已删除。

## 未完成的生产验收

本机没有可用的 Linux systemd 测试环境，未执行真实 dpkg/rpm 安装和 systemd 启动、重启、开机恢复。没有目标 VPS、域名和公网证书挑战条件，未完成 Mieru TCP/UDP 公网转发、NaïveProxy ACME 签发/续期、ShadowTLS + SS + 客户端完整公网验收。

AnyTLS Windows TCP 通过只证明该固定版本的参数、认证和数据路径可用，不代表 Linux Agent 安装、UoT、mihomo 或 IPv6 已实测通过。资源存在、归档可解压、交叉编译通过均不能替代真实服务验收。

上述五项已修复；下一步需在 Debian/Ubuntu 与 RPM 系发行版、amd64/arm64 上验收客户端请求、重启、密码与证书更新、停用和删除清理。

## 上游证据

本次在线读取以下源码：

- https://raw.githubusercontent.com/klzgrad/forwardproxy/v2.11.7-naive2/forwardproxy.go：AuthCredentials 类型与比较逻辑。
- https://raw.githubusercontent.com/klzgrad/forwardproxy/v2.11.7-naive2/caddyfile.go：EncodeAuthCredentials。
- https://raw.githubusercontent.com/anytls/anytls-go/v0.0.13/cmd/server/main.go：服务参数和临时证书；同目录 inbound_tcp.go、outbound_tcp.go：UoT 分支。
- https://raw.githubusercontent.com/ihciah/shadow-tls/v0.2.25/src/main.rs：v3 server 参数。
- https://raw.githubusercontent.com/enfein/mieru/v3.38.0/build/package/mita/amd64/debian/lib/systemd/system/mita.service 及 DEBIAN/postinst：用户、程序路径和目录。
- MetaCubeX/mihomo 的 Meta 活动分支：adapter/outbound/mieru.go、anytls.go、shadowsocks.go，transport/shadowtls/client.go。该分支未固定不可变提交，不能替代逐版本兼容性验收。

旧 PROTOCOL_FEASIBILITY_ASSESSMENT.md、SUMMARY_PROTOCOL_ASSESSMENT.md 的全部可行、测试覆盖完整/100% 不能作为当前验收结论：测试通过率不等于代码覆盖率，本次已复现现有测试遗漏的功能故障。
