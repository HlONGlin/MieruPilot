# ShadowTLS 自动后端

ShadowTLS 只伪装 TLS 握手，真正的代理由后端数据服务（shadowsocks）完成。后端地址留空时，面板会为该端口分配并托管一个仅本机可访问的 shadowsocks 后端。

## 行为

| 填写 | 结果 |
|------|------|
| 后端地址留空 | 托管：Manager 分配 `127.0.0.1:端口`、生成密码、默认加密 `aes-256-gcm`，Agent 部署 `ssserver` |
| 填写后端地址 | 手动：原样使用，Agent 不部署、不改动。填写的密码与加密方式仅用于生成订阅 |

托管模式下所有值由 Manager 生成并持久化，Agent 不生成任何凭据。因此节点配置与客户端订阅始终一致，重复同步不会改变它们，也不会引起无谓的重启。

## 规则

- 后端端口从 `50000-59999` 中取该节点上最小的空闲值，已被其他端口的监听端口或后端地址占用的会跳过。
- 托管后端必须监听回环地址；Manager 和 Agent 都会校验。
- 托管后端只支持 `aes-128-gcm`、`aes-256-gcm`、`chacha20-ietf-poly1305`。
- 编辑端口时，面板回传未改动的后端地址不会改变托管状态。清空地址会切换为托管，填入新地址会切换为手动。从手动切到托管时，原有的手动密码和加密方式不会被沿用。

## 部署

- 后端 unit：`shadowsocks-backend-<端口ID>.service`，命令行 `ssserver -s <地址> -m <加密> -k <密码>`，不开启 UDP。
- 前端 unit `shadowtls-<端口ID>.service` 使用 `After=` 加 `Wants=` 排在后端之后。不用 `Requires=`，避免后端重启时前端监听被一并停掉。
- 后端地址已被本机其他程序占用时，部署会失败并提示在面板中重新生成。
- 端口停用、删除、改为手动后端，以及 Agent 清理（purge）时，后端 unit 都会一并移除，包括前端已不存在的孤立后端。
- `ssserver` 来自 shadowsocks-rust `v1.18.2`（amd64、arm64、arm），下载走 GitHub，可通过 `--mita-mirror` 加速。

## 兼容性

旧版 Agent 不认识 `backendManaged`，只会启动前端而不启动后端。升级时需同时替换 Manager 和 Agent。

## 已验证与未验证

已验证：Manager 的分配、稳定性、模式切换、订阅一致性；Agent 的 unit 内容、幂等重启、端口占用拒绝、孤立后端清理。这些由单元测试覆盖，安装与端口探测使用注入的替身。

未验证：在真实 Linux 节点上的 systemd 部署与 `ssserver` 的实际运行，以及用 mihomo 客户端经 ShadowTLS 实际连通。上线前需要在测试节点上完整走一遍。
