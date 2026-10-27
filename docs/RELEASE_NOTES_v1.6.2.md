## 更新内容

- 代理端口“更多操作”新增 **详细信息、测试联通性、编辑信息**。
- 支持下载受登录保护的完整客户端 JSON；ShadowTLS 导出包含 Shadowsocks 后端及 ShadowTLS v3 两层配置。
- 修复 AnyTLS SNI 和 TLS 指纹在分享链接、mihomo YAML、sing-box JSON 之间不一致的问题；新增 Chrome / Firefox 指纹选择。
- NaïveProxy 客户端传输新增 HTTPS / TCP 与 QUIC / UDP 选择，生成对应的 naive+https / naive+quic 分享链接及原生客户端 JSON。
- 端口编辑支持监听端口、凭据和协议参数，并校验冲突及非法配置。
- 安装 Manager 协议测试工具后，可验证真实代理认证及 HTTPS 转发，并显示出口 IP；工具缺失时仅报告基础检测。
- 增加配置导出、参数校验、认证保护和菜单操作回归测试。

## 升级与使用

- 备份 Manager 数据文件后更新；旧记录保持 AnyTLS Chrome 指纹及 NaïveProxy HTTPS 默认值。
- 修改 SNI、指纹或传输后，请在客户端重新导入分享链接或下载新的 JSON。
- QUIC 需要服务器和云安全组放行监听端口的 UDP；证书仍严格校验，域名须与证书匹配。
- v2rayN 使用支持相应协议的内核：AnyTLS / ShadowTLS 完整 JSON 使用 sing-box；NaïveProxy 使用 NaïveProxy 内核。
- 托管 anytls-go 沿用临时自签名证书与 insecure 配置；本版本未改变其证书策略。
- Manager 发起的连通性测试不能替代用户实际网络测试，不保证所有线路直连可用。
- 协议测试工具需另行放入 Manager agent-dir 下的 probe-tools（mihomo、naive），并安装 curl；本 Release 不打包第三方客户端。

## 下载

| 平台 | Manager | Agent |
| --- | --- | --- |
| Linux x86_64 | merit-manager-linux-amd64 | merit-agent-linux-amd64 |
| Linux ARM64 | merit-manager-linux-arm64 | merit-agent-linux-arm64 |
| Windows x86_64 | merit-manager-windows-amd64.exe | — |
| macOS Intel | merit-manager-darwin-amd64 | — |
| macOS Apple Silicon | merit-manager-darwin-arm64 | — |

下载后使用 SHA256SUMS 校验文件完整性。

[客户端配置指南](https://github.com/HlONGlin/UnifiedProxy/blob/v1.6.2/docs/CLIENT_CONFIGURATION.md) · [安装与升级](https://github.com/HlONGlin/UnifiedProxy/blob/v1.6.2/README.md)
