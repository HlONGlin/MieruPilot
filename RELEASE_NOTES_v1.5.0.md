# UnifiedProxy v1.5.0

UnifiedProxy v1.5.0 adds operational APIs for TLS certificate management, node diagnostics, and configuration inspection, alongside reliability fixes for AnyTLS and ShadowTLS.

## Highlights

- **TLS certificate management:** upload certificates and private keys, validate PEM formats, expiry, and key matching, and inspect certificate domains, issuer, key type, and validity. Private keys are stored with restrictive permissions and are never returned by the API.
- **Node diagnostics:** run quick or full connectivity checks covering ICMP reachability and configured TCP ports, with latency, status, and troubleshooting guidance.
- **Configuration inspection:** export node, port, user, certificate, traffic, and egress configuration as JSON; sensitive values are masked by default.
- **Proxy reliability:** fix AnyTLS connection handling and improve ShadowTLS v3 compatibility and share links.
- **UI polish and documentation:** refine the interface color palette and add deployment, quick-start, and troubleshooting guides for NaïveProxy and the new management APIs.

## Upgrade

Back up your manager data before replacing the binaries. Existing Mieru nodes and configuration remain supported. The new certificate, diagnostic, and configuration endpoints are available through the Manager API; the accompanying helper UI assets are documented separately and may require frontend integration.

## Downloads

This release includes Manager binaries for Linux amd64/arm64 and Windows amd64, plus Agent binaries for Linux amd64/arm64.

- `merit-manager-linux-amd64`
- `merit-manager-linux-arm64`
- `merit-manager-windows-amd64.exe`
- `merit-agent-linux-amd64`
- `merit-agent-linux-arm64`

## Documentation

- [README](https://github.com/HlONGlin/UnifiedProxy/blob/v1.5.0/README.md)
- [NaïveProxy deployment guide](https://github.com/HlONGlin/UnifiedProxy/blob/v1.5.0/docs/NAIVEPROXY_DEPLOYMENT_GUIDE.md)
- [Troubleshooting](https://github.com/HlONGlin/UnifiedProxy/blob/v1.5.0/docs/TROUBLESHOOTING.md)
- [Proxy fixes](https://github.com/HlONGlin/UnifiedProxy/blob/v1.5.0/docs/PROXY_FIXES.md)

**Full changelog:** https://github.com/HlONGlin/UnifiedProxy/compare/v1.4.0...v1.5.0
