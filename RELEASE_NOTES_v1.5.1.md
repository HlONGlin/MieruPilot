# UnifiedProxy v1.5.1

This patch makes update checks resilient to temporary GitHub or proxy failures and prevents upstream HTML error pages from flooding the settings screen.

## Fixed

- Return update check failures as concise JSON errors when GitHub is unreachable or times out.
- Limit GitHub release API requests to five seconds and cache successful release metadata for ten minutes.
- Compare version components numerically, so releases such as `v1.10.0` are correctly newer than `v1.9.0`.
- Prevent HTML gateway pages and malformed JSON responses from being rendered as application data.
- Point release links to the current UnifiedProxy repository.

## Online update

The updater downloads the Manager binary for the current platform and verifies it against `SHA256SUMS` before replacement. This release includes checksums for all attached binaries.

## Downloads

- Manager: Linux amd64/arm64, macOS amd64/arm64, Windows amd64
- Agent: Linux amd64/arm64

**Full changelog:** https://github.com/HlONGlin/UnifiedProxy/compare/v1.5.0...v1.5.1
