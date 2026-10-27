# 客户端配置导出与直连参数

## 代理入口操作

代理入口的“更多操作”提供：

- **详细信息**：查看参数和分享链接，下载完整客户端 JSON（需要登录）。
- **测试联通性**：从 Manager 发起检查；安装协议测试工具后，验证代理认证和真实 HTTPS 转发，并显示出口 IP。
- **编辑信息**：修改监听端口、凭据和协议参数，校验通过后下发 Agent。

配置和订阅包含代理凭据，应仅交给可信客户端。

## AnyTLS

“TLS 握手域名（SNI）”与“TLS 指纹”会一致写入分享链接、mihomo YAML 和 sing-box JSON。

支持 Chrome、Firefox；已有记录默认保持 Chrome。可根据实际线路测试选择指纹和 SNI，不能保证某一组合适用于所有网络。

托管 anytls-go 使用临时自签名证书，导出配置沿用 insecure 设置。

## NaïveProxy

客户端传输可选 HTTPS / TCP 或 QUIC / UDP。已有记录默认保持 HTTPS。

- HTTPS 分享链接使用 naive+https://，原生客户端 JSON 使用 https://。
- QUIC 分享链接使用 naive+quic://，原生客户端 JSON 使用 quic://。
- QUIC 需要服务器与云安全组放行监听端口的 UDP；TCP 监听仍用于 HTTPS 和证书校验。
- 两种模式均保留证书校验，证书必须匹配配置域名。
- v2rayN 需要支持对应分享链接的版本及 NaïveProxy 内核。修改参数后应重新导入。

## ShadowTLS

独立 ShadowTLS URI 无法表达完整 Shadowsocks 后端凭据。请下载包含两层 outbound 的 sing-box JSON，或使用完整 mihomo YAML。自定义 JSON 在 v2rayN 中选择 sing-box 内核。

## Manager 代理测试工具

在 Manager 的 agent-dir 下建立 probe-tools 目录，并放置具有执行权限、匹配服务器架构的官方客户端：

| 文件名 | 用途 |
| --- | --- |
| mihomo | Mieru、AnyTLS、ShadowTLS 实际转发测试 |
| naive | NaïveProxy HTTPS / QUIC 实际转发测试 |

Manager 还需要 PATH 中的 curl。工具缺失时仅提供基础端口/证书检测，不将其报告为协议转发成功。

测试启动临时本机客户端，通过 SOCKS 请求固定 HTTPS 地址 https://api.ipify.org 并验证返回的出口 IP；限制并发为 4，结束后清理临时进程和配置。

面板测试的来源是 Manager，不能替代用户实际网络中的客户端测试。
