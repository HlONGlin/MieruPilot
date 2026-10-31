# UnifiedProxy v1.6.3

发布日期：2026-10-31

## 更新内容

- 优化 ShadowTLS 分享链接，统一输出为 SS + `shadow-tls` 插件格式：

  ```text
  ss://加密方式:后端密码@服务器:端口?plugin=shadow-tls%3Bhost%3D握手域名%3Bpassword%3DTLS密码%3Bversion%3D3#备注
  ```

- 链接同时携带后端 Shadowsocks 凭据与 ShadowTLS v3 参数，可直接导入支持该插件格式的客户端。
- 握手地址自动提取域名，避免把握手服务端口错误写入 `host` 参数。
- 支持特殊字符、IPv6 地址和节点备注编码。
- 后端密码未配置时不生成不完整的链接；后端加密方式为空时默认使用 `aes-256-gcm`。

## 升级说明

- 本版本基于 v1.6.2，保留客户端配置导出、端口详情/测试/编辑、AnyTLS SNI/指纹和 NaïveProxy QUIC 功能。
- 升级后请重新生成并导入 ShadowTLS 分享链接；旧的 `shadowtls://` 链接不会自动携带后端 SS 认证信息。
- ShadowTLS 仍为 TCP 隧道，不提供 UDP 转发。

## 下载

请从 GitHub Release 下载对应平台的 Manager、Agent 和 `SHA256SUMS`，下载后校验文件完整性。
